package tui

import (
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/theme"
)

const (
	iconWindow = "\uf2d0"
	iconDir    = "\uf07c"
	iconCmd    = "\uf120"
	leftPad    = "  "
)

type iconInfo struct {
	icon  string
	color color.Color
}

type itemDelegate struct {
	icons       map[string]iconInfo
	dimIcon     color.Color
	textFg      color.Color
	dimTextFg   color.Color
	selBg       color.Color
	filterMatch lipgloss.Style
}

func newItemDelegate(t theme.Theme) itemDelegate {
	return itemDelegate{
		icons: map[string]iconInfo{
			"window": {iconWindow, t.TypeWindow},
			"dir":    {iconDir, t.TypeDir},
			"cmd":    {iconCmd, t.TypeCmd},
		},
		dimIcon:     t.Surface2,
		textFg:      t.Text,
		dimTextFg:   t.Overlay0,
		selBg:       t.Surface1,
		filterMatch: lipgloss.NewStyle().Underline(true),
	}
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	it, ok := li.(item.Item)
	if !ok {
		return
	}
	if m.Width() <= 0 {
		return
	}

	info, ok := d.icons[it.Type]
	if !ok {
		slog.Warn("no icon for item type, using fallback", "type", it.Type)
		info = d.icons["cmd"]
	}

	filterState := m.FilterState()
	filtering := filterState == list.Filtering
	filtered := filtering || filterState == list.FilterApplied

	var matchedRunes []int
	if filtered {
		matchedRunes = m.MatchesForItem(index)
	}

	iconWidth := ansi.StringWidth(info.icon)
	availWidth := max(m.Width()-ansi.StringWidth(leftPad)-iconWidth-1, 0)
	display := ansi.Truncate(it.Display, availWidth, "…")

	s := lipgloss.NewStyle().Inline(true)

	var line string
	switch {
	case filtering && m.FilterValue() == "":
		iconStr := s.Foreground(d.dimIcon).Render(info.icon)
		textStr := s.Foreground(d.dimTextFg).Render(display)
		line = leftPad + iconStr + " " + textStr

	case index == m.Index() && !filtering:
		iconStr := s.Foreground(info.color).Background(d.selBg).Render(info.icon)
		textStr := d.renderText(display, matchedRunes, s.Foreground(d.textFg).Background(d.selBg))

		bgOnly := s.Background(d.selBg)
		content := bgOnly.Render(leftPad) + iconStr + bgOnly.Render(" ") + textStr
		if remaining := m.Width() - ansi.StringWidth(content); remaining > 0 {
			content += bgOnly.Render(strings.Repeat(" ", remaining))
		}
		line = content

	default:
		iconStr := s.Foreground(info.color).Render(info.icon)
		textStr := d.renderText(display, matchedRunes, s.Foreground(d.textFg))
		line = leftPad + iconStr + " " + textStr
	}

	fmt.Fprint(w, line)
}

func (d itemDelegate) renderText(display string, matchedRunes []int, style lipgloss.Style) string {
	if len(matchedRunes) > 0 {
		return lipgloss.StyleRunes(display, matchedRunes, style.Inherit(d.filterMatch), style)
	}
	return style.Render(display)
}
