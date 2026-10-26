package cli

import (
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/safetext"
)

func newActionCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action",
		Short: "Run configured actions noninteractively",
		Long: `Run configured actions without opening the launcher.

Only root and dir actions whose effective launch mode is session-window can
run this way; see "cmdk help launching" for the modes. A bare "cmdk action"
prints this help and exits 0.`,
		Args: cobra.NoArgs,
		RunE: runHelp,
	}
	// Flag errors under this group stay usage errors; the group only escapes
	// terminal controls in the flag text before the message reaches stderr.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return terminalSafeActionRunError(markUsageError(err))
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
