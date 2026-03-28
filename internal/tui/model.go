package tui

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	log "charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/theme"
)

type viewMode int

const (
	viewList viewMode = iota
	viewPrompt
	viewPicker
)

type Model struct {
	list             list.Model
	paneID           string
	accumulated      []item.Item
	selected         *item.Item
	registry         *generator.Registry
	ctx              generator.Context
	stackStyle       lipgloss.Style
	filterStyle      lipgloss.Style
	winWidth         int
	winHeight        int
	mode             viewMode
	stageInput       textinput.Model
	stageLabel       string
	pickerList       list.Model
	pickerKey        string
	theme            theme.Theme
	autoSelectSingle bool
	baseItems        []item.Item
	asyncSources     []AsyncSource
	asyncResults     [][]item.Item
	bellToTop        bool
}

const horizontalPadding = 1

func NewModel(items []list.Item, paneID string, accumulated []item.Item, registry *generator.Registry, ctx generator.Context, t theme.Theme, asyncSources []AsyncSource, baseItems []item.Item) Model {
	autoSelect := true
	bellToTop := false
	if ctx.Config != nil {
		autoSelect = ctx.Config.Behavior.ShouldAutoSelectSingle()
		bellToTop = ctx.Config.Behavior.BellToTop
	}

	return Model{
		list:             newFilterList(items, t),
		paneID:           paneID,
		accumulated:      accumulated,
		registry:         registry,
		ctx:              ctx,
		stackStyle:       lipgloss.NewStyle().Foreground(t.Overlay0),
		filterStyle:      lipgloss.NewStyle().Inline(true).Background(t.TextboxBg),
		theme:            t,
		autoSelectSingle: autoSelect,
		baseItems:        baseItems,
		asyncSources:     asyncSources,
		asyncResults:     make([][]item.Item, len(asyncSources)),
		bellToTop:        bellToTop,
	}
}

// newFilterList creates a styled list that starts in filter mode.
// tea.Cmd from the '/' key is intentionally discarded -- it returns
// textinput.Blink which is unused because Cursor.Blink is set to false
// in applyListStyles.
func newFilterList(items []list.Item, t theme.Theme) list.Model {
	l := list.New(items, newItemDelegate(t), 0, 0)
	l.Title = "cmdk"
	l.Filter = multiTermFilter
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	applyListStyles(&l, t)

	l.SetSize(1, 1)
	l, _ = l.Update(tea.KeyPressMsg{Code: rune('/')})
	if l.FilterState() != list.Filtering {
		log.Warn("failed to enter filter mode during list init; falling back to browse mode")
	}
	return l
}

func applyListStyles(l *list.Model, t theme.Theme) {
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, horizontalPadding)

	// Title horizontal padding (1+1=2) must equal TitleBar horizontal padding
	// (left=1, right=1 → 2) because bubbles computes the filter text input
	// width via Title.Render(FilterInput.Prompt). A mismatch causes the filter
	// text input to overflow or truncate.
	l.Styles.Title = lipgloss.NewStyle().
		Background(t.Accent).
		Foreground(t.Base).
		Padding(0, 1)

	// Filter prompt is a pre-rendered ANSI badge followed by a plain space
	// separator. The prompt style overrides DefaultStyles with an unstyled
	// default so the badge's existing ANSI sequences pass through unchanged.
	promptStyle := lipgloss.NewStyle()

	textboxActive := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.TextboxBg)
	textboxDim := lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Background(t.TextboxBg)

	filterStyles := textinput.DefaultStyles(t.IsDark)
	filterStyles.Cursor.Color = t.AccentDim
	filterStyles.Cursor.Blink = false
	filterStyles.Focused.Prompt = promptStyle
	filterStyles.Blurred.Prompt = promptStyle
	filterStyles.Focused.Text = textboxActive
	filterStyles.Blurred.Text = textboxDim
	filterStyles.Focused.Placeholder = textboxDim
	filterStyles.Blurred.Placeholder = textboxDim
	filterStyles.Focused.Suggestion = textboxDim
	filterStyles.Blurred.Suggestion = textboxDim
	l.Styles.Filter = filterStyles
	l.FilterInput.SetStyles(filterStyles)
	badge := lipgloss.NewStyle().
		Background(t.Accent).
		Foreground(t.Base).
		Padding(0, 1).
		Render("cmdk")
	l.FilterInput.Prompt = badge + " "

	l.Styles.DefaultFilterCharacterMatch = lipgloss.NewStyle().Background(t.MatchHighlight)

	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Padding(0, horizontalPadding, 1, horizontalPadding)

	l.Styles.StatusEmpty = lipgloss.NewStyle().Foreground(t.Overlay0)
	l.Styles.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(t.Text)
	l.Styles.StatusBarFilterCount = lipgloss.NewStyle().Foreground(t.Surface2)

	l.Styles.NoItems = lipgloss.NewStyle().
		Foreground(t.Overlay0).
		Padding(0, horizontalPadding)

	l.Styles.PaginationStyle = lipgloss.NewStyle().Padding(0, horizontalPadding)
	l.Styles.HelpStyle = lipgloss.NewStyle().Padding(1, horizontalPadding, 0, horizontalPadding)
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
	if len(m.asyncSources) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, len(m.asyncSources))
	for i, src := range m.asyncSources {
		cmds[i] = fetchSourceCmd(src)
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.winWidth = ws.Width
		m.winHeight = ws.Height
		m.list.SetSize(m.winWidth, max(m.winHeight-m.overheadHeight(), 1))
		if m.mode == viewPicker {
			m.pickerList.SetSize(m.winWidth, max(m.winHeight-m.overheadHeight(), 1))
		}
		return m, nil
	}

	if result, ok := msg.(sourceResultMsg); ok {
		return m.handleSourceResult(result)
	}

	switch m.mode {
	case viewPrompt:
		return m.updatePrompt(msg)
	case viewPicker:
		return m.updatePicker(msg)
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		return m.updateList(key)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if resetWhitespaceFilter(&m.list, msg.String()) {
		return m, nil
	}

	if msg.String() == "enter" {
		sel, ok := resolveListTarget(m.list)
		if ok && sel.Type != "error" && sel.Type != "loading" {
			switch sel.Action {
			case item.ActionExecute:
				m.selected = &sel
				return m, tea.Quit
			case item.ActionStaged:
				m.accumulated = append(slices.Clone(m.accumulated), sel)
				return m.advanceStage(), nil
			case item.ActionNextList:
				return m.handleNextList(sel)
			default:
				log.Error("bug: unknown action type", "action", sel.Action)
				return m, nil
			}
		}
	}

	if msg.String() == "esc" && m.list.FilterState() == list.Unfiltered {
		if len(m.accumulated) > 0 {
			return m.handleBack(), nil
		}
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) updatePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.stageInput, cmd = m.stageInput.Update(msg)
		return m, cmd
	}

	switch key.String() {
	case "enter":
		actionIdx := m.findActionIndex()
		if actionIdx < 0 {
			return m.recoverFromMissingAction(), nil
		}
		stage := m.accumulated[actionIdx].Stages[len(m.accumulated)-actionIdx-1]
		value := m.stageInput.Value()
		return m.pushStageResult(stage.Key, value)

	case "esc":
		return m.stageEsc()

	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.stageInput, cmd = m.stageInput.Update(msg)
	return m, cmd
}

func (m Model) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.pickerList, cmd = m.pickerList.Update(msg)
		return m, cmd
	}

	if resetWhitespaceFilter(&m.pickerList, key.String()) {
		return m, nil
	}

	if key.String() == "enter" {
		sel, ok := resolveListTarget(m.pickerList)
		if ok && sel.Type != "error" {
			return m.pushStageResult(m.pickerKey, sel.Display)
		}
	}

	if key.String() == "esc" && m.pickerList.FilterState() == list.Unfiltered {
		return m.stageEsc()
	}

	var cmd tea.Cmd
	m.pickerList, cmd = m.pickerList.Update(msg)
	return m, cmd
}

// pushStageResult records a stage value and either advances to the next stage or completes.
func (m Model) pushStageResult(key string, value string) (tea.Model, tea.Cmd) {
	actionIdx := m.findActionIndex()
	if actionIdx < 0 {
		return m.recoverFromMissingAction(), nil
	}
	action := m.accumulated[actionIdx]
	stageIdx := len(m.accumulated) - actionIdx - 1

	resultItem := item.NewItem()
	resultItem.Type = "stage-result"
	resultItem.Display = value
	resultItem.Data[key] = value
	m.accumulated = append(slices.Clone(m.accumulated), resultItem)

	if stageIdx+1 >= len(action.Stages) {
		return m.completeStages(), tea.Quit
	}
	return m.advanceStage(), nil
}

// stageEsc handles Esc from prompt or picker stages.
func (m Model) stageEsc() (tea.Model, tea.Cmd) {
	actionIdx := m.findActionIndex()
	if actionIdx < 0 {
		return m.recoverFromMissingAction(), nil
	}
	stageIdx := len(m.accumulated) - actionIdx - 1
	if stageIdx == 0 {
		return m.handleBack(), nil
	}
	popped := m.accumulated[len(m.accumulated)-1]
	m.accumulated = slices.Clone(m.accumulated[:len(m.accumulated)-1])
	m = m.advanceStage()
	// Restore prior input if going back to a prompt stage.
	if m.mode == viewPrompt {
		prevIdx := len(m.accumulated) - actionIdx - 1
		if prevIdx >= 0 && prevIdx < len(m.accumulated[actionIdx].Stages) {
			prevStage := m.accumulated[actionIdx].Stages[prevIdx]
			if prev, ok := popped.Data[prevStage.Key]; ok {
				m.stageInput.SetValue(prev)
			}
		}
	}
	return m, nil
}

// resetWhitespaceFilter resets the filter on navigation keys when the
// effective filter text is empty (blank or whitespace-only).
func resetWhitespaceFilter(l *list.Model, key string) bool {
	if l.FilterState() != list.Filtering || strings.TrimSpace(l.FilterInput.Value()) != "" {
		return false
	}
	switch key {
	case "up", "down", "enter":
		l.ResetFilter()
		return true
	}
	return false
}

func resolveListTarget(l list.Model) (item.Item, bool) {
	switch {
	case l.FilterState() == list.Filtering && len(l.VisibleItems()) == 1:
		sel, ok := l.VisibleItems()[0].(item.Item)
		return sel, ok
	case l.FilterState() != list.Filtering:
		sel, ok := l.SelectedItem().(item.Item)
		return sel, ok
	default:
		return item.Item{}, false
	}
}

// findActionIndex returns the index of the last ActionStaged item in the accumulated stack.
func (m Model) findActionIndex() int {
	for i := len(m.accumulated) - 1; i >= 0; i-- {
		if m.accumulated[i].Action == item.ActionStaged {
			return i
		}
	}
	return -1
}

func (m Model) recoverFromMissingAction() Model {
	log.Error("bug: no ActionStaged item in accumulated stack")
	m.accumulated = nil
	return m.navigateTo(nil)
}

// advanceStage looks at the current stage index and configures the appropriate view.
func (m Model) advanceStage() Model {
	actionIdx := m.findActionIndex()
	if actionIdx < 0 {
		return m.recoverFromMissingAction()
	}
	action := m.accumulated[actionIdx]
	stageIdx := len(m.accumulated) - actionIdx - 1
	if stageIdx >= len(action.Stages) {
		log.Error("bug: stageIdx out of bounds", "stageIdx", stageIdx, "stages", len(action.Stages))
		m.accumulated = slices.Clone(m.accumulated[:actionIdx])
		return m.navigateTo(m.accumulated)
	}

	stage := action.Stages[stageIdx]
	data := execute.FlattenData(m.accumulated)
	if m.paneID != "" {
		data["pane_id"] = m.paneID
	}

	switch stage.Type {
	case item.StagePrompt:
		label, err := execute.RenderCmd(stage.Text, data)
		if err != nil {
			log.Error("failed to render prompt text template", "key", stage.Key, "error", err)
			m.stageLabel = fmt.Sprintf("template error: %s", err)
		} else {
			m.stageLabel = label
		}

		ti := textinput.New()
		if stage.Default != "" {
			def, err := execute.RenderCmd(stage.Default, data)
			if err != nil {
				log.Warn("failed to render stage default template", "key", stage.Key, "error", err)
			} else {
				ti.SetValue(def)
			}
		}
		ti.Focus()
		m.stageInput = ti
		m.mode = viewPrompt

	case item.StagePicker:
		rendered, err := execute.RenderCmd(stage.Source, data)
		if err != nil {
			log.Error("failed to render picker source template", "key", stage.Key, "error", err)
			return m.initPickerWithError(stage.Key, fmt.Sprintf("template error: %s", err))
		}
		var pickerTimeout time.Duration
		if m.ctx.Config != nil {
			pickerTimeout = m.ctx.Config.Timeout.Picker
		}
		items, runErr := runPickerSource(rendered, pickerTimeout)
		if runErr != nil {
			log.Error("picker source command failed", "key", stage.Key, "command", rendered, "error", runErr)
			return m.initPickerWithError(stage.Key, fmt.Sprintf("command error: %s", runErr))
		}
		if len(items) == 0 {
			log.Warn("picker source returned no items", "key", stage.Key, "command", rendered)
			return m.initPickerWithError(stage.Key, "no items returned")
		}
		m = m.initPicker(stage.Key, items)

	default:
		log.Error("bug: unknown stage type", "type", stage.Type)
		m.accumulated = slices.Clone(m.accumulated[:actionIdx])
		return m.navigateTo(m.accumulated)
	}

	return m
}

// runPickerSource executes a shell command and returns one item per output line.
// A zero timeout means no deadline is applied.
// Structured as a standalone function for future conversion to async tea.Cmd.
func runPickerSource(rendered string, timeout time.Duration) ([]item.Item, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "sh", "-c", rendered)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command timed out after %s", timeout)
		}
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
		}
		return nil, fmt.Errorf("command failed: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	var items []item.Item
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		it := item.NewItem()
		it.Type = "pick"
		it.Display = line
		items = append(items, it)
	}
	return items, nil
}

func (m Model) initPicker(key string, items []item.Item) Model {
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}

	pl := newFilterList(listItems, m.theme)
	if m.winHeight > 0 {
		pl.SetSize(m.winWidth, max(m.winHeight-m.overheadHeight(), 1))
	}

	m.pickerList = pl
	m.pickerKey = key
	m.mode = viewPicker
	return m
}

func (m Model) initPickerWithError(key string, errMsg string) Model {
	return m.initPicker(key, []item.Item{{Type: "error", Display: errMsg}})
}

// completeStages extracts the action item as selected and removes it from accumulated,
// leaving stage results in place for FlattenData.
func (m Model) completeStages() Model {
	actionIdx := m.findActionIndex()
	if actionIdx < 0 {
		return m.recoverFromMissingAction()
	}
	action := m.accumulated[actionIdx]
	m.selected = &action
	m.accumulated = slices.Delete(slices.Clone(m.accumulated), actionIdx, actionIdx+1)
	return m
}

func (m Model) handleNextList(sel item.Item) (Model, tea.Cmd) {
	m = m.navigateTo(append(slices.Clone(m.accumulated), sel))

	if m.autoSelectSingle && len(m.list.Items()) == 1 {
		if it, ok := m.list.Items()[0].(item.Item); ok && it.Type == "action" {
			switch it.Action {
			case item.ActionExecute:
				m.selected = &it
				return m, tea.Quit
			case item.ActionStaged:
				m.accumulated = append(slices.Clone(m.accumulated), it)
				return m.advanceStage(), nil
			default:
				log.Error("bug: unknown action type in auto-select", "action", it.Action)
			}
		}
	}
	return m, nil
}

func (m Model) handleBack() Model {
	return m.navigateTo(slices.Clone(m.accumulated[:len(m.accumulated)-1]))
}

func (m Model) handleSourceResult(result sourceResultMsg) (tea.Model, tea.Cmd) {
	srcIdx := slices.IndexFunc(m.asyncSources, func(s AsyncSource) bool {
		return s.Name == result.Name
	})
	if srcIdx < 0 {
		log.Warn("received result for unknown async source", "name", result.Name)
		return m, nil
	}

	src := m.asyncSources[srcIdx]
	if result.Err != nil {
		log.Error("async source failed", "source", src.Name, "error", result.Err)
		errItem := generator.ErrorItem(generator.Source{Name: src.Name, Type: src.Type}, result.Err)
		m.asyncResults[srcIdx] = []item.Item{errItem}
	} else if result.Items != nil {
		m.asyncResults[srcIdx] = result.Items
	} else {
		m.asyncResults[srcIdx] = []item.Item{}
	}

	if len(m.accumulated) == 0 {
		m.rebuildList()
	}
	return m, nil
}

func (m *Model) rebuildList() {
	listItems := m.buildRootItems()
	m.list.SetItems(listItems)
	if state := m.list.FilterState(); state != list.Unfiltered {
		cursorPos := m.list.FilterInput.Position()
		m.list.SetFilterText(m.list.FilterInput.Value())
		if state == list.Filtering {
			m.list.SetFilterState(list.Filtering)
		}
		m.list.FilterInput.SetCursor(cursorPos)
	}
}

func (m Model) buildRootItems() []list.Item {
	var all []item.Item
	all = append(all, m.baseItems...)
	for i, src := range m.asyncSources {
		if m.asyncResults[i] != nil {
			all = append(all, m.asyncResults[i]...)
		} else {
			all = append(all, generator.LoadingItem(generator.Source{Name: src.Name, Type: src.Type}))
		}
	}
	return item.GroupAndOrder(all, m.bellToTop)
}

func (m Model) navigateTo(accumulated []item.Item) Model {
	m.mode = viewList
	var listItems []list.Item

	if len(accumulated) == 0 && len(m.asyncSources) > 0 {
		m.accumulated = accumulated
		listItems = m.buildRootItems()
	} else {
		gen, err := m.registry.Resolve(accumulated)
		if err != nil {
			log.Error("failed to resolve generator", "error", err)
			listItems = []list.Item{item.Item{Type: "error", Display: fmt.Sprintf("navigation error: %s", err)}}
		} else {
			m.accumulated = accumulated
			listItems = item.GroupAndOrder(gen(m.accumulated, m.ctx), m.bellToTop)
		}
	}

	m.list.SetItems(listItems)
	m.list.ResetSelected()
	m.list.ResetFilter()
	if m.winHeight > 0 {
		m.list.SetSize(m.winWidth, max(m.winHeight-m.overheadHeight(), 1))
	}
	return m
}

func (m Model) headerView() string {
	content := m.list.Styles.Title.Render(m.list.Title)
	switch {
	case m.mode == viewList && m.list.FilterState() == list.Filtering:
		content = m.renderFilterHeader(m.list.FilterInput)
	case m.mode == viewPicker && m.pickerList.FilterState() == list.Filtering:
		content = m.renderFilterHeader(m.pickerList.FilterInput)
	}
	return m.list.Styles.TitleBar.Render(content)
}

func (m Model) renderFilterHeader(fi textinput.Model) string {
	filterView := fi.View()
	body, hadPrompt := strings.CutPrefix(filterView, fi.Prompt)
	if hadPrompt {
		return fi.Prompt + m.filterStyle.Render(body)
	}
	return m.filterStyle.Render(filterView)
}

func (m Model) stackView() string {
	if len(m.accumulated) == 0 {
		return ""
	}
	pad := strings.Repeat(" ", horizontalPadding)
	var b strings.Builder
	for _, it := range m.accumulated {
		display := ansi.Truncate(it.Display, max(m.winWidth-2*horizontalPadding, 0), "…")
		b.WriteString(pad + m.stackStyle.Render(display))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

func (m Model) overheadHeight() int {
	h := lipgloss.Height(m.headerView()) + 1
	if len(m.accumulated) > 0 {
		h += len(m.accumulated) + 1
	}
	return h
}

func (m Model) promptView() string {
	pad := strings.Repeat(" ", horizontalPadding)
	return pad + m.stageLabel + "\n" + pad + m.stageInput.View()
}

func (m Model) View() tea.View {
	header := m.headerView()
	stack := m.stackView()

	var body string
	switch m.mode {
	case viewPrompt:
		body = m.promptView()
	case viewPicker:
		body = m.pickerList.View()
	default:
		body = m.list.View()
	}

	return tea.NewView(header + "\n\n" + stack + body)
}
