package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	minimumMajor          = 3
	minimumMinor          = 2
	prerequisiteLimit     = 4096
	prerequisiteTimeout   = 2 * time.Second
	prerequisiteWaitDelay = 100 * time.Millisecond
	prerequisiteRecovery  = "ensure a runnable supported tmux executable is first in PATH"
)

type version struct {
	major int
	minor int
}

type boundedBuffer struct {
	buf      bytes.Buffer
	overflow bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	remaining := prerequisiteLimit - b.buf.Len()
	if remaining <= 0 {
		b.overflow = true
		return originalLen, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.overflow = true
	}
	_, _ = b.buf.Write(p)
	return originalLen, nil
}

func CheckPrerequisite(ctx context.Context) error {
	return checkPrerequisite(ctx, prerequisiteTimeout)
}

func checkPrerequisite(ctx context.Context, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr boundedBuffer
	cmd := exec.CommandContext(ctx, "tmux", "-V")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = prerequisiteWaitDelay
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			if parentErr := context.Cause(parent); parentErr != nil {
				return fmt.Errorf("checking required tmux version: %w", parentErr)
			}
			return fmt.Errorf("tmux 3.2 or newer is required; tmux -V did not respond within %s; %s", timeout, prerequisiteRecovery)
		}
		return fmt.Errorf("checking required tmux version: %w", ctxErr)
	}
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return errors.New("tmux 3.2 or newer is required; install tmux and ensure it is available in PATH")
		}
		if detail := strings.TrimSpace(stderr.buf.String()); detail != "" {
			return fmt.Errorf("tmux 3.2 or newer is required; tmux -V failed: %w: %s; %s", err, detail, prerequisiteRecovery)
		}
		return fmt.Errorf("tmux 3.2 or newer is required; tmux -V failed: %w; %s", err, prerequisiteRecovery)
	}
	if stdout.overflow || stderr.overflow {
		return fmt.Errorf("tmux 3.2 or newer is required; tmux -V returned oversized output; %s", prerequisiteRecovery)
	}

	raw := strings.TrimSpace(stdout.buf.String())
	found, err := parseVersion(raw)
	if err != nil {
		return fmt.Errorf("tmux 3.2 or newer is required; could not parse tmux -V output %q: %w; %s", raw, err, prerequisiteRecovery)
	}
	if found.major < minimumMajor || found.major == minimumMajor && found.minor < minimumMinor {
		return fmt.Errorf("tmux 3.2 or newer is required; found %s; install or upgrade tmux and ensure the supported version is first in PATH", raw)
	}
	return nil
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
