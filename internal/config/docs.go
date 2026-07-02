package config

import (
	"fmt"
	"strings"
)

type FieldDoc struct {
	Name        string
	Type        string
	Description string
	Validation  string
}

type SectionDoc struct {
	Name        string
	Description string
	Fields      []FieldDoc
	Example     string
}

const themeColorValidation = "optional; must be #RRGGBB hex when set"

func themeColorField(name string, description string) FieldDoc {
	return FieldDoc{Name: name, Type: "string", Description: description, Validation: themeColorValidation}
}

func ConfigDocs() []SectionDoc {
	reservedStageKeyValidation := "cannot be empty; must be unique within action; " +
		"cannot be reserved (" + strings.Join(reservedStageKeys, ", ") +
		"; session actions also reserve " + strings.Join(sessionActionExtraReservedStageKeys, ", ") + ")"

	return []SectionDoc{
		{
			Name:        "startup",
			Description: "Defaults for entering a cmdk-managed tmux session from outside tmux.",
			Fields: []FieldDoc{
				{
					Name:        "path",
					Type:        "string",
					Description: "Default directory used by \"cmdk attach\" when no path argument is provided. Leading ~/ is expanded to the current user's home directory.",
					Validation:  "optional; cannot contain control characters; must resolve to an existing directory when cmdk attach is run",
				},
			},
			Example: "[startup]\npath = \"~/Code/github.com/me/project\"",
		},
		{
			Name:        "actions",
			Description: "Actions shown in the launcher. Each action declares which item type\n  it matches, how it launches, and an optional stage pipeline to collect data before execution.",
			Fields: []FieldDoc{
				{Name: "name", Type: "string", Description: "Display name in the launcher.", Validation: "cannot be empty"},
				{Name: "matches", Type: "string", Description: "Item type this action appears for: \"root\" (top-level), \"dir\" (after selecting a directory), or \"session\" (after selecting a tmux session).", Validation: "cannot be empty; must be \"root\", \"dir\", or \"session\""},
				{Name: "cmd", Type: "string", Description: "Shell command or Go template to run after all stages complete. In session-window mode this is the window payload, not a tmux new-window wrapper.", Validation: "cannot be empty"},
				{Name: "icon", Type: "string", Description: "Nerdfont icon: alias like :nf-dev-github:, raw glyph, or \\uXXXX escape. Run \"cmdk icons --filter <term>\" to search aliases.", Validation: "if :nf-*: alias, must be in supported set; otherwise must be a single grapheme cluster"},
				{Name: "launch_mode", Type: "string", Description: "Launch mode: \"detect\" (default), \"session-window\" (create a cmdk-managed tmux window), or \"shell\" (exec in this pane). Detect makes dir actions and actions with launch_path/launch_path_cmd session-window; root/session actions without path fields stay shell.", Validation: "optional; must be \"detect\", \"session-window\", or \"shell\""},
				{Name: "launch_path", Type: "string", Description: "Path template for the action's effective launch directory. Supports safe leading ~, $VAR, and ${VAR} expansion before Go template rendering; no command substitution, globbing, or word splitting.", Validation: "mutually exclusive with launch_path_cmd; rendered path must resolve to an existing directory"},
				{Name: "launch_path_cmd", Type: "string", Description: "Shell command template run via sh -c after stages; stdout must be exactly one absolute existing directory path. Use {{sq ...}} around template variables.", Validation: "mutually exclusive with launch_path; output must be one absolute existing directory path"},
				{Name: "window_name", Type: "string", Description: "Tmux window name template for session-window actions. Defaults to {{.launch_basename}}.", Validation: "only valid for effective session-window actions; rendered name cannot be empty or contain control characters"},
				{Name: "stages", Type: "array", Description: "Optional pipeline of data-collection stages to run before executing cmd. See STAGES section."},
			},
			Example: "[[actions]]\nname = \"Yazi\"\nmatches = \"root\"\nlaunch_mode = \"shell\"\ncmd = \"tmux split-window -h yazi\"\nicon = \":nf-cod-folder:\"\n\n[[actions]]\nname = \"Claude\"\nmatches = \"dir\"\ncmd = \"direnv exec {{sq .launch_path}} claude\"\n\n[[actions]]\nname = \"Dotfiles pi\"\nmatches = \"root\"\nlaunch_path = \"$HOME/Code/github.com/me/dotfiles/main\"\ncmd = \"pi\"\n\n[[actions]]\nname = \"Rename Session\"\nmatches = \"session\"\ncmd = \"tmux rename-session -t {{sq .session_id}} {{sq .new_name}}\"\nstages = [\n  { type = \"prompt\", text = \"New name for {{.session_name}}:\", key = \"new_name\" },\n]",
		},
		{
			Name:        "stages",
			Description: "Stages are declared inline within an action's stages array.\n  Each stage collects one piece of data before the action executes.",
			Fields: []FieldDoc{
				{Name: "type", Type: "string", Description: "Stage type: \"prompt\" (text input) or \"picker\" (shell command → fuzzy list).", Validation: "must be \"prompt\" or \"picker\""},
				{Name: "key", Type: "string", Description: "Template variable name for the stage's output value.", Validation: reservedStageKeyValidation},
				{Name: "text", Type: "string", Description: "Prompt label (Go template). Only for type = \"prompt\".", Validation: "required for prompt; forbidden for picker"},
				{Name: "default", Type: "string", Description: "Default value pre-filled in prompt (Go template). Only for type = \"prompt\"."},
				{Name: "source", Type: "string", Description: "Shell command run via sh -c that produces newline-separated entries (Go template). Only for type = \"picker\".", Validation: "required for picker; forbidden for prompt"},
				{Name: "delimiter", Type: "string", Description: "Field delimiter for splitting source lines into parts. Only for type = \"picker\". Defaults to \"|\" when display or pass is set.", Validation: "forbidden for prompt"},
				{Name: "display", Type: "int", Description: "1-based field index to display and match against. 0 = whole line (default). Only for type = \"picker\".", Validation: "forbidden for prompt; cannot be negative"},
				{Name: "pass", Type: "int", Description: "1-based field index to pass as the stage result value. 0 = whole line (default). Only for type = \"picker\".", Validation: "forbidden for prompt; cannot be negative"},
				{Name: "allow_empty", Type: "bool", Description: "Allow empty input for prompt stages. When false (default), pressing Enter on an empty or whitespace-only prompt shows an error. Only for type = \"prompt\".", Validation: "forbidden for picker"},
			},
			Example: "stages = [\n  { type = \"prompt\", text = \"Branch name:\", key = \"branch\", default = \"feature/\" },\n  { type = \"picker\", source = \"find {{.path}} -type f\", key = \"file\" },\n  { type = \"picker\", source = \"printf 'Alice|alice@co\\nBob|bob@co'\", key = \"user\", delimiter = \"|\", display = 1, pass = 2 },\n]",
		},
		{
			Name:        "behavior",
			Description: "Global behavior settings.",
			Fields: []FieldDoc{
				{Name: "auto_select_single", Type: "bool", Description: "Skip the action list when only one action matches. Default: true."},
				{Name: "bell_to_top", Type: "bool", Description: "Sort tmux windows with bell activity above normal items, after errors and loading placeholders. Default: true."},
				{Name: "wrap_list", Type: "bool", Description: "Wrap cursor to the opposite end when navigating past the first or last item. Default: true."},
				{Name: "start_in_filter", Type: "bool", Description: "Open lists in filter mode, ready for typing. When false, lists open in browse mode; press / to filter. Default: true."},
				{Name: "inline_actions", Type: "bool", Description: "Expand directory actions inline in the root list instead of requiring drill-down. Each directory gets one entry per action, displayed as \"path » action\". Default: false."},
			},
			Example: "[behavior]\nauto_select_single = false\nbell_to_top = true\nwrap_list = false\nstart_in_filter = true\ninline_actions = false",
		},
		{
			Name:        "theme",
			Description: "Per-mode color overrides. cmdk starts from the active built-in theme\n  (light = Catppuccin Latte, dark = Catppuccin Frappe) and applies\n  [theme.light] or [theme.dark] overrides. Empty --theme auto-detects\n  light/dark from the terminal background when supported. cmdk does not\n  paint a full-screen background; the terminal/tmux popup background remains visible.\n\n  To apply an arbitrary palette:\n  1. Pick the table for the palette appearance: [theme.dark] for dark\n     palettes or [theme.light] for light palettes. Pass --theme=dark or\n     --theme=light when auto-detection is unreliable.\n  2. Fill the semantic tokens from the source palette: accent = primary\n     accent; accent_text = a base/background color that contrasts with accent;\n     cursor = cursor or secondary accent; text = main foreground; muted =\n     comments/overlay text; subtle = divider or dim surface; selected_bg =\n     selection surface; input_bg = input/background surface; match_bg =\n     low-contrast highlight, often a surface tinted toward accent; info = blue;\n     success = green; secondary = teal/cyan; warning = yellow/orange; error = red.\n  3. Leave roles unset unless a specific icon needs to differ. Roles derive\n     from semantic tokens: window <- accent, dir <- info, action <- success,\n     session <- secondary, unknown/loading <- muted, bell <- warning, error <- error.\n  4. Use #RRGGBB hex values only and validate with\n     cmdk config --validate /path/to/config.toml.",
			Fields: []FieldDoc{
				themeColorField("accent", "Primary accent for the cmdk badge. Window icons derive from this unless roles.window_icon is set."),
				themeColorField("accent_text", "Text color used on top of accent backgrounds, such as the cmdk badge."),
				themeColorField("cursor", "Filter input cursor color."),
				themeColorField("text", "Primary item and active-filter text color."),
				themeColorField("muted", "Muted/status text color. Unknown, loading, and picker icons derive from this unless role overrides are set."),
				themeColorField("subtle", "Subtle divider, count, and inactive pagination color."),
				themeColorField("selected_bg", "Selected row background color."),
				themeColorField("input_bg", "Filter input background color."),
				themeColorField("match_bg", "Background color for fuzzy-match highlights."),
				themeColorField("info", "Informational color. Directory icons derive from this unless roles.dir_icon is set."),
				themeColorField("success", "Success/action color. Action icons derive from this unless roles.action_icon is set."),
				themeColorField("secondary", "Secondary accent. Session icons derive from this unless roles.session_icon is set."),
				themeColorField("warning", "Warning color. Bell icons derive from this unless roles.bell_icon is set."),
				themeColorField("error", "Error text color. Error icons derive from this unless roles.error_icon is set."),
				themeColorField("roles.window_icon", "Optional explicit color for tmux window icons."),
				themeColorField("roles.dir_icon", "Optional explicit color for directory icons."),
				themeColorField("roles.action_icon", "Optional explicit color for action icons."),
				themeColorField("roles.session_icon", "Optional explicit color for tmux session icons."),
				themeColorField("roles.unknown_icon", "Optional explicit color for unknown item and picker icons."),
				themeColorField("roles.loading_icon", "Optional explicit color for loading placeholder icons."),
				themeColorField("roles.bell_icon", "Optional explicit color for tmux bell icons."),
				themeColorField("roles.error_icon", "Optional explicit color for error item icons."),
			},
			Example: "[theme.dark]\n# Catppuccin Frappe-style semantic mapping. For another dark theme,\n# replace these with that palette's equivalent slots.\naccent = \"#ca9ee6\"      # mauve / primary accent\naccent_text = \"#303446\" # base/background that contrasts with accent\ncursor = \"#babbf1\"      # lavender / cursor\ntext = \"#c6d0f5\"        # main foreground\nmuted = \"#838ba7\"       # comments / overlay text\nsubtle = \"#626880\"      # divider / dim surface\nselected_bg = \"#51576d\" # selected row surface\ninput_bg = \"#414559\"    # input/background surface\nmatch_bg = \"#5b4b8a\"    # low-contrast accent-tinted highlight\ninfo = \"#8caaee\"        # blue\nsuccess = \"#a6d189\"     # green\nsecondary = \"#81c8be\"   # teal/cyan\nwarning = \"#e5c890\"     # yellow/orange\nerror = \"#e78284\"       # red\n\n# Optional: override roles only when an icon should break derivation.\n[theme.dark.roles]\n# session_icon = \"#81c8be\"",
		},
		{
			Name:        "timeout",
			Description: "Timeouts for async operations.",
			Fields: []FieldDoc{
				{Name: "fetch", Type: "duration", Description: "Max wait for source data. Accepts Go duration strings: ms, s, m, h.", Validation: "cannot be negative; if non-zero, must be >= 1ms"},
				{Name: "picker", Type: "duration", Description: "Max wait for picker stage source commands. Zero means no timeout.", Validation: "cannot be negative; if non-zero, must be >= 1ms"},
			},
			Example: "[timeout]\nfetch = \"5s\"    # e.g. 500ms, 2s, 1m\npicker = \"2s\"",
		},
		{
			Name:        "sources",
			Description: "Per-source tuning. Available sources: zoxide.",
			Fields: []FieldDoc{
				{Name: "limit", Type: "int", Description: "Max results; 0 = unlimited.", Validation: "cannot be negative"},
				{Name: "min_score", Type: "float64", Description: "Minimum score filter; 0 = disabled.", Validation: "cannot be negative"},
			},
			Example: "[sources.zoxide]\nlimit = 5\nmin_score = 2.5",
		},
		{
			Name:        "display",
			Description: "Path display formatting. Most string values support inline Nerdfont icon\n  aliases (e.g. :nf-dev-github:) which are resolved to glyphs at load time.",
			Fields: []FieldDoc{
				{Name: "shorten_home", Type: "string", Description: "Replace $HOME prefix in display paths; empty string disables. Supports icon aliases."},
				{Name: "truncation_length", Type: "int", Description: "Number of rightmost path segments to display for directory/path rows; 0 disables. Paths with this many or fewer segments are left unchanged. Applied after home shortening and rules.", Validation: "cannot be negative"},
				{Name: "truncation_symbol", Type: "string", Description: "String prepended (with an implied trailing /) when directory/path truncation occurs. Empty string means no prefix. For example, \":nf-cod-ellipsis:\" produces \"\uea7c/foo/bar\". Supports icon aliases."},
				{Name: "tmux_session_truncation_length", Type: "int", Description: "Number of rightmost path segments to display for tmux session keys and window owner labels; 0 disables. Uses shorten_home and rules before truncating. Default: 2.", Validation: "cannot be negative"},
				{Name: "tmux_session_truncation_symbol", Type: "string", Description: "String prepended (with an implied trailing /) when tmux session key truncation occurs. Empty string means no prefix. Supports icon aliases."},
				{Name: "rules", Type: "map[string]string", Description: "Literal substring replacements applied to display paths, including tmux session keys. Keys are substrings to match, values are replacements. Replacement values support icon aliases; match keys do not.", Validation: "match key cannot be empty"},
			},
			Example: "[display]\nshorten_home = \"~\"\ntruncation_length = 3\ntruncation_symbol = \":nf-cod-ellipsis:\"\ntmux_session_truncation_length = 2\ntmux_session_truncation_symbol = \"\"\n\n[display.rules]\n\"github.com\" = \":nf-dev-github:gh\"\n\"~/Code\" = \":nf-cod-folder:Code\"",
		},
	}
}

func RenderHelp() string {
	defaults := DefaultConfig()
	docs := ConfigDocs()

	var b strings.Builder
	b.WriteString("CONFIGURATION REFERENCE\n\n")
	b.WriteString("Config file: $XDG_CONFIG_HOME/cmdk/config.toml\n")
	b.WriteString("    default: ~/.config/cmdk/config.toml\n")
	for _, section := range docs {
		b.WriteString("\n")
		b.WriteString(strings.ToUpper(section.Name))
		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s\n\n", section.Description)

		for _, field := range section.Fields {
			def := defaultValue(defaults, section.Name, field.Name)
			if def != "" {
				fmt.Fprintf(&b, "  %-14s %s (default: %s)\n", field.Name, field.Type, def)
			} else {
				fmt.Fprintf(&b, "  %-14s %s\n", field.Name, field.Type)
			}
			fmt.Fprintf(&b, "      %s\n", field.Description)
			if field.Validation != "" {
				fmt.Fprintf(&b, "      Constraint: %s\n", field.Validation)
			}
		}

		if section.Example != "" {
			b.WriteString("\n  Example:\n")
			for line := range strings.SplitSeq(section.Example, "\n") {
				fmt.Fprintf(&b, "      %s\n", line)
			}
		}
	}

	fmt.Fprint(&b, `
TEMPLATE VARIABLES
  Action commands and stage config fields support Go text/template syntax.

  Available variables (from stack):
      {{.path}}           directory path (for dir-matching actions)
      {{.pane_id}}        tmux pane ID (when --pane-id is set)
      {{.session_id}}     stable tmux session ID (from window or session items)
      {{.session_key}}    cmdk-managed canonical session path when present (from window or session items)
      {{.session_name}}   display-safe tmux session name (from window or session items)
      {{.window_id}}      stable tmux window ID (from window items in the selection stack)
      {{.window_index}}   tmux window index (from window items in the selection stack)
      {{.window_name}}    display-safe tmux window name (from window items in the selection stack)
      {{.launch_path}}    final validated launch directory (final cmd/window_name only)
      {{.launch_basename}} base name of launch_path (final cmd/window_name only)
      {{.session_attached}} tmux attached client count (from session items)
      {{.session_display}}  display string for the selected session
      {{.session_kind}}     session classification; "external" in this phase
      {{.session_windows}}  tmux window count (from session items)
      {{.<key>}}          stage output keyed by the stage's key field

  Available functions:
      {{sq .path}}     shell-safe single-quoting

  Security: variables such as path, session_name, and stage inputs
  come from external sources (tmux, zoxide, user input) and may
  contain shell metacharacters. session_name renders tabs and newlines as
  display glyphs (⇥, ↵), but it is not a stable tmux target and is not
  shell-safe. session_key is a path-like grouping key for cmdk-managed
  sessions, not a tmux target. Use session_id for tmux targets, and wrap
  command arguments with {{sq .var}}, e.g. {{sq .session_name}}, not
  {{.session_name}}. Built-in
  built-in session switch and child-window actions target by session_id and
  shell-quote the tmux target.

  Command environment: shell-mode final commands and launch_path_cmd commands
  run with a clean CMDK_* namespace. cmdk removes inherited CMDK_* variables
  and sets CMDK_* variables for data available to that command, such as
  CMDK_PATH, CMDK_PANE_ID, and CMDK_LAUNCH_PATH when available.

  Session-window actions do not pass action or stage data through tmux
  environment variables. Payload commands in session-window mode should use
  template variables such as {{.launch_path}} in cmd. Interactive
  session-window shells do not receive action-specific CMDK_* variables from
  cmdk.

ATTACH
  cmdk attach enters a cmdk-managed tmux session from outside tmux. It refuses
  to run when $TMUX is set, because it is meant to be the outer entry point into
  tmux rather than a nested tmux command.

  cmdk attach <path> resolves the path using the same session resolver as
  "cmdk session resolve". Without a path argument, [startup].path is required.
  Leading ~/ is expanded before resolving the path.

  If the cmdk-managed session for that path already exists, cmdk attaches to it.
  Otherwise cmdk creates the managed session, sets @cmdk_session_kind and
  @cmdk_session_key, then attaches to the new session.

SESSION WINDOWS
  cmdk session window <path> --new resolves an existing directory, finds or
  creates the cmdk-managed tmux session for that path, creates a fresh shell
  window in that session, and switches the current tmux client to it.

  cmdk session window <path> [--name <name>] -- <command> [args...] creates a
  fresh command window. Command args after -- are treated as argv-style input;
  cmdk shell-quotes each arg before passing one shell-command string to tmux.
  Metacharacters such as $, |, >, and ; are literal by default. Use an explicit
  shell for shell behavior, e.g.:

      cmdk session window . --name tests -- sh -lc 'npm test | tee test.log'

  Command-window lifecycle is direct tmux behavior: when the command exits,
  cmdk does not hold the window, set remain-on-exit, or drop into a shell.

  Repo worktree paths use one managed session per repo/container. Directory
  paths use one managed session per canonical directory. Each invocation creates
  a new tmux window; cmdk does not search for or reuse existing window names.

  Cmdk recognizes managed sessions by the @cmdk_session_key tmux option. When
  cmdk creates a session it sets only @cmdk_session_kind and @cmdk_session_key.
  Managed sessions are found by exact key match.

  session window requires a current tmux client for switch-client. It does not
  attach from outside tmux and does not fall back to attach-session.

  In the TUI, tmux windows sort above sessions, directories, and actions. When
  selecting a tmux session, windows in that session sort above the built-in
  "Switch to session" action and configured session actions.

EXECUTION
  Actions run in one of two launch modes. In session-window mode, cmdk resolves
  the effective launch path, renders the payload command, creates/switches to a
  fresh window in the cmdk-managed tmux session for that path, and runs the
  rendered cmd there via sh -lc. Dir-matching actions default to session-window.
  cmdk does not pass CMDK_* action/stage data to tmux with -e or set it in the
  managed session environment; use template variables in cmd.

  In shell mode, commands are passed to sh -c via syscall.Exec, replacing the
  cmdk process in the current pane. If an effective launch path exists, cmdk
  chdirs there first; shell mode means "do not create a session window", not
  "ignore the directory context".

  Picker source commands and launch_path_cmd commands are also run via sh -c.
  launch_path_cmd must print exactly one absolute existing directory path.

  Root/session actions without launch_path or launch_path_cmd default to shell
  mode and inherit the working directory from where cmdk was launched. Relative
  paths in cmd (e.g. "./scripts/deploy.sh") resolve from that directory unless
  shell mode chdirs to an effective launch path.

  For dir-matching actions, use {{.launch_path}} for the final launch directory
  in payload commands. Use {{.path}} only when you specifically need the
  originally selected directory before launch_path overrides/generation.
`)

	return b.String()
}

func defaultValue(cfg Config, section, field string) string {
	switch section {
	case "behavior":
		switch field {
		case "auto_select_single":
			return fmt.Sprintf("%t", cfg.Behavior.AutoSelectSingle)
		case "bell_to_top":
			return fmt.Sprintf("%t", cfg.Behavior.BellToTop)
		case "wrap_list":
			return fmt.Sprintf("%t", cfg.Behavior.WrapList)
		case "start_in_filter":
			return fmt.Sprintf("%t", cfg.Behavior.StartInFilter)
		case "inline_actions":
			return fmt.Sprintf("%t", cfg.Behavior.InlineActions)
		}
	case "timeout":
		switch field {
		case "fetch":
			return fmt.Sprintf("%q", cfg.Timeout.Fetch.String())
		case "picker":
			return fmt.Sprintf("%q", cfg.Timeout.Picker.String())
		}
	case "sources":
		zoxide := cfg.Sources["zoxide"]
		switch field {
		case "limit":
			return fmt.Sprintf("%d", zoxide.Limit)
		case "min_score":
			return fmt.Sprintf("%g", zoxide.MinScore)
		}
	case "display":
		switch field {
		case "shorten_home":
			return fmt.Sprintf("%q", cfg.Display.ShortenHome)
		case "truncation_length":
			return fmt.Sprintf("%d", cfg.Display.TruncationLength)
		case "truncation_symbol":
			if cfg.Display.TruncationSymbol != "" {
				return fmt.Sprintf("%q", cfg.Display.TruncationSymbol)
			}
		case "tmux_session_truncation_length":
			return fmt.Sprintf("%d", cfg.Display.TmuxSessionTruncationLength)
		case "tmux_session_truncation_symbol":
			if cfg.Display.TmuxSessionTruncationSymbol != "" {
				return fmt.Sprintf("%q", cfg.Display.TmuxSessionTruncationSymbol)
			}
		}
	}
	return ""
}
