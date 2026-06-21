package tmux

import (
	"fmt"

	"github.com/jmcampanini/cmdk/internal/item"
)

func newTmuxParseErrorItem(command string, malformedRows int) item.Item {
	rowWord := "row"
	if malformedRows != 1 {
		rowWord = "rows"
	}

	it := item.NewItem()
	it.Type = "error"
	it.Source = "tmux"
	it.Display = fmt.Sprintf("tmux parse error: %d unparseable %s %s", malformedRows, command, rowWord)
	return it
}
