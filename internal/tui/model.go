package tui

import (
	"log/slog"
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/list"

	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
)

type Model struct {
	list        list.Model
	paneID      string
	accumulated []item.Item
	selected    *item.Item
	registry    *generator.Registry
	ctx         generator.Context
}

func NewModel(items []list.Item, paneID string, accumulated []item.Item, registry *generator.Registry, ctx generator.Context) Model {
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "cmdk"
	return Model{
		list:        l,
		paneID:      paneID,
		accumulated: accumulated,
		registry:    registry,
		ctx:         ctx,
	}
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
