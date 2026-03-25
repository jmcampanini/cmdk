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

func ConfigDocs() []SectionDoc {
	return []SectionDoc{
		{
			Name:        "actions",
			Description: "Actions shown in the launcher. Each action declares which item type\n  it matches and an optional stage pipeline to collect data before execution.",
			Fields: []FieldDoc{
				{Name: "name", Type: "string", Description: "Display name in the launcher.", Validation: "cannot be empty"},
				{Name: "matches", Type: "string", Description: "Item type this action appears for: \"root\" (top-level) or \"dir\" (after selecting a directory).", Validation: "cannot be empty; must be \"root\" or \"dir\""},
				{Name: "cmd", Type: "string", Description: "Shell command or Go template to execute after all stages complete.", Validation: "cannot be empty"},
				{Name: "icon", Type: "string", Description: "Nerdfont icon: alias like :nf-dev-github:, raw glyph, or \\uXXXX escape. Run \"cmdk docs icons\" to list aliases.", Validation: "if :nf-*: alias, must be in supported set; otherwise must be a single grapheme cluster"},
				{Name: "stages", Type: "array", Description: "Optional pipeline of data-collection stages to run before executing cmd. See STAGES section."},
			},
			Example: "[[actions]]\nname = \"Yazi\"\nmatches = \"root\"\ncmd = \"tmux split-window -h yazi\"\nicon = \":nf-md-folder:\"\n\n[[actions]]\nname = \"New Window\"\nmatches = \"dir\"\ncmd = \"tmux new-window -c {{sq .path}}\"\n\n[[actions]]\nname = \"New Branch\"\nmatches = \"dir\"\ncmd = \"git checkout -b {{.branch}}\"\nstages = [\n  { type = \"prompt\", text = \"Branch name:\", key = \"branch\" },\n]",
		},
		{
			Name:        "stages",
			Description: "Stages are declared inline within an action's stages array.\n  Each stage collects one piece of data before the action executes.",
			Fields: []FieldDoc{
				{Name: "type", Type: "string", Description: "Stage type: \"prompt\" (text input) or \"picker\" (shell command → fuzzy list).", Validation: "must be \"prompt\" or \"picker\""},
				{Name: "key", Type: "string", Description: "Template variable name for the stage's output value.", Validation: "cannot be empty; must be unique within action; cannot be reserved (path, pane_id, session, window_index)"},
				{Name: "text", Type: "string", Description: "Prompt label (Go template). Only for type = \"prompt\".", Validation: "required for prompt; forbidden for picker"},
				{Name: "default", Type: "string", Description: "Default value pre-filled in prompt (Go template). Only for type = \"prompt\"."},
				{Name: "source", Type: "string", Description: "Shell command run via sh -c that produces newline-separated entries (Go template). Only for type = \"picker\".", Validation: "required for picker; forbidden for prompt"},
			},
			Example: "stages = [\n  { type = \"prompt\", text = \"Branch name:\", key = \"branch\", default = \"feature/\" },\n  { type = \"picker\", source = \"find {{.path}} -type f\", key = \"file\" },\n]",
		},
		{
			Name:        "behavior",
			Description: "Global behavior settings.",
			Fields: []FieldDoc{
				{Name: "auto_select_single", Type: "bool", Description: "Skip the action list when only one action matches. Default: true."},
				{Name: "bell_to_top", Type: "bool", Description: "Sort tmux windows with bell activity to the top of the list, above all other items."},
			},
			Example: "[behavior]\nauto_select_single = false\nbell_to_top = true",
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
			Description: "Path display formatting.",
			Fields: []FieldDoc{
				{Name: "shorten_home", Type: "string", Description: "Replace $HOME prefix in display paths; empty string disables."},
				{Name: "rules", Type: "map[string]string", Description: "Literal substring replacements applied to display paths. Keys are substrings to match, values are replacements.", Validation: "match key cannot be empty"},
			},
			Example: "[display]\nshorten_home = \"~\"\n\n[display.rules]\n\"github.com\" = \"gh\"",
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
			def := defaultValue(&defaults, section.Name, field.Name)
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
      {{.session}}        tmux session name (from window items in the selection stack)
      {{.window_index}}   tmux window index (from window items in the selection stack)
      {{.<key>}}          stage output keyed by the stage's key field

  Available functions:
      {{sq .path}}     shell-safe single-quoting

  Environment variables CMDK_PATH, CMDK_PANE_ID, etc. are also
  set when executing commands.

EXECUTION
  Commands are passed to sh -c via syscall.Exec, replacing the
  cmdk process in the current pane. Shell features (pipes,
  redirects) are supported.

  Picker source commands are also run via sh -c.

  The working directory is inherited from where cmdk was
  launched, not the config file location. Relative paths in
  cmd (e.g. "./scripts/deploy.sh") resolve from the launch
  directory.

  For dir-matching actions, use {{.path}} to reference the
  selected directory.
`)

	return b.String()
}

func defaultValue(cfg *Config, section, field string) string {
	switch section {
	case "behavior":
		switch field {
		case "bell_to_top":
			return fmt.Sprintf("%t", cfg.Behavior.BellToTop)
		case "auto_select_single":
			return "true"
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
		if field == "shorten_home" && cfg.Display.ShortenHome != nil {
			return fmt.Sprintf("%q", *cfg.Display.ShortenHome)
		}
	}
	return ""
}
