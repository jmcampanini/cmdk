package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
)

// checkLauncherTerminal checks each capability the launcher needs before it
// starts: it draws on stdout, reads keys from stdin, and moves the cursor, so
// each is checked on its own rather than inferred from the others. Without
// the stdin check bubbletea would open /dev/tty silently.
func checkLauncherTerminal(stdin *os.File, stdout *os.File) error {
	if !term.IsTerminal(stdout.Fd()) {
		return errors.New("stdout is not a terminal; run cmdk from a terminal or a tmux popup, or pass --timings to print startup timings without the launcher")
	}
	if !term.IsTerminal(stdin.Fd()) {
		return errors.New("stdin is not a terminal; run cmdk from a terminal or a tmux popup without redirecting its input")
	}
	return checkCursorControl(os.Getenv("TERM"))
}

// checkCursorControl treats cursor control as its own capability: a stream
// can be a terminal that still cannot move the cursor when TERM is unset or
// names the dumb terminal.
func checkCursorControl(termType string) error {
	switch termType {
	case "":
		return errors.New("TERM is not set, so the terminal cannot carry cursor control; set TERM to your terminal's type (for example tmux-256color)")
	case "dumb":
		return fmt.Errorf("TERM is %s, so the terminal cannot carry cursor control; set TERM to your terminal's type (for example tmux-256color)", termType)
	default:
		return nil
	}
}
