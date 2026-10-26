package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/icon"
)

type iconOptions struct {
	cod    bool
	dev    bool
	oct    bool
	filter string
	fzf    bool
}

func newIconsCommand(_ *rootOptions) *cobra.Command {
	var options iconOptions
	cmd := &cobra.Command{
		Use:   "icons",
		Short: "List the Nerd Font icon aliases configuration accepts",
		Long: `List the Nerd Font icon aliases cmdk accepts in configuration.

Aliases come from three icon sets:

  cod   Codicons (VS Code icons): UI actions, terminals, files, debugging
  dev   Devicons: language and tool logos
  oct   Octicons (GitHub icons): git workflows, project management

Use an alias in configuration with colon syntax, icon = ":nf-cod-terminal:",
or paste the raw glyph. Display strings such as display.shorten_home and
display.rules values accept aliases inline.

Stdout: the matching aliases grouped by set under a heading per set, one line
per alias with its glyph, alias, and description; --fzf prints one flat line
per alias (glyph, alias, set, description) with no headings for piping into
fzf. With no set flag every set is listed; --cod, --dev, and --oct restrict
the sets, and --filter keeps the aliases whose alias or description contains
the text, case-insensitively. Stderr: when nothing matches, a hint on how to
widen the search, still with exit 0. The command reads no file and never
prompts.`,
		Example: `  cmdk icons
  cmdk icons --cod
  cmdk icons --dev --oct
  cmdk icons --filter terminal
  cmdk icons --cod --filter folder
  cmdk icons --fzf | fzf`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runIconsCommand(options)
		},
	}
	cmd.Flags().BoolVar(&options.cod, "cod", false, "include Codicons")
	cmd.Flags().BoolVar(&options.dev, "dev", false, "include Devicons")
	cmd.Flags().BoolVar(&options.oct, "oct", false, "include Octicons")
	cmd.Flags().StringVar(&options.filter, "filter", "", "keep aliases whose alias or description contains this")
	cmd.Flags().BoolVar(&options.fzf, "fzf", false, "one flat line per alias, for piping to fzf")
	return cmd
}

func runIconsCommand(options iconOptions) error {
	showAll := !options.cod && !options.dev && !options.oct
	filterLower := strings.ToLower(options.filter)
	filtered := filterIconEntries(icon.All(), options, showAll, filterLower)
	if len(filtered) == 0 {
		emitIconsEmptyHint(options)
		return nil
	}

	if options.fzf {
		printIconsFzf(filtered)
		return nil
	}
	printIconsGrouped(filtered)
	return nil
}

func filterIconEntries(entries []icon.Entry, options iconOptions, showAll bool, filterLower string) []icon.Entry {
	var filtered []icon.Entry
	for _, e := range entries {
		set := setFromAlias(e.Alias)
		if !showAll && !matchesSetFlag(set, options) {
			continue
		}
		if filterLower != "" && !matchesFilter(e, filterLower) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func printIconsFzf(entries []icon.Entry) {
	for _, e := range entries {
		set := setFromAlias(e.Alias)
		fmt.Printf("%s  %-40s [%s]  %s\n", e.Icon, ":"+e.Alias+":", set, e.Description)
	}
}

func printIconsGrouped(entries []icon.Entry) {
	var curPrefix string
	for _, e := range entries {
		prefix := aliasPrefix(e.Alias)
		if prefix != curPrefix {
			if curPrefix != "" {
				fmt.Println()
			}
			fmt.Println(prefixHeading(prefix))
			curPrefix = prefix
		}
		fmt.Printf("  %s  %-40s %s\n", e.Icon, ":"+e.Alias+":", e.Description)
	}
}

func emitIconsEmptyHint(options iconOptions) {
	sets := []struct {
		name   string
		active bool
	}{
		{"cod", options.cod},
		{"dev", options.dev},
		{"oct", options.oct},
	}

	var active, missing []string
	for _, s := range sets {
		if s.active {
			active = append(active, s.name)
		} else {
			missing = append(missing, "--"+s.name)
		}
	}

	scope := "sets: all"
	if len(active) > 0 {
		scope = "sets: " + strings.Join(active, ",")
	}

	if options.filter != "" {
		fmt.Fprintf(os.Stderr, "no results for filter=%q (%s)\n", options.filter, scope)
	} else {
		fmt.Fprintf(os.Stderr, "no results (%s)\n", scope)
	}
	fmt.Fprintln(os.Stderr, "try:")
	if len(active) > 0 && len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "  %-24s include other icon sets\n", strings.Join(missing, " "))
	}
	if options.filter != "" {
		fmt.Fprintf(os.Stderr, "  %-24s broader match\n", "--filter=<shorter>")
	}
	fmt.Fprintf(os.Stderr, "  %-24s interactive fuzzy search\n", "cmdk icons --fzf | fzf")
}

func matchesSetFlag(set string, options iconOptions) bool {
	switch set {
	case "cod":
		return options.cod
	case "dev":
		return options.dev
	case "oct":
		return options.oct
	}
	return false
}

func matchesFilter(e icon.Entry, lower string) bool {
	return strings.Contains(strings.ToLower(e.Alias), lower) ||
		strings.Contains(strings.ToLower(e.Description), lower)
}

func setFromAlias(alias string) string {
	parts := strings.SplitN(alias, "-", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func aliasPrefix(alias string) string {
	if set := setFromAlias(alias); set != "" {
		return "nf-" + set
	}
	return alias
}

func prefixHeading(prefix string) string {
	switch prefix {
	case "nf-cod":
		return "Codicons (cod)"
	case "nf-dev":
		return "Devicons (dev)"
	case "nf-oct":
		return "Octicons (oct)"
	default:
		return prefix
	}
}
