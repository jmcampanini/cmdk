package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	resolver "github.com/jmcampanini/cmdk/internal/session"
)

func resolveSessionPlanWithConfig(cmd *cobra.Command, path string, cfg config.Config) (resolver.Plan, error) {
	ctx, cancel := sessionResolveContext(cmd, cfg)
	defer cancel()

	plan, err := resolver.Resolve(ctx, path, cfg.Timeout.EffectiveFetch())
	if err != nil {
		return resolver.Plan{}, err
	}
	return plan, nil
}

func sessionResolveContext(cmd *cobra.Command, cfg config.Config) (context.Context, context.CancelFunc) {
	return context.WithTimeout(commandContext(cmd), cfg.Timeout.EffectiveFetch())
}
