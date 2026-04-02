package pathfmt

import (
	"slices"
	"strings"
)

type Rule struct {
	Match   string
	Replace string
}

type Truncation struct {
	Length int
	Symbol string
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

func DisplayPath(path, home, shortenHome string, rules []Rule, trunc Truncation) string {
	path = replaceHome(path, home, shortenHome)
	for _, r := range rules {
		path = strings.Replace(path, r.Match, r.Replace, 1)
	}
	return truncateParts(path, trunc)
}

func truncateParts(path string, trunc Truncation) string {
	if trunc.Length <= 0 {
		return path
	}
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	if len(parts) <= trunc.Length {
		return path
	}
	tail := strings.Join(parts[len(parts)-trunc.Length:], "/")
	if trunc.Symbol != "" {
		return trunc.Symbol + "/" + tail
	}
	return tail
}

func replaceHome(path, home, shortenHome string) string {
	if shortenHome == "" || home == "" {
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
