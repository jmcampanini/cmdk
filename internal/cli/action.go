package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/safetext"
)

func newActionCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action",
		Short: "Run configured actions noninteractively",
		Long: `Run configured actions without opening the interactive picker.

Only configured root and directory actions whose effective launch mode is
session-window are supported.`,
		Args: cobra.NoArgs,
		RunE: runHelp,
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return terminalSafeActionRunError(err)
	})
	cmd.AddCommand(newActionRunCommand(root))
	return cmd
}

type actionRunError struct {
	cause error
}

func (e *actionRunError) Error() string {
	return safetext.EscapeTerminalControls(e.cause.Error())
}

func (e *actionRunError) Unwrap() error {
	return e.cause
}

func terminalSafeActionRunError(err error) error {
	if err == nil {
		return nil
	}
	return &actionRunError{cause: err}
}
