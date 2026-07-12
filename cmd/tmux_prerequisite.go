package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/tmux"
)

var checkTmuxPrerequisite = tmux.CheckPrerequisite

func requireTmux(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return checkTmuxPrerequisite(ctx)
}
