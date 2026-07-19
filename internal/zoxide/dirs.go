package zoxide

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	log "charm.land/log/v2"
	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

const (
	// A mature zoxide database legitimately produces thousands of
	// "score path" lines (tens of KiB to low MiB); the cap sits far above
	// that, and exceeding it fails the source rather than showing a
	// silently shortened list.
	zoxideMaxStdout = 4 << 20
	zoxideMaxStderr = 64 << 10
)

func splitScorePath(line string) (float64, string, bool) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	score, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, "", false
	}
	path := parts[1]
	if path == "" {
		return 0, "", false
	}
	return score, path, true
}

func ParseDirs(output string, minScore float64, home, shortenHome string, rules []pathfmt.Rule, trunc pathfmt.Truncation) []item.Item {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	type entry struct {
		score float64
		item  item.Item
	}

	var entries []entry
	var filtered int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		score, path, ok := splitScorePath(line)
		if !ok {
			log.Debug("zoxide: skipping unparseable line", "line", line)
			continue
		}
		if minScore > 0 && score < minScore {
			filtered++
			continue
		}

		it := item.NewItem()
		it.Type = "dir"
		it.Source = "zoxide"
		it.Display = pathfmt.DisplayPath(path, home, shortenHome, rules, trunc)
		it.Action = item.ActionNextList
		it.Data["path"] = path

		entries = append(entries, entry{score: score, item: it})
	}

	if filtered > 0 {
		log.Debug("zoxide: filtered entries below min_score", "min_score", minScore, "filtered", filtered, "kept", len(entries))
	}

	slices.SortStableFunc(entries, func(a, b entry) int {
		return cmp.Compare(b.score, a.score)
	})

	items := make([]item.Item, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	return items
}

// ListDirs lists zoxide's scored directories. timeout bounds the zoxide
// invocation (callers pass the configured fetch timeout).
func ListDirs(ctx context.Context, timeout time.Duration, minScore float64, home, shortenHome string, rules []pathfmt.Rule, trunc pathfmt.Truncation) ([]item.Item, error) {
	res, err := cmdrun.Query(ctx, cmdrun.QuerySpec{
		Op:        "zoxide query",
		Argv:      []string{"zoxide", "query", "--list", "--score"},
		Timeout:   timeout,
		Shape:     cmdrun.ShapeLines,
		MaxStdout: zoxideMaxStdout,
		MaxStderr: zoxideMaxStderr,
	})
	if err != nil {
		return nil, err
	}
	return ParseDirs(res.Stdout, minScore, home, shortenHome, rules, trunc), nil
}
