package config

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// FieldDoc describes a configuration field's type, meaning, and constraints.
type FieldDoc struct {
	Name        string
	Type        string
	Description string
	Validation  string
}

// SectionDoc groups configuration fields with their description and TOML example.
type SectionDoc struct {
	// Name is the TOML key the fields live under and the key used by the
	// documentation test; Table is the header the rendered reference shows.
	Name        string
	Table       string
	Description string
	Fields      []FieldDoc
	Example     string
}

const themeColorValidation = "optional; must be #RRGGBB hex when set"

func themeColorField(name string, description string) FieldDoc {
	return FieldDoc{Name: name, Type: "string", Description: description, Validation: themeColorValidation}
}

// Docs returns the configuration reference in its display order.
func Docs() []SectionDoc {
	reservedStageKeyValidation := "cannot be empty; must be an identifier (letters, digits, and underscores, " +
		"not starting with a digit); compared case-insensitively, must be unique within the action " +
		"and cannot be reserved (" + strings.Join(reservedStageKeys, ", ") +
		"; session actions also reserve " + strings.Join(sessionActionExtraReservedStageKeys, ", ") + ")"

	return []SectionDoc{
		{
			Name:        "startup",
			Table:       "[startup]",
			Description: "Defaults for entering a cmdk-managed tmux session from outside tmux.",
			Fields: []FieldDoc{
				{
					Name:        "path",
					Type:        "string",
					Description: "Default directory used by \"cmdk attach\" when no path operand is given. A leading ~/ expands to the current user's home directory.",
					Validation:  "optional; cannot contain control characters; must resolve to an existing directory when \"cmdk attach\" runs",
				},
			},
			Example: "[startup]\npath = \"~/Code/github.com/me/project\"",
		},
		{
			Name:        "actions",
			Table:       "[[actions]]",
			Description: "Actions shown in the launcher. Each action declares which item type it matches, how it launches, and an optional stage pipeline that collects action inputs before it runs. See \"cmdk help templates\" for the template variables cmd and the other template fields accept, and \"cmdk help launching\" for launch modes.",
			Fields: []FieldDoc{
				{Name: "name", Type: "string", Description: "Display name in the launcher and the exact name \"cmdk action run\" takes.", Validation: "cannot be empty"},
				{Name: "matches", Type: "string", Description: "Item type this action appears for: \"root\" (top level), \"dir\" (after selecting a directory), or \"session\" (after selecting a tmux session).", Validation: "cannot be empty; must be \"root\", \"dir\", or \"session\""},
				{Name: "cmd", Type: "string", Description: "Shell command template to run after all stages complete. In session-window mode this is the window payload, not a tmux new-window wrapper.", Validation: "cannot be empty"},
				{Name: "icon", Type: "string", Description: "Nerd Font icon: an alias like :nf-dev-github:, a raw glyph, or a \\uXXXX escape. Run \"cmdk icons --filter <term>\" to search aliases.", Validation: "a :nf-*: alias must be in the supported set; anything else must be a single grapheme cluster"},
				{Name: "launch_mode", Type: "string", Description: "\"detect\" (default), \"session-window\" (create a window in a cmdk-managed tmux session), or \"shell\" (replace the cmdk process in the current pane). detect makes dir actions and actions with launch_path or launch_path_cmd session-window; root and session actions without a path field stay shell.", Validation: "optional; must be \"detect\", \"session-window\", or \"shell\""},
				{Name: "launch_path", Type: "string", Description: "Path template for the action's launch directory. A leading ~, $VAR, and ${VAR} expand before template rendering; there is no command substitution, globbing, or word splitting.", Validation: "mutually exclusive with launch_path_cmd; the rendered path must resolve to an existing directory"},
				{Name: "launch_path_cmd", Type: "string", Description: "Shell command template run through sh -c after the stages; its stdout must be exactly one absolute path to an existing directory (at most one line, 8 KiB). Wrap template variables in {{sq ...}}. Runs under timeout.picker. A failure (nonzero exit, timeout, invalid output) opens an error screen in the launcher showing the rendered command, exit status, stdout, and stderr, and writes the same details to the log file.", Validation: "mutually exclusive with launch_path; output must be one absolute existing directory path"},
				{Name: "window_name", Type: "string", Description: "tmux window name template for session-window actions. Defaults to {{.launch_basename}}. Rendered names longer than behavior.window_name_max_length are end-truncated with a trailing ….", Validation: "only valid when the effective launch mode is session-window; the rendered name cannot be empty or contain control characters"},
				{Name: "stages", Type: "array", Description: "Optional pipeline of input stages run before cmd. See the stages table below."},
			},
			Example: "[[actions]]\nname = \"Yazi\"\nmatches = \"root\"\nlaunch_mode = \"shell\"\ncmd = \"tmux split-window -h yazi\"\nicon = \":nf-cod-folder:\"\n\n[[actions]]\nname = \"Claude\"\nmatches = \"dir\"\ncmd = \"direnv exec {{sq .launch_path}} claude\"\n\n[[actions]]\nname = \"Dotfiles pi\"\nmatches = \"root\"\nlaunch_path = \"$HOME/Code/github.com/me/dotfiles/main\"\ncmd = \"pi\"\n\n[[actions]]\nname = \"Rename Session\"\nmatches = \"session\"\ncmd = \"tmux rename-session -t {{sq .session_id}} {{sq .new_name}}\"\nstages = [\n  { type = \"prompt\", text = \"New name:\", key = \"new_name\" },\n]",
		},
		{
			Name:        "stages",
			Table:       "[[actions]] stages",
			Description: "Stages are declared inline in an action's stages array. Each stage collects one action input before the action runs; templates read the value as {{.<key>}}.",
			Fields: []FieldDoc{
				{Name: "type", Type: "string", Description: "\"prompt\" (text input) or \"picker\" (shell command output shown as a fuzzy list).", Validation: "must be \"prompt\" or \"picker\""},
				{Name: "key", Type: "string", Description: "Name of the action input.", Validation: reservedStageKeyValidation},
				{Name: "text", Type: "string", Description: "Prompt label template. Only for type = \"prompt\".", Validation: "required for prompt; forbidden for picker"},
				{Name: "default", Type: "string", Description: "Default value template pre-filled in the prompt. Only for type = \"prompt\".", Validation: "forbidden for picker"},
				{Name: "source", Type: "string", Description: "Shell command template run through sh -c that prints newline-separated entries. Only for type = \"picker\". stdout is capped at 4 MiB and stderr at 64 KiB; stdout beyond the cap fails the source with an error screen instead of a partial list.", Validation: "required for picker; forbidden for prompt"},
				{Name: "delimiter", Type: "string", Description: "Field delimiter that splits each source line into parts. Only for type = \"picker\". Defaults to \"|\" when display or pass is set.", Validation: "forbidden for prompt"},
				{Name: "display", Type: "int", Description: "1-based field shown and matched in the list; 0 (default) uses the whole line. Only for type = \"picker\".", Validation: "forbidden for prompt; cannot be negative"},
				{Name: "pass", Type: "int", Description: "1-based field passed as the stage's value; 0 (default) passes the whole line. Only for type = \"picker\".", Validation: "forbidden for prompt; cannot be negative"},
				{Name: "allow_empty", Type: "bool", Description: "Accept an empty prompt value. When false (default), Enter on an empty or whitespace-only prompt shows an error. Only for type = \"prompt\".", Validation: "forbidden for picker"},
			},
			Example: "stages = [\n  { type = \"prompt\", text = \"Branch:\", key = \"branch\", default = \"feat/\" },\n  { type = \"picker\", source = \"find {{sq .path}} -type f\", key = \"file\" },\n  { type = \"picker\", source = \"cat users.psv\", key = \"user\", pass = 2 },\n]",
		},
		{
			Name:        "behavior",
			Table:       "[behavior]",
			Description: "Launcher navigation and tmux window naming.",
			Fields: []FieldDoc{
				{Name: "auto_select_single", Type: "bool", Description: "Skip the action list when only one action matches."},
				{Name: "bell_to_top", Type: "bool", Description: "Sort tmux windows with bell activity above normal items, after errors and loading placeholders."},
				{Name: "wrap_list", Type: "bool", Description: "Wrap the cursor to the opposite end when moving past the first or last item."},
				{Name: "start_in_filter", Type: "bool", Description: "Open lists in filter mode, ready for typing. When false, lists open in browse mode and / starts filtering."},
				{Name: "inline_actions", Type: "bool", Description: "Expand directory actions inline in the root list instead of requiring drill-down. Each directory gets one entry per action, shown as \"path » action\"."},
				{Name: "window_name_max_length", Type: "int", Description: "Maximum length in characters of tmux window names cmdk creates; longer names are end-truncated with a trailing …, which counts toward the limit. Applies to default names, --name values, and rendered window_name templates. 0 disables truncation.", Validation: "cannot be negative"},
			},
			Example: "[behavior]\nauto_select_single = false\nbell_to_top = true\nwrap_list = false\nstart_in_filter = true\ninline_actions = false\nwindow_name_max_length = 20",
		},
		{
			Name:        "theme",
			Table:       "[theme.light] and [theme.dark]",
			Description: "Per-mode color overrides. cmdk starts from the active built-in theme (light is Catppuccin Latte, dark is Catppuccin Frappe) and applies [theme.light] or [theme.dark] on top. See \"cmdk --help\" for how --theme and --color select the mode. cmdk does not paint a full-screen background; the terminal or tmux popup background stays visible.\n\nTo apply another palette:\n1. Pick the table for the palette's appearance: [theme.dark] for dark palettes or [theme.light] for light ones. Pass --theme=dark or --theme=light when auto-detection is unreliable.\n2. Fill the tokens from the source palette: accent = primary accent; accent_text = a base color that contrasts with accent; cursor = cursor or secondary accent; text = main foreground; muted = comment or overlay text; subtle = divider or dim surface; selected_bg = selection surface; input_bg = input surface; match_bg = low-contrast highlight, often a surface tinted toward accent; info = blue; success = green; secondary = teal or cyan; warning = yellow or orange; error = red.\n3. Leave roles unset unless one icon needs to differ. Roles derive from the tokens: window from accent, dir from info, action from success, session from secondary, unknown and loading from muted, bell from warning, error from error.\n4. Use #RRGGBB values only and check the file with \"cmdk config --validate <path>\".",
			Fields: []FieldDoc{
				themeColorField("accent", "Primary accent for the cmdk badge. Window icons derive from it unless roles.window_icon is set."),
				themeColorField("accent_text", "Text color on accent backgrounds, such as the cmdk badge."),
				themeColorField("cursor", "Filter input cursor color."),
				themeColorField("text", "Primary item and active-filter text color."),
				themeColorField("muted", "Muted and status text color. Unknown, loading, and picker icons derive from it unless role overrides are set."),
				themeColorField("subtle", "Divider, count, and inactive pagination color."),
				themeColorField("selected_bg", "Selected row background."),
				themeColorField("input_bg", "Filter input background."),
				themeColorField("match_bg", "Background of fuzzy-match highlights."),
				themeColorField("info", "Informational color. Directory icons derive from it unless roles.dir_icon is set."),
				themeColorField("success", "Success and action color. Action icons derive from it unless roles.action_icon is set."),
				themeColorField("secondary", "Secondary accent. Session icons derive from it unless roles.session_icon is set."),
				themeColorField("warning", "Warning color. Bell icons derive from it unless roles.bell_icon is set."),
				themeColorField("error", "Error text color. Error icons derive from it unless roles.error_icon is set."),
				themeColorField("roles.window_icon", "Explicit color for tmux window icons."),
				themeColorField("roles.dir_icon", "Explicit color for directory icons."),
				themeColorField("roles.action_icon", "Explicit color for action icons."),
				themeColorField("roles.session_icon", "Explicit color for tmux session icons."),
				themeColorField("roles.unknown_icon", "Explicit color for unknown item and picker icons."),
				themeColorField("roles.loading_icon", "Explicit color for loading placeholder icons."),
				themeColorField("roles.bell_icon", "Explicit color for tmux bell icons."),
				themeColorField("roles.error_icon", "Explicit color for error item icons."),
			},
			Example: "[theme.dark]\n# Catppuccin Frappe-style mapping. For another dark theme, replace\n# these with that palette's equivalent slots.\naccent = \"#ca9ee6\"      # mauve, primary accent\naccent_text = \"#303446\" # base that contrasts with accent\ncursor = \"#babbf1\"      # lavender\ntext = \"#c6d0f5\"        # main foreground\nmuted = \"#838ba7\"       # comments, overlay text\nsubtle = \"#626880\"      # divider, dim surface\nselected_bg = \"#51576d\" # selected row surface\ninput_bg = \"#414559\"    # input surface\nmatch_bg = \"#5b4b8a\"    # low-contrast accent-tinted highlight\ninfo = \"#8caaee\"        # blue\nsuccess = \"#a6d189\"     # green\nsecondary = \"#81c8be\"   # teal\nwarning = \"#e5c890\"     # yellow\nerror = \"#e78284\"       # red\n\n# Override a role only when an icon should break derivation.\n[theme.dark.roles]\n# session_icon = \"#81c8be\"",
		},
		{
			Name:        "timeout",
			Table:       "[timeout]",
			Description: "Deadlines for the external commands cmdk runs. Values are Go duration strings such as 500ms, 2s, or 1m.",
			Fields: []FieldDoc{
				{Name: "fetch", Type: "duration", Description: "Longest wait for read-only queries that populate lists and resolve sessions (tmux list commands, git probes, zoxide). Queries always run under a deadline.", Validation: "must be at least 1ms"},
				{Name: "picker", Type: "duration", Description: "Longest wait for picker stage source commands and launch_path_cmd. Zero means no deadline, so a hanging command holds the launcher until it exits or cmdk is signaled.", Validation: "cannot be negative; when nonzero, must be at least 1ms"},
				{Name: "mutation", Type: "duration", Description: "Longest wait for state-changing tmux commands (new-session, new-window, set-option, switch-client). Mutations always run under a deadline so a wedged tmux server cannot hang cmdk. Attaching to a session is not a mutation and never times out.", Validation: "must be at least 1ms"},
			},
			Example: "[timeout]\nfetch = \"5s\"\npicker = \"2s\"\nmutation = \"5s\"",
		},
		{
			Name:        "sources",
			Table:       "[sources.zoxide]",
			Description: "Per-source tuning. zoxide is the only supported source name; other names are rejected.",
			Fields: []FieldDoc{
				{Name: "limit", Type: "int", Description: "Maximum number of entries; 0 means unlimited.", Validation: "cannot be negative"},
				{Name: "min_score", Type: "float", Description: "Lowest zoxide score an entry needs; 0 disables the filter.", Validation: "cannot be negative"},
			},
			Example: "[sources.zoxide]\nlimit = 5\nmin_score = 2.5",
		},
		{
			Name:        "display",
			Table:       "[display]",
			Description: "Path display formatting. String values support inline Nerd Font icon aliases such as :nf-dev-github:, which resolve to glyphs at load time. Preview the result with \"cmdk shorten\".",
			Fields: []FieldDoc{
				{Name: "shorten_home", Type: "string", Description: "Replacement for the $HOME prefix in display paths; an empty string disables it."},
				{Name: "truncation_length", Type: "int", Description: "Number of rightmost path segments to show for directory rows; 0 disables truncation. Paths with this many or fewer segments are unchanged. Applied after home shortening and rules.", Validation: "cannot be negative"},
				{Name: "truncation_symbol", Type: "string", Description: "Prefix (with an implied trailing /) added when a directory path is truncated. An empty string adds no prefix. For example \":nf-cod-ellipsis:\" renders \"/foo/bar\"."},
				{Name: "tmux_session_truncation_length", Type: "int", Description: "Number of rightmost path segments to show for tmux session keys and window owner labels; 0 disables truncation. shorten_home and rules apply first.", Validation: "cannot be negative"},
				{Name: "tmux_session_truncation_symbol", Type: "string", Description: "Prefix (with an implied trailing /) added when a tmux session key is truncated. An empty string adds no prefix."},
				{Name: "rules", Type: "table of string to string", Description: "Literal substring replacements applied to display paths, including tmux session keys. Keys are the substrings to match and values their replacements. Values support icon aliases; keys do not.", Validation: "a match key cannot be empty"},
			},
			Example: "[display]\nshorten_home = \"~\"\ntruncation_length = 3\ntruncation_symbol = \":nf-cod-ellipsis:\"\ntmux_session_truncation_length = 2\ntmux_session_truncation_symbol = \"\"\n\n[display.rules]\n\"github.com\" = \":nf-dev-github:gh\"\n\"~/Code\" = \":nf-cod-folder:Code\"",
		},
	}
}

// referenceWidth is the widest help line cmdk renders, so the reference reads
// in a default terminal without wrapping mid-word.
const referenceWidth = 80

// RenderReference renders every configuration key with its meaning,
// constraints, and default as wrapped help text for "cmdk config --help".
func RenderReference() string {
	defaults := DefaultConfig()

	var b strings.Builder
	for i, section := range Docs() {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(section.Table)
		b.WriteString("\n")
		writeWrapped(&b, section.Description, "  ")

		for _, field := range section.Fields {
			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s  %s", field.Name, field.Type)
			if def := defaultValue(defaults, section.Name, field.Name); def != "" {
				fmt.Fprintf(&b, " (default %s)", def)
			}
			b.WriteString("\n")
			writeWrapped(&b, field.Description, "      ")
			if field.Validation != "" {
				writeWrapped(&b, "Constraint: "+field.Validation+".", "      ")
			}
		}

		if section.Example != "" {
			b.WriteString("\n  Example:\n")
			for line := range strings.SplitSeq(section.Example, "\n") {
				if line == "" {
					b.WriteString("\n")
					continue
				}
				fmt.Fprintf(&b, "    %s\n", line)
			}
		}
	}
	return b.String()
}

// writeWrapped writes text as paragraphs wrapped to referenceWidth under
// indent. A blank line separates paragraphs; a line that starts with a list
// number such as "1. " starts its own paragraph with a hanging indent.
func writeWrapped(b *strings.Builder, text string, indent string) {
	for i, paragraph := range splitParagraphs(text) {
		if i > 0 && !isListNumber(paragraph) {
			b.WriteString("\n")
		}
		hanging := ""
		if isListNumber(paragraph) {
			hanging = "   "
		}
		for i, line := range wrapWords(paragraph, referenceWidth-len(indent)-len(hanging)) {
			b.WriteString(indent)
			if i > 0 {
				b.WriteString(hanging)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
}

func splitParagraphs(text string) []string {
	var paragraphs []string
	var current []string
	flush := func() {
		if len(current) > 0 {
			paragraphs = append(paragraphs, strings.Join(current, " "))
			current = nil
		}
	}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isListNumber(line) {
			flush()
		}
		if line != "" {
			current = append(current, line)
		}
	}
	flush()
	return paragraphs
}

func isListNumber(line string) bool {
	digits, _, ok := strings.Cut(line, ". ")
	if !ok || digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// wrapWords breaks text on spaces so no line is wider than width; a single
// word wider than width stands on its own line.
func wrapWords(text string, width int) []string {
	var lines []string
	var line strings.Builder
	lineWidth := 0
	for _, word := range strings.Fields(text) {
		wordWidth := ansi.StringWidth(word)
		if lineWidth > 0 && lineWidth+1+wordWidth > width {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}
		if lineWidth > 0 {
			line.WriteByte(' ')
			lineWidth++
		}
		line.WriteString(word)
		lineWidth += wordWidth
	}
	if lineWidth > 0 {
		lines = append(lines, line.String())
	}
	return lines
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
		case "window_name_max_length":
			return fmt.Sprintf("%d", cfg.Behavior.WindowNameMaxLength)
		}
	case "timeout":
		switch field {
		case "fetch":
			return fmt.Sprintf("%q", cfg.Timeout.Fetch.String())
		case "picker":
			return fmt.Sprintf("%q", cfg.Timeout.Picker.String())
		case "mutation":
			return fmt.Sprintf("%q", cfg.Timeout.Mutation.String())
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
