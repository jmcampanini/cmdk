package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/icon"
)

var docsIconsCmd = &cobra.Command{
	Use:   "icons",
	Short: "List supported icon aliases",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries := icon.All()

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
			fmt.Printf("  %s  %-30s %s\n", e.Icon, ":"+e.Alias+":", e.Description)
		}
		return nil
	},
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
	case "nf-dev":
		return "Dev Tools / Languages (nf-dev-*)"
	case "nf-md":
		return "Material Design (nf-md-*)"
	default:
		return prefix
	}
}

func init() {
	docsCmd.AddCommand(docsIconsCmd)
}
