package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

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
--truncate and --truncate-symbol override the configured values for one run
before validation; --truncate-symbol accepts the same icon aliases.

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

	trunc := pathfmt.Truncation{Length: cfg.Display.TruncationLength, Symbol: cfg.Display.TruncationSymbol}
	shortenHome := cfg.Display.ShortenHome
	rules := pathfmt.CompileRules(cfg.Display.Rules)
	home, err := os.UserHomeDir()
	if err != nil && shortenHome != "" {
		return fmt.Errorf("cannot shorten home prefix: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), pathfmt.DisplayPath(path, home, shortenHome, rules, trunc))
	return err
}

func shortenInputPath(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	if term.IsTerminal(os.Stdin.Fd()) {
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
