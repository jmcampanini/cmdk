package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Show configuration reference",
	Long:  config.RenderHelp(),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Print(config.RenderHelp())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
