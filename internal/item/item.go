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
	Type    StageType
	Key     string
	Text    string
	Default string
	Source  string
}

type Item struct {
	Type    string
	Source  string
	Display string
	Data    map[string]string
	Action  ActionType
	Cmd     string
	Icon    string
	Stages  []Stage
}

func NewItem() Item {
	return Item{Data: make(map[string]string)}
}

func (i Item) FilterValue() string { return i.Display }
