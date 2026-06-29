package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/logging"
)

func newLogsPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the log file path",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := resolveLogPath()
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		},
	}
}

func resolveLogPath() (string, error) {
	path := logging.DefaultLogPath()
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("cannot determine log file location: $HOME is not set")
	}
	return path, nil
}
