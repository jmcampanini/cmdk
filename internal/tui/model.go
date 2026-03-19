package tui

import (
	"log/slog"
	"slices"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/theme"
)

type Model struct {
	list        list.Model
	paneID      string
	accumulated []item.Item
	selected    *item.Item
	registry    *generator.Registry
	ctx         generator.Context
}

func NewModel(items []list.Item, paneID string, accumulated []item.Item, registry *generator.Registry, ctx generator.Context, t theme.Theme, startFiltered bool) Model {
	l := list.New(items, newItemDelegate(t), 0, 0)
	l.Title = "cmdk"
	l.Filter = pathAwareFilter
	applyListStyles(&l, t)

	if startFiltered {
		l.SetFilterState(list.Filtering)
	}

	return Model{
		list:        l,
		paneID:      paneID,
		accumulated: accumulated,
		registry:    registry,
		ctx:         ctx,
	}
}

func applyListStyles(l *list.Model, t theme.Theme) {
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 2)

	l.Styles.Title = lipgloss.NewStyle().
		Background(t.Accent).
		Foreground(t.Base).
		Padding(0, 1)

	prompt := lipgloss.NewStyle().Foreground(t.Accent)
	filterStyles := textinput.DefaultStyles(t.IsDark)
	filterStyles.Cursor.Color = t.AccentDim
	filterStyles.Blurred.Prompt = prompt
	filterStyles.Focused.Prompt = prompt
	l.Styles.Filter = filterStyles
	l.FilterInput.SetStyles(filterStyles)

	l.Styles.DefaultFilterCharacterMatch = lipgloss.NewStyle().Underline(true)

	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Padding(0, 0, 1, 2)

	l.Styles.StatusEmpty = lipgloss.NewStyle().Foreground(t.Overlay0)
	l.Styles.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(t.Text)
	l.Styles.StatusBarFilterCount = lipgloss.NewStyle().Foreground(t.Surface2)

	l.Styles.NoItems = lipgloss.NewStyle().Foreground(t.Overlay0)

	l.Styles.PaginationStyle = lipgloss.NewStyle().PaddingLeft(2)
	l.Styles.HelpStyle = lipgloss.NewStyle().Padding(1, 0, 0, 2)
	l.Styles.ArabicPagination = lipgloss.NewStyle().Foreground(t.Overlay0)

	dot := lipgloss.NewStyle().SetString("\u2022")
	activeDot := dot.Foreground(t.Overlay1)
	inactiveDot := dot.Foreground(t.Surface2)
	l.Styles.ActivePaginationDot = activeDot
	l.Styles.InactivePaginationDot = inactiveDot
	l.Paginator.ActiveDot = activeDot.String()
	l.Paginator.InactiveDot = inactiveDot.String()

	l.Styles.DividerDot = lipgloss.NewStyle().
		Foreground(t.Surface2).
		SetString(" \u2022 ")
}

func (m Model) Accumulated() []item.Item {
	return m.accumulated
}

func (m Model) Selected() *item.Item {
	return m.selected
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "enter" && m.list.FilterState() != list.Filtering {
			if sel, ok := m.list.SelectedItem().(item.Item); ok {
				switch sel.Action {
				case item.ActionExecute:
					m.selected = &sel
					return m, tea.Quit
				case item.ActionNextList:
					return m.handleNextList(sel), nil
				}
			}
		}
		if msg.String() == "esc" && m.list.FilterState() == list.Unfiltered {
			if len(m.accumulated) > 0 {
				return m.handleBack(), nil
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleNextList(sel item.Item) Model {
	return m.navigateTo(append(slices.Clone(m.accumulated), sel))
}

func (m Model) handleBack() Model {
	return m.navigateTo(slices.Clone(m.accumulated[:len(m.accumulated)-1]))
}

func (m Model) navigateTo(accumulated []item.Item) Model {
	gen, err := m.registry.Resolve(accumulated)
	if err != nil {
		slog.Error("failed to resolve generator", "error", err)
		return m
	}

	m.accumulated = accumulated
	listItems := item.GroupAndOrder(gen(m.accumulated, m.ctx))
	m.list.SetItems(listItems)
	m.list.ResetSelected()
	m.list.ResetFilter()
	return m
}

func (m Model) View() tea.View {
	return tea.NewView(m.list.View())
}
