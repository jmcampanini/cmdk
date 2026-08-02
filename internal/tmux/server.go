package tmux

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

// RequireRunningServer verifies a tmux server is reachable before any
// side-effectful work starts. list-sessions is a query, so a missing server
// fails instead of being started.
func RequireRunningServer(ctx context.Context, timeout time.Duration) error {
	return requireRunningServer(ctx, timeout, execTmuxRunner{})
}

func requireRunningServer(ctx context.Context, timeout time.Duration, runner tmuxRunner) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := runner.query(ctx, tmuxQuerySpec(cmdrun.ShapeLines, timeout, "list-sessions", "-F", "#{session_id}"))
	if err == nil {
		return nil
	}
	if isTmuxListSessionsUnavailable(err) {
		return errors.New("no running tmux server; cmdk does not start one")
	}
	return fmt.Errorf("checking for a running tmux server: %w", err)
}
