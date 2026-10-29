package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

type windowCommandOptions struct {
	paneID string
}

var switchRelativeWindow = tmux.SwitchRelativeWindow

// windowOrderHelp is the ordering contract the window group and both of its
// leaves share.
const windowOrderHelp = `Sessions are ordered by numeric tmux session_id and the windows within a
session by window_index; navigation wraps at both ends. --pane-id anchors the
current window to the pane that invoked the command, which a tmux key binding
passes as #{pane_id}; without it cmdk uses TMUX_PANE, then tmux's current
client.`

// windowLeafHelp is the stream and program contract both leaves share.
const windowLeafHelp = `Stdout: nothing. Stderr: diagnostics; exit 1 when no tmux server or client
can be found or the switch fails. The command runs tmux to list windows and
switch the client, and never prompts.

` + tmuxPrerequisiteHelp + `

` + configLoadingHelpWithKeys

func newWindowCommand(root *rootOptions) *cobra.Command {
	options := &windowCommandOptions{}
	cmd := &cobra.Command{
		Use:   "window",
		Short: "Switch between tmux windows",
		Long: `Switch between tmux windows in a fixed circular order.

` + windowOrderHelp + `

A bare "cmdk window" prints this help and exits 0.

` + helpOutputContract,
		Args: cobra.NoArgs,
		RunE: runHelp,
	}
	cmd.PersistentFlags().StringVar(&options.paneID, "pane-id", "", "tmux pane that anchors the current window")
	cmd.AddCommand(newWindowNextCommand(root, options), newWindowPreviousCommand(root, options))
	return cmd
}

func runWindowCommand(root *rootOptions, cmd *cobra.Command, direction tmux.WindowDirection, options windowCommandOptions) error {
	loaded, err := root.loadConfig(cmd)
	if err != nil {
		return err
	}
	return switchRelativeWindow(commandContext(cmd), direction, tmux.WindowSwitchOptions{PaneID: options.paneID, Timeouts: tmux.TimeoutsFromConfig(loaded.config)})
}
