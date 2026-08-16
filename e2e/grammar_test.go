package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestE2E_RejectedOperandsLeaveStateUntouched proves that rejected operands
// exit nonzero before any application-managed state changes: fixture files,
// tmux sessions, windows, panes, and the attached client's target all stay
// exactly as they were.
func TestE2E_RejectedOperandsLeaveStateUntouched(t *testing.T) {
	useIsolatedTmuxSocket(t)
	socketPath := startGrammarTmuxServer(t)
	cfgHome, canary, marker := writeGrammarFixture(t)
	env := grammarEnv(cfgHome, "TMUX="+socketPath+",0,0")

	// The fixture must be valid so a rejection cannot be blamed on config.
	if code, _, stderr := runCmdkBinary(t, env, "", "config"); code != 0 {
		t.Fatalf("fixture config is not valid: exit %d\n%s", code, stderr)
	}

	tmuxBaseline := grammarTmuxSnapshot(t)
	fixtureBaseline := dirSnapshot(t, canary) + dirSnapshot(t, cfgHome)

	rows := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"root operand", []string{"stray-operand"}, `unknown command "stray-operand" for "cmdk"`},
		{"action run missing name", []string{"action", "run"}, "accepts 1 arg(s), received 0"},
		{"action run extra operand", []string{"action", "run", "grammar-marker", "extra"}, "accepts 1 arg(s), received 2"},
		{"attach extra operand", []string{"attach", canary, "extra"}, "accepts at most 1 arg(s), received 2"},
		{"session window missing mode", []string{"session", "window", canary}, "--new or command args after --"},
		{"session window command without delimiter", []string{"session", "window", canary, "extra"}, "command args must follow --"},
		{"session window multiple paths", []string{"session", "window", canary, canary, "--", "true"}, "exactly one path before --"},
		{"window next operand", []string{"window", "next", "extra"}, `unknown command "extra" for "cmdk window next"`},
		{"window previous operand", []string{"window", "previous", "extra"}, `unknown command "extra" for "cmdk window previous"`},
		{"window prev alias operand", []string{"window", "prev", "extra"}, `unknown command "extra" for "cmdk window previous"`},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			code, stdout, stderr := runCmdkBinary(t, env, "", row.args...)
			if code != 1 {
				t.Errorf("cmdk %s: exit = %d, want 1\nstdout: %s\nstderr: %s", strings.Join(row.args, " "), code, stdout, stderr)
			}
			if !strings.Contains(stderr, row.wantStderr) {
				t.Errorf("cmdk %s: stderr = %q, want substring %q", strings.Join(row.args, " "), stderr, row.wantStderr)
			}
			if got := grammarTmuxSnapshot(t); got != tmuxBaseline {
				t.Errorf("cmdk %s changed tmux state:\nbefore:\n%s\nafter:\n%s", strings.Join(row.args, " "), tmuxBaseline, got)
			}
			if got := dirSnapshot(t, canary) + dirSnapshot(t, cfgHome); got != fixtureBaseline {
				t.Errorf("cmdk %s changed fixture files:\nbefore:\n%s\nafter:\n%s", strings.Join(row.args, " "), fixtureBaseline, got)
			}
			if _, err := os.Stat(marker); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("cmdk %s: marker file %s exists; the configured action ran", strings.Join(row.args, " "), marker)
			}
		})
	}
}

func TestE2E_GrammarValidFlowsSmoke(t *testing.T) {
	cfgHome, canary, _ := writeGrammarFixture(t)
	env := grammarEnv(cfgHome)

	rows := []struct {
		name    string
		args    []string
		stdin   string
		wantOut string
	}{
		{"version flag", []string{"--version"}, "", "cmdk version"},
		{"help flag", []string{"--help"}, "", "Keyboard-driven tmux launcher"},
		{"help topic", []string{"exit-codes"}, "", "Exit codes returned by cmdk"},
		{"help command", []string{"help", "session"}, "", "Resolve and manage cmdk sessions"},
		{"completion script", []string{"completion", "bash"}, "", "cmdk"},
		{"bare action group", []string{"action"}, "", "Run configured actions"},
		{"bare session group", []string{"session"}, "", "Resolve and manage cmdk sessions"},
		{"bare window group", []string{"window"}, "", "Switch between tmux windows"},
		{"window prev alias help", []string{"window", "prev", "--help"}, "", "previous tmux window"},
		{"shorten operand", []string{"shorten", "/usr/local/bin/tool"}, "", "/usr/local/bin/tool"},
		{"shorten stdin", []string{"shorten"}, "/opt/grammar-tool\n", "/opt/grammar-tool"},
		{"session resolve", []string{"session", "resolve", canary}, "", "session_kind:"},
		{"config", []string{"config"}, "", "shorten_home"},
		{"docs", []string{"docs"}, "", "display"},
		{"icons", []string{"icons"}, "", "ICON ALIASES"},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			code, stdout, stderr := runCmdkBinary(t, env, row.stdin, row.args...)
			if code != 0 {
				t.Fatalf("cmdk %s: exit = %d\nstdout: %s\nstderr: %s", strings.Join(row.args, " "), code, stdout, stderr)
			}
			if !strings.Contains(stdout, row.wantOut) {
				t.Errorf("cmdk %s: stdout missing %q\n%s", strings.Join(row.args, " "), row.wantOut, stdout)
			}
		})
	}
}

// startGrammarTmuxServer creates a base session with two fixed-name windows on
// the isolated socket and attaches a real client to it, so switch-centric
// commands would visibly move the client if they ran. The client's terminal is
// a pane of a second session on the same server, with TMUX unset so tmux
// allows the nested attach.
func startGrammarTmuxServer(t *testing.T) string {
	t.Helper()
	mustTmux(t, "new-session", "-d", "-s", "grammar-base", "-n", "one", "-x", "80", "-y", "24")
	mustTmux(t, "set-option", "-g", "automatic-rename", "off")
	mustTmux(t, "new-window", "-t", "grammar-base", "-n", "two")
	mustTmux(t, "select-window", "-t", "grammar-base:one")

	socketPath := strings.TrimSpace(mustTmux(t, "display-message", "-p", "-t", "grammar-base", "#{socket_path}"))
	mustTmux(t, "new-session", "-d", "-s", "grammar-host", "-n", "host", "-x", "120", "-y", "40",
		"env", "-u", "TMUX", "tmux", "-S", socketPath, "-f", "/dev/null", "attach-session", "-t", "grammar-base")

	deadline := time.Now().Add(defaultTimeout)
	for {
		clients, _ := tmuxCmd("list-clients", "-F", "#{client_session}").CombinedOutput()
		if strings.Contains(string(clients), "grammar-base") {
			return socketPath
		}
		if time.Now().After(deadline) {
			t.Fatalf("no client attached to grammar-base; clients: %q", clients)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func writeGrammarFixture(t *testing.T) (cfgHome, canary, marker string) {
	t.Helper()
	cfgHome = t.TempDir()
	canary = t.TempDir()
	marker = filepath.Join(canary, "grammar-marker")
	if err := os.WriteFile(filepath.Join(canary, "canary.txt"), []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(cfgHome, "cmdk")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`[startup]
path = %q

[display]
shorten_home = "~"

[[actions]]
name = "grammar-marker"
matches = "root"
launch_path = %q
cmd = "touch %s"
`, canary, canary, marker)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgHome, canary, marker
}

func grammarEnv(cfgHome string, overrides ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "XDG_CONFIG_HOME=") || strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TMUX_PANE=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "XDG_CONFIG_HOME="+cfgHome)
	return append(env, overrides...)
}

func runCmdkBinary(t *testing.T, env []string, stdin string, args ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Env = env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("cmdk %s timed out\nstdout: %s\nstderr: %s", strings.Join(args, " "), stdout.String(), stderr.String())
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("cmdk %s: %v", strings.Join(args, " "), err)
		}
		code = exitErr.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

func grammarTmuxSnapshot(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, spec := range [][]string{
		{"list-sessions", "-F", "session #{session_id} #{session_name}"},
		{"list-windows", "-a", "-F", "window #{session_id} #{window_id} #{window_index} #{window_name} active=#{window_active}"},
		{"list-panes", "-a", "-F", "pane #{window_id} #{pane_id} #{pane_index}"},
		{"list-clients", "-F", "client #{client_session}"},
	} {
		b.WriteString(mustTmux(t, spec...))
	}
	return b.String()
}

func dirSnapshot(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			entries = append(entries, rel+"/")
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s %q", rel, content))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	sort.Strings(entries)
	return root + ":\n" + strings.Join(entries, "\n") + "\n"
}

func mustTmux(t *testing.T, args ...string) string {
	t.Helper()
	out, err := tmuxCmd(args...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %v: %v\n%s", args, err, out)
	}
	return string(out)
}
