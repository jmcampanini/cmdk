package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
)

func TestMultiTermFilter_EmptyTerm(t *testing.T) {
	targets := []string{"~/dotfiles/main", "~/projects/foo"}
	got := multiTermFilter("", targets)
	if len(got) != len(targets) {
		t.Errorf("expected %d items (all), got %d", len(targets), len(got))
	}
}

func TestMultiTermFilter_WhitespaceOnly(t *testing.T) {
	targets := []string{"~/dotfiles/main", "~/projects/foo"}
	got := multiTermFilter("   ", targets)
	if len(got) != len(targets) {
		t.Errorf("expected %d items (all), got %d", len(targets), len(got))
	}
}

func TestMultiTermFilter_SingleTerm(t *testing.T) {
	targets := []string{"main:1 zsh", "dev:1 node"}
	got := multiTermFilter("main", targets)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Index != 0 {
		t.Errorf("expected index 0, got %d", got[0].Index)
	}
}

func TestMultiTermFilter_TwoTerms(t *testing.T) {
	targets := []string{"~/dotfiles/main", "~/projects/foo", "~/dotfiles/dev"}
	got := multiTermFilter("dotfiles main", targets)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Index != 0 {
		t.Errorf("expected index 0 (~/dotfiles/main), got %d", got[0].Index)
	}
}

func TestMultiTermFilter_OrderIndependent(t *testing.T) {
	targets := []string{"~/dotfiles/main"}
	forward := multiTermFilter("dotfiles main", targets)
	reverse := multiTermFilter("main dotfiles", targets)
	if len(forward) != 1 || len(reverse) != 1 {
		t.Fatalf("expected 1 match each, got forward=%d reverse=%d", len(forward), len(reverse))
	}
	if forward[0].Index != reverse[0].Index {
		t.Error("order should not matter")
	}
}

func TestMultiTermFilter_PartialTermMiss(t *testing.T) {
	targets := []string{"~/dotfiles/main"}
	got := multiTermFilter("dotfiles qqq", targets)
	if got != nil {
		t.Errorf("expected nil (AND miss), got %v", got)
	}
}

func TestMultiTermFilter_NoMatch(t *testing.T) {
	targets := []string{"~/dotfiles/main"}
	got := multiTermFilter("zzz qqq", targets)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestMultiTermFilter_HighlightsMerged(t *testing.T) {
	targets := []string{"~/dotfiles/main"}
	got := multiTermFilter("dot main", targets)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	indexes := got[0].MatchedIndexes
	hasDot := containsAll(indexes, findSubstringIndexes("~/dotfiles/main", "dot"))
	hasMain := containsAll(indexes, findSubstringIndexes("~/dotfiles/main", "main"))
	if !hasDot || !hasMain {
		t.Errorf("expected highlights for both 'dot' and 'main', got indexes %v", indexes)
	}
}

func TestMultiTermFilter_HighlightsDeduped(t *testing.T) {
	targets := []string{"~/dotfiles/main"}
	got := multiTermFilter("dot dot", targets)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	indexes := got[0].MatchedIndexes
	if !slices.IsSorted(indexes) {
		t.Error("indexes should be sorted")
	}
	for i := 1; i < len(indexes); i++ {
		if indexes[i] == indexes[i-1] {
			t.Errorf("duplicate index %d at position %d", indexes[i], i)
		}
	}
}

func TestMultiTermFilter_ExtraWhitespace(t *testing.T) {
	targets := []string{"~/dotfiles/main", "~/projects/foo"}
	got := multiTermFilter("  dot   main  ", targets)
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Index != 0 {
		t.Errorf("expected index 0, got %d", got[0].Index)
	}
}

func TestMultiTermFilter_MultipleMatches(t *testing.T) {
	targets := []string{"~/dotfiles/main", "~/dotfiles/main-backup", "~/projects/foo"}
	got := multiTermFilter("dot main", targets)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	indexes := []int{got[0].Index, got[1].Index}
	slices.Sort(indexes)
	if indexes[0] != 0 || indexes[1] != 1 {
		t.Errorf("expected indexes [0,1], got %v", indexes)
	}
}

func TestMultiTermFilter_Regression_DotfilesMainRanking(t *testing.T) {
	targets := []string{
		"tmux: 0:2 dotfiles/main",
		"~/Code/github.com/jmcampanini/dotfiles",
		"~/Code/github.com/jmcampanini/dotfiles/main",
		"~/Code/github.com/jmcampanini/cmdk/wt-filter-with-spaces",
	}
	got := multiTermFilter("dotfiles main", targets)
	if len(got) == 0 {
		t.Fatal("expected at least one match")
	}

	idxOf := func(target int) int {
		for i, r := range got {
			if r.Index == target {
				return i
			}
		}
		return -1
	}

	tmuxRank := idxOf(0)
	dotfilesMainRank := idxOf(2)
	bareDotfilesRank := idxOf(1)

	if dotfilesMainRank < 0 {
		t.Fatal("dotfiles/main should match")
	}
	if tmuxRank < 0 {
		t.Fatal("tmux dotfiles/main should match")
	}
	if bareDotfilesRank >= 0 && dotfilesMainRank > bareDotfilesRank {
		t.Errorf("dotfiles/main (rank %d) should rank above bare dotfiles (rank %d)",
			dotfilesMainRank, bareDotfilesRank)
	}
}

func TestMultiTermFilter_Ranking(t *testing.T) {
	tests := []struct {
		name      string
		targets   []string
		query     string
		wantFirst int
		wantAbove []int // wantFirst must rank above each of these (if present)
	}{
		// ── Edge cases (contiguous vs scattered, AND precision, depth) ──

		{
			name: "worktree beats bare repo when second term is contiguous",
			targets: []string{
				"~/Code/github.com/jmcampanini/cmdk/main",
				"~/Code/github.com/jmcampanini/cmdk/wt-filter-with-spaces",
				"~/Code/github.com/jmcampanini/cmdk",
				"~/Code/github.com/jmcampanini/dotfiles/main",
			},
			query:     "cmdk main",
			wantFirst: 0,
			wantAbove: []int{2}, // bare cmdk has scattered "main"
		},
		{
			name: "two-term AND selects specific worktree",
			targets: []string{
				"~/Code/github.com/jmcampanini/cmdk/main",
				"~/Code/github.com/jmcampanini/cmdk/wt-filter-with-spaces",
				"~/Code/github.com/jmcampanini/cmdk",
				"~/Code/github.com/jmcampanini/dotfiles/main",
			},
			query:     "cmdk filter",
			wantFirst: 1,
		},
		{
			name: "AND eliminates non-matching targets",
			targets: []string{
				"~/Code/github.com/jmcampanini/cmdk/main",
				"~/Code/github.com/jmcampanini/dotfiles/main",
				"~/Code/github.com/jmcampanini/grove-cli/main",
			},
			query:     "grove main",
			wantFirst: 2,
		},
		{
			name: "AND across depth levels picks only deep match",
			targets: []string{
				"~/projects/api",
				"~/projects/api-gateway",
				"~/projects/api-gateway/src/api",
				"~/projects/internal-api",
				"~/projects/web/api-client",
			},
			query:     "api src",
			wantFirst: 2,
		},
		{
			name: "contiguous /main beats scattered main in jmcampanini",
			targets: []string{
				"~/Code/github.com/jmcampanini",
				"~/Code/github.com/jmcampanini/cmdk",
				"~/Code/github.com/jmcampanini/cmdk/main",
				"~/Code/github.com/jmcampanini/dotfiles/main",
			},
			query:     "jmc main",
			wantFirst: 2,
			wantAbove: []int{0, 1}, // targets without /main have only scattered "main"
		},
		{
			name: "path beats tmux for same segments",
			targets: []string{
				"tmux: dotfiles:1 nvim",
				"tmux: dotfiles-main:2 shell",
				"~/Code/github.com/jmcampanini/dotfiles/main",
				"~/Code/github.com/jmcampanini/dotfiles",
				"~/Code/github.com/jmcampanini/cmdk/main",
			},
			query:     "dot main",
			wantFirst: 2,
			wantAbove: []int{1, 3}, // path beats tmux; both beat bare dotfiles
		},
		{
			name: "three-term AND narrows to matching subset",
			targets: []string{
				"~/Code/github.com/jmcampanini/cmdk/main",
				"~/Code/github.com/jmcampanini/cmdk/main/cmd",
				"~/Code/github.com/jmcampanini/dotfiles/main",
				"~/Code/github.com/jmcampanini/grove-cli/main",
			},
			query:     "cmdk main cmd",
			wantFirst: 0,
		},
		{
			name: "camelCase boundary bonus",
			targets: []string{
				"tmux: apiGateway:1 server",
				"tmux: apigateway:1 server",
				"~/projects/api-gateway",
			},
			query:     "ag",
			wantFirst: 2,
			wantAbove: []int{1}, // camelCase/delimiter > flat lowercase
		},

		// ── Common cases (tiebreak, straightforward matches) ──

		{
			name: "single term tiebreaks by index",
			targets: []string{
				"~/Code/github.com/jmcampanini/cmdk",
				"~/Code/github.com/jmcampanini/cmdk/main",
				"~/Code/github.com/jmcampanini/cmdk/wt-filter-with-spaces",
				"tmux: cmdk:1 shell",
			},
			query:     "cmdk",
			wantFirst: 0,
			wantAbove: []int{3}, // path boundary beats tmux colon boundary
		},
		{
			name: "straightforward two-term single match",
			targets: []string{
				"~/Code/github.com/acme/api/main",
				"~/Code/github.com/acme/api-gateway/main",
				"~/Code/github.com/acme/web-app/main",
			},
			query:     "gateway main",
			wantFirst: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := multiTermFilter(tt.query, tt.targets)
			if len(got) == 0 {
				t.Fatal("expected at least one match")
			}

			if got[0].Index != tt.wantFirst {
				t.Errorf("want first=%d (%s), got first=%d (%s)",
					tt.wantFirst, tt.targets[tt.wantFirst],
					got[0].Index, tt.targets[got[0].Index])
			}

			rankOf := func(target int) int {
				for i, r := range got {
					if r.Index == target {
						return i
					}
				}
				return -1
			}

			firstRank := rankOf(tt.wantFirst)
			for _, above := range tt.wantAbove {
				aboveRank := rankOf(above)
				if aboveRank >= 0 && firstRank > aboveRank {
					t.Errorf("%s (rank %d) should rank above %s (rank %d)",
						tt.targets[tt.wantFirst], firstRank,
						tt.targets[above], aboveRank)
				}
			}
		})
	}
}

func TestDedupIndexes(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"nil", nil, nil},
		{"empty", []int{}, nil},
		{"single", []int{5}, []int{5}},
		{"already sorted unique", []int{1, 3, 5}, []int{1, 3, 5}},
		{"unsorted with dups", []int{5, 1, 3, 1, 5}, []int{1, 3, 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupIndexes(slices.Clone(tt.in))
			if !slices.Equal(got, tt.want) {
				t.Errorf("dedupIndexes(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSingleTermFilter_MatchesBehavior(t *testing.T) {
	targets := []string{"main:1 zsh", "dev:1 node", "~/projects/foo"}
	single := singleTermFilter("main", targets)
	multi := multiTermFilter("main", targets)
	if len(single) != len(multi) {
		t.Fatalf("single=%d multi=%d, expected same count", len(single), len(multi))
	}
	for i := range single {
		if single[i].Index != multi[i].Index {
			t.Errorf("result[%d] index mismatch: single=%d multi=%d", i, single[i].Index, multi[i].Index)
		}
	}
}

func containsAll(set, subset []int) bool {
	m := make(map[int]bool, len(set))
	for _, v := range set {
		m[v] = true
	}
	for _, v := range subset {
		if !m[v] {
			return false
		}
	}
	return true
}

func findSubstringIndexes(s, sub string) []int {
	idx := strings.Index(s, sub)
	if idx < 0 {
		return nil
	}
	result := make([]int, len(sub))
	for i := range sub {
		result[i] = idx + i
	}
	return result
}

var _ func(string, []string) []list.Rank = multiTermFilter
