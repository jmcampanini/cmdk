package tmux

import (
	"context"
	"io"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

// Byte limits per call-site class. Small covers single-line results (IDs,
// version strings) and expect-empty mutations; list covers list-sessions and
// list-windows output, whose size scales with user state (session and window
// names are user-controlled), so the cap sits far above the measured
// legitimate worst case of a few hundred KiB.
const (
	tmuxSmallStdoutLimit = 4 << 10
	tmuxSmallStderrLimit = 32 << 10
	tmuxListStdoutLimit  = 4 << 20
	tmuxListStderrLimit  = 64 << 10
)

// Timeouts carries the per-command deadlines tmux invocations run under.
// Callers build it from config: Query from timeout.fetch, Mutation from
// timeout.mutation. Both are required by the underlying runner; attach is a
// streaming command and deliberately runs without a deadline.
type Timeouts struct {
	// Query bounds read-only commands: list-sessions, list-windows,
	// display-message.
	Query time.Duration
	// Mutation bounds state-changing commands: new-session, new-window,
	// set-option, switch-client.
	Mutation time.Duration
}

// TerminalIO carries the caller-injected streams for terminal-takeover
// commands (attach-session). Reference os.Stdin/os.Stdout/os.Stderr only at
// the composition root (the cmd package).
type TerminalIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// tmuxRunner is the seam tests fake. Production code always executes through
// cmdrun, so every tmux call site declares its shape, limits, and timeout.
type tmuxRunner interface {
	query(ctx context.Context, spec cmdrun.QuerySpec) (cmdrun.Result, error)
	stream(ctx context.Context, spec cmdrun.StreamSpec) error
}

type execTmuxRunner struct{}

func (execTmuxRunner) query(ctx context.Context, spec cmdrun.QuerySpec) (cmdrun.Result, error) {
	return cmdrun.Query(ctx, spec)
}

func (execTmuxRunner) stream(ctx context.Context, spec cmdrun.StreamSpec) error {
	return cmdrun.Stream(ctx, spec)
}

// tmuxQuery executes a tmux query outside any runner seam; list sources use
// it directly and are tested through their pure Parse* halves plus real-tmux
// integration tests.
func tmuxQuery(ctx context.Context, spec cmdrun.QuerySpec) (cmdrun.Result, error) {
	return execTmuxRunner{}.query(ctx, spec)
}

// tmuxQuerySpec builds the QuerySpec for one tmux invocation, deriving the
// byte limits from the declared shape: ShapeLines gets the list limits,
// everything else the small ones.
func tmuxQuerySpec(shape cmdrun.Shape, timeout time.Duration, args ...string) cmdrun.QuerySpec {
	maxStdout, maxStderr := tmuxSmallStdoutLimit, tmuxSmallStderrLimit
	if shape == cmdrun.ShapeLines {
		maxStdout, maxStderr = tmuxListStdoutLimit, tmuxListStderrLimit
	}
	return cmdrun.QuerySpec{
		Op:        "tmux " + args[0],
		Argv:      append([]string{"tmux"}, args...),
		Timeout:   timeout,
		Shape:     shape,
		MaxStdout: maxStdout,
		MaxStderr: maxStderr,
	}
}

func tmuxStreamSpec(terminal TerminalIO, args ...string) cmdrun.StreamSpec {
	return cmdrun.StreamSpec{
		Op:     "tmux " + args[0],
		Argv:   append([]string{"tmux"}, args...),
		Stdin:  terminal.Stdin,
		Stdout: terminal.Stdout,
		Stderr: terminal.Stderr,
	}
}
