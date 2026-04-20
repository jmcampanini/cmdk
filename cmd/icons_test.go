package cmd

import (
	"io"
	"os"
	"strings"
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

func TestEmitIconsEmptyHint(t *testing.T) {
	tests := []struct {
		name     string
		cod      bool
		dev      bool
		oct      bool
		filter   string
		contains []string
		absent   []string
	}{
		{
			name:   "single set with filter",
			cod:    true,
			filter: "xyz",
			contains: []string{
				`no results for filter="xyz" (sets: cod)`,
				"try:",
				"--dev --oct",
				"include other icon sets",
				"--filter=<shorter>",
				"broader match",
				"cmdk icons --fzf | fzf",
				"interactive fuzzy search",
			},
		},
		{
			name:   "filter only, all sets",
			filter: "xyz",
			contains: []string{
				`no results for filter="xyz" (sets: all)`,
				"--filter=<shorter>",
				"cmdk icons --fzf | fzf",
			},
			absent: []string{"include other icon sets"},
		},
		{
			name:   "all three sets explicit with filter",
			cod:    true,
			dev:    true,
			oct:    true,
			filter: "xyz",
			contains: []string{
				`no results for filter="xyz" (sets: cod,dev,oct)`,
				"--filter=<shorter>",
				"cmdk icons --fzf | fzf",
			},
			absent: []string{"include other icon sets"},
		},
		{
			name: "single set, no filter",
			cod:  true,
			contains: []string{
				"no results (sets: cod)",
				"--dev --oct",
				"include other icon sets",
				"cmdk icons --fzf | fzf",
			},
			absent: []string{
				"for filter=",
				"broader match",
			},
		},
		{
			name: "no flags, no filter",
			contains: []string{
				"no results (sets: all)",
				"cmdk icons --fzf | fzf",
			},
			absent: []string{
				"for filter=",
				"include other icon sets",
				"broader match",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagIconCod, flagIconDev, flagIconOct = tt.cod, tt.dev, tt.oct
			flagIconFilter = tt.filter
			defer func() {
				flagIconCod, flagIconDev, flagIconOct = false, false, false
				flagIconFilter = ""
			}()

			got := captureStderr(t, emitIconsEmptyHint)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("stderr missing %q\nstderr:\n%s", want, got)
				}
			}
			for _, banned := range tt.absent {
				if strings.Contains(got, banned) {
					t.Errorf("stderr should not contain %q\nstderr:\n%s", banned, got)
				}
			}
		})
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()
	_ = w.Close()

	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(buf)
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
