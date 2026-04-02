package pathfmt

import "testing"

const testHome = "/home/testuser"

func TestDisplayPath_ReplacesHomePrefix(t *testing.T) {
	got := DisplayPath(testHome+"/Code/project", testHome, "~", nil, Truncation{})
	want := "~/Code/project"
	if got != want {
		t.Errorf("DisplayPath(%q) = %q, want %q", testHome+"/Code/project", got, want)
	}
}

func TestDisplayPath_ExactHomeDir(t *testing.T) {
	got := DisplayPath(testHome, testHome, "~", nil, Truncation{})
	if got != "~" {
		t.Errorf("DisplayPath(%q) = %q, want %q", testHome, got, "~")
	}
}

func TestDisplayPath_NonHomePath(t *testing.T) {
	got := DisplayPath("/tmp/scratch", testHome, "~", nil, Truncation{})
	if got != "/tmp/scratch" {
		t.Errorf("DisplayPath(%q) = %q, want unchanged", "/tmp/scratch", got)
	}
}

func TestDisplayPath_SimilarPrefix(t *testing.T) {
	path := testHome + "XYZ/not-a-child"
	got := DisplayPath(path, testHome, "~", nil, Truncation{})
	if got != path {
		t.Errorf("DisplayPath(%q) = %q, want unchanged", path, got)
	}
}

func TestDisplayPath_EmptyPath(t *testing.T) {
	got := DisplayPath("", testHome, "~", nil, Truncation{})
	if got != "" {
		t.Errorf("DisplayPath(%q) = %q, want empty", "", got)
	}
}

func TestDisplayPath_ShortenHomeDisabled(t *testing.T) {
	path := testHome + "/Code/project"
	got := DisplayPath(path, testHome, "", nil, Truncation{})
	if got != path {
		t.Errorf("DisplayPath(%q) with empty shortenHome = %q, want unchanged", path, got)
	}
}

func TestDisplayPath_EmptyHome(t *testing.T) {
	path := "/some/path"
	got := DisplayPath(path, "", "~", nil, Truncation{})
	if got != path {
		t.Errorf("DisplayPath(%q) with empty home = %q, want unchanged", path, got)
	}
}

func TestDisplayPath_SingleRule(t *testing.T) {
	rules := CompileRules(map[string]string{
		"github.palantir.build": "gpb",
	})
	got := DisplayPath("~/Code/github.palantir.build/PRX/iris", "", "", rules, Truncation{})
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
	got := DisplayPath("~/Code/github.palantir.build/PRX/iris", "", "", rules, Truncation{})
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
	got := DisplayPath("_abcdef_", "", "", rules, Truncation{})
	want := "_Y_"
	if got != want {
		t.Errorf("got %q, want %q (longest key should match first)", got, want)
	}
}

func TestDisplayPath_ShortenHomeThenRules(t *testing.T) {
	rules := CompileRules(map[string]string{
		"~/Code": "~/\uf121 ",
	})
	got := DisplayPath(testHome+"/Code/myproject", testHome, "~", rules, Truncation{})
	want := "~/\uf121 /myproject"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDisplayPath_NoRulesNoShortenHome(t *testing.T) {
	got := DisplayPath("/some/path", "", "", nil, Truncation{})
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

func TestTruncateParts_Basic(t *testing.T) {
	got := truncateParts("~/Code/gh/foo/bar", Truncation{Length: 2})
	if got != "foo/bar" {
		t.Errorf("got %q, want %q", got, "foo/bar")
	}
}

func TestTruncateParts_WithSymbol(t *testing.T) {
	got := truncateParts("~/Code/gh/foo/bar", Truncation{Length: 2, Symbol: "…"})
	if got != "…/foo/bar" {
		t.Errorf("got %q, want %q", got, "…/foo/bar")
	}
}

func TestTruncateParts_ZeroDisabled(t *testing.T) {
	path := "~/Code/gh/foo/bar"
	got := truncateParts(path, Truncation{Symbol: "…"})
	if got != path {
		t.Errorf("got %q, want unchanged %q", got, path)
	}
}

func TestTruncateParts_FewerPartsThanLength(t *testing.T) {
	path := "~/foo"
	got := truncateParts(path, Truncation{Length: 5, Symbol: "…"})
	if got != path {
		t.Errorf("got %q, want unchanged %q", got, path)
	}
}

func TestTruncateParts_ExactLength(t *testing.T) {
	path := "a/b/c"
	got := truncateParts(path, Truncation{Length: 3, Symbol: "…"})
	if got != path {
		t.Errorf("got %q, want unchanged %q (equal parts)", got, path)
	}
}

func TestTruncateParts_SinglePart(t *testing.T) {
	got := truncateParts("~/Code/gh/foo/bar", Truncation{Length: 1})
	if got != "bar" {
		t.Errorf("got %q, want %q", got, "bar")
	}
}

func TestDisplayPath_FullPipeline_WithTruncation(t *testing.T) {
	rules := CompileRules(map[string]string{
		"github.com": "gh",
	})
	got := DisplayPath("/home/testuser/Code/github.com/foo/bar", testHome, "~", rules, Truncation{Length: 2, Symbol: "…"})
	want := "…/foo/bar"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTruncateParts_EmptySymbolNoPrefix(t *testing.T) {
	got := truncateParts("a/b/c/d", Truncation{Length: 2})
	if got != "c/d" {
		t.Errorf("got %q, want %q", got, "c/d")
	}
}

func TestTruncateParts_AbsolutePathLeadingSlash(t *testing.T) {
	got := truncateParts("/usr/local/bin/foo", Truncation{Length: 2})
	if got != "bin/foo" {
		t.Errorf("got %q, want %q", got, "bin/foo")
	}
}

func TestTruncateParts_AbsolutePathExactSegments(t *testing.T) {
	got := truncateParts("/usr/local", Truncation{Length: 2, Symbol: "…"})
	if got != "/usr/local" {
		t.Errorf("got %q, want %q (no truncation — only 2 real segments)", got, "/usr/local")
	}
}
