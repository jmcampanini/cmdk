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
		return resolveAlias(alias)
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

func resolveAlias(alias string) (string, error) {
	if glyph, ok := registry[alias]; ok {
		return glyph, nil
	}
	formatted := ":" + alias + ":"
	if suggestion := suggestAlias(alias); suggestion != "" {
		return "", fmt.Errorf("unknown icon alias %q; did you mean :%s:?", formatted, suggestion)
	}
	return "", fmt.Errorf("unknown icon alias %q; run \"cmdk icons --filter <term>\" to search aliases", formatted)
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

func ResolveInline(s string) (string, error) {
	if !strings.Contains(s, ":") {
		return s, nil
	}

	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		colonPos := strings.IndexByte(s[i:], ':')
		if colonPos == -1 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+colonPos])
		i += colonPos

		endPos := strings.IndexByte(s[i+1:], ':')
		if endPos == -1 {
			if strings.HasPrefix(s[i:], ":nf-") {
				return "", fmt.Errorf("unterminated icon alias in %q (missing closing colon)", s)
			}
			b.WriteString(s[i:])
			break
		}

		alias := s[i+1 : i+1+endPos]
		if !strings.HasPrefix(alias, "nf-") {
			b.WriteByte(':')
			i++
			continue
		}

		glyph, err := resolveAlias(alias)
		if err != nil {
			return "", err
		}
		b.WriteString(glyph)
		i += 1 + endPos + 1
	}
	return b.String(), nil
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
