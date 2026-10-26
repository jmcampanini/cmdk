package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	resolver "github.com/jmcampanini/cmdk/internal/session"
)

func resolveSessionPlanWithConfig(cmd *cobra.Command, path string, cfg config.Config) (resolver.Plan, error) {
	ctx, cancel := context.WithTimeout(commandContext(cmd), cfg.Timeout.Fetch)
	defer cancel()
	return resolver.Resolve(ctx, path, cfg.Timeout.Fetch)
}
