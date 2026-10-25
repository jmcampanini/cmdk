package tmux

import "github.com/jmcampanini/cmdk/internal/config"

// TimeoutsFromConfig maps the configured timeouts onto the tmux package's
// per-command deadlines.
func TimeoutsFromConfig(cfg config.Config) Timeouts {
	return Timeouts{
		Query:    cfg.Timeout.EffectiveFetch(),
		Mutation: cfg.Timeout.EffectiveMutation(),
	}
}
