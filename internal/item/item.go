package item

type ActionType string

const (
	ActionNextList ActionType = "next-list"
	ActionExecute  ActionType = "execute"
	ActionStaged   ActionType = "staged"
)

type StageType string

const (
	StagePrompt StageType = "prompt"
	StagePicker StageType = "picker"
)

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

type Item struct {
	Type         string
	Source       string
	Display      string
	Value        string
	Data         map[string]string
	Action       ActionType
	Cmd          string
	Icon         string
	Stages       []Stage
	InlineParent *Item
}

func NewItem() Item {
	return Item{Data: make(map[string]string)}
}

func (i Item) FilterValue() string { return i.Display }
