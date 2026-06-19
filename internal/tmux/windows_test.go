package tmux

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestParseWindows_MultiSession(t *testing.T) {
	output := "dev:1 node\t0\nmain:1 zsh\t0\nmain:2 vim\t0\n"
	items := ParseWindows(output)

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	want := []struct {
		display string
		session string
		index   string
	}{
		{"tmux: dev:1 node", "dev", "1"},
		{"tmux: main:1 zsh", "main", "1"},
		{"tmux: main:2 vim", "main", "2"},
	}

	for i, w := range want {
		if items[i].Display != w.display {
			t.Errorf("item[%d].Display = %q, want %q", i, items[i].Display, w.display)
		}
		if items[i].Data["session"] != w.session {
			t.Errorf("item[%d].Data[session] = %q, want %q", i, items[i].Data["session"], w.session)
		}
		if items[i].Data["window_index"] != w.index {
			t.Errorf("item[%d].Data[window_index] = %q, want %q", i, items[i].Data["window_index"], w.index)
		}
		if items[i].Action != item.ActionExecute {
			t.Errorf("item[%d].Action = %q, want %q", i, items[i].Action, item.ActionExecute)
		}
		if items[i].Cmd != "tmux switch-client -t '{{.session}}:{{.window_index}}'" {
			t.Errorf("item[%d].Cmd = %q", i, items[i].Cmd)
		}
	}
}

func TestParseWindows_SortBySessionThenIndex(t *testing.T) {
	output := "z:2 bash\t0\na:3 zsh\t0\na:1 vim\t0\nz:1 fish\t0\n"
	items := ParseWindows(output)

	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}

	wantOrder := []string{"tmux: a:1 vim", "tmux: a:3 zsh", "tmux: z:1 fish", "tmux: z:2 bash"}
	for i, w := range wantOrder {
		if items[i].Display != w {
			t.Errorf("item[%d].Display = %q, want %q", i, items[i].Display, w)
		}
	}
}

func TestParseWindows_WindowNameWithSpaces(t *testing.T) {
	output := "work:1 my cool app\t0\n"
	items := ParseWindows(output)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "tmux: work:1 my cool app" {
		t.Errorf("Display = %q, want %q", items[0].Display, "tmux: work:1 my cool app")
	}
	if items[0].Data["session"] != "work" {
		t.Errorf("session = %q, want %q", items[0].Data["session"], "work")
	}
	if items[0].Data["window_index"] != "1" {
		t.Errorf("window_index = %q, want %q", items[0].Data["window_index"], "1")
	}
}

func TestParseWindows_BellFlag(t *testing.T) {
	output := "main:1 zsh\t1\nmain:2 vim\t0\n"
	items := ParseWindows(output)

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
	output := "main:1 zsh\t0\nmain:2 vim\t1\nmain:3 fish\t0\n"
	items := ParseWindows(output)

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

func TestParseWindows_EmptyOutput(t *testing.T) {
	items := ParseWindows("")
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}

func TestParseWindowsForSession(t *testing.T) {
	session := item.NewItem()
	session.Type = "session"
	session.Data["session_id"] = "$3"
	session.Data["session_name"] = "work"
	session.Data["session_kind"] = "external"

	items := ParseWindowsForSession("2\tvim\t0\n1\tzsh\t1\n", session)

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "window 1 zsh" {
		t.Errorf("items[0].Display = %q, want window 1 zsh", items[0].Display)
	}
	if items[1].Display != "window 2 vim" {
		t.Errorf("items[1].Display = %q, want window 2 vim", items[1].Display)
	}
	if items[0].Data["session_id"] != "$3" {
		t.Errorf("session_id = %q, want $3", items[0].Data["session_id"])
	}
	if items[0].Data["session"] != "work" {
		t.Errorf("session = %q, want work", items[0].Data["session"])
	}
	if items[0].Data["window_index"] != "1" {
		t.Errorf("window_index = %q, want 1", items[0].Data["window_index"])
	}
	if items[0].Cmd != `tmux switch-client -t {{sq (printf "%s:%s" .session_id .window_index)}}` {
		t.Errorf("Cmd = %q", items[0].Cmd)
	}
	if _, ok := items[0].Data["bell"]; ok {
		t.Error("session child windows should not set bell data that would reorder them above Connect")
	}
}

func TestParseWindowsForSession_SkipsMalformed(t *testing.T) {
	session := item.NewItem()
	session.Data["session_id"] = "$1"

	items := ParseWindowsForSession("bad\nnope\tname\t0\n1\tvalid\t0\n", session)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "window 1 valid" {
		t.Errorf("Display = %q, want window 1 valid", items[0].Display)
	}
}
