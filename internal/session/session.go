package session

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

const (
	KindRepo      = "repo"
	KindDirectory = "directory"
)

const (
	// Probe output is at most one filesystem path.
	gitProbeMaxStdout = 4 << 10
	gitProbeMaxStderr = 32 << 10
)

var primaryBranchDirs = [...]string{"main", "develop", "master"}

type Plan struct {
	SessionKind string `json:"session_kind"`
	SessionKey  string `json:"session_key"`
}

// Resolve turns an existing directory into a session plan. probeTimeout is
// the required per-command deadline for each git probe (callers pass the
// configured fetch timeout); the caller's ctx additionally bounds the
// overall resolve budget across every probe one Resolve may spawn.
func Resolve(ctx context.Context, inputPath string, probeTimeout time.Duration) (Plan, error) {
	if inputPath == "" {
		return Plan{}, errors.New("path is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}

	absPath, err := resolveExistingDirectory(inputPath)
	if err != nil {
		return Plan{}, err
	}

	p := prober{timeout: probeTimeout}
	worktree, ok, err := p.gitWorktreeTop(ctx, absPath)
	if err != nil {
		return Plan{}, err
	}
	if ok {
		sessionKey, err := p.sessionKeyForWorktree(ctx, worktree)
		if err != nil {
			return Plan{}, err
		}
		return newRepoPlan(sessionKey), nil
	}

	hasAnchor, err := p.hasGroveAnchor(ctx, absPath)
	if err != nil {
		return Plan{}, err
	}
	if hasAnchor {
		return newRepoPlan(absPath), nil
	}

	return newDirectoryPlan(absPath), nil
}

// prober runs git rev-parse probes with a caller-chosen per-command
// deadline.
type prober struct {
	timeout time.Duration
}

func resolveExistingDirectory(inputPath string) (string, error) {
	absPath, err := filepath.Abs(inputPath)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("path does not exist: %s", absPath)
		}
		return "", fmt.Errorf("path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}
	return absPath, nil
}

func (p prober) sessionKeyForWorktree(ctx context.Context, worktree string) (string, error) {
	container, ok, err := p.groveContainerForWorktree(ctx, worktree)
	if err != nil {
		return "", err
	}
	if ok {
		return container, nil
	}
	return worktree, nil
}

func newRepoPlan(sessionKey string) Plan {
	return Plan{SessionKind: KindRepo, SessionKey: canonicalPath(sessionKey)}
}

func newDirectoryPlan(path string) Plan {
	return Plan{SessionKind: KindDirectory, SessionKey: canonicalPath(path)}
}

func (p prober) hasGroveAnchor(ctx context.Context, dir string) (bool, error) {
	var firstStatErr error
	for _, name := range primaryBranchDirs {
		child := filepath.Join(dir, name)
		valid, err := p.validGitWorktreeRoot(ctx, child)
		if err != nil {
			if rememberWorktreeStatError(err, &firstStatErr) {
				continue
			}
			return false, err
		}
		if valid {
			return true, nil
		}
	}
	return false, firstStatErr
}

func (p prober) groveContainerForWorktree(ctx context.Context, worktree string) (string, bool, error) {
	parent := filepath.Dir(worktree)
	worktreeCommonDir, haveWorktreeCommonDir, err := p.gitCommonDir(ctx, worktree)
	if err != nil {
		return "", false, err
	}

	for _, name := range primaryBranchDirs {
		child := filepath.Join(parent, name)
		valid, err := p.validGitWorktreeRoot(ctx, child)
		if err != nil {
			if isContextError(err) {
				return "", false, err
			}
			continue
		}
		if !valid {
			continue
		}
		if samePath(child, worktree) {
			return parent, true, nil
		}
		if !haveWorktreeCommonDir {
			continue
		}
		childCommonDir, ok, err := p.gitCommonDir(ctx, child)
		if err != nil {
			if isContextError(err) {
				return "", false, err
			}
			continue
		}
		if ok && childCommonDir == worktreeCommonDir {
			return parent, true, nil
		}
	}
	return "", false, nil
}

func (p prober) validGitWorktreeRoot(ctx context.Context, dir string) (bool, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, &worktreeStatError{path: dir, err: err}
	}
	if !info.IsDir() {
		return false, nil
	}
	top, ok, err := p.gitWorktreeTop(ctx, dir)
	if err != nil {
		return false, err
	}
	return ok && samePath(top, dir), nil
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func rememberWorktreeStatError(err error, firstErr *error) bool {
	var statErr *worktreeStatError
	if !errors.As(err, &statErr) {
		return false
	}
	if *firstErr == nil {
		*firstErr = err
	}
	return true
}

type worktreeStatError struct {
	path string
	err  error
}

func (e *worktreeStatError) Error() string {
	return fmt.Sprintf("stat %s: %v", e.path, e.err)
}

func (e *worktreeStatError) Unwrap() error {
	return e.err
}

func (p prober) gitWorktreeTop(ctx context.Context, dir string) (string, bool, error) {
	hasMarker, err := hasGitMarker(dir)
	if err != nil {
		return "", false, err
	}
	if !hasMarker {
		return "", false, nil
	}

	out, err := p.gitOutput(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", false, err
	}
	top := trimCommandLine(out)
	if top == "" {
		return "", false, nil
	}
	absTop, err := filepath.Abs(top)
	if err != nil {
		return "", false, err
	}
	return canonicalPath(absTop), true, nil
}

func (p prober) gitCommonDir(ctx context.Context, worktree string) (string, bool, error) {
	out, err := p.gitOutput(ctx, worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", false, err
	}
	commonDir := trimCommandLine(out)
	if commonDir == "" {
		return "", false, nil
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(worktree, commonDir)
	}
	return canonicalPath(commonDir), true, nil
}

func (p prober) gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	res, err := cmdrun.Query(ctx, cmdrun.QuerySpec{
		Op:        fmt.Sprintf("git -C %s %s", dir, strings.Join(args, " ")),
		Argv:      append([]string{"git", "-C", dir}, args...),
		Env:       withoutGitEnv(os.Environ()),
		Timeout:   p.timeout,
		Shape:     cmdrun.ShapeSingleLine,
		MaxStdout: gitProbeMaxStdout,
		MaxStderr: gitProbeMaxStderr,
	})
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}

func hasGitMarker(path string) (bool, error) {
	canonical, err := canonicalExistingPath(path)
	if err != nil {
		return false, err
	}
	return hasGitMarkerInAncestors(canonical)
}

func canonicalExistingPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	absPath = filepath.Clean(absPath)
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks %s: %w", absPath, err)
	}
	return filepath.Clean(resolved), nil
}

func hasGitMarkerInAncestors(path string) (bool, error) {
	dir := filepath.Clean(path)
	for {
		marker := filepath.Join(dir, ".git")
		_, err := os.Lstat(marker)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("stat %s: %w", marker, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return false, nil
		}
		dir = parent
	}
}

func withoutGitEnv(env []string) []string {
	// Keep Git diagnostics stable and prevent caller GIT_* variables from changing discovery.
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_") || strings.HasPrefix(entry, "LC_ALL=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "LC_ALL=C")
}

func trimCommandLine(out string) string {
	return strings.TrimSuffix(out, "\n")
}

func samePath(a, b string) bool {
	return canonicalPath(a) == canonicalPath(b)
}

func canonicalPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	absPath = filepath.Clean(absPath)
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		return filepath.Clean(resolved)
	}
	return absPath
}
