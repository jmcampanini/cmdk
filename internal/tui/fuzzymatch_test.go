package tui

import (
	"testing"
)

func TestFuzzyMatch_EmptyPattern(t *testing.T) {
	r := FuzzyMatch(false, "anything", "")
	if r.Score != 0 {
		t.Errorf("empty pattern should return zero score, got %d", r.Score)
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	r := FuzzyMatch(false, "hello world", "xyz")
	if r.Score != 0 {
		t.Errorf("expected no match, got score %d", r.Score)
	}
}

func TestFuzzyMatch_SingleChar(t *testing.T) {
	r := FuzzyMatch(false, "/main", "m")
	if r.Score == 0 {
		t.Fatal("expected a match")
	}
	if len(r.Positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(r.Positions))
	}
}

func TestFuzzyMatch_ExactSubstring(t *testing.T) {
	r := FuzzyMatch(false, "~/dotfiles/main", "main")
	if r.Score == 0 {
		t.Fatal("expected a match")
	}
	if len(r.Positions) != 4 {
		t.Fatalf("expected 4 positions, got %d", len(r.Positions))
	}
	// "main" starts at rune index 11
	for i, pos := range r.Positions {
		if pos != 11+i {
			t.Errorf("position[%d] = %d, want %d", i, pos, 11+i)
		}
	}
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	r := FuzzyMatch(false, "FooBar", "foobar")
	if r.Score == 0 {
		t.Fatal("case insensitive match should succeed")
	}
}

func TestFuzzyMatch_CaseSensitive(t *testing.T) {
	r := FuzzyMatch(true, "FooBar", "foobar")
	if r.Score != 0 {
		t.Error("case sensitive match should fail for mismatched case")
	}
}

func TestFuzzyMatch_ContiguousBeatsScattered(t *testing.T) {
	contiguous := FuzzyMatch(false, "foo_bar_baz", "bar")
	scattered := FuzzyMatch(false, "bXaXrXXXXX", "bar")
	if contiguous.Score <= scattered.Score {
		t.Errorf("contiguous match (%d) should score higher than scattered (%d)",
			contiguous.Score, scattered.Score)
	}
}

func TestFuzzyMatch_BoundaryBonus(t *testing.T) {
	atBoundary := FuzzyMatch(false, "foo/bar", "bar")
	midWord := FuzzyMatch(false, "fooXbar", "bar")
	if atBoundary.Score <= midWord.Score {
		t.Errorf("boundary match (%d) should score higher than mid-word (%d)",
			atBoundary.Score, midWord.Score)
	}
}

func TestFuzzyMatch_CamelCaseBonus(t *testing.T) {
	r := FuzzyMatch(false, "getElementById", "gei")
	if r.Score == 0 {
		t.Fatal("expected camelCase match")
	}
}

func TestFuzzyMatch_PositionsAreValid(t *testing.T) {
	input := "~/Code/github.com/jmcampanini/dotfiles/main"
	r := FuzzyMatch(false, input, "main")
	if r.Score == 0 {
		t.Fatal("expected a match")
	}
	runes := []rune(input)
	for i, pos := range r.Positions {
		if pos < 0 || pos >= len(runes) {
			t.Errorf("position[%d] = %d out of bounds [0, %d)", i, pos, len(runes))
		}
	}
}

func TestFuzzyMatch_PositionsAscending(t *testing.T) {
	r := FuzzyMatch(false, "abcdefghij", "aceg")
	if r.Score == 0 {
		t.Fatal("expected a match")
	}
	for i := 1; i < len(r.Positions); i++ {
		if r.Positions[i] <= r.Positions[i-1] {
			t.Errorf("positions not ascending: %v", r.Positions)
			break
		}
	}
}

// Regression: sahilm/fuzzy greedily matched "main" against scattered chars
// in "jmcampanini" instead of finding the contiguous "/main" at the end.
func TestFuzzyMatch_Regression_DotfilesMain(t *testing.T) {
	withMain := FuzzyMatch(false,
		"~/Code/github.com/jmcampanini/dotfiles/main", "main")
	withoutMain := FuzzyMatch(false,
		"~/Code/github.com/jmcampanini/dotfiles", "main")

	if withMain.Score == 0 {
		t.Fatal("dotfiles/main should match 'main'")
	}
	if withMain.Score <= withoutMain.Score {
		t.Errorf("dotfiles/main (%d) should score higher than dotfiles (%d) for 'main'",
			withMain.Score, withoutMain.Score)
	}

	// The match should be at the contiguous "/main", not scattered in "jmcampanini".
	positions := withMain.Positions
	for i := 1; i < len(positions); i++ {
		if positions[i] != positions[i-1]+1 {
			t.Errorf("expected contiguous match, got positions %v", positions)
			break
		}
	}
}

func TestFuzzyMatch_FuzzyStillWorks(t *testing.T) {
	r := FuzzyMatch(false, "abXcdXef", "ace")
	if r.Score == 0 {
		t.Fatal("scattered fuzzy match should still work")
	}
	if len(r.Positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(r.Positions))
	}
}

func TestFuzzyMatch_WideGapBacktrace(t *testing.T) {
	r := FuzzyMatch(false,
		"~/Code/github.com/jmcampanini/cmdk/wt-filter-with-spaces", "dot")
	if r.Score == 0 {
		t.Fatal("expected a match")
	}
	if len(r.Positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(r.Positions))
	}
	for i, pos := range r.Positions {
		if pos == 0 && i > 0 {
			t.Errorf("position[%d] = 0, likely garbage from failed backtrace; positions=%v",
				i, r.Positions)
		}
	}
	for i := 1; i < len(r.Positions); i++ {
		if r.Positions[i] <= r.Positions[i-1] {
			t.Errorf("positions not ascending: %v", r.Positions)
			break
		}
	}
}

func TestCharClassOrdering(t *testing.T) {
	for _, wc := range []charClass{charDelimiter, charLower, charUpper, charLetter, charNumber} {
		if wc <= charNonWord {
			t.Errorf("charClass %d must be > charNonWord (%d) for computeBonus", wc, charNonWord)
		}
	}
}
