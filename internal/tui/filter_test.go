package tui

import (
	"os"
	"testing"
)

func TestRemapAbsToDisplay(t *testing.T) {
	// homeLen=14 simulates /Users/testuser (14 chars)
	// Absolute: /Users/testuser/Code/proj
	// Display:  ~/Code/proj
	// Index mapping: abs[14:] == display[1:]

	tests := []struct {
		name    string
		indexes []int
		homeLen int
		want    []int
	}{
		{
			name:    "shared suffix indexes",
			indexes: []int{15, 16, 17, 18},
			homeLen: 14,
			want:    []int{2, 3, 4, 5},
		},
		{
			name:    "home prefix indexes map to tilde",
			indexes: []int{0, 1, 2, 3, 4, 5},
			homeLen: 14,
			want:    []int{0},
		},
		{
			name:    "mixed home prefix and suffix",
			indexes: []int{0, 5, 14, 15, 16},
			homeLen: 14,
			want:    []int{0, 1, 2, 3},
		},
		{
			name:    "exact boundary index",
			indexes: []int{14},
			homeLen: 14,
			want:    []int{1},
		},
		{
			name:    "empty indexes",
			indexes: []int{},
			homeLen: 14,
			want:    []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remapAbsToDisplay(tt.indexes, tt.homeLen)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPathAwareFilter_TildeFormMatch(t *testing.T) {
	targets := []string{"~/Code/project", "/tmp/scratch", "~/Documents"}
	ranks := pathAwareFilter("Code", targets)

	if len(ranks) != 1 {
		t.Fatalf("got %d ranks, want 1", len(ranks))
	}
	if ranks[0].Index != 0 {
		t.Errorf("matched index %d, want 0", ranks[0].Index)
	}
}

func TestPathAwareFilter_AbsolutePathMatch(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	if len(home) < 3 {
		t.Skipf("home dir too short for test: %q", home)
	}

	term := home[:3]
	targets := []string{"~/Code/project", "/tmp/scratch"}
	ranks := pathAwareFilter(term, targets)

	found := false
	for _, r := range ranks {
		if r.Index == 0 {
			found = true
			for _, idx := range r.MatchedIndexes {
				if idx >= len(targets[0]) {
					t.Errorf("remapped index %d exceeds display length %d", idx, len(targets[0]))
				}
			}
		}
	}
	if !found {
		t.Errorf("expected absolute path match on item 0, got ranks: %v", ranks)
	}
}

func TestPathAwareFilter_NonTildeTargetsUnchanged(t *testing.T) {
	targets := []string{"/tmp/scratch", "/var/log"}
	ranks := pathAwareFilter("tmp", targets)

	if len(ranks) != 1 {
		t.Fatalf("got %d ranks, want 1", len(ranks))
	}
	if ranks[0].Index != 0 {
		t.Errorf("matched index %d, want 0", ranks[0].Index)
	}
}
