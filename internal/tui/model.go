package tui

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	log "charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
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
	viewErrorDetails
)

type Model struct {
	list              list.Model
	paneID            string
	accumulated       []item.Item
	selected          *item.Item
	launch            *execute.Launch
	registry          *generator.Registry
	ctx               generator.Context
	stackStyle        lipgloss.Style
	filterStyle       lipgloss.Style
	winWidth          int
	winHeight         int
	mode              viewMode
	stageInput        textinput.Model
	stageLabel        string
	stageError        string
	errorStyle        lipgloss.Style
	pickerList        list.Model
	pickerKey         string
	errorReturnMode   viewMode
	errorDetailItem   item.Item
	errorDetailScroll int
	theme             theme.Theme
	autoSelectSingle  bool
	baseItems         []item.Item
	asyncSources      []AsyncSource
	asyncResults      [][]item.Item
	bellToTop         bool
	wrapList          bool
	startInFilter     bool
	autoDetectTheme   bool
	inline            bool
}

const horizontalPadding = 1

func NewModel(items []list.Item, paneID string, accumulated []item.Item, registry *generator.Registry, ctx generator.Context, t theme.Theme, asyncSources []AsyncSource, baseItems []item.Item) Model {
	beh := ctx.Config.Behavior

	m := Model{
		list:             newFilterList(items, t, beh.WrapList, beh.StartInFilter),
		paneID:           paneID,
		accumulated:      accumulated,
		registry:         registry,
		ctx:              ctx,
		autoSelectSingle: beh.AutoSelectSingle,
		baseItems:        baseItems,
		asyncSources:     asyncSources,
		asyncResults:     make([][]item.Item, len(asyncSources)),
		bellToTop:        beh.BellToTop,
		wrapList:         beh.WrapList,
		startInFilter:    beh.StartInFilter,
		inline:           beh.InlineActions,
	}
	m.setThemeStyles(t)
	if beh.InlineActions && baseItems != nil {
		m.list.SetItems(m.buildRootItems())
	}
	return m
}

func (m Model) WithAutoThemeDetection() Model {
	m.autoDetectTheme = true
	return m
}

func newFilterList(items []list.Item, t theme.Theme, wrapList bool, startInFilter bool) list.Model {
	l := list.New(items, newItemDelegate(t), 0, 0)
	l.Title = "cmdk"
	l.Filter = multiTermFilter
	l.InfiniteScrolling = wrapList
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	applyListStyles(&l, t)

	l.SetSize(1, 1)
	if startInFilter {
		l = enterFilterMode(l)
	}
	return l
}

// enterFilterMode sends the '/' key to activate the list's built-in filter.
// The returned tea.Cmd (textinput.Blink) is intentionally discarded because
// Cursor.Blink is set to false in applyListStyles.
func enterFilterMode(l list.Model) list.Model {
	l, _ = l.Update(tea.KeyPressMsg{Code: rune('/')})
	if l.FilterState() != list.Filtering {
		log.Warn("failed to enter filter mode; the list likely has no items", "items", len(l.Items()))
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
		Background(t.Tokens.Accent).
		Foreground(t.Tokens.AccentText).
		Padding(0, 1)

	// Filter prompt is a pre-rendered ANSI badge followed by a plain space
	// separator. The prompt style overrides DefaultStyles with an unstyled
	// default so the badge's existing ANSI sequences pass through unchanged.
	promptStyle := lipgloss.NewStyle()

	textboxActive := lipgloss.NewStyle().
		Foreground(t.Tokens.Text).
		Background(t.Tokens.InputBg)
	textboxDim := lipgloss.NewStyle().
		Foreground(t.Tokens.Muted).
		Background(t.Tokens.InputBg)

	filterStyles := textinput.DefaultStyles(t.IsDark)
	filterStyles.Cursor.Color = t.Tokens.Cursor
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
		Background(t.Tokens.Accent).
		Foreground(t.Tokens.AccentText).
		Padding(0, 1).
		Render("cmdk")
	l.FilterInput.Prompt = badge + " "

	l.Styles.DefaultFilterCharacterMatch = lipgloss.NewStyle().Background(t.Tokens.MatchBg)

	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(t.Tokens.Muted).
		Padding(0, horizontalPadding, 1, horizontalPadding)

	l.Styles.StatusEmpty = lipgloss.NewStyle().Foreground(t.Tokens.Muted)
	l.Styles.StatusBarActiveFilter = lipgloss.NewStyle().Foreground(t.Tokens.Text)
	l.Styles.StatusBarFilterCount = lipgloss.NewStyle().Foreground(t.Tokens.Subtle)

	l.Styles.NoItems = lipgloss.NewStyle().
		Foreground(t.Tokens.Muted).
		Padding(0, horizontalPadding)

	l.Styles.PaginationStyle = lipgloss.NewStyle().Padding(0, horizontalPadding)
	l.Styles.HelpStyle = lipgloss.NewStyle().Padding(1, horizontalPadding, 0, horizontalPadding)
	l.Styles.ArabicPagination = lipgloss.NewStyle().Foreground(t.Tokens.Muted)

	dot := lipgloss.NewStyle().SetString("\u2022")
	activeDot := dot.Foreground(t.Tokens.Muted)
	inactiveDot := dot.Foreground(t.Tokens.Subtle)
	l.Styles.ActivePaginationDot = activeDot
	l.Styles.InactivePaginationDot = inactiveDot
	l.Paginator.ActiveDot = activeDot.String()
	l.Paginator.InactiveDot = inactiveDot.String()

	l.Styles.DividerDot = lipgloss.NewStyle().
		Foreground(t.Tokens.Subtle).
		SetString(" \u2022 ")
}

func (m Model) Accumulated() []item.Item {
	return m.accumulated
}

func (m Model) Selected() *item.Item {
	return m.selected
}

func (m Model) Launch() *execute.Launch {
	return m.launch
}

func (m Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.asyncSources)+1)
	if m.autoDetectTheme {
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	for _, src := range m.asyncSources {
		cmds = append(cmds, fetchSourceCmd(src))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Once a launch is committed the program is quitting; keys queued behind
	// the blocking resolve must not mutate state or re-run user commands.
	if m.launch != nil {
		if _, ok := msg.(tea.KeyPressMsg); ok {
			return m, nil
		}
	}

	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.winWidth = ws.Width
		m.winHeight = ws.Height
		m.resizeListsForWindow()
		if m.mode == viewErrorDetails {
			m = m.clampErrorDetailScroll()
		}
		return m, nil
	}

	if bg, ok := msg.(tea.BackgroundColorMsg); ok {
		return m.handleBackgroundColor(bg), nil
	}

	if result, ok := msg.(sourceResultMsg); ok {
		return m.handleSourceResult(result)
	}

	switch m.mode {
	case viewPrompt:
		return m.updatePrompt(msg)
	case viewPicker:
		return m.updatePicker(msg)
	case viewErrorDetails:
		return m.updateErrorDetails(msg)
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		return m.updateList(key)
	}

	var cmd tea.Cmd
	m.list, cmd = updateFilterableList(m.list, msg)
	return m, cmd
}

func (m *Model) resizeListsForWindow() {
	listHeight := max(m.winHeight-m.overheadHeight(), 1)
	m.list.SetSize(m.winWidth, listHeight)
	if m.shouldResizePickerList() {
		m.pickerList.SetSize(m.winWidth, listHeight)
	}
}

func (m Model) shouldResizePickerList() bool {
	return m.mode == viewPicker || m.errorReturnMode == viewPicker || len(m.pickerList.Items()) > 0
}

func (m Model) handleBackgroundColor(msg tea.BackgroundColorMsg) Model {
	if !m.autoDetectTheme {
		return m
	}

	next := theme.FromBackground(msg.IsDark(), m.ctx.Config.Theme)
	log.Debug("theme auto-detected", "theme", next.Name, "background", msg.String())
	if next.Name == m.theme.Name {
		return m
	}
	return m.applyTheme(next)
}

func (m Model) applyTheme(t theme.Theme) Model {
	m.setThemeStyles(t)
	applyFilterListTheme(&m.list, t)
	if m.mode == viewPicker || len(m.pickerList.Items()) > 0 {
		applyFilterListTheme(&m.pickerList, t)
	}
	return m
}

func (m *Model) setThemeStyles(t theme.Theme) {
	m.theme = t
	m.stackStyle = lipgloss.NewStyle().Foreground(t.Tokens.Muted)
	m.filterStyle = lipgloss.NewStyle().Inline(true).Background(t.Tokens.InputBg)
	m.errorStyle = lipgloss.NewStyle().Foreground(t.Tokens.Error)
}

func applyFilterListTheme(l *list.Model, t theme.Theme) {
	l.SetDelegate(newItemDelegate(t))
	applyListStyles(l, t)
}

func (m Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if navigateActiveFilter(&m.list, key) {
		return m, nil
	}

	if key == "enter" {
		wasFiltering := refreshActiveFilter(&m.list)
		sel, ok := selectedListItem(m.list)
		if ok && sel.Type == "error" {
			return m.openErrorDetails(sel), nil
		}
		if ok && sel.Type != "loading" {
			// Reconstruct the accumulated stack as if the user drilled down,
			// so template variables (e.g. {{.path}}) resolve correctly. The
			// candidate stays local until the selection commits so a failed
			// launch resolution leaves the model untouched.
			candidate := m.accumulated
			if sel.InlineParent != nil {
				candidate = append(slices.Clone(m.accumulated), *sel.InlineParent)
				sel.Display = sel.Value
				sel.InlineParent = nil
			}

			switch sel.Action {
			case item.ActionExecute:
				return m.finalizeSelection(sel, candidate)
			case item.ActionStaged:
				m.accumulated = append(slices.Clone(candidate), sel)
				return m.advanceStage(), nil
			case item.ActionNextList:
				return m.handleNextList(sel, candidate)
			default:
				log.Error("bug: unknown action type", "action", sel.Action)
				return m, nil
			}
		}
		if wasFiltering {
			return m, nil
		}
	}

	if key == "esc" && m.list.FilterState() == list.Unfiltered {
		if len(m.accumulated) > 0 {
			return m.handleBack(), nil
		}
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.list, cmd = updateFilterableList(m.list, msg)
	return m, cmd
}

func (m Model) updatePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Clear error on non-key messages (e.g. tea.PasteMsg) so pasted text dismisses a stale "required" error.
		m.stageError = ""
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
		if !stage.AllowEmpty && strings.TrimSpace(value) == "" {
			log.Debug("blocked empty prompt submission", "key", stage.Key)
			m.stageError = "required"
			return m, nil
		}
		return m.pushStageResult(stage.Key, value, value)

	case "esc":
		return m.stageEsc()

	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.stageInput, cmd = m.stageInput.Update(msg)
	m.stageError = ""
	return m, cmd
}

func (m Model) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.pickerList, cmd = updateFilterableList(m.pickerList, msg)
		return m, cmd
	}

	key := keyMsg.String()
	if navigateActiveFilter(&m.pickerList, key) {
		return m, nil
	}

	if key == "enter" {
		wasFiltering := refreshActiveFilter(&m.pickerList)
		sel, ok := selectedListItem(m.pickerList)
		if ok && sel.Type == "error" {
			return m.openErrorDetails(sel), nil
		}
		if ok {
			return m.pushStageResult(m.pickerKey, sel.Display, sel.Value)
		}
		if wasFiltering {
			return m, nil
		}
	}

	if key == "esc" && m.pickerList.FilterState() == list.Unfiltered {
		return m.stageEsc()
	}

	var cmd tea.Cmd
	m.pickerList, cmd = updateFilterableList(m.pickerList, msg)
	return m, cmd
}

// pushStageResult records a stage value and either advances to the next stage or completes.
func (m Model) pushStageResult(key string, display string, value string) (tea.Model, tea.Cmd) {
	actionIdx := m.findActionIndex()
	if actionIdx < 0 {
		return m.recoverFromMissingAction(), nil
	}
	action := m.accumulated[actionIdx]
	stageIdx := len(m.accumulated) - actionIdx - 1

	resultItem := item.NewItem()
	resultItem.Type = "stage-result"
	resultItem.Display = display
	resultItem.Data[key] = value
	withResult := append(slices.Clone(m.accumulated), resultItem)

	if stageIdx+1 >= len(action.Stages) {
		// The candidate keeps stage results for FlattenData but drops the
		// action item itself; it commits only if resolution succeeds.
		candidate := slices.Delete(slices.Clone(withResult), actionIdx, actionIdx+1)
		return m.finalizeSelection(action, candidate)
	}
	m.accumulated = withResult
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

func updateFilterableList(l list.Model, msg tea.Msg) (list.Model, tea.Cmd) {
	filterTextBefore := l.FilterInput.Value()
	updated, cmd := l.Update(msg)
	if updated.FilterState() == list.Filtering && updated.FilterInput.Value() != filterTextBefore {
		filterText := updated.FilterInput.Value()
		cursorPos := updated.FilterInput.Position()

		updated.SetFilterText(filterText)
		updated.SetFilterState(list.Filtering)
		updated.FilterInput.SetCursor(cursorPos)

		// SetFilterText runs the filter synchronously. Drop Bubbles' async
		// FilterMatchesMsg command so a stale result cannot update a later list
		// or picker after navigation.
		return updated, nil
	}
	return updated, cmd
}

// While the filter input is focused, cmdk follows picker semantics: an empty
// effective filter shows all items, so navigation still moves through visible
// results instead of accepting or resetting the filter.
func navigateActiveFilter(l *list.Model, key string) bool {
	switch key {
	case "down", "ctrl+j":
		if !refreshActiveFilter(l) {
			return false
		}
		l.CursorDown()
		return true
	case "up", "ctrl+k":
		if !refreshActiveFilter(l) {
			return false
		}
		l.CursorUp()
		return true
	default:
		return false
	}
}

func refreshActiveFilter(l *list.Model) bool {
	if l.FilterState() != list.Filtering {
		return false
	}

	filterText := l.FilterInput.Value()
	cursorPos := l.FilterInput.Position()
	selectedIndex := l.Index()

	l.SetFilterText(filterText)
	l.SetFilterState(list.Filtering)
	l.FilterInput.SetCursor(cursorPos)

	visibleCount := len(l.VisibleItems())
	if visibleCount == 0 {
		l.Select(0)
		return true
	}
	if selectedIndex >= visibleCount {
		selectedIndex = visibleCount - 1
	}
	l.Select(selectedIndex)
	return true
}

func selectedListItem(l list.Model) (item.Item, bool) {
	sel, ok := l.SelectedItem().(item.Item)
	return sel, ok
}

func (m Model) openErrorDetails(it item.Item) Model {
	m.errorReturnMode = m.mode
	m.errorDetailItem = safeErrorDetailItem(it)
	m.errorDetailScroll = 0
	m.mode = viewErrorDetails
	return m.clampErrorDetailScroll()
}

func safeErrorDetailItem(it item.Item) item.Item {
	it.Display = escapeTerminalControls(it.Display)
	it.Source = escapeTerminalControls(it.Source)
	it.Diagnostics = safeDiagnostics(it.Diagnostics)
	return it
}

func safeDiagnostics(d *item.Diagnostics) *item.Diagnostics {
	if d == nil {
		return nil
	}

	diagnostics := *d
	diagnostics.Summary = escapeTerminalControls(diagnostics.Summary)
	diagnostics.Fields = slices.Clone(diagnostics.Fields)
	for i := range diagnostics.Fields {
		diagnostics.Fields[i].Label = escapeTerminalControls(diagnostics.Fields[i].Label)
		diagnostics.Fields[i].Value = escapeTerminalControls(diagnostics.Fields[i].Value)
	}
	diagnostics.Sections = slices.Clone(diagnostics.Sections)
	for i := range diagnostics.Sections {
		diagnostics.Sections[i].Title = escapeTerminalControls(diagnostics.Sections[i].Title)
		diagnostics.Sections[i].Body = escapeTerminalControls(diagnostics.Sections[i].Body)
	}
	return &diagnostics
}

func (m Model) updateErrorDetails(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	pageSize := max(m.errorDetailsBodyHeight(), 1)
	switch key.String() {
	case "esc":
		m.mode = m.errorReturnMode
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "down", "j":
		m.errorDetailScroll++
	case "up", "k":
		m.errorDetailScroll--
	case "pgdown", "space", " ":
		m.errorDetailScroll += pageSize
	case "pgup":
		m.errorDetailScroll -= pageSize
	case "home":
		m.errorDetailScroll = 0
	case "end":
		m.errorDetailScroll = m.maxErrorDetailScroll()
	}

	return m.clampErrorDetailScroll(), nil
}

func (m Model) maxErrorDetailScroll() int {
	return errorDetailMaxScroll(len(m.errorDetailsLines()), m.errorDetailsBodyHeight())
}

func errorDetailMaxScroll(lineCount int, bodyHeight int) int {
	if bodyHeight <= 0 {
		return 0
	}
	return max(lineCount-bodyHeight, 0)
}

func (m Model) clampErrorDetailScroll() Model {
	m.errorDetailScroll = min(max(m.errorDetailScroll, 0), m.maxErrorDetailScroll())
	return m
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
	m.stageError = ""
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
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			log.Warn("could not determine working directory for picker diagnostics", "key", stage.Key, "error", cwdErr)
		}
		rendered, err := execute.RenderCmd(stage.Source, data)
		if err != nil {
			log.Error("failed to render picker source template", "key", stage.Key, "error", err)
			return m.initPickerWithErrorItem(stage.Key, pickerErrorItem(stage, data, cwd, cwdErr, err))
		}
		pickerTimeout := m.ctx.Config.Timeout.Picker
		result, runErr := runPickerSource(rendered, pickerTimeout, stage)
		if runErr != nil {
			log.Error("picker source command failed", append([]any{"key", stage.Key}, commandErrorLogFields(runErr)...)...)
			return m.initPickerWithErrorItem(stage.Key, pickerErrorItem(stage, data, cwd, cwdErr, runErr))
		}
		if len(result.Items) == 0 {
			noItems := &cmdrun.CommandError{
				Op:       "picker source",
				Kind:     cmdrun.KindOutput,
				Command:  rendered,
				Timeout:  pickerTimeout,
				ExitCode: 0,
				Stdout:   result.Stdout,
				Stderr:   result.Stderr,
				Err:      errors.New("returned no items"),
			}
			log.Warn("picker source returned no items", append([]any{"key", stage.Key}, commandErrorLogFields(noItems)...)...)
			return m.initPickerWithErrorItem(stage.Key, pickerErrorItem(stage, data, cwd, cwdErr, noItems))
		}
		m = m.initPicker(stage.Key, result.Items)

	default:
		log.Error("bug: unknown stage type", "type", stage.Type)
		m.accumulated = slices.Clone(m.accumulated[:actionIdx])
		return m.navigateTo(m.accumulated)
	}

	return m
}

type pickerRunResult struct {
	Items  []item.Item
	Stdout string
	Stderr string
}

const (
	// Picker stdout is parsed into an interactive fuzzy list; 4 MiB is far
	// beyond a usable list size, so the cap only guards against runaway
	// producers. Exceeding it fails the source rather than parsing a
	// silently shortened list.
	pickerMaxStdoutBytes = 4 << 20
	pickerMaxStderrBytes = 64 << 10
)

// runPickerSource executes a rendered picker source command and parses one
// item per stdout line. A zero timeout means no deadline is applied.
func runPickerSource(rendered string, timeout time.Duration, stage item.Stage) (pickerRunResult, error) {
	res, err := cmdrun.Run(cmdrun.Spec{
		Op:        "picker source",
		Rendered:  rendered,
		Timeout:   timeout,
		MaxStdout: pickerMaxStdoutBytes,
		MaxStderr: pickerMaxStderrBytes,
	})
	if err != nil {
		return pickerRunResult{}, err
	}

	return pickerRunResult{
		Items:  pickerItemsFromOutput(res.Stdout, stage),
		Stdout: res.Stdout,
		Stderr: res.AnnotatedStderr(),
	}, nil
}

func pickerItemsFromOutput(output string, stage item.Stage) []item.Item {
	delim := stage.EffectiveDelimiter()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	items := make([]item.Item, 0, len(lines))
	var warnedDisplay, warnedPass bool

	for _, line := range lines {
		if delim != "" {
			line = strings.TrimRight(line, "\r")
		} else {
			line = strings.TrimSpace(line)
		}
		if line == "" {
			continue
		}

		it := item.NewItem()
		it.Type = "pick"
		if delim != "" {
			nFields := len(strings.Split(line, delim))
			if stage.Display > nFields && !warnedDisplay {
				log.Warn("display index exceeds field count, using whole line", "index", stage.Display, "fields", nFields)
				warnedDisplay = true
			}
			if stage.Pass > nFields && !warnedPass {
				log.Warn("pass index exceeds field count, using whole line", "index", stage.Pass, "fields", nFields)
				warnedPass = true
			}
			it.Display = extractField(line, delim, stage.Display)
			it.Value = extractField(line, delim, stage.Pass)
		} else {
			it.Display = line
			it.Value = line
		}
		items = append(items, it)
	}

	return items
}

func (m Model) initPicker(key string, items []item.Item) Model {
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}

	pl := newFilterList(listItems, m.theme, m.wrapList, m.startInFilter)
	if m.winHeight > 0 {
		pl.SetSize(m.winWidth, max(m.winHeight-m.overheadHeight(), 1))
	}

	m.pickerList = pl
	m.pickerKey = key
	m.mode = viewPicker
	return m
}

func (m Model) initPickerWithErrorItem(key string, errItem item.Item) Model {
	errItem.Type = "error"
	if errItem.Source == "" {
		errItem.Source = "picker"
	}
	return m.initPicker(key, []item.Item{errItem})
}

func pickerErrorItem(stage item.Stage, data map[string]string, cwd string, cwdErr error, err error) item.Item {
	display := commandFailureDisplay("template error", err)

	it := item.NewItem()
	it.Type = "error"
	it.Source = "picker"
	it.Display = display
	contextFields := []item.DiagnosticField{
		{Label: "Stage key", Value: stage.Key},
		{Label: "Working directory", Value: workingDirectoryValue(cwd, cwdErr)},
	}
	it.Diagnostics = commandFailureDiagnostics(display, contextFields, stage.Source, data, err)
	return it
}

func launchErrorItem(action item.Item, data map[string]string, cwd string, cwdErr error, err error) item.Item {
	display := commandFailureDisplay("launch error", err)

	it := item.NewItem()
	it.Type = "error"
	it.Source = "launch"
	it.Display = display
	contextFields := []item.DiagnosticField{
		{Label: "Action", Value: action.Display},
		{Label: "Working directory", Value: workingDirectoryValue(cwd, cwdErr)},
	}
	cmdTemplate := ""
	var cmdErr *cmdrun.CommandError
	if errors.As(err, &cmdErr) {
		cmdTemplate = action.LaunchPathCmd
	}
	it.Diagnostics = commandFailureDiagnostics(display, contextFields, cmdTemplate, data, err)
	return it
}

func commandFailureDisplay(genericPrefix string, err error) string {
	var cmdErr *cmdrun.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.Headline()
	}
	return fmt.Sprintf("%s: %s", genericPrefix, err)
}

func workingDirectoryValue(cwd string, cwdErr error) string {
	if cwdErr != nil {
		return fmt.Sprintf("unknown: %s", cwdErr)
	}
	return cwd
}

// commandFailureDiagnostics builds the error-details body shared by picker
// and launch failures. The Error field carries the headline only; captured
// stdout/stderr render as their own sections.
func commandFailureDiagnostics(summary string, contextFields []item.DiagnosticField, cmdTemplate string, data map[string]string, err error) *item.Diagnostics {
	fields := slices.Clone(contextFields)
	var cmdErr *cmdrun.CommandError
	if errors.As(err, &cmdErr) {
		timeoutValue := "none"
		if cmdErr.Timeout > 0 {
			timeoutValue = cmdErr.Timeout.String()
		}
		fields = append(fields, item.DiagnosticField{Label: "Timeout", Value: timeoutValue})
		if cmdErr.ExitCode >= 0 {
			fields = append(fields, item.DiagnosticField{Label: "Exit code", Value: strconv.Itoa(cmdErr.ExitCode)})
		}
		fields = append(fields, item.DiagnosticField{Label: "Error", Value: cmdErr.Headline()})
	} else if err != nil {
		fields = append(fields, item.DiagnosticField{Label: "Error", Value: err.Error()})
	}

	sections := []item.DiagnosticSection{
		{Title: "Data fields", Body: formatDiagnosticData(data)},
	}
	if cmdTemplate != "" {
		sections = append(sections, item.DiagnosticSection{Title: "Command template", Body: cmdTemplate})
	}
	if cmdErr != nil {
		if cmdErr.Command != "" {
			sections = append(sections, item.DiagnosticSection{Title: "Rendered command", Body: cmdErr.Command})
		}
		sections = append(sections,
			item.DiagnosticSection{Title: "stdout", Body: formatCapturedCommandOutput(cmdErr.Stdout)},
			item.DiagnosticSection{Title: "stderr", Body: formatCapturedCommandOutput(cmdErr.Stderr)},
		)
	}

	return &item.Diagnostics{Summary: summary, Fields: fields, Sections: sections}
}

// commandErrorLogFields returns the structured fields both picker and launch
// failure paths log so command failures are searchable identically.
func commandErrorLogFields(err error) []any {
	var cmdErr *cmdrun.CommandError
	if errors.As(err, &cmdErr) {
		return []any{
			"error", cmdErr.Headline(),
			"command", cmdErr.Command,
			"timeout", cmdErr.Timeout,
			"exit_code", cmdErr.ExitCode,
			"stdout", logExcerpt(cmdErr.Stdout),
			"stderr", logExcerpt(cmdErr.Stderr),
		}
	}
	return []any{"error", err}
}

// logExcerpt caps captured streams in log records; picker captures can reach
// 4 MiB, which belongs in the on-demand error-details view, not in every
// appended line of the unrotated log file.
func logExcerpt(s string) string {
	const maxBytes = 8 << 10
	if len(s) <= maxBytes {
		return s
	}
	return fmt.Sprintf("%s[... %d bytes omitted]", s[:maxBytes], len(s)-maxBytes)
}

func formatDiagnosticData(data map[string]string) string {
	if len(data) == 0 {
		return "(none)"
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s: %s", k, data[k])
	}
	return b.String()
}

func formatCapturedCommandOutput(out string) string {
	if out == "" {
		return "(empty)"
	}
	return out
}

// finalizeSelection resolves the launch for a chosen execute action. It
// commits candidate state only on success; on failure the model is exactly
// what it was, so Esc from the error screen restores the prior view and a
// second Enter retries.
func (m Model) finalizeSelection(action item.Item, candidate []item.Item) (Model, tea.Cmd) {
	launch, data, err := execute.ResolveLaunch(candidate, action, m.paneID, m.ctx.Config)
	if err != nil {
		log.Error("launch resolution failed", append([]any{"action", action.Display}, commandErrorLogFields(err)...)...)
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			log.Warn("could not determine working directory for launch diagnostics", "action", action.Display, "error", cwdErr)
		}
		return m.openErrorDetails(launchErrorItem(action, data, cwd, cwdErr, err)), nil
	}
	m.accumulated = candidate
	m.selected = &action
	m.launch = &launch
	return m, tea.Quit
}

func (m Model) handleNextList(sel item.Item, candidate []item.Item) (Model, tea.Cmd) {
	m = m.navigateTo(append(slices.Clone(candidate), sel))

	if m.autoSelectSingle && len(m.list.Items()) == 1 {
		if it, ok := m.list.Items()[0].(item.Item); ok && it.Type == "action" {
			switch it.Action {
			case item.ActionExecute:
				return m.finalizeSelection(it, m.accumulated)
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
		errItem := generator.ErrorItem(generator.Source{Name: src.Name}, result.Err)
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
			all = append(all, generator.LoadingItem(generator.Source{Name: src.Name}))
		}
	}
	if m.inline {
		all = expandInline(all, m.registry, m.ctx)
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
	if m.startInFilter {
		m.list = enterFilterMode(m.list)
	}
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
	view := pad + m.stageLabel + "\n" + pad + m.stageInput.View()
	if m.stageError != "" {
		view += "\n" + pad + m.errorStyle.Render(m.stageError)
	}
	return view
}

func (m Model) errorDetailsWindowWidth() int {
	width := m.winWidth
	if width <= 0 {
		if m.errorReturnMode == viewPicker {
			width = m.pickerList.Width()
		} else {
			width = m.list.Width()
		}
	}
	if width <= 0 {
		width = 80
	}
	return width
}

func (m Model) errorDetailsWindowHeight() int {
	height := m.winHeight
	if height <= 0 {
		if m.errorReturnMode == viewPicker {
			height = m.pickerList.Height() + m.overheadHeight()
		} else {
			height = m.list.Height() + m.overheadHeight()
		}
	}
	if height <= 0 {
		height = 24
	}
	return height
}

func (m Model) errorDetailsContentWidth() int {
	return max(m.errorDetailsWindowWidth()-2*horizontalPadding, 1)
}

func (m Model) errorDetailsFrameHeight() int {
	height := 4 // header, blank before body, blank before footer, footer
	if m.errorDetailItem.Source != "" {
		height++
	}
	return height
}

func (m Model) errorDetailsBodyHeight() int {
	return max(m.errorDetailsWindowHeight()-m.errorDetailsFrameHeight(), 0)
}

func (m Model) errorDetailsLines() []string {
	body := m.errorDetailItem.Display
	if m.errorDetailItem.Diagnostics != nil {
		body = formatDiagnosticsBody(m.errorDetailItem.Diagnostics)
	}
	wrapped := ansi.Wrap(body, m.errorDetailsContentWidth(), " ")
	if wrapped == "" {
		return []string{""}
	}
	return strings.Split(wrapped, "\n")
}

func formatDiagnosticsBody(d *item.Diagnostics) string {
	if d == nil {
		return ""
	}

	var b strings.Builder
	if d.Summary != "" {
		b.WriteString(d.Summary)
	}
	for _, field := range d.Fields {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		writeDiagnosticLabelValue(&b, field.Label, field.Value)
	}
	for _, section := range d.Sections {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(section.Title)
		b.WriteString(":")
		body := section.Body
		if body == "" {
			body = "(empty)"
		}
		b.WriteByte('\n')
		b.WriteString(body)
	}
	return b.String()
}

func writeDiagnosticLabelValue(b *strings.Builder, label, value string) {
	if value == "" {
		value = "(empty)"
	}
	if !strings.Contains(value, "\n") {
		fmt.Fprintf(b, "%s: %s", label, value)
		return
	}

	b.WriteString(label)
	b.WriteString(":")
	for _, line := range strings.Split(value, "\n") {
		b.WriteString("\n  ")
		b.WriteString(line)
	}
}

func (m Model) errorDetailsView() string {
	pad := strings.Repeat(" ", horizontalPadding)
	lines := m.errorDetailsLines()
	bodyHeight := m.errorDetailsBodyHeight()
	maxScroll := errorDetailMaxScroll(len(lines), bodyHeight)
	scroll := min(max(m.errorDetailScroll, 0), maxScroll)
	end := min(scroll+bodyHeight, len(lines))

	var b strings.Builder
	b.WriteString(pad + m.errorStyle.Render("Error details") + "\n")
	if m.errorDetailItem.Source != "" {
		b.WriteString(pad + m.stackStyle.Render("Source: "+m.errorDetailItem.Source) + "\n")
	}
	b.WriteByte('\n')
	for _, line := range lines[scroll:end] {
		b.WriteString(pad)
		if line != "" {
			b.WriteString(m.errorStyle.Render(line))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(pad + m.stackStyle.Render("Esc back • ↑/↓ scroll • PgUp/PgDn page"))
	return b.String()
}

func (m Model) View() tea.View {
	if m.mode == viewErrorDetails {
		return tea.NewView(m.errorDetailsView())
	}

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
