package cmdrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func wantCommandError(t *testing.T, err error, kind Kind) *CommandError {
	t.Helper()
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error = %v, want *CommandError", err)
	}
	if cmdErr.Kind != kind {
		t.Fatalf("Kind = %q, want %q (error: %v)", cmdErr.Kind, kind, err)
	}
	return cmdErr
}

func wantPanic(t *testing.T, substr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("no panic, want panic containing %q", substr)
		}
		if !strings.Contains(fmt.Sprint(r), substr) {
			t.Fatalf("panic = %v, want to contain %q", r, substr)
		}
	}()
	fn()
}

func TestRun_MultiLineSuccessCapturesBoth(t *testing.T) {
	res, err := Run(Spec{
		Op:        "testcmd",
		Rendered:  "printf 'one\\ntwo\\n'; printf diag >&2",
		Timeout:   time.Second,
		MaxStdout: 1024,
		MaxStderr: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "one\ntwo\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "one\ntwo\n")
	}
	if res.Stderr != "diag" {
		t.Errorf("Stderr = %q, want %q", res.Stderr, "diag")
	}
	if res.StderrTruncated {
		t.Error("StderrTruncated = true, want false")
	}
}

func TestRun_MultiLineOverflowFails(t *testing.T) {
	res, err := Run(Spec{
		Op:        "testcmd",
		Rendered:  "printf '12345678'",
		Timeout:   time.Second,
		MaxStdout: 8,
		MaxStderr: 1024,
	})
	if err != nil {
		t.Fatalf("at-limit output should succeed, got %v", err)
	}
	if res.Stdout != "12345678" {
		t.Errorf("Stdout = %q, want full at-limit payload", res.Stdout)
	}

	_, err = Run(Spec{
		Op:        "testcmd",
		Rendered:  "printf '123456789'",
		Timeout:   time.Second,
		MaxStdout: 8,
		MaxStderr: 1024,
	})
	cmdErr := wantCommandError(t, err, KindOutput)
	if !strings.Contains(cmdErr.Err.Error(), "output exceeds 8 bytes") {
		t.Errorf("Err = %v, want output-exceeds violation", cmdErr.Err)
	}
	if !strings.Contains(cmdErr.Stdout, "[truncated after 8 bytes]") {
		t.Errorf("Stdout = %q, want annotated partial payload", cmdErr.Stdout)
	}
}

func TestRun_ExitCapturesStdoutStderr(t *testing.T) {
	_, err := Run(Spec{
		Op:        "testcmd",
		Rendered:  "echo out; echo err >&2; exit 23",
		Timeout:   time.Second,
		MaxStdout: 1024,
		MaxStderr: 1024,
	})
	cmdErr := wantCommandError(t, err, KindExit)
	if cmdErr.ExitCode != 23 {
		t.Errorf("ExitCode = %d, want 23", cmdErr.ExitCode)
	}
	if !strings.Contains(cmdErr.Stdout, "out") {
		t.Errorf("Stdout = %q, want to contain %q", cmdErr.Stdout, "out")
	}
	if !strings.Contains(cmdErr.Stderr, "err") {
		t.Errorf("Stderr = %q, want to contain %q", cmdErr.Stderr, "err")
	}
	if !strings.Contains(err.Error(), "out") || !strings.Contains(err.Error(), "err") {
		t.Errorf("Error() = %q, want to contain both streams", err.Error())
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
		t.Errorf("errors.As(*exec.ExitError) = %v (%v), want exit code 23", exitErr, err)
	}
}

func TestRun_TimeoutIncludesCapturedOutput(t *testing.T) {
	_, err := Run(Spec{
		Op:        "testcmd",
		Rendered:  "printf out; printf err >&2; sleep 1",
		Timeout:   50 * time.Millisecond,
		MaxStdout: 1024,
		MaxStderr: 1024,
	})
	cmdErr := wantCommandError(t, err, KindTimeout)
	if cmdErr.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", cmdErr.ExitCode)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(DeadlineExceeded) = false for %v", err)
	}
	if !strings.Contains(err.Error(), "timed out after") {
		t.Errorf("Error() = %q, want timeout headline", err.Error())
	}
	if !strings.Contains(cmdErr.Stdout, "out") || !strings.Contains(cmdErr.Stderr, "err") {
		t.Errorf("Stdout/Stderr = %q/%q, want captured partial output", cmdErr.Stdout, cmdErr.Stderr)
	}
}

func TestRun_DoesNotWaitForGrandchildInheritedPipes(t *testing.T) {
	start := time.Now()
	res, err := Run(Spec{
		Op:         "testcmd",
		Rendered:   "printf '/tmp\\n'; (sleep 1) &",
		SingleLine: true,
		MaxStdout:  1024,
		MaxStderr:  1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "/tmp\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "/tmp\n")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Run took %s; likely waited for inherited stdout/stderr FDs", elapsed)
	}
}

func TestRun_SignalCancellationKillsCommandGroup(t *testing.T) {
	oldNotify := signalNotifyContext
	signalNotifyContext = func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
		if !containsSignal(signals, os.Interrupt) {
			t.Errorf("signals = %#v, want os.Interrupt", signals)
		}
		if !containsSignal(signals, syscall.SIGTERM) {
			t.Errorf("signals = %#v, want SIGTERM", signals)
		}
		ctx, cancel := context.WithCancel(parent)
		go func() {
			time.Sleep(25 * time.Millisecond)
			cancel()
		}()
		return ctx, cancel
	}
	t.Cleanup(func() { signalNotifyContext = oldNotify })

	start := time.Now()
	_, err := Run(Spec{
		Op:         "testcmd",
		Rendered:   "trap '' INT TERM; sleep 5; printf /tmp",
		SingleLine: true,
		MaxStdout:  1024,
		MaxStderr:  1024,
	})
	_ = wantCommandError(t, err, KindCanceled)
	if !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("Error() = %q, want canceled", err.Error())
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Run took %s; signal cancellation did not kill the command group", elapsed)
	}
}

func containsSignal(signals []os.Signal, want os.Signal) bool {
	for _, sig := range signals {
		if sig == want {
			return true
		}
	}
	return false
}

func TestRun_SingleLineOversizedStdout(t *testing.T) {
	limit := 64
	cmd := fmt.Sprintf("i=0; while [ $i -le %d ]; do printf x; i=$((i+1)); done", limit)
	_, err := Run(Spec{
		Op:         "testcmd",
		Rendered:   cmd,
		Timeout:    time.Second,
		SingleLine: true,
		MaxStdout:  limit,
		MaxStderr:  1024,
	})
	_ = wantCommandError(t, err, KindOutput)
	if !strings.Contains(err.Error(), "output exceeds") {
		t.Fatalf("Error() = %q, want output limit", err.Error())
	}
}

func TestRun_SingleLineSecondLineFailsFast(t *testing.T) {
	_, err := Run(Spec{
		Op:         "testcmd",
		Rendered:   "printf 'diag' >&2; printf 'a\\nb\\n'",
		Timeout:    time.Second,
		SingleLine: true,
		MaxStdout:  1024,
		MaxStderr:  1024,
	})
	cmdErr := wantCommandError(t, err, KindOutput)
	if !strings.Contains(err.Error(), "must contain exactly one line") {
		t.Fatalf("Error() = %q, want exactly-one-line violation", err.Error())
	}
	if !strings.Contains(cmdErr.Stdout, "a") {
		t.Errorf("Stdout = %q, want captured partial line", cmdErr.Stdout)
	}
	if cmdErr.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 after group kill", cmdErr.ExitCode)
	}
}

func TestRun_StderrTruncationAnnotated(t *testing.T) {
	res, err := Run(Spec{
		Op:        "testcmd",
		Rendered:  "printf '01234567890123456789' >&2",
		Timeout:   time.Second,
		MaxStdout: 1024,
		MaxStderr: 8,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.StderrTruncated {
		t.Fatal("StderrTruncated = false, want true")
	}
	if res.Stderr != "01234567" {
		t.Errorf("Stderr = %q, want first 8 bytes", res.Stderr)
	}
	want := "[truncated after 8 bytes]"
	if !strings.Contains(res.AnnotatedStderr(), want) {
		t.Errorf("AnnotatedStderr() = %q, want to contain %q", res.AnnotatedStderr(), want)
	}

	_, err = Run(Spec{
		Op:        "testcmd",
		Rendered:  "printf '01234567890123456789' >&2; exit 3",
		Timeout:   time.Second,
		MaxStdout: 1024,
		MaxStderr: 8,
	})
	cmdErr := wantCommandError(t, err, KindExit)
	if cmdErr.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", cmdErr.ExitCode)
	}
	if !strings.Contains(cmdErr.Stderr, want) {
		t.Errorf("Stderr = %q, want truncation note", cmdErr.Stderr)
	}
}

func TestRun_EnvNilInherits(t *testing.T) {
	t.Setenv("CMDRUN_TEST_INHERIT", "yes")
	res, err := Run(Spec{
		Op:        "testcmd",
		Rendered:  `printf '%s' "$CMDRUN_TEST_INHERIT"`,
		Timeout:   time.Second,
		MaxStdout: 1024,
		MaxStderr: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "yes" {
		t.Errorf("Stdout = %q, want inherited env value", res.Stdout)
	}
}

func TestRun_EnvOverride(t *testing.T) {
	t.Setenv("CMDRUN_TEST_INHERIT", "yes")
	res, err := Run(Spec{
		Op:        "testcmd",
		Rendered:  `printf '%s|%s' "$CMDRUN_TEST_INHERIT" "$CMDRUN_TEST_SET"`,
		Timeout:   time.Second,
		Env:       []string{"CMDRUN_TEST_SET=only"},
		MaxStdout: 1024,
		MaxStderr: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "|only" {
		t.Errorf("Stdout = %q, want explicit env only", res.Stdout)
	}
}

func TestRun_InvalidSpecPanics(t *testing.T) {
	valid := Spec{Op: "testcmd", Rendered: "true", MaxStdout: 1, MaxStderr: 1}

	missingOp := valid
	missingOp.Op = ""
	wantPanic(t, "Spec.Op is required", func() { _, _ = Run(missingOp) })

	missingRendered := valid
	missingRendered.Rendered = ""
	wantPanic(t, "Spec.Rendered is required", func() { _, _ = Run(missingRendered) })

	zeroStdout := valid
	zeroStdout.MaxStdout = 0
	wantPanic(t, "Spec.MaxStdout must be positive", func() { _, _ = Run(zeroStdout) })

	zeroStderr := valid
	zeroStderr.MaxStderr = 0
	wantPanic(t, "Spec.MaxStderr must be positive", func() { _, _ = Run(zeroStderr) })
}

func queryOf(shape Shape, script string) QuerySpec {
	return QuerySpec{
		Op:        "testquery",
		Argv:      []string{"sh", "-c", script},
		Timeout:   time.Second,
		Shape:     shape,
		MaxStdout: 1024,
		MaxStderr: 1024,
	}
}

func TestQuery_ShapeSuccesses(t *testing.T) {
	tests := []struct {
		name       string
		shape      Shape
		script     string
		wantStdout string
	}{
		{"single line", ShapeSingleLine, "printf '/tmp/some/path\\n'", "/tmp/some/path\n"},
		{"single line without newline", ShapeSingleLine, "printf '/tmp/some/path'", "/tmp/some/path"},
		{"empty", ShapeEmpty, "true", ""},
		{"empty with stderr diagnostics", ShapeEmpty, "printf diag >&2", ""},
		{"lines", ShapeLines, "printf 'one\\ntwo\\nthree\\n'", "one\ntwo\nthree\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Query(context.Background(), queryOf(tt.shape, tt.script))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", res.Stdout, tt.wantStdout)
			}
		})
	}
}

func TestQuery_LinesLimitBoundary(t *testing.T) {
	spec := queryOf(ShapeLines, "")
	spec.MaxStdout = 8

	for _, tt := range []struct {
		name   string
		script string
	}{
		{"below limit", "printf '1234567'"},
		{"at limit", "printf '12345678'"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := spec
			s.Argv = []string{"sh", "-c", tt.script}
			if _, err := Query(context.Background(), s); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	t.Run("above limit", func(t *testing.T) {
		s := spec
		s.Argv = []string{"sh", "-c", "printf '123456789'"}
		_, err := Query(context.Background(), s)
		cmdErr := wantCommandError(t, err, KindOutput)
		if !strings.Contains(cmdErr.Err.Error(), "output exceeds 8 bytes") {
			t.Errorf("Err = %v, want output-exceeds violation", cmdErr.Err)
		}
		if !strings.Contains(cmdErr.Stdout, "12345678") || !strings.Contains(cmdErr.Stdout, "[truncated after 8 bytes]") {
			t.Errorf("Stdout = %q, want annotated partial payload", cmdErr.Stdout)
		}
	})
}

func TestQuery_SingleLineLimitBoundary(t *testing.T) {
	spec := queryOf(ShapeSingleLine, "")
	spec.MaxStdout = 8

	t.Run("at limit including newline", func(t *testing.T) {
		s := spec
		s.Argv = []string{"sh", "-c", "printf '1234567\\n'"}
		res, err := Query(context.Background(), s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Stdout != "1234567\n" {
			t.Errorf("Stdout = %q, want full line", res.Stdout)
		}
	})

	t.Run("above limit", func(t *testing.T) {
		s := spec
		s.Argv = []string{"sh", "-c", "printf '123456789'"}
		_, err := Query(context.Background(), s)
		_ = wantCommandError(t, err, KindOutput)
	})
}

func TestQuery_SingleLineSecondLineFails(t *testing.T) {
	_, err := Query(context.Background(), queryOf(ShapeSingleLine, "printf 'a\\nb\\n'"))
	cmdErr := wantCommandError(t, err, KindOutput)
	if !strings.Contains(cmdErr.Err.Error(), "must contain exactly one line") {
		t.Errorf("Err = %v, want exactly-one-line violation", cmdErr.Err)
	}
}

func TestQuery_EmptyShapeRejectsStdout(t *testing.T) {
	_, err := Query(context.Background(), queryOf(ShapeEmpty, "printf x"))
	cmdErr := wantCommandError(t, err, KindOutput)
	if !strings.Contains(cmdErr.Err.Error(), "expected none") {
		t.Errorf("Err = %v, want expected-none violation", cmdErr.Err)
	}
}

func TestQuery_NonzeroExitCapturesDiagnostics(t *testing.T) {
	_, err := Query(context.Background(), queryOf(ShapeLines, "echo out; echo err >&2; exit 23"))
	cmdErr := wantCommandError(t, err, KindExit)
	if cmdErr.ExitCode != 23 {
		t.Errorf("ExitCode = %d, want 23", cmdErr.ExitCode)
	}
	if !strings.Contains(cmdErr.Stdout, "out") || !strings.Contains(cmdErr.Stderr, "err") {
		t.Errorf("Stdout/Stderr = %q/%q, want both streams captured", cmdErr.Stdout, cmdErr.Stderr)
	}
	if cmdErr.Command != "sh -c echo out; echo err >&2; exit 23" {
		t.Errorf("Command = %q, want joined argv", cmdErr.Command)
	}
}

func TestQuery_StderrTruncationAnnotatedOnFailure(t *testing.T) {
	spec := queryOf(ShapeLines, "printf '01234567890123456789' >&2; exit 3")
	spec.MaxStderr = 8
	_, err := Query(context.Background(), spec)
	cmdErr := wantCommandError(t, err, KindExit)
	if !strings.Contains(cmdErr.Stderr, "[truncated after 8 bytes]") {
		t.Errorf("Stderr = %q, want truncation note", cmdErr.Stderr)
	}
}

func TestQuery_SpecTimeoutBoundsHang(t *testing.T) {
	spec := queryOf(ShapeLines, "sleep 5")
	spec.Timeout = 25 * time.Millisecond
	start := time.Now()
	_, err := Query(context.Background(), spec)
	cmdErr := wantCommandError(t, err, KindTimeout)
	if cmdErr.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", cmdErr.ExitCode)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(DeadlineExceeded) = false for %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Query took %s; spec timeout did not bound the hang", elapsed)
	}
}

func TestQuery_ParentCancellationKillsGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	spec := queryOf(ShapeLines, "trap '' INT TERM; sleep 5")
	spec.Timeout = 5 * time.Second
	start := time.Now()
	_, err := Query(ctx, spec)
	_ = wantCommandError(t, err, KindCanceled)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Query took %s; cancellation did not kill the signal-ignoring group", elapsed)
	}
}

func TestQuery_ParentDeadlineWinsWhenShorter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	spec := queryOf(ShapeLines, "sleep 5")
	spec.Timeout = 5 * time.Second
	start := time.Now()
	_, err := Query(ctx, spec)
	cmdErr := wantCommandError(t, err, KindTimeout)
	if cmdErr.Timeout != 0 {
		t.Errorf("Timeout = %s, want 0: the caller's deadline fired, not the spec's", cmdErr.Timeout)
	}
	if !strings.Contains(cmdErr.Headline(), "caller deadline exhausted") {
		t.Errorf("Headline = %q, want caller-deadline attribution, not a blamed spec duration", cmdErr.Headline())
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Query took %s; parent deadline did not apply", elapsed)
	}
}

func TestQuery_OwnDeadlineReportsSpecTimeout(t *testing.T) {
	spec := queryOf(ShapeLines, "sleep 5")
	spec.Timeout = 25 * time.Millisecond
	_, err := Query(context.Background(), spec)
	cmdErr := wantCommandError(t, err, KindTimeout)
	if cmdErr.Timeout != 25*time.Millisecond {
		t.Errorf("Timeout = %s, want the spec's own 25ms deadline", cmdErr.Timeout)
	}
	if !strings.Contains(cmdErr.Headline(), "timed out after 25ms") {
		t.Errorf("Headline = %q, want the spec deadline named", cmdErr.Headline())
	}
}

func TestQuery_DeadlineDuringPipeDrainStillSucceeds(t *testing.T) {
	// The child exits 0 with its full payload within a few ms; the orphaned
	// grandchild holds the inherited pipes so the bounded drain (100ms) is
	// still running when the 80ms deadline fires. A completed command must
	// not be reclassified as a timeout by drain-window noise.
	spec := queryOf(ShapeSingleLine, "printf '/tmp\\n'; (sleep 1) &")
	spec.Timeout = 80 * time.Millisecond
	res, err := Query(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "/tmp\n" {
		t.Errorf("Stdout = %q, want the complete payload", res.Stdout)
	}
}

// waitForProcessGone polls until pid no longer exists, proving the group
// kill reached a descendant and not just the direct child.
func waitForProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant pid %d still alive after group kill", pid)
}

func grandchildPID(t *testing.T, stdout string) int {
	t.Helper()
	line := strings.TrimSpace(strings.SplitN(stdout, "\n", 2)[0])
	pid, err := strconv.Atoi(line)
	if err != nil {
		t.Fatalf("could not parse grandchild pid from %q: %v", stdout, err)
	}
	return pid
}

func TestQuery_TimeoutKillsDescendants(t *testing.T) {
	spec := queryOf(ShapeLines, "sleep 30 & echo $!; sleep 5")
	spec.Timeout = 50 * time.Millisecond
	_, err := Query(context.Background(), spec)
	cmdErr := wantCommandError(t, err, KindTimeout)
	waitForProcessGone(t, grandchildPID(t, cmdErr.Stdout))
}

func TestQuery_ShapeViolationKillsDescendants(t *testing.T) {
	// First line is the grandchild pid; the second line violates
	// ShapeSingleLine, which must SIGKILL the whole group immediately.
	spec := queryOf(ShapeSingleLine, "sleep 30 & echo $!; echo second-line; sleep 5")
	spec.Timeout = 5 * time.Second
	start := time.Now()
	_, err := Query(context.Background(), spec)
	cmdErr := wantCommandError(t, err, KindOutput)
	waitForProcessGone(t, grandchildPID(t, cmdErr.Stdout))
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Query took %s; shape violation did not kill the group promptly", elapsed)
	}
}

func TestQuery_DoesNotWaitForGrandchildInheritedPipes(t *testing.T) {
	start := time.Now()
	res, err := Query(context.Background(), queryOf(ShapeSingleLine, "printf '/tmp\\n'; (sleep 1) &"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Stdout != "/tmp\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "/tmp\n")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Query took %s; likely waited for inherited stdout/stderr FDs", elapsed)
	}
}

func TestQuery_MissingBinary(t *testing.T) {
	spec := QuerySpec{
		Op:        "testquery",
		Argv:      []string{"cmdk-test-definitely-missing-binary"},
		Timeout:   time.Second,
		Shape:     ShapeSingleLine,
		MaxStdout: 1024,
		MaxStderr: 1024,
	}
	_, err := Query(context.Background(), spec)
	cmdErr := wantCommandError(t, err, KindExit)
	if cmdErr.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", cmdErr.ExitCode)
	}
	var execErr *exec.Error
	if !errors.As(err, &execErr) {
		t.Errorf("errors.As(*exec.Error) = false for %v", err)
	}
}

func TestQuery_InvalidSpecPanics(t *testing.T) {
	valid := QuerySpec{
		Op:        "testquery",
		Argv:      []string{"true"},
		Timeout:   time.Second,
		Shape:     ShapeEmpty,
		MaxStdout: 1,
		MaxStderr: 1,
	}

	wantPanic(t, "ctx is required", func() {
		var nilCtx context.Context
		_, _ = Query(nilCtx, valid)
	})

	missingArgv := valid
	missingArgv.Argv = nil
	wantPanic(t, "QuerySpec.Argv is required", func() { _, _ = Query(context.Background(), missingArgv) })

	zeroTimeout := valid
	zeroTimeout.Timeout = 0
	wantPanic(t, "QuerySpec.Timeout must be positive", func() { _, _ = Query(context.Background(), zeroTimeout) })

	zeroShape := valid
	zeroShape.Shape = 0
	wantPanic(t, "QuerySpec.Shape must be a declared Shape", func() { _, _ = Query(context.Background(), zeroShape) })

	zeroStdout := valid
	zeroStdout.MaxStdout = 0
	wantPanic(t, "QuerySpec.MaxStdout must be positive", func() { _, _ = Query(context.Background(), zeroStdout) })

	zeroStderr := valid
	zeroStderr.MaxStderr = 0
	wantPanic(t, "QuerySpec.MaxStderr must be positive", func() { _, _ = Query(context.Background(), zeroStderr) })
}

func TestStream_InjectedStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Stream(context.Background(), StreamSpec{
		Op:     "teststream",
		Argv:   []string{"sh", "-c", `read line; printf 'got %s' "$line"; printf diag >&2`},
		Stdin:  strings.NewReader("hello\n"),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "got hello" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "got hello")
	}
	if stderr.String() != "diag" {
		t.Errorf("stderr = %q, want %q", stderr.String(), "diag")
	}
}

func TestStream_ExitStatusPropagates(t *testing.T) {
	err := Stream(context.Background(), StreamSpec{
		Op:   "teststream",
		Argv: []string{"sh", "-c", "exit 3"},
	})
	cmdErr := wantCommandError(t, err, KindExit)
	if cmdErr.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", cmdErr.ExitCode)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("errors.As(*exec.ExitError) = false for %v", err)
	}
}

func TestStream_CancellationTerminatesChild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := Stream(ctx, StreamSpec{
		Op:   "teststream",
		Argv: []string{"sh", "-c", "sleep 5"},
	})
	_ = wantCommandError(t, err, KindCanceled)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Stream took %s; cancellation did not terminate the child", elapsed)
	}
}

func TestStream_DoesNotWaitForGrandchildInheritedPipes(t *testing.T) {
	var stdout bytes.Buffer
	start := time.Now()
	err := Stream(context.Background(), StreamSpec{
		Op:     "teststream",
		Argv:   []string{"sh", "-c", "printf hi; (sleep 1) &"},
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "hi" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "hi")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Stream took %s; likely waited for inherited pipes", elapsed)
	}
}

func TestStream_MissingBinary(t *testing.T) {
	err := Stream(context.Background(), StreamSpec{
		Op:   "teststream",
		Argv: []string{"cmdk-test-definitely-missing-binary"},
	})
	cmdErr := wantCommandError(t, err, KindExit)
	if cmdErr.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", cmdErr.ExitCode)
	}
}

func TestStream_InvalidSpecPanics(t *testing.T) {
	wantPanic(t, "StreamSpec.Argv is required", func() {
		_ = Stream(context.Background(), StreamSpec{Op: "teststream"})
	})
	wantPanic(t, "StreamSpec.Op is required", func() {
		_ = Stream(context.Background(), StreamSpec{Argv: []string{"true"}})
	})
}

func TestReplace_FailureReturnsCommandError(t *testing.T) {
	err := Replace("/cmdk-test-definitely-missing/binary", []string{"binary"}, nil)
	cmdErr := wantCommandError(t, err, KindExit)
	if cmdErr.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1", cmdErr.ExitCode)
	}
	if cmdErr.Err == nil {
		t.Error("Err = nil, want underlying exec failure")
	}
}

func TestReplace_InvalidArgsPanic(t *testing.T) {
	wantPanic(t, "path is required", func() { _ = Replace("", []string{"x"}, nil) })
	wantPanic(t, "argv is required", func() { _ = Replace("/bin/sh", nil, nil) })
}

func TestCommandError_HeadlinePerKind(t *testing.T) {
	tests := []struct {
		name string
		err  *CommandError
		want string
	}{
		{
			"timeout",
			&CommandError{Op: "picker source", Kind: KindTimeout, Timeout: 2 * time.Second},
			"picker source timed out after 2s",
		},
		{
			"canceled",
			&CommandError{Op: "launch_path_cmd", Kind: KindCanceled, Err: context.Canceled},
			"launch_path_cmd canceled: context canceled",
		},
		{
			"output",
			&CommandError{Op: "launch_path_cmd", Kind: KindOutput, Err: errors.New("output must contain exactly one line")},
			"launch_path_cmd output must contain exactly one line",
		},
		{
			"exit",
			&CommandError{Op: "picker source", Kind: KindExit, Err: errors.New("exit status 7")},
			"picker source failed: exit status 7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Headline(); got != tt.want {
				t.Errorf("Headline() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommandError_ErrorAppendsStreams(t *testing.T) {
	cmdErr := &CommandError{Op: "testcmd", Kind: KindExit, Err: errors.New("exit status 23"), Stdout: "out\n", Stderr: "err\n"}
	got := cmdErr.Error()
	want := "testcmd failed: exit status 23\nstdout: out\n\nstderr: err\n"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	bare := &CommandError{Op: "testcmd", Kind: KindExit, Err: errors.New("exit status 5")}
	if bare.Error() != "testcmd failed: exit status 5" {
		t.Errorf("Error() = %q, want headline only when streams empty", bare.Error())
	}
}
