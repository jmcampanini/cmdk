//go:build unix

// Package cmdrun owns every way cmdk starts another process. cmdk is a
// Unix-only program (it drives tmux); this package encodes that as a build
// constraint so a non-Unix build fails here, deliberately, instead of deep
// inside syscall.
//
// No other package may execute subprocesses (enforced by lint): every call
// site picks one of the four entry points below, and each entry point forces
// the caller to declare what it expects — output shape, byte limits, and
// deadline — so an unclassified subprocess call cannot be written.
//
// Choosing an entry point:
//
//   - [Query] — a fixed-argv, repository-owned binary (tmux, git, zoxide)
//     whose output cmdk parses. Requires an output Shape, byte limits, and a
//     timeout. Use this unless one of the cases below applies.
//   - [Run] — a shell command string the user authored in their config
//     (picker sources, launch_path_cmd), executed via sh -c. The only mode
//     that invokes a shell; never route repository-owned commands through it.
//   - [Stream] — a command that takes over the caller's terminal
//     (tmux attach-session). Nothing is captured, no deadline is applied,
//     and the caller injects the streams; package-global stdio is forbidden.
//   - [Replace] — replaces the cmdk process entirely via exec(2), for
//     shell-mode launches. Never returns on success.
//
// The capture modes (Query, Run) share one contract:
//
//   - Stdout is payload and never truncates: it fits the declared shape and
//     limit, or the command fails with [KindOutput] and the process group is
//     killed. Parsing partial output is not representable.
//   - Stderr is diagnostics: captured up to its own limit, truncated beyond
//     it, and annotated when truncated.
//   - The child runs in its own process group; timeout, cancellation, and
//     shape violations SIGKILL the whole group, so descendants cannot
//     outlive the call.
//   - After the child exits, an inherited pipe held open by an orphaned
//     descendant delays return by at most a short drain window instead of
//     forever.
//   - Failures are *CommandError values classified by [Kind], carrying the
//     exit code and bounded captured streams — never unbounded content.
//
// Stream inherits the caller's process group on purpose: moving a
// terminal-takeover command like tmux attach out of the foreground process
// group would break tty signal delivery. Cancellation still terminates the
// child; only the group-kill guarantee is traded away, and only there.
//
// Spec validation failures (a zero limit, a missing shape, an empty argv)
// are programmer errors, not runtime conditions, and panic immediately so
// any test that executes the call site catches them.
package cmdrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Kind classifies a command failure. Exactly one Kind applies to any
// *CommandError, so callers can branch on failure cause without parsing
// message text.
type Kind string

const (
	// KindTimeout: a deadline elapsed before the command finished — the
	// command's own timeout, or the caller's context deadline (in which
	// case CommandError.Timeout is 0). The process group was killed (the
	// child alone, for Stream).
	KindTimeout Kind = "timeout"
	// KindCanceled: the caller's context was canceled (or, for Run, a
	// SIGINT/SIGTERM arrived) before the command finished. The process
	// group was killed (the child alone, for Stream).
	KindCanceled Kind = "canceled"
	// KindOutput: the command violated its declared output contract — too
	// many stdout bytes, a second line in single-line mode, or any stdout
	// in expect-empty mode. The process group was killed as soon as the
	// violation was seen. Callers synthesizing contract violations for
	// commands that exited zero (e.g. "returned no items") also use this
	// Kind, with ExitCode 0.
	KindOutput Kind = "output"
	// KindExit: the command finished on its own but failed — a nonzero
	// exit, a signal death, or a start failure such as the binary missing
	// from PATH (unwraps to *exec.Error in that case).
	KindExit Kind = "exit"
)

// pipeDrainDelay bounds how long a finished command's orphaned descendants
// can delay return by holding inherited stdout/stderr open.
const pipeDrainDelay = 100 * time.Millisecond

var signalNotifyContext = signal.NotifyContext

// Shape declares what legitimate stdout looks like for a Query. The zero
// value is invalid so every call site states its class explicitly.
type Shape int

const (
	// ShapeSingleLine: stdout is at most one line (a trailing newline is
	// permitted, and so is empty output — callers must handle it). Use for
	// small-result probes — an ID, a version string, a filesystem path. A
	// second line fails the command immediately.
	ShapeSingleLine Shape = iota + 1
	// ShapeEmpty: stdout is empty. Use for commands run purely for their
	// side effect (tmux set-option, switch-client); any stdout byte fails
	// the command immediately. Stderr is still captured for diagnostics.
	ShapeEmpty
	// ShapeLines: stdout is a list of lines whose size scales with user
	// state (tmux list-*, zoxide query). Choose MaxStdout well above the
	// measured legitimate worst case: exceeding it fails the command
	// rather than truncating, because a silently shortened list parses
	// into silently wrong behavior.
	ShapeLines
)

func (s Shape) String() string {
	switch s {
	case ShapeSingleLine:
		return "single-line"
	case ShapeEmpty:
		return "empty"
	case ShapeLines:
		return "lines"
	default:
		return fmt.Sprintf("Shape(%d)", int(s))
	}
}

// Spec configures [Run] — the mode for shell command strings the user
// authored in their config. If the command is repository-owned, use [Query]
// with a fixed argv instead.
type Spec struct {
	// Op labels the command in error text, e.g. "launch_path_cmd".
	Op string
	// Rendered is the fully rendered command line, run via sh -c.
	// Template rendering happens in callers; a failure to render is a
	// config error, not a command error.
	Rendered string
	// Timeout <= 0 means no deadline. Unlike Query, Run permits this: the
	// command is the user's own, and their patience with it is theirs to
	// configure (timeout.picker = 0 is documented as "no timeout").
	Timeout time.Duration
	// Env of nil inherits the parent environment.
	Env []string
	// SingleLine requires stdout to be at most one line, failing the
	// command on a second line exactly like ShapeSingleLine.
	SingleLine bool
	// MaxStdout and MaxStderr are required, positive capture limits.
	// Stdout beyond MaxStdout fails the command with KindOutput; stderr
	// beyond MaxStderr is truncated and annotated.
	MaxStdout int
	MaxStderr int
}

// QuerySpec configures [Query] — the mode for fixed-argv, repository-owned
// binaries whose output cmdk parses.
type QuerySpec struct {
	// Op labels the command in error text, e.g. "tmux list-sessions".
	Op string
	// Argv is the full argument vector; Argv[0] is the binary name,
	// resolved via PATH. No shell is involved, so nothing in Argv is
	// interpreted.
	Argv []string
	// Env of nil inherits the parent environment.
	Env []string
	// Timeout is required and positive: a fresh deadline for this command,
	// independent of whatever budget remains on ctx. A query that can wait
	// forever is not representable. (The caller's ctx still applies too —
	// whichever bound is reached first wins.)
	Timeout time.Duration
	// Shape declares what legitimate stdout looks like; see the Shape
	// constants for how to choose.
	Shape Shape
	// MaxStdout and MaxStderr are required, positive capture limits, with
	// the same semantics as in Spec.
	MaxStdout int
	MaxStderr int
}

// StreamSpec configures [Stream] — the mode for commands that take over the
// caller's terminal.
type StreamSpec struct {
	// Op labels the command in error text, e.g. "tmux attach-session".
	Op string
	// Argv is the full argument vector; Argv[0] is resolved via PATH.
	Argv []string
	// Env of nil inherits the parent environment.
	Env []string
	// Stdin, Stdout, and Stderr are the child's streams, injected by the
	// caller. Reference os.Stdin/os.Stdout/os.Stderr only at the
	// composition root (the cmd package), never here or in libraries, so
	// streaming stays testable and redirectable. A nil stream reads from
	// or writes to the null device, per os/exec.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Result is the successful outcome of a capture-mode command.
type Result struct {
	// Stdout is the complete payload. It is never truncated: output that
	// would not fit the declared shape and limit fails the command
	// instead.
	Stdout string
	// Stderr is captured diagnostics, up to the configured limit.
	Stderr string
	// StderrTruncated reports whether stderr exceeded its limit and was
	// cut. Render it with AnnotatedStderr wherever a human will read it.
	StderrTruncated bool
}

// AnnotatedStderr returns captured stderr with an explicit truncation note
// appended when the capture limit cut it short.
func (r Result) AnnotatedStderr() string {
	return annotate(r.Stderr, r.StderrTruncated)
}

func annotate(text string, truncated bool) string {
	if !truncated {
		return text
	}
	note := fmt.Sprintf("[truncated after %d bytes]", len(text))
	if text != "" && !strings.HasSuffix(text, "\n") {
		return text + "\n" + note
	}
	return text + note
}

// CommandError is the failure type for every cmdrun mode. Kind states what
// happened; Stdout and Stderr carry bounded, annotated captures for
// diagnostics (empty for Stream, which captures nothing).
type CommandError struct {
	Op   string
	Kind Kind
	// Command is the human-readable command line: the rendered sh string
	// for Run, the joined argv for Query, Stream, and Replace.
	Command string
	// Timeout is the per-command deadline that was in force. It is 0 for a
	// KindTimeout caused by the caller's context deadline expiring rather
	// than the command's own timeout elapsing.
	Timeout time.Duration
	// ExitCode is -1 when unknown (never ran, signal-killed, canceled).
	// Callers building KindOutput errors for commands that exited zero but
	// produced invalid output set it to 0.
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func (e *CommandError) Unwrap() error { return e.Err }

// Headline is the one-line failure summary, without captured streams. Use
// it for list rows and log records; use Error when the streams belong
// inline.
func (e *CommandError) Headline() string {
	switch e.Kind {
	case KindTimeout:
		if e.Timeout > 0 {
			return fmt.Sprintf("%s timed out after %s", e.Op, e.Timeout)
		}
		return fmt.Sprintf("%s timed out: caller deadline exhausted", e.Op)
	case KindCanceled:
		return fmt.Sprintf("%s canceled: %v", e.Op, e.Err)
	case KindOutput:
		return fmt.Sprintf("%s %v", e.Op, e.Err)
	default:
		return fmt.Sprintf("%s failed: %v", e.Op, e.Err)
	}
}

func (e *CommandError) Error() string {
	var b strings.Builder
	b.WriteString(e.Headline())
	if e.Stdout != "" {
		b.WriteString("\nstdout: ")
		b.WriteString(e.Stdout)
	}
	if e.Stderr != "" {
		b.WriteString("\nstderr: ")
		b.WriteString(e.Stderr)
	}
	return b.String()
}

// Run executes a user-authored shell command string via sh -c under the
// capture contract (see the package doc). It builds its own signal-aware
// context — SIGINT/SIGTERM cancel the command — because its callers fire
// user commands from the TUI rather than carrying a request context.
//
// Use Run only for command strings that come from user configuration. A
// repository-owned invocation has a fixed argv and belongs in Query.
func Run(spec Spec) (Result, error) {
	mustSpec(spec.Op != "", "Run", "Spec.Op is required")
	mustSpec(spec.Rendered != "", spec.Op, "Spec.Rendered is required")
	mustSpec(spec.MaxStdout > 0, spec.Op, "Spec.MaxStdout must be positive")
	mustSpec(spec.MaxStderr > 0, spec.Op, "Spec.MaxStderr must be positive")

	ctx, cancel := commandContext(spec.Timeout)
	defer cancel()

	shape := ShapeLines
	if spec.SingleLine {
		shape = ShapeSingleLine
	}
	return runCapture(ctx, captureRequest{
		op:        spec.Op,
		command:   spec.Rendered,
		argv:      []string{"sh", "-c", spec.Rendered},
		env:       spec.Env,
		timeout:   spec.Timeout,
		shape:     shape,
		maxStdout: spec.MaxStdout,
		maxStderr: spec.MaxStderr,
	})
}

// Query executes a fixed-argv, repository-owned binary under the capture
// contract (see the package doc). The spec's Timeout is applied as a fresh
// deadline layered over ctx, so every query is bounded even when ctx has no
// deadline; ctx cancellation still propagates (the TUI quitting, an
// enclosing fetch budget expiring).
//
// Choose the Shape and limits from the call site's class: what legitimate
// output looks like, and a limit far enough above it that exceeding the
// limit always means something is genuinely wrong.
func Query(ctx context.Context, spec QuerySpec) (Result, error) {
	mustSpec(spec.Op != "", "Query", "QuerySpec.Op is required")
	mustSpec(ctx != nil, spec.Op, "ctx is required")
	mustSpec(len(spec.Argv) > 0, spec.Op, "QuerySpec.Argv is required")
	mustSpec(spec.Timeout > 0, spec.Op, "QuerySpec.Timeout must be positive")
	mustSpec(spec.Shape == ShapeSingleLine || spec.Shape == ShapeEmpty || spec.Shape == ShapeLines,
		spec.Op, "QuerySpec.Shape must be a declared Shape")
	mustSpec(spec.MaxStdout > 0, spec.Op, "QuerySpec.MaxStdout must be positive")
	mustSpec(spec.MaxStderr > 0, spec.Op, "QuerySpec.MaxStderr must be positive")

	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	return runCapture(ctx, captureRequest{
		op:        spec.Op,
		command:   strings.Join(spec.Argv, " "),
		argv:      spec.Argv,
		env:       spec.Env,
		parent:    parent,
		timeout:   spec.Timeout,
		shape:     spec.Shape,
		maxStdout: spec.MaxStdout,
		maxStderr: spec.MaxStderr,
	})
}

// Stream executes a fixed-argv binary connected to caller-injected streams,
// for commands that take over the terminal (tmux attach-session). Nothing
// is captured and no deadline is applied: blocking until the user is done
// is the point. Cancelling ctx terminates the child and returns its
// CommandError.
//
// Deliberate contract differences from the capture modes: the child stays
// in the caller's process group (moving a terminal-takeover command out of
// the foreground group would break tty signal delivery), so cancellation
// reaches the child but not descendants it may have spawned; and there are
// no output limits because no output is held in memory.
func Stream(ctx context.Context, spec StreamSpec) error {
	mustSpec(spec.Op != "", "Stream", "StreamSpec.Op is required")
	mustSpec(ctx != nil, spec.Op, "ctx is required")
	mustSpec(len(spec.Argv) > 0, spec.Op, "StreamSpec.Argv is required")

	cmd := exec.CommandContext(ctx, spec.Argv[0], spec.Argv[1:]...)
	cmd.Env = spec.Env
	cmd.Stdin = spec.Stdin
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	// When the injected streams are not *os.File (tests, future callers),
	// os/exec copies through pipes; bound the drain exactly like capture.
	cmd.WaitDelay = pipeDrainDelay

	waitErr := cmd.Run()

	fail := func(kind Kind, exitCode int, err error) error {
		return &CommandError{
			Op:       spec.Op,
			Kind:     kind,
			Command:  strings.Join(spec.Argv, " "),
			ExitCode: exitCode,
			Err:      err,
		}
	}
	// Mirror the capture modes: a child that exited 0 stays a success even
	// when ctx expired while the bounded pipe drain was still running.
	if cmd.ProcessState != nil && cmd.ProcessState.Success() && isDrainNoise(waitErr) {
		return nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return fail(KindTimeout, -1, ctxErr)
		}
		return fail(KindCanceled, -1, ctxErr)
	}
	if waitErr != nil && !errors.Is(waitErr, exec.ErrWaitDelay) {
		return fail(KindExit, exitCodeOf(waitErr), waitErr)
	}
	return nil
}

// Replace replaces the cmdk process with the given program via exec(2), for
// shell-mode launches where cmdk's job is finished and the launched command
// takes over the terminal, the exit status, and the process entirely. On
// success it never returns. Env of nil inherits the parent environment.
func Replace(path string, argv []string, env []string) error {
	mustSpec(path != "", "Replace", "path is required")
	mustSpec(len(argv) > 0, "Replace", "argv is required")
	if env == nil {
		env = os.Environ()
	}
	err := syscall.Exec(path, argv, env)
	// Exec only returns on failure.
	return &CommandError{
		Op:       "exec",
		Kind:     KindExit,
		Command:  strings.Join(argv, " "),
		ExitCode: -1,
		Err:      err,
	}
}

// LookPath resolves a binary name via PATH. It exists so packages outside
// cmdrun never import os/exec (the lint-enforced execution boundary);
// callers use it to fail early, at resolve time, when a binary is missing.
func LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// IsNotFound reports whether err means the command's binary was not found
// on PATH, across every cmdrun mode.
func IsNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

type captureRequest struct {
	op      string
	command string
	argv    []string
	env     []string
	// parent is the caller's context, before the per-command timeout was
	// layered on; nil when the mode owns its context entirely (Run). It
	// distinguishes "this command's own deadline fired" from "the caller's
	// budget ran out" when classifying a timeout.
	parent    context.Context
	timeout   time.Duration
	shape     Shape
	maxStdout int
	maxStderr int
}

func runCapture(ctx context.Context, req captureRequest) (Result, error) {
	payload := &payloadCapture{limit: req.maxStdout, shape: req.shape}
	stderrCapture := &truncatingCapture{limit: req.maxStderr}

	cmd := exec.CommandContext(ctx, req.argv[0], req.argv[1:]...)
	cmd.Env = req.env
	cmd.Stdout = payload
	cmd.Stderr = stderrCapture
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killCommandGroup(cmd) }
	cmd.WaitDelay = pipeDrainDelay
	// A shape violation kills the whole group immediately: the output
	// contract is already broken, so letting the command keep producing
	// output would only burn time until the timeout.
	payload.onViolation = func() { _ = killCommandGroup(cmd) }

	// Do not use StdoutPipe/StderrPipe here: os/exec closes those readers from
	// Wait, so racing Wait against reader goroutines can produce partial output.
	// Supplying Writers lets os/exec own that synchronization. WaitDelay bounds
	// the only tolerated local recovery case: the child exited after producing
	// its output, but an orphaned descendant kept stdout/stderr open.
	waitErr := cmd.Run()

	res := Result{
		Stdout:          payload.buf.String(),
		Stderr:          stderrCapture.buf.String(),
		StderrTruncated: stderrCapture.truncated,
	}

	reportTimeout := req.timeout
	fail := func(kind Kind, exitCode int, err error) (Result, error) {
		return Result{}, &CommandError{
			Op:       req.op,
			Kind:     kind,
			Command:  req.command,
			Timeout:  reportTimeout,
			ExitCode: exitCode,
			Stdout:   annotate(res.Stdout, payload.capped),
			Stderr:   res.AnnotatedStderr(),
			Err:      err,
		}
	}

	// A command that exited 0 with a contract-clean payload stays a success
	// even when ctx expired while the bounded pipe drain was still running:
	// os/exec injects ctx.Err() into waitErr whenever the deadline fires
	// before Wait finishes, so the child's own exit status — not waitErr —
	// is the signal that the command actually completed.
	if payload.err == nil && cmd.ProcessState != nil && cmd.ProcessState.Success() && isDrainNoise(waitErr) {
		return res, nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			if req.parent != nil && req.parent.Err() != nil {
				// The caller's budget expired, not this command's own
				// deadline; Timeout 0 makes Headline say so instead of
				// blaming a duration that never elapsed.
				reportTimeout = 0
			}
			return fail(KindTimeout, -1, ctxErr)
		}
		return fail(KindCanceled, -1, ctxErr)
	}
	if payload.err != nil {
		return fail(KindOutput, -1, payload.err)
	}
	if waitErr != nil && !errors.Is(waitErr, exec.ErrWaitDelay) {
		return fail(KindExit, exitCodeOf(waitErr), waitErr)
	}
	return res, nil
}

// isDrainNoise reports whether waitErr carries no information beyond "the
// wait was cut short after the child already exited": nil, the bounded
// WaitDelay firing, or the ctx error os/exec injects when a deadline or
// cancellation raced the pipe drain. Any other error (a nonzero exit, a
// signal death, a pipe copy failure) is real.
func isDrainNoise(waitErr error) bool {
	return waitErr == nil ||
		errors.Is(waitErr, exec.ErrWaitDelay) ||
		errors.Is(waitErr, context.DeadlineExceeded) ||
		errors.Is(waitErr, context.Canceled)
}

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// commandContext is Run's signal-aware context: SIGINT/SIGTERM cancel the
// command, and a positive timeout layers a deadline on top.
func commandContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, stopSignals := signalNotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if timeout <= 0 {
		return ctx, stopSignals
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx, func() {
		timeoutCancel()
		stopSignals()
	}
}

func killCommandGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func mustSpec(ok bool, op, msg string) {
	if !ok {
		panic(fmt.Sprintf("cmdrun: %s: %s", op, msg))
	}
}

// payloadCapture enforces a Shape on stdout. Payload never truncates: the
// first byte past the declared contract records a violation, kills the
// process group via onViolation, and fails the copy, so partial output can
// reach error diagnostics but never a parser.
type payloadCapture struct {
	buf         bytes.Buffer
	limit       int
	shape       Shape
	seenNewline bool
	// capped reports the byte limit specifically was hit, so error
	// diagnostics can annotate the attached partial stdout.
	capped      bool
	err         error
	onViolation func()
}

func (c *payloadCapture) Write(p []byte) (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	switch c.shape {
	case ShapeEmpty:
		if len(p) == 0 {
			return 0, nil
		}
		return 0, c.fail(errors.New("produced stdout; expected none"))
	case ShapeSingleLine:
		for i, b := range p {
			if c.seenNewline {
				return i, c.fail(errors.New("output must contain exactly one line"))
			}
			if c.buf.Len() >= c.limit {
				c.capped = true
				return i, c.fail(fmt.Errorf("output exceeds %d bytes", c.limit))
			}
			c.buf.WriteByte(b)
			if b == '\n' {
				c.seenNewline = true
			}
		}
		return len(p), nil
	default: // ShapeLines
		remaining := c.limit - c.buf.Len()
		if len(p) > remaining {
			c.buf.Write(p[:remaining])
			c.capped = true
			return remaining, c.fail(fmt.Errorf("output exceeds %d bytes", c.limit))
		}
		c.buf.Write(p)
		return len(p), nil
	}
}

func (c *payloadCapture) fail(err error) error {
	if c.err == nil {
		c.err = err
		if c.onViolation != nil {
			c.onViolation()
		}
	}
	return c.err
}

// truncatingCapture keeps the first limit bytes and drops the rest,
// flagging that it did. It is only ever used for diagnostics (stderr) —
// bytes read by humans — where a cut-and-annotated capture beats failing
// the command that is already being reported on.
type truncatingCapture struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (c *truncatingCapture) Write(p []byte) (int, error) {
	written := len(p)
	if written == 0 {
		return 0, nil
	}

	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		c.truncated = true
		return written, nil
	}
	if written > remaining {
		p = p[:remaining]
		c.truncated = true
	}
	c.buf.Write(p)
	return written, nil
}
