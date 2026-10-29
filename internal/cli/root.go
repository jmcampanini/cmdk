// Package cli defines cmdk's commands and wires the interactive launcher.
package cli

import (
	"context"
	"fmt"
	"io"
	log "log/slog"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/config"
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

type rootOptions struct {
	configPath  string
	paneID      string
	themeFlag   themeMode
	colorMode   colorMode
	timingsFlag bool
	timingsJSON bool
	version     bool
}

func newRootCommand() *cobra.Command {
	root := &rootOptions{colorMode: colorAuto}
	cmd := &cobra.Command{
		Use:   "cmdk",
		Short: "Keyboard-driven tmux launcher",
		Long: `Keyboard-driven tmux launcher.

Without a subcommand cmdk opens the launcher: a fuzzy-filterable list of tmux
windows and sessions, zoxide directories, and configured actions. Enter runs
the selection. A session-window action creates a tmux window and cmdk exits
0; a shell-mode action replaces the cmdk process with its command, whose exit
status becomes cmdk's. Quitting without a selection exits 0. See "cmdk help
launching" for the launch modes and "cmdk help templates" for the variables
actions use.

The launcher is interactive only. It draws on the terminal attached to stdout
and reads keys from stdin, so both must be terminals and TERM must name a
terminal type other than dumb; otherwise cmdk exits 1 with a message saying
what to fix. Run it from a terminal or a tmux popup; the Examples show the
tmux binding. Help, --version, help topics, and "cmdk config" need no
terminal. --version prints the build version to stdout and exits without
loading configuration or running external programs.

Streams: the launcher writes nothing to stdout, except that --timings and
--timings-json run the startup phases without drawing and write a table or
JSON of their durations to stdout. Diagnostics go to stderr. Every launcher
run appends to the log file $HOME/.local/state/cmdk/cmdk.log, creating the
file and its directory when absent; a log that cannot be opened is a stderr
warning, not an error.

Color: --color=auto (the default) detects the stdout terminal, where a
nonempty NO_COLOR, TERM=dumb, or a non-terminal disables color; --color=never
disables color; --color=always forces it even when NO_COLOR is set.
--theme=light or --theme=dark picks the palette. When --theme is empty and
color is on, cmdk asks the terminal for its background once per run and
falls back to the dark palette when the terminal does not answer.

External programs: tmux 3.2 or newer first in PATH, which the launcher
queries for windows and sessions and never starts; git, run to resolve a
path to its repository; sh, which runs configured commands; zoxide,
optional, for directory entries; a Nerd Font, optional, for icons. Linux is
verified in required CI; other Unix-like systems are unverified, and Windows
is unsupported.

` + configLoadingHelpWithKeys + `

Common tasks:
  Open the launcher for a pane:   cmdk --pane-id=#{pane_id}
  Enter a managed session:        cmdk attach [path]
  Check a configuration file:     cmdk config --validate <path>
  Open a shell window for a path: cmdk session window <path> --new
  Run a configured action:        cmdk action run <exact-name>

Command groups:
  action      Configured actions:           cmdk action --help
  completion  Shell completion scripts:     cmdk completion --help
  session     Session plans and windows:    cmdk session --help
  window      Switching between windows:    cmdk window --help`,
		Example: `  bind-key Space display-popup -E "cmdk --pane-id=#{pane_id}"
  cmdk --timings-json`,
		DisableSuggestions: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if root.version {
				return nil
			}
			return requireTmux(cmd, args)
		},
		RunE: root.runRootCommand,
	}
	cmd.PersistentFlags().StringVarP(&root.configPath, "config", "c", "", "configuration file to load; must exist")
	cmd.Flags().StringVar(&root.paneID, "pane-id", "", "tmux pane the launcher was opened from")
	cmd.Flags().Var(&root.themeFlag, "theme", "palette: light or dark; empty detects it")
	cmd.Flags().Var(&root.colorMode, "color", "color output: auto, always, or never")
	cmd.Flags().BoolVar(&root.timingsFlag, "timings", false, "print startup phase durations instead of the launcher")
	cmd.Flags().BoolVar(&root.timingsJSON, "timings-json", false, "print the durations as JSON (implies --timings)")
	cmd.Flags().BoolVarP(&root.version, "version", "v", false, "print the build version")
	cmd.InitDefaultHelpFlag()
	// Flag parsing errors are usage errors; the action group overrides this
	// to escape terminal controls first.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return markUsageError(err)
	})
	cmd.AddGroup(
		&cobra.Group{ID: commandGroupID, Title: "Commands:"},
		&cobra.Group{ID: topicGroupID, Title: "Help topics:"},
	)
	help := newHelpCommand()
	cmd.SetHelpCommand(help)
	cmd.AddCommand(
		help,
		newActionCommand(root),
		newAttachCommand(root),
		newCompletionCommand(),
		newConfigCommand(root),
		newExitCodesCommand(root),
		newIconsCommand(root),
		newLaunchingCommand(root),
		newSessionCommand(root),
		newShortenCommand(root),
		newTemplatesCommand(root),
		newWindowCommand(root),
	)
	for _, child := range cmd.Commands() {
		if child.GroupID == "" {
			child.GroupID = commandGroupID
		}
	}
	markUsageErrors(cmd)
	return cmd
}

func (root *rootOptions) runRootCommand(cmd *cobra.Command, _ []string) error {
	if root.version {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "cmdk version %s\n", Version)
		return err
	}

	root.timingsFlag = root.timingsFlag || root.timingsJSON

	tr := trace.Noop()
	if root.timingsFlag {
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
	loaded, err := root.loadConfig(cmd)
	if err != nil {
		stop()
		return err
	}
	cfg := loaded.config
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

	// Timings mode runs without a renderer or input and writes to stdout, so
	// only the launcher needs the terminal contract.
	if !root.timingsFlag {
		if err := checkLauncherTerminal(os.Stdin, os.Stdout); err != nil {
			return err
		}
	}

	tmuxDisplay := tmux.DisplayOptions{
		Home:              home,
		ShortenHome:       shortenHome,
		Rules:             rules,
		SessionTruncation: tmuxTrunc,
	}
	fetchTimeout := cfg.Timeout.Fetch
	sources := []generator.Source{
		traceSource(tr, "source/windows", generator.Source{Name: "windows", Async: true, Fetch: func(ctx context.Context) ([]item.Item, error) {
			return tmux.ListWindowsWithDisplay(ctx, fetchTimeout, tmuxDisplay)
		}}),
		traceSource(tr, "source/sessions", generator.Source{Name: "sessions", Async: true, Fetch: func(ctx context.Context) ([]item.Item, error) {
			return tmux.ListSessionsWithDisplay(ctx, fetchTimeout, tmuxDisplay)
		}}),
		traceSource(tr, "source/zoxide", generator.Source{Name: "zoxide", Limit: zoxideCfg.Limit, Async: true, Fetch: func(ctx context.Context) ([]item.Item, error) {
			return zoxide.ListDirs(ctx, fetchTimeout, zoxideCfg.MinScore, home, shortenHome, rules, trunc)
		}}),
	}
	sources = append(sources, traceSource(tr, "source/actions", generator.Source{Name: "actions", Fetch: config.MatchingActions(cfg, "root")}))

	reg := generator.NewRegistry()
	reg.Register("dir-actions", generator.NewActionsGenerator())
	reg.Register("session-children", generator.NewSessionGenerator(func(ctx context.Context, session item.Item) ([]item.Item, error) {
		return tmux.ListWindowsForSessionWithDisplay(ctx, fetchTimeout, session, tmuxDisplay)
	}))
	reg.MapType("dir", "dir-actions")
	reg.MapType("session", "session-children")

	ctx := generator.Context{PaneID: root.paneID, Config: cfg}

	if root.timingsFlag {
		reg.Register("root", generator.NewRootGenerator(fetchTimeout, sources...))
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

		t, _, err := root.resolveTheme(tr, cfg.Theme.Overrides())
		if err != nil {
			return err
		}

		stop = tr.Begin("model")
		model := tui.NewModel(listItems, root.paneID, nil, reg, ctx, t, nil, nil)
		stop()

		// Timings mode writes reports to stdout, so avoid terminal probing commands
		// such as RequestBackgroundColor that would emit escape sequences first.
		stop = tr.Begin("tea-ready")
		wrapper := timingsModel{inner: model}
		p := tea.NewProgram(wrapper, tea.WithoutRenderer(), tea.WithInput(nil), tea.WithColorProfile(colorprofile.ASCII))
		_, err = p.Run()
		stop()
		if err != nil {
			return err
		}
		if root.timingsJSON {
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

	reg.Register("root", generator.NewRootGenerator(fetchTimeout, syncSources...))
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

	themeOverrides := cfg.Theme.Overrides()
	t, autoDetectTheme, err := root.resolveTheme(tr, themeOverrides)
	if err != nil {
		return err
	}

	tuiAsync := make([]tui.AsyncSource, len(asyncSources))
	for i, src := range asyncSources {
		tuiAsync[i] = tui.AsyncSource{
			Name:    src.Name,
			Limit:   src.Limit,
			Timeout: fetchTimeout,
			Fetch:   src.Fetch,
		}
	}

	stop = tr.Begin("model")
	model := tui.NewModel(listItems, root.paneID, nil, reg, ctx, t, tuiAsync, syncItems)
	stop()

	// The background query is a round trip to the terminal, so it runs only
	// when its answer can change what the person sees: the palette is
	// adaptive, stdout is a terminal (checked above), and the profile can
	// render color. Otherwise the resolved theme stands.
	profile := resolveColorProfile(root.colorMode, term.IsTerminal(os.Stdout.Fd()), os.Environ())
	if rendersColor(profile) && profile < colorprofile.TrueColor && os.Getenv("TMUX") != "" && tmux.SupportsTrueColor(cmd.Context(), fetchTimeout) {
		profile = colorprofile.TrueColor
	}
	if autoDetectTheme && rendersColor(profile) {
		stop = tr.Begin("theme/auto-detect")
		model = model.WithAutoThemeDetection(themeOverrides)
		stop()
	}

	p := tea.NewProgram(model, tea.WithColorProfile(profile))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("internal error: unexpected model type %T", finalModel)
	}
	launch := m.Launch()
	sel := m.Selected()
	if launch == nil || sel == nil {
		return nil
	}
	log.Info("executing", "item", sel.Display, "cmd", sel.Cmd, "data", sel.Data)
	if err := launch.Execute(cmdrun.Replace); err != nil {
		log.Error("launch failed", "item", sel.Display, "error", err)
		return err
	}
	return nil
}

func (root *rootOptions) resolveTheme(tr trace.Tracer, overrides theme.Overrides) (theme.Theme, bool, error) {
	autoDetectTheme := root.themeFlag == ""
	stop := tr.Begin("theme")
	t, err := theme.Resolve(string(root.themeFlag), overrides)
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
		// Cmd is intentionally dropped - tea.Quit terminates the program immediately.
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

// Execute runs the root command and returns its process exit status.
func Execute() int {
	return run(newRootCommand(), os.Stderr)
}

// run executes root, reports a failure as one "cmdk: <message>" line on
// stderr with no usage text, and maps the error to the exit status.
func run(root *cobra.Command, stderr io.Writer) int {
	err := root.Execute()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cmdk: %s\n", foldLines(err.Error()))
	}
	return exitStatus(err)
}

// foldLines joins a multi-line message into one line, so a diagnostic that
// embeds captured output or an input value with a newline still prints as
// one stderr line.
func foldLines(message string) string {
	var lines []string
	for line := range strings.SplitSeq(message, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "; ")
}
