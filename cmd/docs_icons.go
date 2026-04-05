package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/icon"
)

var (
	flagIconCod    bool
	flagIconDev    bool
	flagIconOct    bool
	flagIconFilter string
	flagIconFzf    bool
)

var docsIconsCmd = &cobra.Command{
	Use:   "icons",
	Short: "List supported icon aliases",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasSetFlag := flagIconCod || flagIconDev || flagIconOct
		hasAnyFlag := hasSetFlag || flagIconFilter != "" || flagIconFzf

		if !hasAnyFlag {
			printIconHelp()
			return nil
		}

		showAll := !hasSetFlag

		entries := icon.All()
		filterLower := strings.ToLower(flagIconFilter)

		var filtered []icon.Entry
		for _, e := range entries {
			set := setFromAlias(e.Alias)
			if !showAll && !matchesSetFlag(set) {
				continue
			}
			if filterLower != "" && !matchesFilter(e, filterLower) {
				continue
			}
			filtered = append(filtered, e)
		}

		if len(filtered) == 0 {
			fmt.Fprintln(os.Stderr, "No icons matched the given flags/filter.")
			return nil
		}

		if flagIconFzf {
			for _, e := range filtered {
				set := setFromAlias(e.Alias)
				fmt.Printf("%s  %-40s [%s]  %s\n", e.Icon, ":"+e.Alias+":", set, e.Description)
			}
			return nil
		}

		var curPrefix string
		for _, e := range filtered {
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
		return nil
	},
}

func matchesSetFlag(set string) bool {
	switch set {
	case "cod":
		return flagIconCod
	case "dev":
		return flagIconDev
	case "oct":
		return flagIconOct
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
	parts := strings.SplitN(alias, "-", 3)
	if len(parts) >= 2 {
		return parts[0] + "-" + parts[1]
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

  cmdk docs icons --cod                    Show all Codicons
  cmdk docs icons --dev --oct              Show Devicons and Octicons
  cmdk docs icons --filter terminal        Search all sets for "terminal"
  cmdk docs icons --cod --filter folder    Search Codicons for "folder"
  cmdk docs icons --fzf                    Flat output for piping to fzf
  cmdk docs icons --fzf | fzf             Interactive fuzzy search

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

func init() {
	docsIconsCmd.Flags().BoolVar(&flagIconCod, "cod", false, "Show Codicons")
	docsIconsCmd.Flags().BoolVar(&flagIconDev, "dev", false, "Show Devicons")
	docsIconsCmd.Flags().BoolVar(&flagIconOct, "oct", false, "Show Octicons")
	docsIconsCmd.Flags().StringVar(&flagIconFilter, "filter", "", "Filter by substring match on alias or description")
	docsIconsCmd.Flags().BoolVar(&flagIconFzf, "fzf", false, "Flat output for piping to fzf")
	docsCmd.AddCommand(docsIconsCmd)
}
