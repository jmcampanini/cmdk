package item

type ActionType string

const (
	ActionNextList ActionType = "next-list"
	ActionExecute  ActionType = "execute"
)

type Item struct {
	Type    string
	Source  string
	Display string
	filter  string
	Data    map[string]string
	Action  ActionType
	Cmd     string
}

func NewItem() Item {
	return Item{Data: make(map[string]string)}
}

func (i *Item) SetDisplayPath(display, original string) {
	i.Display = display
	if display != original {
		i.filter = display + " " + original
	}
}

func (i Item) FilterValue() string {
	if i.filter != "" {
		return i.filter
	}
	return i.Display
}
func (i Item) Title() string       { return i.Display }
func (i Item) Description() string { return i.Type }
