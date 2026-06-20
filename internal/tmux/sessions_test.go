package tmux

import (
	"slices"
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestParseSessions_MultiSessionSortedByName(t *testing.T) {
	output := "$2\tcmdk\t3\t1\n$1\tdotfiles\t2\t0\n$3\tscratch\t1\t0\n"
	items := ParseSessions(output)

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	wantDisplays := []string{"tmux: cmdk", "tmux: dotfiles", "tmux: scratch"}
	gotDisplays := make([]string, len(items))
	for i, it := range items {
		gotDisplays[i] = it.Display
	}
	if !slices.Equal(gotDisplays, wantDisplays) {
		t.Fatalf("displays = %v, want %v", gotDisplays, wantDisplays)
	}
	if items[0].Data["session_id"] != "$2" {
		t.Errorf("session_id = %q, want $2", items[0].Data["session_id"])
	}
	if items[0].Data["session_name"] != "cmdk" {
		t.Errorf("session_name = %q, want cmdk", items[0].Data["session_name"])
	}
}

func TestParseSessions_ItemFields(t *testing.T) {
	items := ParseSessions("$7\twork/main\t4\t2\n")
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	it := items[0]
	if it.Type != "session" {
		t.Errorf("Type = %q, want session", it.Type)
	}
	if it.Source != "tmux" {
		t.Errorf("Source = %q, want tmux", it.Source)
	}
	if it.Display != "tmux: work/main" {
		t.Errorf("Display = %q, want tmux: work/main", it.Display)
	}
	if it.Action != item.ActionNextList {
		t.Errorf("Action = %q, want next-list", it.Action)
	}

	wantData := map[string]string{
		"session_attached": "2",
		"session_display":  "tmux: work/main",
		"session_id":       "$7",
		"session_kind":     "external",
		"session_name":     "work/main",
		"session_windows":  "4",
	}
	for k, want := range wantData {
		if it.Data[k] != want {
			t.Errorf("Data[%s] = %q, want %q", k, it.Data[k], want)
		}
	}
}

func TestParseSessions_EmptyOutput(t *testing.T) {
	items := ParseSessions("")
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestParseSessions_SkipsMalformedLines(t *testing.T) {
	output := "not enough fields\n$1\tvalid\t2\t0\n$2\tbad-windows\tnope\t0\n$3\tbad-attached\t1\tnope\n$4\t\t1\t0\n\tmissing-id\t1\t0\n"
	items := ParseSessions(output)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Data["session_name"] != "valid" {
		t.Errorf("session_name = %q, want valid", items[0].Data["session_name"])
	}
}

func TestParseSessions_PreservesSessionName(t *testing.T) {
	items := ParseSessions("$1\tfeature/foo bar\t1\t0\n")
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Data["session_name"] != "feature/foo bar" {
		t.Errorf("session_name = %q, want feature/foo bar", items[0].Data["session_name"])
	}
	if items[0].Display != "tmux: feature/foo bar" {
		t.Errorf("Display = %q, want tmux: feature/foo bar", items[0].Display)
	}
}
