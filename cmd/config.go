package cmd

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show resolved configuration and validate a config file",
	Long:  "Show resolved configuration. Exits non-zero if the config is invalid.\n\nSee \"cmdk docs\" for the configuration reference and \"cmdk icons\" for icon aliases.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := resolveConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}

		fmt.Printf("# %s\n", path)
		return toml.NewEncoder(os.Stdout).Encode(cfg)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
