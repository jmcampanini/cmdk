package cli

import (
	"fmt"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
)

// loadedConfig is the configuration a command runs with, the file it came
// from, and where each value was set.
type loadedConfig struct {
	config config.Config
	path   string
	report configloader.LoadReport
}

// loadConfig loads the configuration every command runs with: the --config
// file when the person supplied one, otherwise the discovered file, which may
// be absent. A malformed or invalid file is an error for every command. Cobra
// merges the root's persistent flags into cmd.Flags() before RunE runs, so the
// executing command sees whether --config was supplied.
func (root *rootOptions) loadConfig(cmd *cobra.Command) (loadedConfig, error) {
	path, err := config.ResolvePath(root.configPath, cmd.Flags().Changed("config"))
	if err != nil {
		return loadedConfig{}, err
	}
	cfg, report, err := config.Load(path)
	if err != nil {
		return loadedConfig{}, fmt.Errorf("loading config: %w", err)
	}
	return loadedConfig{config: cfg, path: path, report: report}, nil
}
