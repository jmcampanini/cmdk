package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

var checkTmuxPrerequisite = tmux.CheckPrerequisite

func requireTmux(cmd *cobra.Command, _ []string) error {
	return checkTmuxPrerequisite(commandContext(cmd))
}
