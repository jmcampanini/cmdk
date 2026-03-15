package cmd

import (
	"fmt"
	"os"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/logging"
	"github.com/jmcampanini/cmdk/internal/tmux"
	"github.com/jmcampanini/cmdk/internal/tui"
	"github.com/jmcampanini/cmdk/internal/zoxide"
)

var paneID string

var rootCmd = &cobra.Command{
	Use:           "cmdk",
	Short:         "Keyboard-driven tmux launcher",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger, err := logging.Setup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: logging setup failed: %v\n", err)
		}
		if logger != nil {
			defer func() { _ = logger.Close() }()
		}

		reg := generator.NewRegistry()
		reg.Register("root", generator.NewRootGenerator(tmux.ListWindows, zoxide.ListDirs))
		reg.Register("dir-actions", generator.NewDirActionsGenerator())
		reg.MapType("", "root")
		reg.MapType("dir", "dir-actions")

		ctx := generator.Context{PaneID: paneID}
		gen, err := reg.Resolve(nil)
		if err != nil {
			return err
		}
		items := gen(nil, ctx)
		listItems := item.GroupAndOrder(items)

		model := tui.NewModel(listItems, paneID, nil, reg, ctx)
		p := tea.NewProgram(model)
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		m, ok := finalModel.(tui.Model)
		if !ok {
			return fmt.Errorf("internal error: unexpected model type %T", finalModel)
		}
		sel := m.Selected()
		if sel == nil {
			return nil
		}
		if logger != nil {
			logger.Info("executing", "item", sel.Display, "cmd", sel.Cmd, "data", sel.Data)
		}
		return execute.Run(m.Accumulated(), *sel, syscall.Exec)
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
