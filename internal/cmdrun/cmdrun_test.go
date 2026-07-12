package cmdrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

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
	if res.StdoutTruncated || res.StderrTruncated {
		t.Errorf("truncated = %v/%v, want false/false", res.StdoutTruncated, res.StderrTruncated)
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
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error = %v, want *CommandError", err)
	}
	if cmdErr.Kind != KindExit {
		t.Errorf("Kind = %q, want %q", cmdErr.Kind, KindExit)
	}
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
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error = %v, want *CommandError", err)
	}
	if cmdErr.Kind != KindTimeout {
		t.Errorf("Kind = %q, want %q", cmdErr.Kind, KindTimeout)
	}
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

func TestRun_Timeout(t *testing.T) {
	_, err := Run(Spec{
		Op:         "testcmd",
		Rendered:   "sleep 1; printf /tmp",
		Timeout:    10 * time.Millisecond,
		SingleLine: true,
		MaxStdout:  1024,
		MaxStderr:  1024,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want timeout", err)
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
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != KindCanceled {
		t.Fatalf("error = %v, want KindCanceled CommandError", err)
	}
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
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != KindOutput {
		t.Fatalf("error = %v, want KindOutput CommandError", err)
	}
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
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != KindOutput {
		t.Fatalf("error = %v, want KindOutput CommandError", err)
	}
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
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) || cmdErr.ExitCode != 3 {
		t.Fatalf("error = %v, want exit 3 CommandError", err)
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
