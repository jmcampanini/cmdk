package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

var checkTmuxPrerequisite = tmux.CheckPrerequisite

func requireTmux(cmd *cobra.Command, _ []string) error {
	return checkTmuxPrerequisite(commandContext(cmd))
}

// tmuxPrerequisiteHelp is the tmux contract every tmux-backed command composes
// into its long description.
const tmuxPrerequisiteHelp = `Requires tmux 3.2 or newer first in PATH; a missing or older tmux exits 1.`
