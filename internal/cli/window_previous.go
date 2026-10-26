package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

func newWindowPreviousCommand(root *rootOptions, options *windowCommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "previous",
		Aliases: []string{"prev"},
		Short:   "Switch to the previous tmux window",
		Long: `Switch the current tmux client to the previous window.

` + windowOrderHelp + `

` + windowLeafHelp,
		Example: `  bind-key p run-shell "cmdk window previous --pane-id=#{pane_id}"`,
		Args:    cobra.NoArgs,
		PreRunE: requireTmux,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWindowCommand(root, cmd, tmux.WindowPrevious, *options)
		},
	}
}
