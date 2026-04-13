package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/246859/tail"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/logging"
)

const maxTailLines = 10000

var tailLines int

func readTail(f *os.File, n int) ([]byte, error) {
	data, err := tail.Tail(f, n+1)
	if err != nil {
		return nil, fmt.Errorf("reading log file tail: %w", err)
	}
	// The tail library treats \n as a separator, so a file ending in \n
	// has an empty trailing segment that eats one of our n requested lines.
	// Over-fetch by one and trim back to compensate.
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return bytes.Join(lines, nil), nil
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Inspect cmdk log file",
}

var logsPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the log file path",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := logging.DefaultLogPath()
		if !filepath.IsAbs(path) {
			return fmt.Errorf("cannot determine log file location: $HOME is not set")
		}
		fmt.Println(path)
		return nil
	},
}

var logsTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Print the last lines of the log file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if tailLines <= 0 {
			return fmt.Errorf("line count must be positive, got %d", tailLines)
		}
		if tailLines > maxTailLines {
			return fmt.Errorf("line count must be at most %d, got %d", maxTailLines, tailLines)
		}

		path := logging.DefaultLogPath()
		if !filepath.IsAbs(path) {
			return fmt.Errorf("cannot determine log file location: $HOME is not set")
		}
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("log file does not exist: %s", path)
			}
			return fmt.Errorf("could not open log file %s: %w", path, err)
		}
		defer func() { _ = f.Close() }()

		data, err := readTail(f, tailLines)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

func init() {
	logsTailCmd.Flags().IntVarP(&tailLines, "lines", "n", 25, "number of lines to display")
	logsCmd.AddCommand(logsPathCmd)
	logsCmd.AddCommand(logsTailCmd)
	rootCmd.AddCommand(logsCmd)
}
