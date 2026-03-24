package tui

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"github.com/charmbracelet/x/ansi"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/theme"
)

var _ list.ItemDelegate = itemDelegate{}

func testDelegate() itemDelegate {
	return newItemDelegate(theme.Dark())
}

func renderItem(d itemDelegate, items []list.Item, width int, index int) string {
	l := list.New(items, d, width, 10)
	var buf bytes.Buffer
	d.Render(&buf, l, index, items[index])
	return buf.String()
}

func TestDelegate_HeightAndSpacing(t *testing.T) {
	d := testDelegate()
	if d.Height() != 1 {
		t.Errorf("Height() = %d, want 1", d.Height())
	}
	if d.Spacing() != 0 {
		t.Errorf("Spacing() = %d, want 0", d.Spacing())
	}
}

func TestDelegate_RenderContainsIcon(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh"},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconWindow) {
		t.Errorf("expected window icon in output, got %q", out)
	}
	if !strings.Contains(out, "main:1 zsh") {
		t.Errorf("expected display text in output, got %q", out)
	}
}

func TestDelegate_RenderUsesSingleLeftPaddingColumn(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh"},
	}
	out := ansi.Strip(renderItem(d, items, 80, 0))

	wantPrefix := " " + iconWindow + itemGap + "main:1 zsh"
	if !strings.HasPrefix(out, wantPrefix) {
		t.Errorf("expected %q prefix, got %q", wantPrefix, out)
	}
	if strings.HasPrefix(out, "  "+iconWindow) {
		t.Errorf("expected a single left padding column, got %q", out)
	}
}

func TestDelegate_RenderDirIcon(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "dir", Display: "/some/path"},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconDir) {
		t.Errorf("expected dir icon in output, got %q", out)
	}
}

func TestDelegate_RenderCmdIcon(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "cmd", Display: "echo hello"},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconCmd) {
		t.Errorf("expected cmd icon in output, got %q", out)
	}
}

func TestDelegate_UnknownTypeFallsToCmdIcon(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "alien", Display: "some item"},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconCmd) {
		t.Errorf("expected cmd icon fallback for unknown type, got %q", out)
	}
	if !strings.Contains(out, "some item") {
		t.Errorf("expected display text in output, got %q", out)
	}
}

func TestDelegate_ZeroWidth_NoOutput(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "window", Display: "main:1 zsh"},
	}
	out := renderItem(d, items, 0, 0)

	if out != "" {
		t.Errorf("expected no output at zero width, got %q", out)
	}
}

func TestDelegate_NonItemType_NoOutput(t *testing.T) {
	d := testDelegate()
	l := list.New(nil, d, 80, 10)
	var buf bytes.Buffer
	d.Render(&buf, l, 0, stringItem("not an item.Item"))

	if buf.Len() != 0 {
		t.Errorf("expected no output for non-item.Item, got %q", buf.String())
	}
}

type stringItem string

func (s stringItem) FilterValue() string { return string(s) }

func TestDelegate_TruncatesLongText(t *testing.T) {
	d := testDelegate()
	longName := strings.Repeat("a", 200)
	items := []list.Item{
		item.Item{Type: "window", Display: longName},
	}
	out := renderItem(d, items, 40, 0)

	if strings.Contains(out, longName) {
		t.Error("expected text to be truncated at narrow width")
	}
	if !strings.Contains(out, "…") {
		t.Errorf("expected ellipsis in truncated output, got %q", out)
	}
}

func TestDelegate_RenderCustomIconOverridesType(t *testing.T) {
	d := testDelegate()
	customIcon := "\ue709"
	items := []list.Item{
		item.Item{Type: "cmd", Display: "GitHub", Icon: customIcon},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, customIcon) {
		t.Errorf("expected custom icon %q in output, got %q", customIcon, out)
	}
	if strings.Contains(out, iconCmd) {
		t.Errorf("custom icon should replace default cmd icon, got %q", out)
	}
}

func TestDelegate_EmptyIconUsesDefault(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "cmd", Display: "test", Icon: ""},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconCmd) {
		t.Errorf("expected default cmd icon when Icon is empty, got %q", out)
	}
}

func TestDelegate_ConstructorWiresAllTypes(t *testing.T) {
	d := testDelegate()
	for _, typ := range []string{"window", "dir", "cmd"} {
		if _, ok := d.icons[typ]; !ok {
			t.Errorf("missing icon entry for type %q", typ)
		}
	}
}
