package tui

import (
	"fmt"
	"image/color"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	log "charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/theme"
)

const (
	iconWindow = "\uf2d0"
	iconDir    = "\uf07c"
	iconCmd    = "\uf120"
	iconBell   = "\U000f009e"
	itemGap    = "  "
)

type iconInfo struct {
	icon  string
	color color.Color
}

type itemDelegate struct {
	icons       map[string]iconInfo
	textFg      color.Color
	selBg       color.Color
	bellColor   color.Color
	filterMatch lipgloss.Style
}

func newItemDelegate(t theme.Theme) itemDelegate {
	return itemDelegate{
		icons: map[string]iconInfo{
			"window":  {iconWindow, t.TypeWindow},
			"dir":     {iconDir, t.TypeDir},
			"cmd":     {iconCmd, t.TypeCmd},
			"action":  {iconCmd, t.TypeCmd},
			"pick":    {iconCmd, t.TypeCmd},
			"error":   {iconCmd, t.TypeCmd},
			"loading": {iconCmd, t.TypeCmd},
		},
		textFg:      t.Text,
		selBg:       t.Surface1,
		bellColor:   t.Bell,
		filterMatch: lipgloss.NewStyle().Background(t.MatchHighlight),
	}
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	it, ok := li.(item.Item)
	if !ok {
		log.Warn("delegate received non-item.Item type", "type", fmt.Sprintf("%T", li))
		return
	}
	if m.Width() <= 0 {
		return
	}

	info, ok := d.icons[it.Type]
	if !ok {
		log.Warn("no icon for item type, using fallback", "type", it.Type)
		info = d.icons["cmd"]
	}
	if it.Icon != "" {
		info = iconInfo{icon: it.Icon, color: info.color}
	}

	filterState := m.FilterState()
	filtering := filterState == list.Filtering
	filtered := filtering || filterState == list.FilterApplied

	var matchedRunes []int
	if filtered {
		matchedRunes = m.MatchesForItem(index)
	}

	hasBell := it.Data["bell"] == "1"
	bellWidth := 0
	if hasBell {
		bellWidth = ansi.StringWidth(iconBell) + ansi.StringWidth(itemGap)
	}

	leftPad := strings.Repeat(" ", horizontalPadding)
	iconWidth := ansi.StringWidth(info.icon)
	availWidth := max(m.Width()-ansi.StringWidth(leftPad)-iconWidth-ansi.StringWidth(itemGap)-bellWidth, 0)
	display := ansi.Truncate(it.Display, availWidth, "…")

	s := lipgloss.NewStyle().Inline(true)
	selected := index == m.Index() && !filtering

	var content string
	if selected {
		bgOnly := s.Background(d.selBg)
		iconStr := s.Foreground(info.color).Background(d.selBg).Render(info.icon)
		textStr := d.renderText(display, matchedRunes, s.Foreground(d.textFg).Background(d.selBg))
		content = bgOnly.Render(leftPad) + iconStr + bgOnly.Render(itemGap) + textStr
	} else {
		iconStr := s.Foreground(info.color).Render(info.icon)
		textStr := d.renderText(display, matchedRunes, s.Foreground(d.textFg))
		content = leftPad + iconStr + itemGap + textStr
	}

	remaining := m.Width() - ansi.StringWidth(content)

	if hasBell {
		bellStr := d.styledBell(s, selected)
		if gap := remaining - ansi.StringWidth(iconBell); gap > 0 {
			content += d.renderFill(s, selected, gap) + bellStr
		} else {
			content += bellStr
		}
	} else if selected && remaining > 0 {
		content += d.renderFill(s, selected, remaining)
	}

	_, _ = fmt.Fprint(w, content)
}

func (d itemDelegate) styledBell(s lipgloss.Style, selected bool) string {
	bellStyle := s.Foreground(d.bellColor)
	if selected {
		bellStyle = bellStyle.Background(d.selBg)
	}
	return bellStyle.Render(iconBell)
}

func (d itemDelegate) renderFill(s lipgloss.Style, selected bool, width int) string {
	spaces := strings.Repeat(" ", width)
	if selected {
		return s.Background(d.selBg).Render(spaces)
	}
	return spaces
}

func (d itemDelegate) renderText(display string, matchedRunes []int, style lipgloss.Style) string {
	if len(matchedRunes) > 0 {
		return lipgloss.StyleRunes(display, matchedRunes, style.Inherit(d.filterMatch), style)
	}
	return style.Render(display)
}
