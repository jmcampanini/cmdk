package tui

import (
	"bytes"
	"image/color"
	"reflect"
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

func assertIconInfo(t *testing.T, got iconInfo, wantIcon string, wantColor color.Color) {
	t.Helper()
	if got.icon != wantIcon {
		t.Errorf("icon = %q, want %q", got.icon, wantIcon)
	}
	if !reflect.DeepEqual(got.color, wantColor) {
		t.Errorf("color = %v, want %v", got.color, wantColor)
	}
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

func TestDelegate_RenderActionIcon(t *testing.T) {
	dark := theme.Dark()
	d := newItemDelegate(dark)
	it := item.Item{Type: "action", Display: "echo hello"}
	items := []list.Item{it}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconAction) {
		t.Errorf("expected action icon in output, got %q", out)
	}
	assertIconInfo(t, d.iconInfoForItem(it), iconAction, dark.TypeAction)
}

func TestDelegate_UnknownTypeUsesDefaultPresentation(t *testing.T) {
	dark := theme.Dark()
	d := newItemDelegate(dark)
	it := item.Item{Type: "alien", Display: "some item"}
	items := []list.Item{it}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconUnknown) {
		t.Errorf("expected unknown fallback icon, got %q", out)
	}
	if !strings.Contains(out, "some item") {
		t.Errorf("expected display text in output, got %q", out)
	}
	assertIconInfo(t, d.iconInfoForItem(it), iconUnknown, dark.TypeUnknown)
}

func TestDelegate_PickerUsesDefaultPresentation(t *testing.T) {
	dark := theme.Dark()
	d := newItemDelegate(dark)
	it := item.Item{Type: "pick", Display: "alpha"}

	assertIconInfo(t, d.iconInfoForItem(it), iconUnknown, dark.TypeUnknown)
}

func TestDelegate_ErrorUsesErrorPresentation(t *testing.T) {
	dark := theme.Dark()
	d := newItemDelegate(dark)
	it := item.Item{Type: "error", Display: "zoxide error: command not found", Source: "zoxide", Data: map[string]string{"source_type": "dir"}}

	assertIconInfo(t, d.iconInfoForItem(it), iconError, dark.Error)
}

func TestDelegate_LoadingUsesSourceDerivedColor(t *testing.T) {
	dark := theme.Dark()
	d := newItemDelegate(dark)

	assertIconInfo(t, d.iconInfoForItem(item.Item{Type: "loading", Data: map[string]string{"source_type": "window"}}), iconLoading, dark.TypeWindow)
	assertIconInfo(t, d.iconInfoForItem(item.Item{Type: "loading", Data: map[string]string{"source_type": "dir"}}), iconLoading, dark.TypeDir)
	assertIconInfo(t, d.iconInfoForItem(item.Item{Type: "loading", Data: map[string]string{"source_type": "action"}}), iconLoading, dark.TypeAction)
	assertIconInfo(t, d.iconInfoForItem(item.Item{Type: "loading", Data: map[string]string{"source_type": "alien"}}), iconLoading, dark.TypeUnknown)
	assertIconInfo(t, d.iconInfoForItem(item.Item{Type: "loading"}), iconLoading, dark.TypeUnknown)
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
		item.Item{Type: "action", Display: "GitHub", Icon: customIcon},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, customIcon) {
		t.Errorf("expected custom icon %q in output, got %q", customIcon, out)
	}
	if strings.Contains(out, iconAction) {
		t.Errorf("custom icon should replace default action icon, got %q", out)
	}
}

func TestDelegate_EmptyIconUsesDefault(t *testing.T) {
	d := testDelegate()
	items := []list.Item{
		item.Item{Type: "action", Display: "test", Icon: ""},
	}
	out := renderItem(d, items, 80, 0)

	if !strings.Contains(out, iconAction) {
		t.Errorf("expected default action icon when Icon is empty, got %q", out)
	}
}

func TestDelegate_ConstructorWiresAllTypes(t *testing.T) {
	d := testDelegate()
	for _, typ := range []string{"window", "dir", "action", "pick", "error", "loading"} {
		if _, ok := d.icons[typ]; !ok {
			t.Errorf("missing icon entry for type %q", typ)
		}
	}
	if _, ok := d.icons["cmd"]; ok {
		t.Error("cmd should not have a dedicated icon entry")
	}
}
