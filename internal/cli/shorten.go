package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

func newShortenCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shorten [path]",
		Short: "Preview how the launcher displays a path",
		Long: `Preview how the launcher displays a path.

Stdout: one line, the display form of the path. With no operand cmdk reads
the path from the first line of stdin, which must not be a terminal. Stderr:
diagnostics; no operand with a terminal or empty stdin exits 1. The command
reads no file but the configuration and never prompts.

How paths are formatted:

  1. Replace the $HOME prefix with display.shorten_home.
  2. Apply display.rules literal substring replacements.
  3. Keep the rightmost display.truncation_length segments, prefixed with
     display.truncation_symbol.

Rules replace the first occurrence of each literal match. Longer rule keys run
before shorter keys, with lexical order as the tie-breaker. Replacement values
accept icon aliases such as :nf-dev-github:; match keys are literal.
--truncate and --truncate-symbol override the configured values for one run.

` + configLoadingHelpWithKeys,
		Example: `  cmdk shorten /Users/me/Code/github.com/acme/project
  echo /usr/local/bin/foo | cmdk shorten
  cmdk shorten --truncate 2 /usr/local/bin/foo

  # With this configuration:
  #   [display]
  #   shorten_home = "~"
  #   truncation_length = 3
  #   truncation_symbol = "…"
  #
  #   [display.rules]
  #   "github.com" = "gh"
  #   "~/Code" = "Code"
  #
  # cmdk shorten /Users/me/Code/github.com/acme/project
  # prints …/gh/acme/project`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShortenCommand(root, cmd, args)
		},
	}
	cmd.Flags().Int("truncate", 0, "rightmost segments to keep; 0 disables")
	cmd.Flags().String("truncate-symbol", "", "prefix of a truncated path; / is implied")
	return cmd
}

func runShortenCommand(root *rootOptions, cmd *cobra.Command, args []string) error {
	path, err := shortenInputPath(args)
	if err != nil {
		return err
	}

	loaded, err := root.loadConfig(cmd)
	if err != nil {
		return err
	}
	cfg := loaded.config

	trunc, err := shortenTruncation(cmd, cfg)
	if err != nil {
		return err
	}
	shortenHome := cfg.Display.ShortenHome
	rules := pathfmt.CompileRules(cfg.Display.Rules)
	home, err := os.UserHomeDir()
	if err != nil && shortenHome != "" {
		return fmt.Errorf("cannot shorten home prefix: %w", err)
	}

	fmt.Println(pathfmt.DisplayPath(path, home, shortenHome, rules, trunc))
	return nil
}

func shortenInputPath(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	stat, err := os.Stdin.Stat()
	if err != nil || stat.Mode()&os.ModeCharDevice != 0 {
		return "", errors.New("no path operand and stdin is a terminal; pass a path or pipe one line to stdin")
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return "", errors.New("no input on stdin")
	}
	return scanner.Text(), nil
}

func shortenTruncation(cmd *cobra.Command, cfg config.Config) (pathfmt.Truncation, error) {
	trunc := pathfmt.Truncation{Length: cfg.Display.TruncationLength, Symbol: cfg.Display.TruncationSymbol}
	if cmd.Flags().Changed("truncate") {
		v, err := cmd.Flags().GetInt("truncate")
		if err != nil {
			return pathfmt.Truncation{}, fmt.Errorf("reading --truncate flag: %w", err)
		}
		if v < 0 {
			return pathfmt.Truncation{}, fmt.Errorf("--truncate cannot be negative")
		}
		trunc.Length = v
	}
	if cmd.Flags().Changed("truncate-symbol") {
		v, err := cmd.Flags().GetString("truncate-symbol")
		if err != nil {
			return pathfmt.Truncation{}, fmt.Errorf("reading --truncate-symbol flag: %w", err)
		}
		trunc.Symbol = v
	}
	return trunc, nil
}
