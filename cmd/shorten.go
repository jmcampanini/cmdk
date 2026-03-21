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
			stat, _ := os.Stdin.Stat()
			if stat.Mode()&os.ModeCharDevice != 0 {
				return fmt.Errorf("usage: cmdk shorten <path> or echo <path> | cmdk shorten")
			}
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("no input on stdin")
			}
			path = scanner.Text()
		}

		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		shortenHome := *cfg.Display.ShortenHome
		rules := pathfmt.CompileRules(cfg.Display.Rules)

		fmt.Println(pathfmt.DisplayPath(path, shortenHome, rules))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(shortenCmd)
}
