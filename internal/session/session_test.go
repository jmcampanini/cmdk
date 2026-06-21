package session

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/cmdk/internal/pathfmt"
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

func resolveForTest(t *testing.T, path string, display DisplayOptions) Plan {
	t.Helper()
	plan, err := Resolve(context.Background(), path, display)
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

func assertPlan(t *testing.T, got Plan, want Plan) {
	t.Helper()
	if got != want {
		t.Errorf("plan mismatch\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestResolve_PathDoesNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := Resolve(context.Background(), missing, DisplayOptions{})
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
	_, err := Resolve(context.Background(), path, DisplayOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "path is not a directory") {
		t.Errorf("error = %q, want path is not a directory", err.Error())
	}
}

func TestResolve_EmptyPathIsRequired(t *testing.T) {
	_, err := Resolve(context.Background(), "", DisplayOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "path is required" {
		t.Errorf("error = %q, want path is required", err.Error())
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
	plan := resolveForTest(t, subdir, DisplayOptions{})
	assertPlan(t, plan, Plan{
		SessionKind:            KindRepo,
		SessionKey:             repoReal,
		DisplayLabel:           repoReal,
		LaunchPath:             repoReal,
		PlannedTmuxSessionName: TmuxSafeSessionName(repoReal),
		PlannedTmuxWindowName:  "project",
	})
}

func TestResolve_StandaloneRepoFallback(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "standalone")
	initRepo(t, repo)

	repoReal := realPath(t, repo)
	plan := resolveForTest(t, repo, DisplayOptions{})
	if plan.SessionKind != KindRepo {
		t.Errorf("SessionKind = %q, want %q", plan.SessionKind, KindRepo)
	}
	if plan.SessionKey != repoReal {
		t.Errorf("SessionKey = %q, want standalone repo path %q", plan.SessionKey, repoReal)
	}
	if plan.LaunchPath != repoReal {
		t.Errorf("LaunchPath = %q, want %q", plan.LaunchPath, repoReal)
	}
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
	developReal := realPath(t, develop)
	plan := resolveForTest(t, subdir, DisplayOptions{})
	assertPlan(t, plan, Plan{
		SessionKind:            KindRepo,
		SessionKey:             containerReal,
		DisplayLabel:           containerReal,
		LaunchPath:             developReal,
		PlannedTmuxSessionName: TmuxSafeSessionName(containerReal),
		PlannedTmuxWindowName:  "develop",
	})
}

func TestResolve_GroveContainerRootContainingMain(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	main := filepath.Join(container, "main")
	initRepo(t, main)

	plan := resolveForTest(t, container, DisplayOptions{})
	assertPlan(t, plan, Plan{
		SessionKind:            KindRepo,
		SessionKey:             container,
		DisplayLabel:           container,
		LaunchPath:             main,
		PlannedTmuxSessionName: TmuxSafeSessionName(container),
		PlannedTmuxWindowName:  "main",
	})
}

func TestResolve_GrovePrimaryBranchPriority(t *testing.T) {
	container := filepath.Join(t.TempDir(), "project")
	develop := filepath.Join(container, "develop")
	master := filepath.Join(container, "master")
	initRepo(t, develop)
	initRepo(t, master)

	plan := resolveForTest(t, container, DisplayOptions{})
	if plan.LaunchPath != develop {
		t.Errorf("LaunchPath = %q, want first valid primary child %q", plan.LaunchPath, develop)
	}
	if plan.PlannedTmuxWindowName != "develop" {
		t.Errorf("PlannedTmuxWindowName = %q, want develop", plan.PlannedTmuxWindowName)
	}
}

func TestResolve_NonGitDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	plan := resolveForTest(t, dir, DisplayOptions{})
	assertPlan(t, plan, Plan{
		SessionKind:            KindDirectory,
		SessionKey:             dir,
		DisplayLabel:           dir,
		LaunchPath:             dir,
		PlannedTmuxSessionName: TmuxSafeSessionName(dir),
		PlannedTmuxWindowName:  "scratch",
	})
}

func TestResolve_GroupingMainAndDevelopUnderSameContainer(t *testing.T) {
	container := filepath.Join(t.TempDir(), "dotfiles")
	main := filepath.Join(container, "main")
	develop := filepath.Join(container, "develop")
	initRepo(t, main)
	runGit(t, main, "worktree", "add", "-b", "cmdk-test-develop", develop)

	containerReal := realPath(t, container)
	mainPlan := resolveForTest(t, main, DisplayOptions{})
	developPlan := resolveForTest(t, develop, DisplayOptions{})

	if mainPlan.SessionKey != containerReal {
		t.Errorf("main SessionKey = %q, want %q", mainPlan.SessionKey, containerReal)
	}
	if developPlan.SessionKey != containerReal {
		t.Errorf("develop SessionKey = %q, want %q", developPlan.SessionKey, containerReal)
	}
	if mainPlan.SessionKey != developPlan.SessionKey {
		t.Errorf("worktrees should share session key: main=%q develop=%q", mainPlan.SessionKey, developPlan.SessionKey)
	}
	if mainPlan.PlannedTmuxWindowName != "main" {
		t.Errorf("main window name = %q, want main", mainPlan.PlannedTmuxWindowName)
	}
	if developPlan.PlannedTmuxWindowName != "develop" {
		t.Errorf("develop window name = %q, want develop", developPlan.PlannedTmuxWindowName)
	}
}

func TestTmuxSafeSessionName(t *testing.T) {
	got := TmuxSafeSessionName("/Users/me/Code/github.com/acme/foo:bar/baz.qux")
	want := "Users/me/Code/github_com/acme/foo_bar/baz_qux"
	if got != want {
		t.Errorf("TmuxSafeSessionName() = %q, want %q", got, want)
	}
	if !strings.Contains(got, "/") {
		t.Errorf("TmuxSafeSessionName() = %q, want slashes preserved", got)
	}
}

func TestTmuxSafeSessionNameRootFallback(t *testing.T) {
	got := TmuxSafeSessionName("/")
	if got != "_" {
		t.Errorf("TmuxSafeSessionName(/) = %q, want _", got)
	}
}

func TestResolve_DisplayLabelUsesPathFormatting(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	dir := filepath.Join(home, "Code", "github.com", "acme", "project")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	display := DisplayOptions{
		Home:        home,
		ShortenHome: "~",
		Rules:       pathfmt.CompileRules(map[string]string{"github.com": "gh"}),
		Truncation:  pathfmt.Truncation{Length: 3, Symbol: "…"},
	}

	plan := resolveForTest(t, dir, display)
	if plan.DisplayLabel != "…/gh/acme/project" {
		t.Errorf("DisplayLabel = %q, want formatted display path", plan.DisplayLabel)
	}
	if plan.SessionKey != dir {
		t.Errorf("SessionKey = %q, want unformatted identity %q", plan.SessionKey, dir)
	}
}

func TestPlanJSONDoesNotUseSessionIDForCmdkIdentity(t *testing.T) {
	plan := Plan{
		SessionKind:            KindDirectory,
		SessionKey:             "/tmp/scratch",
		DisplayLabel:           "/tmp/scratch",
		LaunchPath:             "/tmp/scratch",
		PlannedTmuxSessionName: "tmp/scratch",
		PlannedTmuxWindowName:  "scratch",
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
}
