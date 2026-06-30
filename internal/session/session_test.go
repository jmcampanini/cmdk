package session

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=cmdk test",
		"GIT_AUTHOR_EMAIL=cmdk@example.invalid",
		"GIT_COMMITTER_NAME=cmdk test",
		"GIT_COMMITTER_EMAIL=cmdk@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s failed: %v\n%s", dir, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func initRepo(t *testing.T, path string) {
	t.Helper()
	requireGit(t)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	runGit(t, path, "config", "user.name", "cmdk test")
	runGit(t, path, "config", "user.email", "cmdk@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, path, "add", "README.md")
	runGit(t, path, "commit", "-m", "initial")
}

func resolveForTest(t *testing.T, path string) Plan {
	t.Helper()
	plan, err := Resolve(context.Background(), path)
	if err != nil {
		t.Fatalf("Resolve(%q) returned error: %v", path, err)
	}
	return plan
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return filepath.Clean(resolved)
}

func symlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
}

func assertPlan(t *testing.T, got Plan, want Plan) {
	t.Helper()
	if got != want {
		t.Errorf("plan mismatch\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestResolve_PathDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := Resolve(context.Background(), missing)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "path does not exist") {
		t.Errorf("error = %q, want path does not exist", err.Error())
	}
}

func TestResolve_PathIsNotDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(context.Background(), path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "path is not a directory") {
		t.Errorf("error = %q, want path is not a directory", err.Error())
	}
}

func TestResolve_EmptyPathIsRequired(t *testing.T) {
	_, err := Resolve(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "path is required" {
		t.Errorf("error = %q, want path is required", err.Error())
	}
}

func TestTrimCommandLinePreservesPathTrailingNewlines(t *testing.T) {
	got := trimCommandLine([]byte("/tmp/repo\n\n"))
	want := "/tmp/repo\n"
	if got != want {
		t.Errorf("trimCommandLine() = %q, want %q", got, want)
	}
}

func TestResolve_PropagatesCanceledGitProbe(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Resolve(ctx, dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestResolve_PropagatesGitExecFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")

	_, err := Resolve(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error")
	}
	var execErr *exec.Error
	if !errors.As(err, &execErr) {
		t.Errorf("error = %T %[1]v, want *exec.Error", err)
	}
}

func TestResolve_DoesNotRunGitWithoutMarker(t *testing.T) {
	bin := t.TempDir()
	gitPath := filepath.Join(bin, "git")
	marker := filepath.Join(t.TempDir(), "git-invoked")
	script := `#!/bin/sh
printf invoked > "$CMDK_GIT_MARKER"
exit 42
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("CMDK_GIT_MARKER", marker)

	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	plan := resolveForTest(t, dir)
	if plan.SessionKind != KindDirectory {
		t.Errorf("SessionKind = %q, want %q", plan.SessionKind, KindDirectory)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("git should not be invoked when no .git marker exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat marker: %v", err)
	}
}

func TestResolve_PropagatesCorruptGitMetadata(t *testing.T) {
	requireGit(t)
	dir := filepath.Join(t.TempDir(), "bad")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /nonexistent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Resolve(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "git -C") {
		t.Errorf("error = %q, want git command context", err.Error())
	}
}

func TestResolve_InsideNormalGitRepo(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "project")
	initRepo(t, repo)
	subdir := filepath.Join(repo, "cmd")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	repoReal := realPath(t, repo)
	plan := resolveForTest(t, subdir)
	assertPlan(t, plan, Plan{
		SessionKind: KindRepo,
		SessionKey:  repoReal,
	})
}

func TestResolve_StandaloneRepoFallback(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "standalone")
	initRepo(t, repo)

	repoReal := realPath(t, repo)
	plan := resolveForTest(t, repo)
	if plan.SessionKind != KindRepo {
		t.Errorf("SessionKind = %q, want %q", plan.SessionKind, KindRepo)
	}
	if plan.SessionKey != repoReal {
		t.Errorf("SessionKey = %q, want standalone repo path %q", plan.SessionKey, repoReal)
	}
}

func TestResolve_StandaloneRepoFallbackIgnoresCorruptGroveSibling(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "foo")
	initRepo(t, repo)
	main := filepath.Join(root, "main")
	if err := os.MkdirAll(main, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, ".git"), []byte("gitdir: /nonexistent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repoReal := realPath(t, repo)
	plan := resolveForTest(t, repo)
	assertPlan(t, plan, Plan{
		SessionKind: KindRepo,
		SessionKey:  repoReal,
	})
}

func TestResolve_SymlinkedStandaloneRepoUsesCanonicalSessionKey(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	initRepo(t, repo)
	link := filepath.Join(t.TempDir(), "repo-link")
	symlinkOrSkip(t, repo, link)

	repoReal := realPath(t, repo)
	plan := resolveForTest(t, link)
	assertPlan(t, plan, Plan{
		SessionKind: KindRepo,
		SessionKey:  repoReal,
	})
}

func TestResolve_SymlinkedRepoSubdirUsesCanonicalSessionKey(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	initRepo(t, repo)
	subdir := filepath.Join(repo, "cmd")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "cmd-link")
	symlinkOrSkip(t, subdir, link)

	repoReal := realPath(t, repo)
	plan := resolveForTest(t, link)
	assertPlan(t, plan, Plan{
		SessionKind: KindRepo,
		SessionKey:  repoReal,
	})
}

func TestResolve_InsideLinkedGitWorktree(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	main := filepath.Join(container, "main")
	develop := filepath.Join(container, "develop")
	initRepo(t, main)
	runGit(t, main, "worktree", "add", "-b", "cmdk-test-develop", develop)
	subdir := filepath.Join(develop, "pkg")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	containerReal := realPath(t, container)
	plan := resolveForTest(t, subdir)
	assertPlan(t, plan, Plan{
		SessionKind: KindRepo,
		SessionKey:  containerReal,
	})
}

func TestResolve_GroveContainerRootContainingMain(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	main := filepath.Join(container, "main")
	initRepo(t, main)

	containerReal := realPath(t, container)
	plan := resolveForTest(t, container)
	assertPlan(t, plan, Plan{
		SessionKind: KindRepo,
		SessionKey:  containerReal,
	})
}

func TestResolve_GroveProbeContinuesAfterStatError(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	if err := os.MkdirAll(container, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, "main", filepath.Join(container, "main"))
	develop := filepath.Join(container, "develop")
	initRepo(t, develop)

	containerReal := realPath(t, container)
	plan := resolveForTest(t, container)
	assertPlan(t, plan, Plan{
		SessionKind: KindRepo,
		SessionKey:  containerReal,
	})
}

func TestResolve_GroveProbeReturnsStatErrorWhenNoValidChild(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	if err := os.MkdirAll(container, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, "main", filepath.Join(container, "main"))

	_, err := Resolve(context.Background(), container)
	if err == nil {
		t.Fatal("expected error")
	}
	var statErr *worktreeStatError
	if !errors.As(err, &statErr) {
		t.Errorf("error = %T %[1]v, want worktreeStatError", err)
	}
}

func TestResolve_SymlinkedGroveContainerUsesCanonicalSessionKey(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	main := filepath.Join(container, "main")
	initRepo(t, main)
	link := filepath.Join(t.TempDir(), "dotfiles-link")
	symlinkOrSkip(t, container, link)

	containerReal := realPath(t, container)
	containerPlan := resolveForTest(t, link)
	childPlan := resolveForTest(t, filepath.Join(link, "main"))

	for name, plan := range map[string]Plan{"container": containerPlan, "child": childPlan} {
		if plan.SessionKey != containerReal {
			t.Errorf("%s SessionKey = %q, want canonical container %q", name, plan.SessionKey, containerReal)
		}
	}
	if containerPlan.SessionKey != childPlan.SessionKey {
		t.Errorf("symlinked container and child should share session key: container=%q child=%q", containerPlan.SessionKey, childPlan.SessionKey)
	}
}

func TestResolve_GroveContainerWithPrimaryBranchesUsesContainerKey(t *testing.T) {
	container := filepath.Join(t.TempDir(), "project")
	develop := filepath.Join(container, "develop")
	master := filepath.Join(container, "master")
	initRepo(t, develop)
	initRepo(t, master)

	containerReal := realPath(t, container)
	plan := resolveForTest(t, container)
	if plan.SessionKey != containerReal {
		t.Errorf("SessionKey = %q, want container %q", plan.SessionKey, containerReal)
	}
}

func TestResolve_NonGitDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	dirReal := realPath(t, dir)
	plan := resolveForTest(t, dir)
	assertPlan(t, plan, Plan{
		SessionKind: KindDirectory,
		SessionKey:  dirReal,
	})
}

func TestResolve_SymlinkedNonGitDirectoryUsesCanonicalSessionKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "scratch-link")
	symlinkOrSkip(t, dir, link)

	dirReal := realPath(t, dir)
	plan := resolveForTest(t, link)
	assertPlan(t, plan, Plan{
		SessionKind: KindDirectory,
		SessionKey:  dirReal,
	})
}

func TestResolve_SymlinkInsideRepoToNonGitDirectoryUsesTargetDirectoryPlan(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	initRepo(t, repo)
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, "scratch-link")
	symlinkOrSkip(t, dir, link)

	dirReal := realPath(t, dir)
	plan := resolveForTest(t, link)
	assertPlan(t, plan, Plan{
		SessionKind: KindDirectory,
		SessionKey:  dirReal,
	})
}

func TestResolve_IgnoresGitEnvironment(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	initRepo(t, repo)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", filepath.Join(repo, ".git"))
	t.Setenv("GIT_WORK_TREE", repo)

	plan := resolveForTest(t, outside)
	if plan.SessionKind != KindDirectory {
		t.Fatalf("SessionKind = %q, want %q", plan.SessionKind, KindDirectory)
	}
	outsideReal := realPath(t, outside)
	if plan.SessionKey != outsideReal {
		t.Errorf("SessionKey = %q, want %q", plan.SessionKey, outsideReal)
	}
}

func TestResolve_GroupingMainAndDevelopUnderSameContainer(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	main := filepath.Join(container, "main")
	develop := filepath.Join(container, "develop")
	initRepo(t, main)
	runGit(t, main, "worktree", "add", "-b", "cmdk-test-develop", develop)

	containerReal := realPath(t, container)
	mainPlan := resolveForTest(t, main)
	developPlan := resolveForTest(t, develop)

	if mainPlan.SessionKey != containerReal {
		t.Errorf("main SessionKey = %q, want %q", mainPlan.SessionKey, containerReal)
	}
	if developPlan.SessionKey != containerReal {
		t.Errorf("develop SessionKey = %q, want %q", developPlan.SessionKey, containerReal)
	}
	if mainPlan.SessionKey != developPlan.SessionKey {
		t.Errorf("worktrees should share session key: main=%q develop=%q", mainPlan.SessionKey, developPlan.SessionKey)
	}
}

func TestPlanJSONDoesNotUseSessionIDForCmdkIdentity(t *testing.T) {
	plan := Plan{
		SessionKind: KindDirectory,
		SessionKey:  "/tmp/scratch",
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "session_id") {
		t.Fatalf("JSON should not contain session_id: %s", data)
	}
	if !strings.Contains(string(data), "session_key") {
		t.Fatalf("JSON should contain session_key: %s", data)
	}
	if strings.Contains(string(data), "session_display") {
		t.Fatalf("JSON should not contain session_display: %s", data)
	}
	if strings.Contains(string(data), "display_label") {
		t.Fatalf("JSON should not contain display_label: %s", data)
	}
}
