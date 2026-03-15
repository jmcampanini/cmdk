package tmux

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestParseWindows_MultiSession(t *testing.T) {
	output := "dev:1 node\nmain:1 zsh\nmain:2 vim\n"
	items := ParseWindows(output)

	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	want := []struct {
		display string
		session string
		index   string
	}{
		{"dev:1 node", "dev", "1"},
		{"main:1 zsh", "main", "1"},
		{"main:2 vim", "main", "2"},
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
	output := "z:2 bash\na:3 zsh\na:1 vim\nz:1 fish\n"
	items := ParseWindows(output)

	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}

	wantOrder := []string{"a:1 vim", "a:3 zsh", "z:1 fish", "z:2 bash"}
	for i, w := range wantOrder {
		if items[i].Display != w {
			t.Errorf("item[%d].Display = %q, want %q", i, items[i].Display, w)
		}
	}
}

func TestParseWindows_WindowNameWithSpaces(t *testing.T) {
	output := "work:1 my cool app\n"
	items := ParseWindows(output)

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "work:1 my cool app" {
		t.Errorf("Display = %q, want %q", items[0].Display, "work:1 my cool app")
	}
	if items[0].Data["session"] != "work" {
		t.Errorf("session = %q, want %q", items[0].Data["session"], "work")
	}
	if items[0].Data["window_index"] != "1" {
		t.Errorf("window_index = %q, want %q", items[0].Data["window_index"], "1")
	}
}

func TestParseWindows_EmptyOutput(t *testing.T) {
	items := ParseWindows("")
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}
