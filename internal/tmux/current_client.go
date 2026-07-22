package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

var currentClientPaneFormat = tmuxFormatFields(
	tmuxEscapedFormat("client_name"),
	"#{pane_id}",
)

func CurrentClientPane(ctx context.Context, timeout time.Duration) (string, error) {
	return currentClientPane(ctx, timeout, execTmuxRunner{})
}

func currentClientPane(ctx context.Context, timeout time.Duration, runner tmuxRunner) (string, error) {
	if os.Getenv("TMUX") == "" {
		return "", errors.New("no current tmux client; TMUX is not set")
	}
	envPaneID, err := validatePaneID("TMUX_PANE", os.Getenv("TMUX_PANE"))
	if err != nil {
		return "", fmt.Errorf("no current tmux client: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	res, err := runner.query(ctx, tmuxQuerySpec(
		cmdrun.ShapeSingleLine,
		timeout,
		"display-message", "-p", currentClientPaneFormat,
	))
	if err != nil {
		return "", fmt.Errorf("current tmux client: %w", err)
	}

	lines := tmuxLines(res.Stdout)
	if len(lines) != 1 {
		return "", fmt.Errorf("current tmux client: expected 1 row, got %d", len(lines))
	}
	fields, ok := splitTmuxFields(lines[0], 2)
	if !ok {
		return "", errors.New("current tmux client: expected client_name and pane_id")
	}
	if fields[0] == "" {
		return "", errors.New("no current tmux client; run cmdk action from inside an attached tmux client")
	}
	paneID, err := validatePaneID("current tmux client pane_id", fields[1])
	if err != nil {
		return "", err
	}
	if paneID != envPaneID {
		return "", fmt.Errorf("current tmux client pane_id %q does not match TMUX_PANE %q", paneID, envPaneID)
	}
	return paneID, nil
}
