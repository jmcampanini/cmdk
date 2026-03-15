package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/list"

	"github.com/jmcampanini/cmdk/internal/item"
)

type Model struct {
	list        list.Model
	paneID      string
	accumulated []item.Item
	selected    *item.Item
}

func NewModel(items []list.Item, paneID string, accumulated []item.Item) Model {
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "cmdk"
	return Model{list: l, paneID: paneID, accumulated: accumulated}
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
					return m, nil
				}
			}
		}
		if msg.String() == "esc" && m.list.FilterState() == list.Unfiltered {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	return tea.NewView(m.list.View())
}
