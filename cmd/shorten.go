package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

func newShortenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shorten [path]",
		Short: "Preview display path shortening",
		Long: `Preview how cmdk displays paths in the launcher.

If no path argument is supplied, cmdk reads the path from the first line of
stdin. This is useful for testing display config before opening the TUI.

How paths are formatted:

  1. Replace the $HOME prefix with display.shorten_home.
  2. Apply display.rules literal substring replacements.
  3. Apply truncation using display.truncation_length and display.truncation_symbol.

Rules replace the first occurrence of each literal match. Longer rule keys run
before shorter keys, with lexical ordering used as a tie-breaker. Replacement
values support icon aliases such as :nf-dev-github:; match keys are literal and
do not resolve aliases. See "cmdk docs" for the full display config reference.`,
		Example: `  cmdk shorten /Users/me/Code/github.com/acme/project
  echo /usr/local/bin/foo | cmdk shorten
  cmdk shorten --truncate 2 /usr/local/bin/foo

  # Example config:
  #   [display]
  #   shorten_home = "~"
  #   truncation_length = 3
  #   truncation_symbol = "…"
  #
  #   [display.rules]
  #   "github.com" = "gh"
  #   "~/Code" = "Code"
  #
  # With that config:
  #   cmdk shorten /Users/me/Code/github.com/acme/project
  #   => …/gh/acme/project`,
		Args: cobra.MaximumNArgs(1),
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
			shortenHome := cfg.Display.ShortenHome
			trunc := pathfmt.Truncation{Length: cfg.Display.TruncationLength, Symbol: cfg.Display.TruncationSymbol}
			if cmd.Flags().Changed("truncate") {
				v, err := cmd.Flags().GetInt("truncate")
				if err != nil {
					return fmt.Errorf("reading --truncate flag: %w", err)
				}
				if v < 0 {
					return fmt.Errorf("--truncate cannot be negative")
				}
				trunc.Length = v
			}
			if cmd.Flags().Changed("truncate-symbol") {
				v, err := cmd.Flags().GetString("truncate-symbol")
				if err != nil {
					return fmt.Errorf("reading --truncate-symbol flag: %w", err)
				}
				trunc.Symbol = v
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
	cmd.Flags().Int("truncate", 0, "number of rightmost path segments to display; 0 disables (overrides config)")
	cmd.Flags().String("truncate-symbol", "", "string prepended (with implied trailing /) when truncation occurs (overrides config)")
	return cmd
}
