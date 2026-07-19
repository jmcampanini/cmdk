package tmux

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

const (
	minimumMajor         = 3
	minimumMinor         = 2
	prerequisiteTimeout  = 2 * time.Second
	prerequisiteRecovery = "ensure a runnable supported tmux executable is first in PATH"
)

type version struct {
	major int
	minor int
}

// CheckPrerequisite verifies a supported tmux binary is runnable before any
// tmux-backed command starts. It runs under its own short deadline: a version
// probe that cannot answer promptly is as disqualifying as a missing binary.
func CheckPrerequisite(ctx context.Context) error {
	return checkPrerequisite(ctx, prerequisiteTimeout)
}

func checkPrerequisite(ctx context.Context, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}

	res, err := cmdrun.Query(ctx, cmdrun.QuerySpec{
		Op:        "tmux -V",
		Argv:      []string{"tmux", "-V"},
		Timeout:   timeout,
		Shape:     cmdrun.ShapeSingleLine,
		MaxStdout: tmuxSmallStdoutLimit,
		MaxStderr: tmuxSmallStderrLimit,
	})
	if err != nil {
		return prerequisiteError(ctx, err, timeout)
	}

	raw := strings.TrimSpace(res.Stdout)
	found, err := parseVersion(raw)
	if err != nil {
		return fmt.Errorf("tmux 3.2 or newer is required; could not parse tmux -V output %q: %w; %s", raw, err, prerequisiteRecovery)
	}
	if found.major < minimumMajor || found.major == minimumMajor && found.minor < minimumMinor {
		return fmt.Errorf("tmux 3.2 or newer is required; found %s; install or upgrade tmux and ensure the supported version is first in PATH", raw)
	}
	return nil
}

func prerequisiteError(parent context.Context, err error, timeout time.Duration) error {
	// A canceled or expired caller context is not a verdict about tmux;
	// report the caller's cause instead of an install/upgrade message.
	if parentErr := context.Cause(parent); parentErr != nil {
		return fmt.Errorf("checking required tmux version: %w", parentErr)
	}
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) {
		return fmt.Errorf("checking required tmux version: %w", err)
	}
	switch cmdErr.Kind {
	case cmdrun.KindTimeout:
		return fmt.Errorf("tmux 3.2 or newer is required; tmux -V did not respond within %s; %s", timeout, prerequisiteRecovery)
	case cmdrun.KindCanceled:
		return fmt.Errorf("checking required tmux version: %w", cmdErr.Err)
	case cmdrun.KindOutput:
		return fmt.Errorf("tmux 3.2 or newer is required; tmux -V %v; %s", cmdErr.Err, prerequisiteRecovery)
	default:
		if cmdrun.IsNotFound(err) {
			return errors.New("tmux 3.2 or newer is required; install tmux and ensure it is available in PATH")
		}
		if detail := strings.TrimSpace(cmdErr.Stderr); detail != "" {
			return fmt.Errorf("tmux 3.2 or newer is required; tmux -V failed: %w: %s; %s", cmdErr.Err, detail, prerequisiteRecovery)
		}
		return fmt.Errorf("tmux 3.2 or newer is required; tmux -V failed: %w; %s", cmdErr.Err, prerequisiteRecovery)
	}
}

func parseVersion(raw string) (version, error) {
	if strings.ContainsAny(raw, "\r\n") {
		return version{}, errors.New("expected one line")
	}
	versionText, ok := strings.CutPrefix(raw, "tmux ")
	if !ok || versionText == "" || strings.ContainsAny(versionText, " \t\v\f") {
		return version{}, errors.New("expected tmux <major>.<minor>")
	}

	majorText, minorText, ok := strings.Cut(versionText, ".")
	if !ok || majorText == "" || minorText == "" {
		return version{}, errors.New("expected tmux <major>.<minor>")
	}
	minorDigits := 0
	for minorDigits < len(minorText) && minorText[minorDigits] >= '0' && minorText[minorDigits] <= '9' {
		minorDigits++
	}
	if minorDigits == 0 {
		return version{}, errors.New("minor version must be numeric")
	}
	for _, suffix := range minorText[minorDigits:] {
		if suffix < 'a' || suffix > 'z' {
			return version{}, errors.New("version suffix must contain lowercase letters")
		}
	}

	for _, digit := range majorText {
		if digit < '0' || digit > '9' {
			return version{}, errors.New("major version must be numeric")
		}
	}
	major, err := strconv.Atoi(majorText)
	if err != nil {
		return version{}, errors.New("major version must be numeric")
	}
	minor, err := strconv.Atoi(minorText[:minorDigits])
	if err != nil {
		return version{}, errors.New("minor version must be numeric")
	}
	return version{major: major, minor: minor}, nil
}
