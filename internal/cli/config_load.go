package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
)

// loadConfig loads the configuration every command runs with: the --config
// file when the person supplied one, otherwise the discovered file, which may
// be absent. A malformed or invalid file is an error for every command.
func (root *rootOptions) loadConfig(cmd *cobra.Command) (config.Config, error) {
	path, err := root.configFilePath(cmd)
	if err != nil {
		return config.Config{}, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// configFilePath resolves the --config flag as the executing command sees it;
// cobra merges the root's persistent flags into cmd.Flags() before RunE runs.
func (root *rootOptions) configFilePath(cmd *cobra.Command) (string, error) {
	return config.ResolvePath(root.configPath, cmd.Flags().Changed("config"))
}
