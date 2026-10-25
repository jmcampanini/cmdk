package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

func newWindowNextCommand(root *rootOptions, options *windowCommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Switch to the next tmux window",
		Long: `Switch to the next tmux window.

` + windowOrderHelp,
		Args:    cobra.NoArgs,
		PreRunE: requireTmux,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWindowCommand(root, cmd, tmux.WindowNext, *options)
		},
	}
}
