package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
)

func newDocsCommand(_ *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Show configuration reference",
		Long:  config.RenderHelp(),
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Print(config.RenderHelp())
			return nil
		},
	}
}
