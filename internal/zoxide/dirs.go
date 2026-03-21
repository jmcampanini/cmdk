package zoxide

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
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

func ParseDirs(output string, minScore float64, home, shortenHome string, rules []pathfmt.Rule) []item.Item {
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
			continue
		}
		if minScore > 0 && score < minScore {
			filtered++
			continue
		}

		it := item.NewItem()
		it.Type = "dir"
		it.Source = "zoxide"
		it.Display = pathfmt.DisplayPath(path, home, shortenHome, rules)
		it.Action = item.ActionNextList
		it.Data["path"] = path

		entries = append(entries, entry{score: score, item: it})
	}

	if filtered > 0 {
		slog.Debug("zoxide: filtered entries below min_score", "min_score", minScore, "filtered", filtered, "kept", len(entries))
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

func ListDirs(ctx context.Context, minScore float64, home, shortenHome string, rules []pathfmt.Rule) ([]item.Item, error) {
	out, err := exec.CommandContext(ctx, "zoxide", "query", "--list", "--score").Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("zoxide did not respond within the configured timeout")
		}
		return nil, err
	}
	return ParseDirs(string(out), minScore, home, shortenHome, rules), nil
}
