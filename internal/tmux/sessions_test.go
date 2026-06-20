package tmux

import (
	"slices"
	"strings"
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func mustParseSessions(t *testing.T, output string) []item.Item {
	t.Helper()
	items, err := ParseSessions(output)
	if err != nil {
		t.Fatalf("ParseSessions returned error: %v", err)
	}
	return items
}

func TestParseSessions_MultiSessionSortedByName(t *testing.T) {
	output := "$2\tcmdk\t3\t1\n$1\tdotfiles\t2\t0\n$3\tscratch\t1\t0\n"
	items := mustParseSessions(t, output)

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
	items := mustParseSessions(t, "$7\twork/main\t4\t2\n")
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
	items := mustParseSessions(t, "")
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestParseSessions_PartialMalformedAppendsError(t *testing.T) {
	output := "not enough fields\n$1\tvalid\t2\t0\n$2\tbad-windows\tnope\t0\n$3\tbad-attached\t1\tnope\n$4\t\t1\t0\n\tmissing-id\t1\t0\n"
	items := mustParseSessions(t, output)

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Data["session_name"] != "valid" {
		t.Errorf("session_name = %q, want valid", items[0].Data["session_name"])
	}
	if items[1].Type != "error" {
		t.Errorf("items[1].Type = %q, want error", items[1].Type)
	}
	if items[1].Display != "tmux parse error: 5 unparseable list-sessions rows" {
		t.Errorf("items[1].Display = %q", items[1].Display)
	}
}

func TestParseSessions_AllMalformedReturnsError(t *testing.T) {
	items, err := ParseSessions("not enough fields\n$2\tbad-windows\tnope\t0\n")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "could not parse any tmux list-sessions rows (2 unparseable)" {
		t.Errorf("error = %q", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestParseSessions_PreservesDisplaySafeSessionName(t *testing.T) {
	items := mustParseSessions(t, "$1\tfeature/foo bar\t1\t0\n")
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

func TestParseSessions_ReplacesEscapedControlCharsForDisplay(t *testing.T) {
	items := mustParseSessions(t, `$1	feature\tfoo\nbar	1	0`+"\n")
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	wantName := "feature" + tmuxEscapedTab + "foo" + tmuxEscapedNewline + "bar"
	if items[0].Data["session_name"] != wantName {
		t.Errorf("session_name = %q, want %q", items[0].Data["session_name"], wantName)
	}
	if _, ok := items[0].Data["session"]; ok {
		t.Error("Data[session] should not be set; use session_name")
	}
	if items[0].Display != "tmux: "+wantName {
		t.Errorf("Display = %q, want %q", items[0].Display, "tmux: "+wantName)
	}
}

func TestSessionListFormatEscapesControlChars(t *testing.T) {
	if !strings.Contains(sessionListFormat, `#{s|\\t|`+tmuxEscapedTab+`|`) {
		t.Errorf("sessionListFormat should replace escaped tabs: %q", sessionListFormat)
	}
	if !strings.Contains(sessionListFormat, `#{s|\\n|`+tmuxEscapedNewline+`|`) {
		t.Errorf("sessionListFormat should replace escaped newlines: %q", sessionListFormat)
	}
}
