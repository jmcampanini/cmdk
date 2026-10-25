// Command cmdk launches configurable actions and navigates tmux sessions.
package main

import (
	"os"

	"github.com/jmcampanini/cmdk/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
