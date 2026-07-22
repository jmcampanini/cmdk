package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

var currentClientFormat = tmuxFormatFields(
	tmuxEscapedFormat("client_name"),
	"#{pane_id}",
)

type ClientTarget struct {
	Name   string
	PaneID string
}

func CurrentClient(ctx context.Context, timeout time.Duration) (ClientTarget, error) {
	return currentClient(ctx, timeout, execTmuxRunner{})
}

func currentClient(ctx context.Context, timeout time.Duration, runner tmuxRunner) (ClientTarget, error) {
	if os.Getenv("TMUX") == "" {
		return ClientTarget{}, errors.New("no current tmux client; TMUX is not set")
	}
	envPaneID, err := validatePaneID("TMUX_PANE", os.Getenv("TMUX_PANE"))
	if err != nil {
		return ClientTarget{}, fmt.Errorf("no current tmux client: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	res, err := runner.query(ctx, tmuxQuerySpec(
		cmdrun.ShapeSingleLine,
		timeout,
		"display-message", "-p", currentClientFormat,
	))
	if err != nil {
		return ClientTarget{}, fmt.Errorf("current tmux client: %w", err)
	}

	target, err := parseClientTarget(res.Stdout)
	if err != nil {
		return ClientTarget{}, fmt.Errorf("current tmux client: %w", err)
	}
	if target.PaneID != envPaneID {
		return ClientTarget{}, fmt.Errorf("current tmux client pane_id %q does not match TMUX_PANE %q", target.PaneID, envPaneID)
	}
	if err := validateAttachedClient(ctx, timeout, runner, target); err != nil {
		return ClientTarget{}, fmt.Errorf("current tmux client: %w", err)
	}
	return target, nil
}

func validateAttachedClient(ctx context.Context, timeout time.Duration, runner tmuxRunner, target ClientTarget) error {
	if err := validateClientTarget(target); err != nil {
		return err
	}
	res, err := runner.query(ctx, tmuxQuerySpec(
		cmdrun.ShapeLines,
		timeout,
		"list-clients", "-F", currentClientFormat,
	))
	if err != nil {
		return fmt.Errorf("validate attached client %q: %w", target.Name, err)
	}

	var attached *ClientTarget
	for i, line := range tmuxLines(res.Stdout) {
		candidate, err := parseClientTargetLine(line)
		if err != nil {
			return fmt.Errorf("validate attached client row %d: %w", i+1, err)
		}
		if candidate.Name != target.Name {
			continue
		}
		if attached != nil {
			return fmt.Errorf("multiple attached tmux clients are named %q", target.Name)
		}
		attached = &candidate
	}
	if attached == nil {
		return fmt.Errorf("client %q is no longer attached", target.Name)
	}
	if *attached != target {
		return fmt.Errorf("client %q is attached to pane %q, not invoking pane %q", target.Name, attached.PaneID, target.PaneID)
	}
	return nil
}

func parseClientTarget(output string) (ClientTarget, error) {
	lines := tmuxLines(output)
	if len(lines) != 1 {
		return ClientTarget{}, fmt.Errorf("expected 1 row, got %d", len(lines))
	}
	return parseClientTargetLine(lines[0])
}

func parseClientTargetLine(line string) (ClientTarget, error) {
	fields, ok := splitTmuxFields(line, 2)
	if !ok {
		return ClientTarget{}, errors.New("expected client_name and pane_id")
	}
	target := ClientTarget{Name: fields[0], PaneID: fields[1]}
	if err := validateClientTarget(target); err != nil {
		return ClientTarget{}, err
	}
	return target, nil
}

func validateClientTarget(target ClientTarget) error {
	if target.Name == "" {
		return errors.New("no current tmux client; run cmdk action from inside an attached tmux client")
	}
	if containsControl(target.Name) {
		return errors.New("current tmux client name contains control characters")
	}
	_, err := validatePaneID("current tmux client pane_id", target.PaneID)
	return err
}

func (target ClientTarget) isZero() bool {
	return target == ClientTarget{}
}
