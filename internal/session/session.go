package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

const (
	KindRepo      = "repo"
	KindDirectory = "directory"
)

var primaryBranchDirs = [...]string{"main", "develop", "master"}

type DisplayOptions struct {
	Home        string
	ShortenHome string
	Rules       []pathfmt.Rule
	Truncation  pathfmt.Truncation
}

type Plan struct {
	SessionKind            string `json:"session_kind"`
	SessionKey             string `json:"session_key"`
	DisplayLabel           string `json:"display_label"`
	LaunchPath             string `json:"launch_path"`
	PlannedTmuxSessionName string `json:"planned_tmux_session_name"`
	PlannedTmuxWindowName  string `json:"planned_tmux_window_name"`
}

func Resolve(ctx context.Context, inputPath string, display DisplayOptions) (Plan, error) {
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

	worktree, ok, err := gitWorktreeTop(ctx, absPath)
	if err != nil {
		return Plan{}, err
	}
	if ok {
		sessionKey, err := sessionKeyForWorktree(ctx, worktree)
		if err != nil {
			return Plan{}, err
		}
		return newRepoPlan(sessionKey, worktree, display), nil
	}

	anchor, ok, err := groveAnchorFromContainer(ctx, absPath)
	if err != nil {
		return Plan{}, err
	}
	if ok {
		return newRepoPlan(absPath, anchor, display), nil
	}

	return newDirectoryPlan(absPath, display), nil
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

func sessionKeyForWorktree(ctx context.Context, worktree string) (string, error) {
	container, ok, err := groveContainerForWorktree(ctx, worktree)
	if err != nil {
		return "", err
	}
	if ok {
		return container, nil
	}
	return worktree, nil
}

func newRepoPlan(sessionKey, launchPath string, display DisplayOptions) Plan {
	return newPlanFromCanonicalPaths(
		KindRepo,
		canonicalPath(sessionKey),
		canonicalPath(launchPath),
		display,
	)
}

func newDirectoryPlan(path string, display DisplayOptions) Plan {
	canonicalDirectoryPath := canonicalPath(path)
	return newPlanFromCanonicalPaths(KindDirectory, canonicalDirectoryPath, canonicalDirectoryPath, display)
}

func newPlanFromCanonicalPaths(kind, sessionKey, launchPath string, display DisplayOptions) Plan {
	return Plan{
		SessionKind:            kind,
		SessionKey:             sessionKey,
		DisplayLabel:           displayLabel(sessionKey, display),
		LaunchPath:             launchPath,
		PlannedTmuxSessionName: TmuxSafeSessionName(sessionKey),
		PlannedTmuxWindowName:  filepath.Base(filepath.Clean(launchPath)),
	}
}

func displayLabel(path string, display DisplayOptions) string {
	return pathfmt.DisplayPath(path, display.Home, display.ShortenHome, display.Rules, display.Truncation)
}

func TmuxSafeSessionName(sessionKey string) string {
	name := filepath.ToSlash(filepath.Clean(sessionKey))
	name = strings.TrimLeft(name, "/")
	name = strings.NewReplacer(".", "_", ":", "_").Replace(name)
	if name == "" || name == "." {
		return "_"
	}
	return name
}

func groveAnchorFromContainer(ctx context.Context, dir string) (string, bool, error) {
	var firstStatErr error
	for _, name := range primaryBranchDirs {
		child := filepath.Join(dir, name)
		valid, err := validPrimaryWorktree(ctx, child, &firstStatErr)
		if err != nil {
			return "", false, err
		}
		if valid {
			return child, true, nil
		}
	}
	return "", false, firstStatErr
}

func groveContainerForWorktree(ctx context.Context, worktree string) (string, bool, error) {
	parent := filepath.Dir(worktree)
	worktreeCommonDir, haveWorktreeCommonDir, err := gitCommonDir(ctx, worktree)
	if err != nil {
		return "", false, err
	}

	var firstStatErr error
	for _, name := range primaryBranchDirs {
		child := filepath.Join(parent, name)
		valid, err := validPrimaryWorktree(ctx, child, &firstStatErr)
		if err != nil {
			return "", false, err
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
		childCommonDir, ok, err := gitCommonDir(ctx, child)
		if err != nil {
			return "", false, err
		}
		if ok && childCommonDir == worktreeCommonDir {
			return parent, true, nil
		}
	}
	return "", false, firstStatErr
}

func validPrimaryWorktree(ctx context.Context, dir string, firstStatErr *error) (bool, error) {
	valid, err := validGitWorktreeRoot(ctx, dir)
	if err == nil {
		return valid, nil
	}
	if rememberWorktreeStatError(err, firstStatErr) {
		return false, nil
	}
	return false, err
}

func validGitWorktreeRoot(ctx context.Context, dir string) (bool, error) {
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
	top, ok, err := gitWorktreeTop(ctx, dir)
	if err != nil {
		return false, err
	}
	return ok && samePath(top, dir), nil
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

func gitWorktreeTop(ctx context.Context, dir string) (string, bool, error) {
	hasMarker, err := hasGitMarker(dir)
	if err != nil {
		return "", false, err
	}
	if !hasMarker {
		return "", false, nil
	}

	out, err := gitOutput(ctx, dir, "rev-parse", "--show-toplevel")
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

func gitCommonDir(ctx context.Context, worktree string) (string, bool, error) {
	out, err := gitOutput(ctx, worktree, "rev-parse", "--git-common-dir")
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

func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = withoutGitEnv(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, fmt.Errorf("git -C %s %s: %w", dir, strings.Join(args, " "), ctxErr)
	}
	return nil, gitCommandError(dir, args, err, stderr.String())
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

func gitCommandError(dir string, args []string, err error, stderr string) error {
	if stderr == "" {
		return fmt.Errorf("git -C %s %s: %w", dir, strings.Join(args, " "), err)
	}
	return fmt.Errorf("git -C %s %s: %w: %s", dir, strings.Join(args, " "), err, strings.TrimSpace(stderr))
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

func trimCommandLine(out []byte) string {
	return strings.TrimRight(string(out), "\r\n")
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
