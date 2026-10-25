package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/tmux"
)

type windowCommandOptions struct {
	paneID string
}

var switchRelativeWindow = tmux.SwitchRelativeWindow

const windowOrderHelp = `Sessions are ordered by numeric tmux session_id. Windows within a session are
ordered by numeric window_index. Navigation wraps at the ends.

For tmux key bindings, pass --pane-id=#{pane_id} so cmdk can anchor the current
window to the pane that invoked the binding:

  bind-key n run-shell "cmdk window next --pane-id=#{pane_id}"
  bind-key p run-shell "cmdk window previous --pane-id=#{pane_id}"

If --pane-id is omitted, cmdk falls back to TMUX_PANE, then tmux's default
current context.`

func newWindowCommand(root *rootOptions) *cobra.Command {
	options := &windowCommandOptions{}
	cmd := &cobra.Command{
		Use:   "window",
		Short: "Switch between tmux windows",
		Long: `Switch between tmux windows in a deterministic circular order.

` + windowOrderHelp,
		Args: cobra.NoArgs,
		RunE: runHelp,
	}
	cmd.PersistentFlags().StringVar(&options.paneID, "pane-id", "", "tmux pane ID to use as the current-window anchor")
	cmd.AddCommand(newWindowNextCommand(root, options), newWindowPreviousCommand(root, options))
	return cmd
}

func runWindowCommand(root *rootOptions, cmd *cobra.Command, direction tmux.WindowDirection, options windowCommandOptions) error {
	cfgPath, err := config.ResolvePath(root.configPath)
	if err != nil {
		return err
	}
	// Window navigation runs from tmux key bindings; a broken config file must
	// not break navigation, so use the defaults Load returns alongside its
	// error, like the root TUI does.
	cfg, _ := config.Load(cfgPath)
	return switchRelativeWindow(commandContext(cmd), direction, tmux.WindowSwitchOptions{PaneID: options.paneID, Timeouts: tmux.TimeoutsFromConfig(cfg)})
}
