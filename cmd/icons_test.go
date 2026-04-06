package cmd

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/icon"
)

func TestSetFromAlias(t *testing.T) {
	tests := []struct {
		alias string
		want  string
	}{
		{"nf-cod-terminal", "cod"},
		{"nf-dev-go", "dev"},
		{"nf-oct-git_branch", "oct"},
		{"nodash", ""},
	}
	for _, tt := range tests {
		got := setFromAlias(tt.alias)
		if got != tt.want {
			t.Errorf("setFromAlias(%q) = %q, want %q", tt.alias, got, tt.want)
		}
	}
}

func TestAliasPrefix(t *testing.T) {
	tests := []struct {
		alias string
		want  string
	}{
		{"nf-cod-terminal", "nf-cod"},
		{"nf-dev-go", "nf-dev"},
		{"nodash", "nodash"},
	}
	for _, tt := range tests {
		got := aliasPrefix(tt.alias)
		if got != tt.want {
			t.Errorf("aliasPrefix(%q) = %q, want %q", tt.alias, got, tt.want)
		}
	}
}

func TestPrefixHeading(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"nf-cod", "Codicons (cod)"},
		{"nf-dev", "Devicons (dev)"},
		{"nf-oct", "Octicons (oct)"},
		{"nf-md", "nf-md"},
	}
	for _, tt := range tests {
		got := prefixHeading(tt.prefix)
		if got != tt.want {
			t.Errorf("prefixHeading(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}

func TestMatchesFilter(t *testing.T) {
	e := icon.Entry{Alias: "nf-cod-terminal_tmux", Description: "Terminal tmux"}

	if !matchesFilter(e, "terminal") {
		t.Error("expected match on alias substring")
	}
	if !matchesFilter(e, "tmux") {
		t.Error("expected match on description substring")
	}
	if !matchesFilter(e, "cod") {
		t.Error("expected match on alias prefix")
	}
	if matchesFilter(e, "nonexistent") {
		t.Error("expected no match")
	}
}

func TestMatchesSetFlag(t *testing.T) {
	flagIconCod = true
	flagIconDev = false
	flagIconOct = false
	defer func() { flagIconCod = false }()

	if !matchesSetFlag("cod") {
		t.Error("expected cod to match when flagIconCod is true")
	}
	if matchesSetFlag("dev") {
		t.Error("expected dev not to match when flagIconDev is false")
	}
	if matchesSetFlag("unknown") {
		t.Error("expected unknown set not to match")
	}
}

func TestIconSetCounts(t *testing.T) {
	counts := iconSetCounts()
	total := counts["cod"] + counts["dev"] + counts["oct"]
	if total != len(icon.All()) {
		t.Errorf("set counts sum to %d, want %d", total, len(icon.All()))
	}
	if counts["cod"] == 0 {
		t.Error("expected non-zero cod count")
	}
	if counts["dev"] == 0 {
		t.Error("expected non-zero dev count")
	}
	if counts["oct"] == 0 {
		t.Error("expected non-zero oct count")
	}
}
