package pathfmt

import (
	"os"
	"testing"
)

func TestDisplayPath_ReplacesHomePrefix(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	got := DisplayPath(home + "/Code/project")
	want := "~/Code/project"
	if got != want {
		t.Errorf("DisplayPath(%q) = %q, want %q", home+"/Code/project", got, want)
	}
}

func TestDisplayPath_ExactHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	got := DisplayPath(home)
	if got != "~" {
		t.Errorf("DisplayPath(%q) = %q, want %q", home, got, "~")
	}
}

func TestDisplayPath_NonHomePath(t *testing.T) {
	got := DisplayPath("/tmp/scratch")
	if got != "/tmp/scratch" {
		t.Errorf("DisplayPath(%q) = %q, want unchanged", "/tmp/scratch", got)
	}
}

func TestDisplayPath_SimilarPrefix(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	path := home + "XYZ/not-a-child"
	got := DisplayPath(path)
	if got != path {
		t.Errorf("DisplayPath(%q) = %q, want unchanged", path, got)
	}
}

func TestDisplayPath_EmptyPath(t *testing.T) {
	got := DisplayPath("")
	if got != "" {
		t.Errorf("DisplayPath(%q) = %q, want empty", "", got)
	}
}
