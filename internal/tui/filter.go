package tui

import (
	"os"
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
)

func pathAwareFilter(term string, targets []string) []list.Rank {
	ranks := list.DefaultFilter(term, targets)

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ranks
	}
	home = strings.TrimRight(home, "/")

	matched := make(map[int]bool, len(ranks))
	for _, r := range ranks {
		matched[r.Index] = true
	}

	var absTargets []string
	var absIndexes []int
	for i, t := range targets {
		if matched[i] {
			continue
		}
		var absPath string
		switch {
		case t == "~":
			absPath = home
		case strings.HasPrefix(t, "~/"):
			absPath = home + t[1:]
		default:
			continue
		}
		absTargets = append(absTargets, absPath)
		absIndexes = append(absIndexes, i)
	}

	if len(absTargets) == 0 {
		return ranks
	}

	absRanks := list.DefaultFilter(term, absTargets)
	for _, r := range absRanks {
		origIdx := absIndexes[r.Index]
		if matched[origIdx] {
			continue
		}
		ranks = append(ranks, list.Rank{
			Index:          origIdx,
			MatchedIndexes: remapAbsToDisplay(r.MatchedIndexes, len(home)),
		})
	}

	return ranks
}

func remapAbsToDisplay(indexes []int, homeLen int) []int {
	seen := make(map[int]bool, len(indexes))
	result := make([]int, 0, len(indexes))
	for _, idx := range indexes {
		var displayIdx int
		if idx < homeLen {
			displayIdx = 0
		} else {
			displayIdx = idx - homeLen + 1
		}
		if !seen[displayIdx] {
			seen[displayIdx] = true
			result = append(result, displayIdx)
		}
	}
	sort.Ints(result)
	return result
}
