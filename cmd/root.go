package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/tui"
)

var paneID string

var rootCmd = &cobra.Command{
	Use:           "cmdk",
	Short:         "Keyboard-driven tmux launcher",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		items := []item.Item{
			{Type: "window", Source: "tmux", Display: "main:1 zsh", Action: item.ActionExecute},
			{Type: "window", Source: "tmux", Display: "main:2 vim", Action: item.ActionExecute},
			{Type: "window", Source: "tmux", Display: "dev:1 node", Action: item.ActionExecute},
			{Type: "dir", Source: "zoxide", Display: "~/projects/foo", Action: item.ActionNextList},
			{Type: "dir", Source: "zoxide", Display: "~/projects/bar", Action: item.ActionNextList},
		}

		listItems := item.GroupAndOrder(items)
		model := tui.NewModel(listItems, paneID)

		p := tea.NewProgram(model)
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.Flags().StringVar(&paneID, "pane-id", "", "tmux pane ID")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
