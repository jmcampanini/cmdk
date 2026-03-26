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

// containsAll checks whether all elements of subset appear in set.
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

// Verify the custom filter conforms to the list.Filter signature.
var _ func(string, []string) []list.Rank = multiTermFilter
