package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/246859/tail"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/logging"
)

const maxTailLines = 10000

var tailLines int

func readTail(f *os.File, n int) ([]byte, error) {
	data, err := tail.Tail(f, n+1)
	if err != nil {
		return nil, err
	}
	// The library counts a trailing newline as an extra line,
	// so we request n+1 and trim back to n.
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
		fmt.Println(logging.DefaultLogPath())
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
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("log file does not exist: %s", path)
			}
			return err
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
