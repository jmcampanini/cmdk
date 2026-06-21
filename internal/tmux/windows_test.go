package tmux

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func mustParseWindows(t *testing.T, output string) []item.Item {
	t.Helper()
	items, err := ParseWindows(output)
	if err != nil {
		t.Fatalf("ParseWindows returned error: %v", err)
	}
	return items
}

func mustParseWindowsForSession(t *testing.T, output string, session item.Item) []item.Item {
	t.Helper()
	items, err := ParseWindowsForSession(output, session)
	if err != nil {
		t.Fatalf("ParseWindowsForSession returned error: %v", err)
	}
	return items
}

func TestParseWindows_MultiSession(t *testing.T) {
	output := "dev\t$2\t1\t@3\tnode\t0\nmain\t$1\t1\t@1\tzsh\t0\nmain\t$1\t2\t@2\tvim\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	want := []struct {
		display     string
		sessionName string
		sessionID   string
		windowIndex string
		windowID    string
	}{
		{"tmux:win: 1 node ‹ dev", "dev", "$2", "1", "@3"},
		{"tmux:win: 1 zsh ‹ main", "main", "$1", "1", "@1"},
		{"tmux:win: 2 vim ‹ main", "main", "$1", "2", "@2"},
	}

	for i, expected := range want {
		if items[i].Display != expected.display {
			t.Errorf("item[%d].Display = %q, want %q", i, items[i].Display, expected.display)
		}
		if _, ok := items[i].Data["session"]; ok {
			t.Errorf("item[%d].Data[session] should not be set; use session_name", i)
		}
		if items[i].Data["session_name"] != expected.sessionName {
			t.Errorf("item[%d].Data[session_name] = %q, want %q", i, items[i].Data["session_name"], expected.sessionName)
		}
		if items[i].Data["session_id"] != expected.sessionID {
			t.Errorf("item[%d].Data[session_id] = %q, want %q", i, items[i].Data["session_id"], expected.sessionID)
		}
		if items[i].Data["window_index"] != expected.windowIndex {
			t.Errorf("item[%d].Data[window_index] = %q, want %q", i, items[i].Data["window_index"], expected.windowIndex)
		}
		if items[i].Data["window_id"] != expected.windowID {
			t.Errorf("item[%d].Data[window_id] = %q, want %q", i, items[i].Data["window_id"], expected.windowID)
		}
		if items[i].Action != item.ActionExecute {
			t.Errorf("item[%d].Action = %q, want %q", i, items[i].Action, item.ActionExecute)
		}
		if items[i].Cmd != "tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}" {
			t.Errorf("item[%d].Cmd = %q", i, items[i].Cmd)
		}
	}
}

func TestParseWindows_SortBySessionThenIndex(t *testing.T) {
	output := "z\t$2\t2\t@4\tbash\t0\na\t$1\t3\t@3\tzsh\t0\na\t$1\t1\t@1\tvim\t0\nz\t$2\t1\t@2\tfish\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}

	wantOrder := []string{"tmux:win: 1 vim ‹ a", "tmux:win: 3 zsh ‹ a", "tmux:win: 1 fish ‹ z", "tmux:win: 2 bash ‹ z"}
	for i, display := range wantOrder {
		if items[i].Display != display {
			t.Errorf("item[%d].Display = %q, want %q", i, items[i].Display, display)
		}
	}
}

func TestParseWindows_WindowNameWithSpaces(t *testing.T) {
	output := "work\t$9\t1\t@7\tmy cool app\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "tmux:win: 1 my cool app ‹ work" {
		t.Errorf("Display = %q, want %q", items[0].Display, "tmux:win: 1 my cool app ‹ work")
	}
	if _, ok := items[0].Data["session"]; ok {
		t.Error("Data[session] should not be set; use session_name")
	}
	if items[0].Data["session_name"] != "work" {
		t.Errorf("session_name = %q, want %q", items[0].Data["session_name"], "work")
	}
	if items[0].Data["session_id"] != "$9" {
		t.Errorf("session_id = %q, want %q", items[0].Data["session_id"], "$9")
	}
	if items[0].Data["window_index"] != "1" {
		t.Errorf("window_index = %q, want %q", items[0].Data["window_index"], "1")
	}
	if items[0].Data["window_id"] != "@7" {
		t.Errorf("window_id = %q, want %q", items[0].Data["window_id"], "@7")
	}
}

func TestParseWindows_EscapedSessionAndDisplaySafeWindowNames(t *testing.T) {
	output := "work\\nnotes\t$9\t1\t@7\tmy" + tmuxEscapedTab + "cool" + tmuxEscapedNewline + "app\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	wantDisplay := "tmux:win: 1 my" + tmuxEscapedTab + "cool" + tmuxEscapedNewline + "app ‹ work" + tmuxEscapedNewline + "notes"
	if items[0].Display != wantDisplay {
		t.Errorf("Display = %q, want %q", items[0].Display, wantDisplay)
	}
	if _, ok := items[0].Data["session"]; ok {
		t.Error("Data[session] should not be set; use session_name")
	}
	if items[0].Data["session_name"] != "work"+tmuxEscapedNewline+"notes" {
		t.Errorf("session_name = %q", items[0].Data["session_name"])
	}
}

func TestParseWindows_PreservesLiteralBackslashSequences(t *testing.T) {
	items := mustParseWindows(t, "work\\\\nnotes\t$9\t1\t@7\tmy\\tcool\\napp\t0\n")
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	wantSession := `work\nnotes`
	wantWindow := `my\tcool\napp`
	wantDisplay := "tmux:win: 1 " + wantWindow + " ‹ " + wantSession
	if items[0].Display != wantDisplay {
		t.Errorf("Display = %q, want %q", items[0].Display, wantDisplay)
	}
	if items[0].Data["session_name"] != wantSession {
		t.Errorf("session_name = %q, want %q", items[0].Data["session_name"], wantSession)
	}
}

func TestParseWindows_RawTabInNameReturnsError(t *testing.T) {
	output := "work\t$9\t1\t@7\tmy\tcool\tapp\t0\n"
	items, err := ParseWindows(output)

	if err == nil {
		t.Fatal("expected error")
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
}

func TestParseWindows_PartialMalformedRowsAddsParseErrorItem(t *testing.T) {
	output := "main\t1\t1\t@1\tzsh\t0\nmain\t$1\t2\t2\tvim\t0\nmain\t$1\tnope\t@3\tbad-index\t0\nmain\t$1\t3\t@3\tfish\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "tmux:win: 3 fish ‹ main" {
		t.Errorf("Display = %q, want %q", items[0].Display, "tmux:win: 3 fish ‹ main")
	}
	parseError := items[1]
	if parseError.Type != "error" {
		t.Errorf("parse error Type = %q, want error", parseError.Type)
	}
	if parseError.Source != "tmux" {
		t.Errorf("parse error Source = %q, want tmux", parseError.Source)
	}
	if parseError.Display != "tmux parse error: 3 unparseable list-windows rows" {
		t.Errorf("parse error Display = %q", parseError.Display)
	}
	if parseError.Action != "" {
		t.Errorf("parse error Action = %q, want empty", parseError.Action)
	}
	if parseError.Cmd != "" {
		t.Errorf("parse error Cmd = %q, want empty", parseError.Cmd)
	}
}

func TestParseWindows_AllMalformedRowsReturnsError(t *testing.T) {
	output := "main\t1\t1\t@1\tzsh\t0\nmain\t$1\t2\t2\tvim\t0\n"
	items, err := ParseWindows(output)

	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "could not parse any tmux list-windows rows (2 unparseable)" {
		t.Errorf("error = %q", err.Error())
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
}

func TestParseWindows_EmptyOutput(t *testing.T) {
	items, err := ParseWindows("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestParseWindows_BellFlag(t *testing.T) {
	output := "main\t$1\t1\t@1\tzsh\t1\nmain\t$1\t2\t@2\tvim\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Data["bell"] != "1" {
		t.Errorf("item[0] bell = %q, want \"1\"", items[0].Data["bell"])
	}
	if items[0].Display != "tmux:win: 1 zsh ‹ main" {
		t.Errorf("item[0].Display = %q, want %q", items[0].Display, "tmux:win: 1 zsh ‹ main")
	}
	if _, ok := items[1].Data["bell"]; ok {
		t.Errorf("item[1] should not have bell key, got %q", items[1].Data["bell"])
	}
}

func TestParseWindows_BellSortedFirst(t *testing.T) {
	output := "main\t$1\t1\t@1\tzsh\t0\nmain\t$1\t2\t@2\tvim\t1\nmain\t$1\t3\t@3\tfish\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Display != "tmux:win: 2 vim ‹ main" {
		t.Errorf("item[0].Display = %q, want bell item first", items[0].Display)
	}
	if items[1].Display != "tmux:win: 1 zsh ‹ main" {
		t.Errorf("item[1].Display = %q, want %q", items[1].Display, "tmux:win: 1 zsh ‹ main")
	}
}

func TestParseWindowsForSession(t *testing.T) {
	session := item.NewItem()
	session.Type = "session"
	session.Data["session_id"] = "$3"
	session.Data["session_name"] = "work"
	session.Data["session_kind"] = "external"

	items := mustParseWindowsForSession(t, "2\t@8\tvim\t0\n1\t@7\tzsh\t1\n", session)

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "tmux:win: 1 zsh ‹ work" {
		t.Errorf("items[0].Display = %q, want tmux:win: 1 zsh ‹ work", items[0].Display)
	}
	if items[1].Display != "tmux:win: 2 vim ‹ work" {
		t.Errorf("items[1].Display = %q, want tmux:win: 2 vim ‹ work", items[1].Display)
	}
	if items[0].Data["session_id"] != "$3" {
		t.Errorf("session_id = %q, want $3", items[0].Data["session_id"])
	}
	if _, ok := items[0].Data["session"]; ok {
		t.Error("Data[session] should not be set; use session_name")
	}
	if items[0].Data["session_name"] != "work" {
		t.Errorf("session_name = %q, want work", items[0].Data["session_name"])
	}
	if items[0].Data["window_index"] != "1" {
		t.Errorf("window_index = %q, want 1", items[0].Data["window_index"])
	}
	if items[0].Data["window_id"] != "@7" {
		t.Errorf("window_id = %q, want @7", items[0].Data["window_id"])
	}
	if items[0].Cmd != `tmux switch-client -t {{sq .session_id}}:{{sq .window_id}}` {
		t.Errorf("Cmd = %q", items[0].Cmd)
	}
	if _, ok := items[0].Data["bell"]; ok {
		t.Error("session child windows should not set bell data that would reorder them above Connect")
	}
}

func TestParseWindowsForSession_DisplaySafeControlGlyphsInName(t *testing.T) {
	session := item.NewItem()
	session.Data["session_id"] = "$1"
	session.Data["session_name"] = "work"

	items := mustParseWindowsForSession(t, "1\t@2\tmy"+tmuxEscapedTab+"cool"+tmuxEscapedNewline+"app\t0\n", session)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	want := "tmux:win: 1 my" + tmuxEscapedTab + "cool" + tmuxEscapedNewline + "app ‹ work"
	if items[0].Display != want {
		t.Errorf("Display = %q, want %q", items[0].Display, want)
	}
}

func TestParseWindowsForSession_RawTabInNameReturnsError(t *testing.T) {
	session := item.NewItem()
	session.Data["session_id"] = "$1"

	items, err := ParseWindowsForSession("1\t@2\tmy\tcool\t0\n", session)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
}

func TestParseWindowsForSession_PartialMalformedAppendsError(t *testing.T) {
	session := item.NewItem()
	session.Data["session_id"] = "$1"
	session.Data["session_name"] = "work"

	items := mustParseWindowsForSession(t, "bad\nnope\t@1\tname\t0\n1\tbogus\tinvalid-id\t0\n1\t@2\tvalid\t0\n", session)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "tmux:win: 1 valid ‹ work" {
		t.Errorf("Display = %q, want tmux:win: 1 valid ‹ work", items[0].Display)
	}
	if items[1].Type != "error" {
		t.Errorf("items[1].Type = %q, want error", items[1].Type)
	}
	if items[1].Display != "tmux parse error: 3 unparseable list-windows rows" {
		t.Errorf("items[1].Display = %q", items[1].Display)
	}
}

func TestParseWindowsForSession_AllMalformedReturnsError(t *testing.T) {
	session := item.NewItem()
	session.Data["session_id"] = "$1"

	items, err := ParseWindowsForSession("bad\nnope\t@1\tname\t0\n", session)
	if err == nil {
		t.Fatal("expected error for all malformed rows")
	}
	if err.Error() != "could not parse any tmux list-windows rows (2 unparseable)" {
		t.Errorf("error = %q", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestSessionTargetRequiresSessionID(t *testing.T) {
	session := item.NewItem()
	session.Data["session_name"] = "work"

	_, err := sessionTarget(session)
	if err == nil {
		t.Fatal("expected error for session without session_id")
	}
}
