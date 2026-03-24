package item

type ActionType string

const (
	ActionNextList  ActionType = "next-list"
	ActionExecute   ActionType = "execute"
	ActionTextInput ActionType = "text-input"
)

type Item struct {
	Type    string
	Source  string
	Display string
	Data    map[string]string
	Action  ActionType
	Cmd     string
	Icon    string
	Prompt  string
}

func NewItem() Item {
	return Item{Data: make(map[string]string)}
}

func (i Item) FilterValue() string { return i.Display }
