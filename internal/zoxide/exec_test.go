package zoxide

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

func writeFakeZoxide(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "zoxide")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin:/bin")
}

func listDirsWithFake(t *testing.T, timeout time.Duration) ([]string, error) {
	t.Helper()
	items, err := ListDirs(context.Background(), timeout, 0, "", "~", nil, pathfmt.Truncation{})
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(items))
	for i, it := range items {
		paths[i] = it.Data["path"]
	}
	return paths, nil
}

func TestListDirs_ParsesFakeZoxideOutput(t *testing.T) {
	writeFakeZoxide(t, `printf '  42.5 /srv/data/projects\n 100.0 /srv/data/work\n'`)

	paths, err := listDirsWithFake(t, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/srv/data/work", "/srv/data/projects"}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Errorf("paths = %q, want %q (score-descending)", paths, want)
	}
}

func TestListDirs_MissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := listDirsWithFake(t, time.Second)
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != cmdrun.KindExit {
		t.Fatalf("error = %v, want KindExit CommandError", err)
	}
	var execErr *exec.Error
	if !errors.As(err, &execErr) {
		t.Fatalf("error = %v, want wrapped *exec.Error for missing binary", err)
	}
}

func TestListDirs_NonzeroExitCapturesStderr(t *testing.T) {
	writeFakeZoxide(t, `printf 'zoxide database is corrupt' >&2; exit 3`)

	_, err := listDirsWithFake(t, time.Second)
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != cmdrun.KindExit || cmdErr.ExitCode != 3 {
		t.Fatalf("error = %v, want KindExit CommandError with code 3", err)
	}
	if !strings.Contains(err.Error(), "zoxide database is corrupt") {
		t.Errorf("error = %q, want captured stderr", err.Error())
	}
}

func TestListDirs_OversizedOutputFailsInsteadOfTruncating(t *testing.T) {
	writeFakeZoxide(t, `yes '  10.0 /tmp/scratch' | head -c 4194305`)

	_, err := listDirsWithFake(t, 5*time.Second)
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != cmdrun.KindOutput {
		t.Fatalf("error = %v, want KindOutput CommandError", err)
	}
	if !strings.Contains(err.Error(), "output exceeds") {
		t.Errorf("error = %q, want output-limit violation", err.Error())
	}
}

func TestListDirs_HangBoundedByTimeout(t *testing.T) {
	writeFakeZoxide(t, `/bin/sleep 5`)

	start := time.Now()
	_, err := listDirsWithFake(t, 25*time.Millisecond)
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != cmdrun.KindTimeout {
		t.Fatalf("error = %v, want KindTimeout CommandError", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("ListDirs took %s; timeout did not bound the hang", elapsed)
	}
}
