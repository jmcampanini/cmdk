package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

// Process exit statuses. Usage errors exit 2 so a script can tell a wrong
// invocation from a command that ran and failed.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

// usageError marks an error cobra raised before any command work: an unknown
// flag, or operands that failed the command's Args validator.
type usageError struct {
	err error
}

func (e *usageError) Error() string {
	return e.err.Error()
}

func (e *usageError) Unwrap() error {
	return e.err
}

func markUsageError(err error) error {
	if err == nil {
		return nil
	}
	return &usageError{err: err}
}

// unknownCommandPrefix is the text cobra's command discovery (Find, via
// legacyArgs) returns for an unknown subcommand of the root. Cobra reports it
// before the root's flag parsing and before any Args validator runs, so it is
// the one usage error that markUsageErrors cannot wrap.
const unknownCommandPrefix = "unknown command "

// isUsageError reports whether err is a usage error: one wrapped by
// markUsageErrors or the root's FlagErrorFunc, or cobra's own unknown-command
// error from command discovery.
func isUsageError(err error) bool {
	var usage *usageError
	return errors.As(err, &usage) || strings.HasPrefix(err.Error(), unknownCommandPrefix)
}

// exitStatus maps a command error to the process exit status.
func exitStatus(err error) int {
	if err == nil {
		return exitSuccess
	}
	if isUsageError(err) {
		return exitUsage
	}
	return exitFailure
}

// markUsageErrors wraps every Args validator in the tree so a positional
// grammar failure is reported as a usage error. Flag errors are marked by the
// FlagErrorFunc each group inherits from the root or sets itself.
func markUsageErrors(cmd *cobra.Command) {
	if args := cmd.Args; args != nil {
		cmd.Args = func(cmd *cobra.Command, operands []string) error {
			return markUsageError(args(cmd, operands))
		}
	}
	for _, child := range cmd.Commands() {
		markUsageErrors(child)
	}
}
