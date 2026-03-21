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
			Name:        "commands",
			Description: "Static commands shown in the root list.",
			Fields: []FieldDoc{
				{Name: "name", Type: "string", Description: "Display name in the launcher.", Validation: "cannot be empty"},
				{Name: "cmd", Type: "string", Description: "Shell command or Go template to execute.", Validation: "cannot be empty"},
			},
			Example: "[[commands]]\nname = \"htop\"\ncmd = \"htop\"",
		},
		{
			Name:        "dir_commands",
			Description: "Additional actions shown when a directory is selected.",
			Fields: []FieldDoc{
				{Name: "name", Type: "string", Description: "Display name in the action list.", Validation: "cannot be empty"},
				{Name: "cmd", Type: "string", Description: "Shell command or Go template to execute.", Validation: "cannot be empty"},
			},
			Example: "[[dir_commands]]\nname = \"Yazi\"\ncmd = \"tmux split-window -h yazi {{sq .path}}\"",
		},
		{
			Name:        "timeout",
			Description: "Timeouts for async operations.",
			Fields: []FieldDoc{
				{Name: "fetch", Type: "duration", Description: "Max wait for source data.", Validation: "must be >= 1ms if non-zero"},
			},
		},
		{
			Name:        "sources",
			Description: "Per-source tuning. Key is the source name (e.g. \"zoxide\").",
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
				{Name: "rules", Type: "map[string]string", Description: "Substring replacements applied to display paths. Keys are match patterns, values are replacements.", Validation: "match key cannot be empty"},
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
  Commands support Go text/template syntax.

  Available variables:
      {{.path}}        directory path (for dir_commands)
      {{.pane_id}}     tmux pane ID (when --pane-id is set)

  Available functions:
      {{sq .value}}    shell-safe single-quoting

  Environment variables CMDK_PATH, CMDK_PANE_ID, etc. are also
  set when executing commands.
`)

	return b.String()
}

func defaultValue(cfg *Config, section, field string) string {
	switch section {
	case "timeout":
		if field == "fetch" {
			return fmt.Sprintf("%q", cfg.Timeout.Fetch.String())
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
