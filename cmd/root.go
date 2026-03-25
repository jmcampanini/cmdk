package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"

	tea "charm.land/bubbletea/v2"
	log "charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/execute"
	"github.com/jmcampanini/cmdk/internal/generator"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/logging"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
	"github.com/jmcampanini/cmdk/internal/theme"
	"github.com/jmcampanini/cmdk/internal/tmux"
	"github.com/jmcampanini/cmdk/internal/tui"
	"github.com/jmcampanini/cmdk/internal/zoxide"
)

// Version is set at build time via ldflags.
var Version = "n/a"

var (
	configPath string
	paneID     string
	themeFlag  string
)

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

		cfgPath, err := resolveConfigPath()
		if err != nil {
			return err
		}
		cfg, cfgErr := config.Load(cfgPath)
		if cfgErr != nil && configPath != "" {
			return cfgErr
		}
		zoxideCfg := cfg.Sources["zoxide"]
		shortenHome := *cfg.Display.ShortenHome
		rules := pathfmt.CompileRules(cfg.Display.Rules)
		home, err := os.UserHomeDir()
		if err != nil {
			log.Warn("could not determine home directory; path shortening disabled", "error", err)
		}

		sources := []generator.Source{
			{Name: "windows", Type: "window", Fetch: tmux.ListWindows},
			{Name: "zoxide", Type: "dir", Limit: zoxideCfg.Limit, Fetch: func(ctx context.Context) ([]item.Item, error) {
				return zoxide.ListDirs(ctx, zoxideCfg.MinScore, home, shortenHome, rules)
			}},
		}
		if cfgErr != nil {
			sources = append(sources, generator.Source{Name: "config", Type: "cmd", Fetch: func(context.Context) ([]item.Item, error) {
				return nil, cfgErr
			}})
		}
		sources = append(sources, generator.Source{Name: "commands", Type: "cmd", Fetch: config.CommandItems(cfg)})

		reg := generator.NewRegistry()
		reg.Register("root", generator.NewRootGenerator(cfg.Timeout.Fetch, sources...))
		reg.Register("dir-actions", generator.NewDirActionsGenerator())
		reg.MapType("", "root")
		reg.MapType("dir", "dir-actions")

		ctx := generator.Context{PaneID: paneID, Config: cfg}
		gen, err := reg.Resolve(nil)
		if err != nil {
			return err
		}
		items := gen(nil, ctx)
		listItems := item.GroupAndOrder(items, cfg.Behaviors.BellToTop)

		t, err := theme.Resolve(themeFlag)
		if err != nil {
			return err
		}
		if themeFlag == "" {
			log.Debug("theme auto-detected", "theme", t.Name)
		}
		model := tui.NewModel(listItems, paneID, nil, reg, ctx, t)
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
		log.Info("executing", "item", sel.Display, "cmd", sel.Cmd, "data", sel.Data)
		return execute.Run(m.Accumulated(), *sel, paneID, syscall.Exec)
	},
}

func resolveConfigPath() (string, error) {
	if configPath != "" {
		fi, err := os.Stat(configPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("config file not found: %s", configPath)
			}
			return "", fmt.Errorf("config file not accessible: %w", err)
		}
		if !fi.Mode().IsRegular() {
			return "", fmt.Errorf("config path is not a regular file: %s", configPath)
		}
		return configPath, nil
	}
	return config.DefaultPath(), nil
}

func init() {
	rootCmd.Version = Version
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config file (also validates; exits 1 on error)")
	rootCmd.Flags().StringVar(&paneID, "pane-id", "", "tmux pane ID")
	rootCmd.Flags().StringVar(&themeFlag, "theme", "", "color theme (light, dark)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
