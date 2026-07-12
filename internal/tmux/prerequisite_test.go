package tmux

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeTmux(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  version
	}{
		{input: "tmux 3.2", want: version{major: 3, minor: 2}},
		{input: "tmux 3.2a", want: version{major: 3, minor: 2}},
		{input: "tmux 3.10", want: version{major: 3, minor: 10}},
		{input: "tmux 4.0", want: version{major: 4, minor: 0}},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := parseVersion(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("parseVersion(%q) = %+v, want %+v", test.input, got, test.want)
			}
		})
	}
}

func TestParseVersionRejectsMalformedOutput(t *testing.T) {
	for _, input := range []string{
		"",
		"3.2",
		"tmux next-3.2",
		"tmux +4.0",
		"tmux  3.2",
		"tmux\t3.2",
		"tmux 3",
		"tmux 3.x",
		"tmux 3.2-a",
		"tmux 3.2 extra",
		"tmux 3.2\ntmux 3.3",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := parseVersion(input); err == nil {
				t.Fatalf("parseVersion(%q) succeeded, want error", input)
			}
		})
	}
}

func TestCheckPrerequisiteAcceptsMinimumAndNewerVersions(t *testing.T) {
	for _, output := range []string{"tmux 3.2", "tmux 3.2a", "tmux 3.10", "tmux 4.0"} {
		t.Run(output, func(t *testing.T) {
			writeFakeTmux(t, "printf '%s\\n' '"+output+"'")
			if err := checkPrerequisite(context.Background(), time.Second); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckPrerequisiteRejectsOldVersion(t *testing.T) {
	writeFakeTmux(t, "printf '%s\\n' 'tmux 3.1c'")
	err := checkPrerequisite(context.Background(), time.Second)
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
	for _, want := range []string{"tmux 3.2 or newer is required", "found tmux 3.1c", "first in PATH"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
}

func TestCheckPrerequisiteRejectsMissingTmux(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := checkPrerequisite(context.Background(), time.Second)
	if err == nil {
		t.Fatal("expected missing tmux error")
	}
	for _, want := range []string{"tmux 3.2 or newer is required", "install tmux", "PATH"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q", err, want)
		}
	}
}

func TestCheckPrerequisiteReportsVersionCommandFailure(t *testing.T) {
	writeFakeTmux(t, "printf 'version probe failed' >&2\nexit 23")
	err := checkPrerequisite(context.Background(), time.Second)
	if err == nil {
		t.Fatal("expected tmux -V failure")
	}
	if !strings.Contains(err.Error(), "tmux -V failed") || !strings.Contains(err.Error(), "version probe failed") {
		t.Fatalf("error = %q, want command and stderr", err)
	}
}

func TestCheckPrerequisiteRejectsMalformedVersion(t *testing.T) {
	writeFakeTmux(t, "printf '%s\\n' 'tmux development-build'")
	err := checkPrerequisite(context.Background(), time.Second)
	if err == nil || !strings.Contains(err.Error(), "could not parse tmux -V output") {
		t.Fatalf("error = %v, want malformed output error", err)
	}
}

func TestCheckPrerequisiteRejectsOversizedOutput(t *testing.T) {
	writeFakeTmux(t, "i=0\nwhile [ \"$i\" -le 5000 ]; do printf x; i=$((i + 1)); done")
	err := checkPrerequisite(context.Background(), time.Second)
	if err == nil || !strings.Contains(err.Error(), "oversized output") {
		t.Fatalf("error = %v, want oversized output error", err)
	}
}

func TestCheckPrerequisiteTimesOut(t *testing.T) {
	writeFakeTmux(t, "/bin/sleep 1")
	err := checkPrerequisite(context.Background(), 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not respond") {
		t.Fatalf("error = %v, want timeout error", err)
	}
}

func TestCheckPrerequisiteReportsParentDeadline(t *testing.T) {
	writeFakeTmux(t, "/bin/sleep 1")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := checkPrerequisite(ctx, time.Second)
	if err == nil || !strings.Contains(err.Error(), "checking required tmux version: context deadline exceeded") {
		t.Fatalf("error = %v, want parent deadline error", err)
	}
	if strings.Contains(err.Error(), "within 1s") {
		t.Fatalf("error = %v, should not report the internal timeout", err)
	}
}

func TestCheckPrerequisiteBoundsInheritedOutputDescriptors(t *testing.T) {
	writeFakeTmux(t, "/bin/sleep 1 &\nprintf '%s\\n' 'tmux 3.2'")
	start := time.Now()
	err := checkPrerequisite(context.Background(), time.Second)
	if err == nil || !strings.Contains(err.Error(), "tmux -V failed") {
		t.Fatalf("error = %v, want inherited output descriptor failure", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("check took %s, want bounded wait", elapsed)
	}
}

func TestCheckPrerequisiteHonorsCancellation(t *testing.T) {
	writeFakeTmux(t, "/bin/sleep 1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := checkPrerequisite(ctx, time.Second)
	if err == nil || !strings.Contains(err.Error(), "checking required tmux version") {
		t.Fatalf("error = %v, want cancellation error", err)
	}
}
