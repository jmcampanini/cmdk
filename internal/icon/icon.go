package icon

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rivo/uniseg"
)

type Entry struct {
	Alias       string
	Icon        string
	Description string
}

var registry map[string]string

func init() {
	registry = make(map[string]string, len(entries))
	for _, e := range entries {
		if _, dup := registry[e.Alias]; dup {
			panic(fmt.Sprintf("icon: duplicate alias %q", e.Alias))
		}
		registry[e.Alias] = e.Icon
	}
}

func Resolve(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("icon value cannot be empty")
	}

	if alias, ok := parseAlias(raw); ok {
		icon, exists := registry[alias]
		if !exists {
			if suggestion := suggestAlias(alias); suggestion != "" {
				return "", fmt.Errorf("unknown icon alias %q; did you mean :%s:?", raw, suggestion)
			}
			return "", fmt.Errorf("unknown icon alias %q; run \"cmdk icons --filter <term>\" to search aliases", raw)
		}
		return icon, nil
	}

	if n := uniseg.GraphemeClusterCount(raw); n != 1 {
		return "", fmt.Errorf("icon must be a single character, got %q (%d grapheme clusters)", raw, n)
	}
	return raw, nil
}

func All() []Entry {
	return slices.Clone(entries)
}

func parseAlias(raw string) (string, bool) {
	if strings.HasPrefix(raw, ":") && strings.HasSuffix(raw, ":") && len(raw) > 2 {
		return raw[1 : len(raw)-1], true
	}
	return "", false
}

func suggestAlias(invalid string) string {
	best := ""
	bestLen := 0
	for _, e := range entries {
		n := commonPrefixLen(invalid, e.Alias)
		if n > bestLen {
			bestLen = n
			best = e.Alias
		}
	}
	if bestLen >= 4 {
		return best
	}
	return ""
}

func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
