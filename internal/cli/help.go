package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newHelpCommand() *cobra.Command {
	var target *cobra.Command
	return &cobra.Command{
		Use:   "help [command ...]",
		Short: "Show help for a command or topic",
		Long: `Show the root help, a command's help, or a help topic.

Name each level of a nested command as a separate operand. An unknown name
exits 2 with a diagnostic on stderr and no help output.

` + helpOutputContract,
		Example: `  cmdk help
  cmdk help session window
  cmdk help launching`,
		Args: func(cmd *cobra.Command, args []string) error {
			var err error
			target, err = findHelpTarget(cmd.Root(), args)
			return err
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
			parent, err := findHelpTarget(cmd.Root(), args)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var matches []string
			for _, child := range parent.Commands() {
				if !child.Hidden && strings.HasPrefix(child.Name(), prefix) {
					matches = append(matches, cobra.CompletionWithDesc(child.Name(), child.Short))
				}
			}
			return matches, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			target.InitDefaultHelpFlag()
			return target.Help()
		},
	}
}

func findHelpTarget(root *cobra.Command, args []string) (*cobra.Command, error) {
	target := root
	for _, name := range args {
		var next *cobra.Command
		for _, child := range target.Commands() {
			if child.Name() == name || child.HasAlias(name) {
				next = child
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("unknown help topic %q; run %q to list commands and topics", strings.Join(args, " "), root.Name()+" --help")
		}
		target = next
	}
	return target, nil
}
