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

	var overrides config.Overrides
	if cmd.Flags().Changed("truncate") {
		value, err := cmd.Flags().GetInt("truncate")
		if err != nil {
			return loadedConfig{}, fmt.Errorf("read --truncate: %w", err)
		}
		overrides.TruncationLength = &value
	}
	if cmd.Flags().Changed("truncate-symbol") {
		value, err := cmd.Flags().GetString("truncate-symbol")
		if err != nil {
			return loadedConfig{}, fmt.Errorf("read --truncate-symbol: %w", err)
		}
		overrides.TruncationSymbol = &value
	}

	cfg, report, err := config.Load(path, overrides)
	if err != nil {
		return loadedConfig{}, fmt.Errorf("loading config: %w", err)
	}
	return loadedConfig{config: cfg, path: path, report: report}, nil
}

// configLoadingHelp is the configuration contract every command that loads
// configuration composes into its long description, so the commands cannot
// drift apart.
const configLoadingHelp = `Configuration: cmdk reads --config when given, otherwise
$XDG_CONFIG_HOME/cmdk/config.toml, or ~/.config/cmdk/config.toml when
XDG_CONFIG_HOME is unset, empty, or not absolute. An absent discovered file is
not an error. A --config file that does not exist, or a malformed or invalid
file from either source, exits 1.`

// configLoadingHelpWithKeys adds the pointer to the key reference for every
// command other than config itself.
const configLoadingHelpWithKeys = configLoadingHelp + `
See "cmdk config --help" for the keys.`
