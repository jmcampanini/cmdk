package session

import (
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

var primaryBranchDirs = []string{"main", "develop", "master"}

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

	absPath, err := filepath.Abs(inputPath)
	if err != nil {
		return Plan{}, fmt.Errorf("resolving absolute path: %w", err)
	}
	absPath = filepath.Clean(absPath)

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Plan{}, fmt.Errorf("path does not exist: %s", absPath)
		}
		return Plan{}, fmt.Errorf("path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return Plan{}, fmt.Errorf("path is not a directory: %s", absPath)
	}

	if worktree, ok := gitWorktreeTop(ctx, absPath); ok {
		sessionKey := worktree
		if container, ok := groveContainerForWorktree(ctx, worktree); ok {
			sessionKey = container
		}
		return newRepoPlan(sessionKey, worktree, display), nil
	}

	if anchor, ok := groveAnchorFromContainer(ctx, absPath); ok {
		return newRepoPlan(absPath, anchor, display), nil
	}

	return newDirectoryPlan(absPath, display), nil
}

func newRepoPlan(sessionKey, launchPath string, display DisplayOptions) Plan {
	canonicalSessionKey := canonicalPath(sessionKey)
	canonicalLaunchPath := canonicalPath(launchPath)
	return Plan{
		SessionKind:            KindRepo,
		SessionKey:             canonicalSessionKey,
		DisplayLabel:           displayLabel(canonicalSessionKey, display),
		LaunchPath:             canonicalLaunchPath,
		PlannedTmuxSessionName: TmuxSafeSessionName(canonicalSessionKey),
		PlannedTmuxWindowName:  filepath.Base(filepath.Clean(canonicalLaunchPath)),
	}
}

func newDirectoryPlan(path string, display DisplayOptions) Plan {
	canonicalDirectoryPath := canonicalPath(path)
	return Plan{
		SessionKind:            KindDirectory,
		SessionKey:             canonicalDirectoryPath,
		DisplayLabel:           displayLabel(canonicalDirectoryPath, display),
		LaunchPath:             canonicalDirectoryPath,
		PlannedTmuxSessionName: TmuxSafeSessionName(canonicalDirectoryPath),
		PlannedTmuxWindowName:  filepath.Base(filepath.Clean(canonicalDirectoryPath)),
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

func groveAnchorFromContainer(ctx context.Context, dir string) (string, bool) {
	for _, name := range primaryBranchDirs {
		child := filepath.Join(dir, name)
		if validGitWorktreeRoot(ctx, child) {
			return child, true
		}
	}
	return "", false
}

func groveContainerForWorktree(ctx context.Context, worktree string) (string, bool) {
	parent := filepath.Dir(worktree)
	worktreeCommonDir, haveWorktreeCommonDir := gitCommonDir(ctx, worktree)

	for _, name := range primaryBranchDirs {
		child := filepath.Join(parent, name)
		if !validGitWorktreeRoot(ctx, child) {
			continue
		}
		if samePath(child, worktree) {
			return parent, true
		}
		if !haveWorktreeCommonDir {
			continue
		}
		childCommonDir, ok := gitCommonDir(ctx, child)
		if ok && childCommonDir == worktreeCommonDir {
			return parent, true
		}
	}
	return "", false
}

func validGitWorktreeRoot(ctx context.Context, dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	top, ok := gitWorktreeTop(ctx, dir)
	return ok && samePath(top, dir)
}

func gitWorktreeTop(ctx context.Context, dir string) (string, bool) {
	out, err := gitOutput(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", false
	}
	top := trimCommandLine(out)
	if top == "" {
		return "", false
	}
	absTop, err := filepath.Abs(top)
	if err != nil {
		return "", false
	}
	return canonicalPath(absTop), true
}

func gitCommonDir(ctx context.Context, worktree string) (string, bool) {
	out, err := gitOutput(ctx, worktree, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", false
	}
	commonDir := trimCommandLine(out)
	if commonDir == "" {
		return "", false
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(worktree, commonDir)
	}
	return canonicalPath(commonDir), true
}

func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = withoutGitEnv(os.Environ())
	return cmd.Output()
}

func withoutGitEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
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
