package cmd

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

type windowCommandOptions struct {
	paneID string
}

var switchRelativeWindow = tmux.SwitchRelativeWindow

func newWindowCommand() *cobra.Command {
	options := &windowCommandOptions{}
	cmd := &cobra.Command{
		Use:   "window",
		Short: "Switch between tmux windows",
		Long: `Switch between tmux windows in a deterministic circular order.

Sessions are ordered by numeric tmux session_id. Windows within a session are
ordered by numeric window_index. Navigation wraps at the ends.

For tmux key bindings, pass --pane-id=#{pane_id} so cmdk can anchor the current
window to the pane that invoked the binding:

  bind-key n run-shell "cmdk window next --pane-id=#{pane_id}"
  bind-key p run-shell "cmdk window previous --pane-id=#{pane_id}"

If --pane-id is omitted, cmdk falls back to TMUX_PANE, then tmux's default
current context.`,
	}
	cmd.PersistentFlags().StringVar(&options.paneID, "pane-id", "", "tmux pane ID to use as the current-window anchor")
	cmd.AddCommand(newWindowNextCommand(options), newWindowPreviousCommand(options))
	return cmd
}

func newWindowNextCommand(options *windowCommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "next",
		Short:   "Switch to the next tmux window",
		Args:    cobra.NoArgs,
		PreRunE: requireTmux,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWindowCommand(cmd, tmux.WindowNext, *options)
		},
	}
}

func newWindowPreviousCommand(options *windowCommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "previous",
		Aliases: []string{"prev"},
		Short:   "Switch to the previous tmux window",
		Args:    cobra.NoArgs,
		PreRunE: requireTmux,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWindowCommand(cmd, tmux.WindowPrevious, *options)
		},
	}
}

func runWindowCommand(cmd *cobra.Command, direction tmux.WindowDirection, options windowCommandOptions) error {
	pane := options.paneID
	if pane == "" {
		// Also honor the root TUI flag when Cobra accepts it before the subcommand,
		// but keep this subcommand usable as: cmdk window next --pane-id=%1.
		pane = paneID
	}
	return switchRelativeWindow(sessionMutationContext(cmd), direction, tmux.WindowSwitchOptions{PaneID: pane})
}
