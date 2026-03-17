package zoxide

import (
	"cmp"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/jmcampanini/cmdk/internal/item"
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

func ParseDirs(output string) []item.Item {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	type entry struct {
		score float64
		item  item.Item
	}

	var entries []entry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		score, path, ok := splitScorePath(line)
		if !ok {
			continue
		}

		it := item.NewItem()
		it.Type = "dir"
		it.Source = "zoxide"
		it.Display = path
		it.Action = item.ActionNextList
		it.Data["path"] = path

		entries = append(entries, entry{score: score, item: it})
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

func ListDirs() ([]item.Item, error) {
	out, err := exec.Command("zoxide", "query", "--list", "--score").Output()
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		return ParseDirs(string(out)), nil
	}
	return nil, nil
}
