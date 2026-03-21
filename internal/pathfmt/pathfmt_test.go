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

	got := DisplayPath(home+"/Code/project", "~", nil)
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

	got := DisplayPath(home, "~", nil)
	if got != "~" {
		t.Errorf("DisplayPath(%q) = %q, want %q", home, got, "~")
	}
}

func TestDisplayPath_NonHomePath(t *testing.T) {
	got := DisplayPath("/tmp/scratch", "~", nil)
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
	got := DisplayPath(path, "~", nil)
	if got != path {
		t.Errorf("DisplayPath(%q) = %q, want unchanged", path, got)
	}
}

func TestDisplayPath_EmptyPath(t *testing.T) {
	got := DisplayPath("", "~", nil)
	if got != "" {
		t.Errorf("DisplayPath(%q) = %q, want empty", "", got)
	}
}

func TestDisplayPath_ShortenHomeDisabled(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	path := home + "/Code/project"
	got := DisplayPath(path, "", nil)
	if got != path {
		t.Errorf("DisplayPath(%q) with empty shortenHome = %q, want unchanged", path, got)
	}
}

func TestDisplayPath_SingleRule(t *testing.T) {
	rules := CompileRules(map[string]string{
		"github.palantir.build": "gpb",
	})
	got := DisplayPath("~/Code/github.palantir.build/PRX/iris", "", rules)
	want := "~/Code/gpb/PRX/iris"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDisplayPath_MultipleRulesApply(t *testing.T) {
	rules := CompileRules(map[string]string{
		"github.palantir.build": "gpb",
		"~/Code":                "~/\uf121 ",
	})
	got := DisplayPath("~/Code/github.palantir.build/PRX/iris", "", rules)
	want := "~/\uf121 /gpb/PRX/iris"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDisplayPath_LongestKeyFirst(t *testing.T) {
	rules := CompileRules(map[string]string{
		"abc":    "X",
		"abcdef": "Y",
	})
	got := DisplayPath("_abcdef_", "", rules)
	want := "_Y_"
	if got != want {
		t.Errorf("got %q, want %q (longest key should match first)", got, want)
	}
}

func TestDisplayPath_ShortenHomeThenRules(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	rules := CompileRules(map[string]string{
		"~/Code": "~/\uf121 ",
	})
	got := DisplayPath(home+"/Code/myproject", "~", rules)
	want := "~/\uf121 /myproject"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDisplayPath_NoRulesNoShortenHome(t *testing.T) {
	got := DisplayPath("/some/path", "", nil)
	if got != "/some/path" {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestCompileRules_SortOrder(t *testing.T) {
	rules := CompileRules(map[string]string{
		"ab":    "1",
		"abcde": "2",
		"abc":   "3",
	})
	if len(rules) != 3 {
		t.Fatalf("got %d rules, want 3", len(rules))
	}
	if rules[0].Match != "abcde" {
		t.Errorf("rules[0].Match = %q, want %q", rules[0].Match, "abcde")
	}
	if rules[1].Match != "abc" {
		t.Errorf("rules[1].Match = %q, want %q", rules[1].Match, "abc")
	}
	if rules[2].Match != "ab" {
		t.Errorf("rules[2].Match = %q, want %q", rules[2].Match, "ab")
	}
}

func TestCompileRules_Empty(t *testing.T) {
	rules := CompileRules(nil)
	if len(rules) != 0 {
		t.Errorf("got %d rules, want 0", len(rules))
	}
}
