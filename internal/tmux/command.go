package tmux

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type tmuxRunner interface {
	Output(context.Context, ...string) ([]byte, error)
}

type execTmuxRunner struct{}

func (execTmuxRunner) Output(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("tmux did not respond within the configured timeout: %w", err)
	}
	return nil, tmuxCommandError(args, err, stderr.String())
}

func tmuxOutput(ctx context.Context, args ...string) ([]byte, error) {
	return execTmuxRunner{}.Output(ctx, args...)
}

func tmuxCommandError(args []string, err error, stderr string) error {
	argString := strings.Join(args, " ")
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		return fmt.Errorf("tmux %s: %w: %s", argString, err, trimmed)
	}
	return fmt.Errorf("tmux %s: %w", argString, err)
}
