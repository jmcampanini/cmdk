// Package cmdrun executes user-configured shell command strings via sh -c
// with bounded output capture, a process-group kill contract, and structured
// errors. Template rendering happens in callers; a failure to render is a
// config error, not a command error. Issue #87 extends this package with an
// output-parse mode and a streaming mode for fixed-binary call sites (tmux,
// git, zoxide); do not add those here yet.
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

type Kind string

const (
	KindTimeout  Kind = "timeout"
	KindCanceled Kind = "canceled"
	KindOutput   Kind = "output"
	KindExit     Kind = "exit"
)

const pipeDrainDelay = 100 * time.Millisecond

var signalNotifyContext = signal.NotifyContext

type Spec struct {
	// Op labels the command in error text, e.g. "launch_path_cmd".
	Op string
	// Rendered is the fully rendered command line, run via sh -c.
	Rendered string
	// Timeout <= 0 means no deadline.
	Timeout time.Duration
	// Env of nil inherits the parent environment.
	Env []string
	// SingleLine requires stdout to be at most one line; a violation
	// SIGKILLs the process group and fails with KindOutput.
	SingleLine bool
	// MaxStdout and MaxStderr are required, positive capture limits.
	MaxStdout int
	MaxStderr int
}

type Result struct {
	Stdout          string
	Stderr          string
	StdoutTruncated bool
	StderrTruncated bool
}

func (r Result) AnnotatedStdout() string {
	return annotate(r.Stdout, r.StdoutTruncated)
}

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

type CommandError struct {
	Op       string
	Kind     Kind
	Rendered string
	Timeout  time.Duration
	// ExitCode is -1 when unknown (never ran, signal-killed, canceled).
	// Callers building KindOutput errors for commands that exited zero but
	// produced invalid output set it to 0.
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func (e *CommandError) Unwrap() error { return e.Err }

func (e *CommandError) Headline() string {
	switch e.Kind {
	case KindTimeout:
		return fmt.Sprintf("%s timed out after %s", e.Op, e.Timeout)
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

func Run(spec Spec) (Result, error) {
	ctx, cancel := commandContext(spec.Timeout)
	defer cancel()

	stderrCapture := &boundedCapture{limit: spec.MaxStderr}
	var stdoutWriter io.Writer
	var singleLine *singleLineCapture
	var multiLine *boundedCapture
	if spec.SingleLine {
		singleLine = &singleLineCapture{limit: spec.MaxStdout}
		stdoutWriter = singleLine
	} else {
		multiLine = &boundedCapture{limit: spec.MaxStdout}
		stdoutWriter = multiLine
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", spec.Rendered)
	cmd.Env = spec.Env
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrCapture
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killCommandGroup(cmd) }
	cmd.WaitDelay = pipeDrainDelay
	if singleLine != nil {
		// A single-line violation kills the whole group immediately: the
		// output contract is already broken, so letting the command keep
		// producing output would only burn time until the limit or timeout.
		singleLine.onError = func() { _ = killCommandGroup(cmd) }
	}

	// Do not use StdoutPipe/StderrPipe here: os/exec closes those readers from
	// Wait, so racing Wait against reader goroutines can produce partial output.
	// Supplying Writers lets os/exec own that synchronization. WaitDelay bounds
	// the only tolerated local recovery case: the shell exited after producing
	// its output, but an orphaned descendant kept stdout/stderr open.
	waitErr := cmd.Run()

	res := Result{Stderr: stderrCapture.buf.String(), StderrTruncated: stderrCapture.truncated}
	if singleLine != nil {
		res.Stdout = singleLine.buf.String()
	} else {
		res.Stdout = multiLine.buf.String()
		res.StdoutTruncated = multiLine.truncated
	}

	fail := func(kind Kind, exitCode int, err error) (Result, error) {
		return Result{}, &CommandError{
			Op:       spec.Op,
			Kind:     kind,
			Rendered: spec.Rendered,
			Timeout:  spec.Timeout,
			ExitCode: exitCode,
			Stdout:   res.AnnotatedStdout(),
			Stderr:   res.AnnotatedStderr(),
			Err:      err,
		}
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return fail(KindTimeout, -1, ctxErr)
		}
		return fail(KindCanceled, -1, ctxErr)
	}
	if singleLine != nil && singleLine.err != nil {
		return fail(KindOutput, -1, singleLine.err)
	}
	if waitErr != nil && !errors.Is(waitErr, exec.ErrWaitDelay) {
		return fail(KindExit, exitCodeOf(waitErr), waitErr)
	}
	return res, nil
}

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

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

type singleLineCapture struct {
	buf         bytes.Buffer
	limit       int
	seenNewline bool
	err         error
	onError     func()
}

func (c *singleLineCapture) Write(p []byte) (int, error) {
	for i, b := range p {
		if c.err != nil {
			return i, c.err
		}
		if c.seenNewline {
			return i, c.fail(errors.New("output must contain exactly one line"))
		}
		if c.buf.Len() >= c.limit {
			return i, c.fail(fmt.Errorf("output exceeds %d bytes", c.limit))
		}
		c.buf.WriteByte(b)
		if b == '\n' {
			c.seenNewline = true
		}
	}
	return len(p), nil
}

func (c *singleLineCapture) fail(err error) error {
	if c.err == nil {
		c.err = err
		if c.onError != nil {
			c.onError()
		}
	}
	return c.err
}

type boundedCapture struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (c *boundedCapture) Write(p []byte) (int, error) {
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
