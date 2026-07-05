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
)

var rootCmd *cobra.Command

func init() {
	rootCmd = newRootCommand()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cmdk",
		Short: "Keyboard-driven tmux launcher",
		Long: `Keyboard-driven tmux launcher.

Log file:
  $HOME/.local/state/cmdk/cmdk.log`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runRootCommand,
	}
	cmd.Version = Version
	cmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config file (also validates; exits 1 on error)")
	cmd.Flags().StringVar(&paneID, "pane-id", "", "tmux pane ID")
	cmd.Flags().StringVar(&themeFlag, "theme", "", "color theme (light, dark)")
	cmd.Flags().BoolVar(&timingsFlag, "timings", false, "measure and print startup phase durations")
	cmd.Flags().BoolVar(&timingsJSON, "timings-json", false, "output timings as JSON (implies --timings)")
	cmd.AddCommand(
		newAttachCommand(),
		newConfigCommand(),
		newDocsCommand(),
		newExitCodesCommand(),
		newIconsCommand(),
		newSessionCommand(),
		newShortenCommand(),
		newWindowCommand(),
	)
	return cmd
}

func runRootCommand(cmd *cobra.Command, _ []string) error {
	timingsFlag = timingsFlag || timingsJSON

	tr := trace.Noop()
	if timingsFlag {
		processStart, err := trace.ProcessStartTime()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not detect process start time: %v\n", err)
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
	shortenHome := cfg.Display.ShortenHome
	trunc := pathfmt.Truncation{Length: cfg.Display.TruncationLength, Symbol: cfg.Display.TruncationSymbol}
	tmuxTrunc := pathfmt.Truncation{Length: cfg.Display.TmuxSessionTruncationLength, Symbol: cfg.Display.TmuxSessionTruncationSymbol}
	rules := pathfmt.CompileRules(cfg.Display.Rules)
	home, err := os.UserHomeDir()
	if err != nil {
		log.Warn("could not determine home directory; path shortening disabled", "error", err)
	}
	stop()

	tmuxDisplay := tmux.DisplayOptions{
		Home:              home,
		ShortenHome:       shortenHome,
		Rules:             rules,
		SessionTruncation: tmuxTrunc,
	}
	sources := []generator.Source{
		traceSource(tr, "source/windows", generator.Source{Name: "windows", Async: true, Fetch: func(ctx context.Context) ([]item.Item, error) {
			return tmux.ListWindowsWithDisplay(ctx, tmuxDisplay)
		}}),
		traceSource(tr, "source/sessions", generator.Source{Name: "sessions", Async: true, Fetch: func(ctx context.Context) ([]item.Item, error) {
			return tmux.ListSessionsWithDisplay(ctx, tmuxDisplay)
		}}),
		traceSource(tr, "source/zoxide", generator.Source{Name: "zoxide", Limit: zoxideCfg.Limit, Async: true, Fetch: func(ctx context.Context) ([]item.Item, error) {
			return zoxide.ListDirs(ctx, zoxideCfg.MinScore, home, shortenHome, rules, trunc)
		}}),
	}
	if cfgErr != nil {
		sources = append(sources, traceSource(tr, "source/config", generator.Source{Name: "config", Fetch: func(context.Context) ([]item.Item, error) {
			return nil, cfgErr
		}}))
	}
	sources = append(sources, traceSource(tr, "source/actions", generator.Source{Name: "actions", Fetch: config.MatchingActions(cfg, "root")}))

	reg := generator.NewRegistry()
	reg.Register("dir-actions", generator.NewActionsGenerator())
	reg.Register("session-children", generator.NewSessionGenerator(func(ctx context.Context, session item.Item) ([]item.Item, error) {
		return tmux.ListWindowsForSessionWithDisplay(ctx, session, tmuxDisplay)
	}))
	reg.MapType("dir", "dir-actions")
	reg.MapType("session", "session-children")

	ctx := generator.Context{PaneID: paneID, Config: cfg}

	if timingsFlag {
		reg.Register("root", generator.NewRootGenerator(cfg.Timeout.Fetch, sources...))
		reg.MapType("", "root")
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

		t, _, err := resolveTheme(tr, cfg)
		if err != nil {
			return err
		}

		stop = tr.Begin("model")
		model := tui.NewModel(listItems, paneID, nil, reg, ctx, t, nil, nil)
		stop()

		// Timings mode writes reports to stdout, so avoid terminal probing commands
		// such as RequestBackgroundColor that would emit escape sequences first.
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

	var syncSources, asyncSources []generator.Source
	for _, src := range sources {
		if src.Async {
			asyncSources = append(asyncSources, src)
		} else {
			syncSources = append(syncSources, src)
		}
	}

	reg.Register("root", generator.NewRootGenerator(cfg.Timeout.Fetch, syncSources...))
	reg.MapType("", "root")
	gen, err := reg.Resolve(nil)
	if err != nil {
		return err
	}
	stop = tr.Begin("sources/sync")
	syncItems := gen(nil, ctx)
	stop()

	var initialAll []item.Item
	initialAll = append(initialAll, syncItems...)
	for _, src := range asyncSources {
		initialAll = append(initialAll, generator.LoadingItem(src))
	}

	stop = tr.Begin("group+order")
	listItems := item.GroupAndOrder(initialAll, cfg.Behavior.BellToTop)
	stop()

	t, autoDetectTheme, err := resolveTheme(tr, cfg)
	if err != nil {
		return err
	}

	tuiAsync := make([]tui.AsyncSource, len(asyncSources))
	for i, src := range asyncSources {
		tuiAsync[i] = tui.AsyncSource{
			Name:    src.Name,
			Limit:   src.Limit,
			Timeout: cfg.Timeout.Fetch,
			Fetch:   src.Fetch,
		}
	}

	stop = tr.Begin("model")
	model := tui.NewModel(listItems, paneID, nil, reg, ctx, t, tuiAsync, syncItems)
	stop()

	if autoDetectTheme {
		stop = tr.Begin("theme/auto-detect")
		model = model.WithAutoThemeDetection()
		stop()
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
	return execute.RunWithConfig(m.Accumulated(), *sel, paneID, cfg, syscall.Exec)
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

func resolveTheme(tr trace.Tracer, cfg config.Config) (theme.Theme, bool, error) {
	autoDetectTheme := themeFlag == ""
	stop := tr.Begin("theme")
	t, err := theme.Resolve(themeFlag, cfg.Theme)
	stop()
	if err != nil {
		return theme.Theme{}, false, err
	}
	if autoDetectTheme {
		log.Debug("theme auto fallback", "theme", t.Name)
	}
	return t, autoDetectTheme, nil
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
