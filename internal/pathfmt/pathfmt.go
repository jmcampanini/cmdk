package pathfmt

import (
	"os"
	"slices"
	"strings"
)

type Rule struct {
	Match   string
	Replace string
}

func CompileRules(rules map[string]string) []Rule {
	compiled := make([]Rule, 0, len(rules))
	for match, replace := range rules {
		compiled = append(compiled, Rule{Match: match, Replace: replace})
	}
	slices.SortFunc(compiled, func(a, b Rule) int {
		if len(a.Match) != len(b.Match) {
			return len(b.Match) - len(a.Match)
		}
		return strings.Compare(a.Match, b.Match)
	})
	return compiled
}

func DisplayPath(path string, shortenHome string, rules []Rule) string {
	path = replaceHome(path, shortenHome)
	for _, r := range rules {
		path = strings.Replace(path, r.Match, r.Replace, 1)
	}
	return path
}

func replaceHome(path string, shortenHome string) string {
	if shortenHome == "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	home = strings.TrimRight(home, "/")
	if path == home {
		return shortenHome
	}
	if strings.HasPrefix(path, home+"/") {
		return shortenHome + path[len(home):]
	}
	return path
}
