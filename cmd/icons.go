package cmd

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

func newIconsCommand() *cobra.Command {
	var options iconOptions
	cmd := &cobra.Command{
		Use:   "icons",
		Short: "List supported icon aliases",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runIconsCommand(options)
		},
	}
	cmd.Flags().BoolVar(&options.cod, "cod", false, "Show Codicons")
	cmd.Flags().BoolVar(&options.dev, "dev", false, "Show Devicons")
	cmd.Flags().BoolVar(&options.oct, "oct", false, "Show Octicons")
	cmd.Flags().StringVar(&options.filter, "filter", "", "Filter by substring match on alias or description")
	cmd.Flags().BoolVar(&options.fzf, "fzf", false, "Flat output for piping to fzf")
	return cmd
}

func runIconsCommand(options iconOptions) error {
	hasSetFlag := options.cod || options.dev || options.oct
	hasAnyFlag := hasSetFlag || options.filter != "" || options.fzf
	if !hasAnyFlag {
		printIconHelp()
		return nil
	}

	showAll := !hasSetFlag
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

func printIconHelp() {
	counts := iconSetCounts()
	total := counts["cod"] + counts["dev"] + counts["oct"]

	fmt.Printf(`ICON ALIASES

  cmdk supports %d Nerdfont icon aliases from three icon sets:

    cod   Codicons (VS Code icons)    — UI actions, terminals, files, debug
    dev   Devicons                    — language and tool logos
    oct   Octicons (GitHub icons)     — git workflows, project management

  Use aliases in config with colon syntax: icon = ":nf-cod-terminal:"
  Or use raw unicode characters directly: icon = "`+"\uebc8"+`"

USAGE

  cmdk icons --cod                    Show all Codicons
  cmdk icons --dev --oct              Show Devicons and Octicons
  cmdk icons --filter terminal        Search all sets for "terminal"
  cmdk icons --cod --filter folder    Search Codicons for "folder"
  cmdk icons --fzf                    Flat output for piping to fzf
  cmdk icons --fzf | fzf             Interactive fuzzy search

FLAGS

  --cod           Show Codicons (%d icons)
  --dev           Show Devicons (%d icons)
  --oct           Show Octicons (%d icons)
  --filter STR    Case-insensitive substring match on alias and description
  --fzf           Flat output (one line per icon, no headers)
`, total, counts["cod"], counts["dev"], counts["oct"])
}

func iconSetCounts() map[string]int {
	counts := make(map[string]int)
	for _, e := range icon.All() {
		counts[setFromAlias(e.Alias)]++
	}
	return counts
}
