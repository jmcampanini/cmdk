package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"time"

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
	"github.com/jmcampanini/cmdk/internal/trace"
	"github.com/jmcampanini/cmdk/internal/tui"
	"github.com/jmcampanini/cmdk/internal/zoxide"
)

// Version is set at build time via ldflags.
var Version = "n/a"

var (
	configPath  string
	paneID      string
	themeFlag   string
	timingsFlag bool
	timingsJSON bool
	startTime   int64
)

var rootCmd = &cobra.Command{
	Use:           "cmdk",
	Short:         "Keyboard-driven tmux launcher",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		timingsFlag = timingsFlag || timingsJSON || startTime != 0

		tr := trace.Noop()
		if timingsFlag {
			var processStart time.Time
			if startTime != 0 {
				processStart = time.UnixMilli(startTime)
			}
			tr = trace.New(processStart)
		}

		stop := tr.Begin("logging")
		logger, err := logging.Setup()
		stop()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: logging setup failed: %v\n", err)
		}
		if logger != nil {
			defer func() { _ = logger.Close() }()
		}

		stop = tr.Begin("config")
		cfgPath, err := resolveConfigPath()
		if err != nil {
			stop()
			return err
		}
		cfg, cfgErr := config.Load(cfgPath)
		if cfgErr != nil && configPath != "" {
			stop()
			return cfgErr
		}
		zoxideCfg := cfg.Sources["zoxide"]
		shortenHome := *cfg.Display.ShortenHome
		rules := pathfmt.CompileRules(cfg.Display.Rules)
		home, err := os.UserHomeDir()
		if err != nil {
			log.Warn("could not determine home directory; path shortening disabled", "error", err)
		}
		stop()

		sources := []generator.Source{
			traceSource(tr, "source/windows", generator.Source{Name: "windows", Type: "window", Fetch: tmux.ListWindows}),
			traceSource(tr, "source/zoxide", generator.Source{Name: "zoxide", Type: "dir", Limit: zoxideCfg.Limit, Fetch: func(ctx context.Context) ([]item.Item, error) {
				return zoxide.ListDirs(ctx, zoxideCfg.MinScore, home, shortenHome, rules)
			}}),
		}
		if cfgErr != nil {
			sources = append(sources, traceSource(tr, "source/config", generator.Source{Name: "config", Type: "action", Fetch: func(context.Context) ([]item.Item, error) {
				return nil, cfgErr
			}}))
		}
		sources = append(sources, traceSource(tr, "source/actions", generator.Source{Name: "actions", Type: "action", Fetch: config.MatchingActions(cfg, "root")}))

		reg := generator.NewRegistry()
		reg.Register("root", generator.NewRootGenerator(cfg.Timeout.Fetch, sources...))
		reg.Register("dir-actions", generator.NewActionsGenerator())
		reg.MapType("", "root")
		reg.MapType("dir", "dir-actions")

		ctx := generator.Context{PaneID: paneID, Config: cfg}
		gen, err := reg.Resolve(nil)
		if err != nil {
			return err
		}
		stop = tr.Begin("sources")
		items := gen(nil, ctx)
		stop()

		stop = tr.Begin("group+order")
		listItems := item.GroupAndOrder(items, cfg.Behavior.BellToTop)
		stop()

		stop = tr.Begin("theme")
		t, err := theme.Resolve(themeFlag)
		stop()
		if err != nil {
			return err
		}
		if themeFlag == "" {
			log.Debug("theme auto-detected", "theme", t.Name)
		}

		stop = tr.Begin("model")
		model := tui.NewModel(listItems, paneID, nil, reg, ctx, t)
		stop()

		if timingsFlag {
			stop = tr.Begin("tea-ready")
			wrapper := timingsModel{inner: model}
			p := tea.NewProgram(wrapper, tea.WithoutRenderer(), tea.WithInput(nil))
			_, err = p.Run()
			stop()
			if err != nil {
				return err
			}
			if timingsJSON {
				return trace.WriteJSON(os.Stdout, tr.Spans())
			}
			return trace.WriteTable(os.Stdout, tr.Spans())
		}

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

func traceSource(tr trace.Tracer, spanName string, src generator.Source) generator.Source {
	orig := src.Fetch
	src.Fetch = func(ctx context.Context) ([]item.Item, error) {
		stop := tr.Begin(spanName)
		defer stop()
		return orig(ctx)
	}
	return src
}

type timingsModel struct {
	inner tea.Model
	sized bool
}

func (m timingsModel) Init() tea.Cmd {
	return m.inner.Init()
}

func (m timingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.WindowSizeMsg); ok && !m.sized {
		m.sized = true
		// Cmd is intentionally dropped — tea.Quit terminates the program immediately.
		inner, _ := m.inner.Update(msg)
		m.inner = inner
		return m, tea.Quit
	}
	inner, cmd := m.inner.Update(msg)
	m.inner = inner
	return m, cmd
}

func (m timingsModel) View() tea.View {
	return m.inner.View()
}

func init() {
	rootCmd.Version = Version
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config file (also validates; exits 1 on error)")
	rootCmd.Flags().StringVar(&paneID, "pane-id", "", "tmux pane ID")
	rootCmd.Flags().StringVar(&themeFlag, "theme", "", "color theme (light, dark)")
	rootCmd.Flags().BoolVar(&timingsFlag, "timings", false, "measure and print startup phase durations")
	rootCmd.Flags().BoolVar(&timingsJSON, "timings-json", false, "output timings as JSON (implies --timings)")
	rootCmd.Flags().Int64Var(&startTime, "start-time", 0, "process start time as epoch milliseconds (implies --timings)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
