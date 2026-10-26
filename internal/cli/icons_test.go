package cli

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
	options := iconOptions{cod: true}

	if !matchesSetFlag("cod", options) {
		t.Error("expected cod to match when flagIconCod is true")
	}
	if matchesSetFlag("dev", options) {
		t.Error("expected dev not to match when flagIconDev is false")
	}
	if matchesSetFlag("unknown", options) {
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
			options := iconOptions{cod: tt.cod, dev: tt.dev, oct: tt.oct, filter: tt.filter}

			got := captureStderr(t, func() { emitIconsEmptyHint(options) })
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

func TestRunIconsCommandWithoutFlagsListsEverySet(t *testing.T) {
	bare := captureStdout(t, func() {
		if err := runIconsCommand(iconOptions{}); err != nil {
			t.Fatal(err)
		}
	})
	explicit := captureStdout(t, func() {
		if err := runIconsCommand(iconOptions{cod: true, dev: true, oct: true}); err != nil {
			t.Fatal(err)
		}
	})
	if bare != explicit {
		t.Error("bare icons output differs from listing every set")
	}
	for _, heading := range []string{"Codicons (cod)", "Devicons (dev)", "Octicons (oct)"} {
		if !strings.Contains(bare, heading+"\n") {
			t.Errorf("bare icons output missing heading %q", heading)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stdout, fn)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stderr, fn)
}

func captureStream(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := *stream
	*stream = w
	defer func() { *stream = orig }()

	// Drain while fn writes, so output larger than the pipe buffer cannot
	// block the writer.
	type read struct {
		data []byte
		err  error
	}
	done := make(chan read, 1)
	go func() {
		data, err := io.ReadAll(r)
		done <- read{data: data, err: err}
	}()

	fn()
	_ = w.Close()

	result := <-done
	if result.err != nil {
		t.Fatalf("read pipe: %v", result.err)
	}
	return string(result.data)
}
