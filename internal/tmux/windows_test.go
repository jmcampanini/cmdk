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

func TestParseWindows_MultiSession(t *testing.T) {
	output := "dev\t$2\t1\t@3\tnode\t0\nmain\t$1\t1\t@1\tzsh\t0\nmain\t$1\t2\t@2\tvim\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	want := []struct {
		display     string
		session     string
		sessionID   string
		windowIndex string
		windowID    string
	}{
		{"tmux: dev:1 node", "dev", "$2", "1", "@3"},
		{"tmux: main:1 zsh", "main", "$1", "1", "@1"},
		{"tmux: main:2 vim", "main", "$1", "2", "@2"},
	}

	for i, expected := range want {
		if items[i].Display != expected.display {
			t.Errorf("item[%d].Display = %q, want %q", i, items[i].Display, expected.display)
		}
		if items[i].Data["session"] != expected.session {
			t.Errorf("item[%d].Data[session] = %q, want %q", i, items[i].Data["session"], expected.session)
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

	wantOrder := []string{"tmux: a:1 vim", "tmux: a:3 zsh", "tmux: z:1 fish", "tmux: z:2 bash"}
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
	if items[0].Display != "tmux: work:1 my cool app" {
		t.Errorf("Display = %q, want %q", items[0].Display, "tmux: work:1 my cool app")
	}
	if items[0].Data["session"] != "work" {
		t.Errorf("session = %q, want %q", items[0].Data["session"], "work")
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

func TestParseWindows_EscapedControlCharsInNames(t *testing.T) {
	output := "work" + tmuxEscapedNewline + "notes\t$9\t1\t@7\tmy" + tmuxEscapedTab + "cool" + tmuxEscapedNewline + "app\t0\n"
	items := mustParseWindows(t, output)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	wantDisplay := "tmux: work" + tmuxEscapedNewline + "notes:1 my" + tmuxEscapedTab + "cool" + tmuxEscapedNewline + "app"
	if items[0].Display != wantDisplay {
		t.Errorf("Display = %q, want %q", items[0].Display, wantDisplay)
	}
	if items[0].Data["session"] != "work"+tmuxEscapedNewline+"notes" {
		t.Errorf("session = %q", items[0].Data["session"])
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
	if items[0].Display != "tmux: main:3 fish" {
		t.Errorf("Display = %q, want %q", items[0].Display, "tmux: main:3 fish")
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
	if parseError.Data["source_type"] != "window" {
		t.Errorf("parse error Data[source_type] = %q, want window", parseError.Data["source_type"])
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
	if items[0].Display != "tmux: main:1 zsh" {
		t.Errorf("item[0].Display = %q, want %q", items[0].Display, "tmux: main:1 zsh")
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
	if items[0].Display != "tmux: main:2 vim" {
		t.Errorf("item[0].Display = %q, want bell item first", items[0].Display)
	}
	if items[1].Display != "tmux: main:1 zsh" {
		t.Errorf("item[1].Display = %q, want %q", items[1].Display, "tmux: main:1 zsh")
	}
}
