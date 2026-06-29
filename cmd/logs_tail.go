package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/246859/tail"
	"github.com/spf13/cobra"
)

const maxTailLines = 10000

type logsTailOptions struct {
	lines int
}

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

func newLogsTailCommand() *cobra.Command {
	var options logsTailOptions
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Print the last lines of the log file",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runLogsTailCommand(options)
		},
	}
	cmd.Flags().IntVarP(&options.lines, "lines", "n", 25, "number of lines to display")
	return cmd
}

func runLogsTailCommand(options logsTailOptions) error {
	if options.lines <= 0 {
		return fmt.Errorf("line count must be positive, got %d", options.lines)
	}
	if options.lines > maxTailLines {
		return fmt.Errorf("line count must be at most %d, got %d", maxTailLines, options.lines)
	}

	path, err := resolveLogPath()
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("log file does not exist: %s", path)
		}
		return fmt.Errorf("could not open log file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := readTail(f, options.lines)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}
