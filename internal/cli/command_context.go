package cli

import (
	"context"

	"github.com/spf13/cobra"
)

// commandContext returns the command's context, which cobra only populates
// during Execute; direct callers (tests) get a background context.
func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
