package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

var shortenCmd = &cobra.Command{
	Use:   "shorten [path]",
	Short: "Apply display rules to shorten a path",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var path string
		if len(args) == 1 {
			path = args[0]
		} else {
			stat, err := os.Stdin.Stat()
			if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
				return fmt.Errorf("usage: cmdk shorten <path> or echo <path> | cmdk shorten")
			}
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				return fmt.Errorf("no input on stdin")
			}
			path = scanner.Text()
		}

		cfgPath, err := resolveConfigPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		shortenHome := *cfg.Display.ShortenHome
		trunc := pathfmt.Truncation{Length: cfg.Display.TruncationLength, Symbol: cfg.Display.TruncationSymbol}
		if cmd.Flags().Changed("truncate") {
			trunc.Length, _ = cmd.Flags().GetInt("truncate")
			if trunc.Length < 0 {
				return fmt.Errorf("--truncate cannot be negative")
			}
		}
		if cmd.Flags().Changed("truncate-symbol") {
			trunc.Symbol, _ = cmd.Flags().GetString("truncate-symbol")
		}
		rules := pathfmt.CompileRules(cfg.Display.Rules)
		home, err := os.UserHomeDir()
		if err != nil && shortenHome != "" {
			return fmt.Errorf("cannot shorten home prefix: %w", err)
		}

		fmt.Println(pathfmt.DisplayPath(path, home, shortenHome, rules, trunc))
		return nil
	},
}

func init() {
	shortenCmd.Flags().Int("truncate", 0, "number of rightmost path segments to display (overrides config)")
	shortenCmd.Flags().String("truncate-symbol", "", "string prepended (with implied trailing /) when truncation occurs (overrides config)")
	rootCmd.AddCommand(shortenCmd)
}
