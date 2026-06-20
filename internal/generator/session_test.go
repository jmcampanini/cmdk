package generator

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/item"
)

func sessionAccumulated() []item.Item {
	s := item.NewItem()
	s.Type = "session"
	s.Display = "tmux: work"
	s.Data["session_id"] = "$2"
	s.Data["session_name"] = "work"
	s.Data["session_kind"] = "external"
	s.Data["session_windows"] = "2"
	return []item.Item{s}
}

func TestSessionGenerator_Ordering(t *testing.T) {
	cfg := config.Config{Actions: []config.Action{
		{Name: "Rename", Cmd: "tmux rename-session", Matches: "session"},
		{Name: "Dir only", Cmd: "echo dir", Matches: "dir"},
	}}
	fetch := func(context.Context, item.Item) ([]item.Item, error) {
		return []item.Item{
			{Type: "window", Display: "window 1 zsh", Action: item.ActionExecute},
			{Type: "window", Display: "window 2 vim", Action: item.ActionExecute},
		}, nil
	}

	items := NewSessionGenerator(fetch)(sessionAccumulated(), Context{PaneID: "%5", Config: cfg})

	got := make([]string, len(items))
	for i, it := range items {
		got[i] = it.Display
	}
	want := []string{"Connect", "Rename", "window 1 zsh", "window 2 vim"}
	if !slices.Equal(got, want) {
		t.Fatalf("displays = %v, want %v", got, want)
	}
	if items[1].Data["session_id"] != "$2" {
		t.Errorf("session action session_id = %q, want $2", items[1].Data["session_id"])
	}
	if items[1].Data["pane_id"] != "%5" {
		t.Errorf("session action pane_id = %q, want %%5", items[1].Data["pane_id"])
	}
}

func TestSessionGenerator_ConnectUsesSessionID(t *testing.T) {
	items := NewSessionGenerator(func(context.Context, item.Item) ([]item.Item, error) {
		return nil, nil
	})(sessionAccumulated(), Context{})

	if len(items) == 0 {
		t.Fatal("got no items")
	}
	connect := items[0]
	if connect.Display != "Connect" {
		t.Fatalf("Display = %q, want Connect", connect.Display)
	}
	if connect.Type != "action" {
		t.Errorf("Type = %q, want action", connect.Type)
	}
	if connect.Source != "builtin" {
		t.Errorf("Source = %q, want builtin", connect.Source)
	}
	if connect.Action != item.ActionExecute {
		t.Errorf("Action = %q, want execute", connect.Action)
	}
	if connect.Cmd != `tmux switch-client -t {{sq .session_id}}` {
		t.Errorf("Cmd = %q", connect.Cmd)
	}
	if connect.Data["session_id"] != "$2" {
		t.Errorf("Data[session_id] = %q, want $2", connect.Data["session_id"])
	}
}

func TestSessionGenerator_MissingSessionIDShowsError(t *testing.T) {
	accumulated := sessionAccumulated()
	delete(accumulated[0].Data, "session_id")

	items := NewSessionGenerator(func(context.Context, item.Item) ([]item.Item, error) {
		return nil, nil
	})(accumulated, Context{})

	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "session error: missing session_id" {
		t.Errorf("Display = %q", items[0].Display)
	}
}

func TestSessionGenerator_WindowFetchErrorAppended(t *testing.T) {
	items := NewSessionGenerator(func(context.Context, item.Item) ([]item.Item, error) {
		return nil, errors.New("tmux failed")
	})(sessionAccumulated(), Context{})

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "Connect" {
		t.Errorf("items[0].Display = %q, want Connect", items[0].Display)
	}
	if items[1].Type != "error" {
		t.Errorf("error item Type = %q, want error", items[1].Type)
	}
	if items[1].Display != "windows error: tmux failed" {
		t.Errorf("error Display = %q", items[1].Display)
	}
}

func TestSessionGenerator_IgnoresNonSession(t *testing.T) {
	items := NewSessionGenerator(func(context.Context, item.Item) ([]item.Item, error) {
		return nil, nil
	})([]item.Item{{Type: "dir"}}, Context{})

	if items != nil {
		t.Errorf("got %v, want nil", items)
	}
}
