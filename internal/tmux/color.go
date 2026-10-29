package tmux

import (
	"context"
	"strings"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
)

// SupportsTrueColor reports whether tmux advertises an enabled Tc or RGB
// capability. Query failures leave the caller's detected color profile intact.
func SupportsTrueColor(ctx context.Context, timeout time.Duration) bool {
	result, err := tmuxQuery(ctx, tmuxQuerySpec(cmdrun.ShapeLines, timeout, "info"))
	if err != nil {
		return false
	}

	for line := range strings.SplitSeq(result.Stdout, "\n") {
		if (strings.Contains(line, "Tc") || strings.Contains(line, "RGB")) && strings.Contains(line, "true") {
			return true
		}
	}
	return false
}
