// Package item defines launcher selections, staged inputs, and diagnostic content.
package item

// ActionType identifies the transition caused by selecting an item.
type ActionType string

const (
	// ActionNextList opens the child list for a selected item.
	ActionNextList ActionType = "next-list"
	// ActionExecute runs the selected item's command without collecting stage inputs.
	ActionExecute ActionType = "execute"
	// ActionStaged collects stage inputs before running the selected item's command.
	ActionStaged ActionType = "staged"
)

// StageType selects text entry or a command-backed picker for an action input.
type StageType string

const (
	// StagePrompt collects a text value from the user.
	StagePrompt StageType = "prompt"
	// StagePicker collects a value selected from command output.
	StagePicker StageType = "picker"
)

// Stage specifies one named input collected before executing an action.
type Stage struct {
	Type       StageType
	Key        string
	Text       string
	Default    string
	Source     string
	Delimiter  string
	Display    int
	Pass       int
	AllowEmpty bool
}

// EffectiveDelimiter returns the delimiter to use for field splitting.
// Returns "" when no splitting is configured (Delimiter, Display, and Pass are all zero-value).
func (s Stage) EffectiveDelimiter() string {
	if s.Delimiter != "" {
		return s.Delimiter
	}
	if s.Display != 0 || s.Pass != 0 {
		return "|"
	}
	return ""
}

// DiagnosticField pairs a label with a diagnostic value.
type DiagnosticField struct {
	Label string
	Value string
}

// DiagnosticSection groups multiline diagnostic text under a heading.
type DiagnosticSection struct {
	Title string
	Body  string
}

// Diagnostics contains the summary and details shown for an item failure.
type Diagnostics struct {
	Summary  string
	Fields   []DiagnosticField
	Sections []DiagnosticSection
}

// Item combines a launcher row with the data and action used when it is selected.
type Item struct {
	Type          string
	Source        string
	Display       string
	Value         string
	Data          map[string]string
	Action        ActionType
	Cmd           string
	Icon          string
	MatchType     string
	LaunchMode    string
	LaunchPath    string
	LaunchPathCmd string
	WindowName    string
	NewShell      bool
	Stages        []Stage
	Diagnostics   *Diagnostics
	InlineParent  *Item
}

// NewItem returns an item with an initialized selection-data map.
func NewItem() Item {
	return Item{Data: make(map[string]string)}
}

// FilterValue returns the display label used by the launcher's fuzzy filter.
func (i Item) FilterValue() string { return i.Display }
