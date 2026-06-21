package tmux

import (
	"context"
	"fmt"
	"os/exec"
)

func tmuxOutput(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "tmux", args...).Output()
	if err != nil && ctx.Err() != nil {
		return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
	}
	return out, err
}
