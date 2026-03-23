package icon

import (
	"fmt"
	"strings"

	"github.com/rivo/uniseg"
)

type Entry struct {
	Alias       string
	Icon        string
	Description string
}

var entries = []Entry{
	// Dev Tools / Languages
	{"nf-dev-apple", "\ue711", "Apple"},
	{"nf-dev-css3", "\ue749", "CSS3"},
	{"nf-dev-docker", "\ue7b0", "Docker"},
	{"nf-dev-git", "\ue702", "Git"},
	{"nf-dev-github", "\ue709", "GitHub"},
	{"nf-dev-go", "\ue724", "Go"},
	{"nf-dev-html5", "\ue736", "HTML5"},
	{"nf-dev-java", "\ue738", "Java"},
	{"nf-dev-javascript", "\ue781", "JavaScript"},
	{"nf-dev-linux", "\ue712", "Linux"},
	{"nf-dev-lua", "\ue826", "Lua"},
	{"nf-dev-nodejs", "\ue719", "Node.js"},
	{"nf-dev-npm", "\ue71e", "npm"},
	{"nf-dev-python", "\ue73c", "Python"},
	{"nf-dev-react", "\ue7ba", "React"},
	{"nf-dev-ruby", "\ue739", "Ruby"},
	{"nf-dev-rust", "\ue7a8", "Rust"},
	{"nf-dev-swift", "\ue755", "Swift"},
	{"nf-dev-typescript", "\ue8ca", "TypeScript"},
	{"nf-dev-vim", "\ue7c5", "Vim"},

	// Material Design (Files, Folders, UI, Actions)
	{"nf-md-archive", "\U000f003c", "Archive"},
	{"nf-md-bookmark", "\U000f00c0", "Bookmark"},
	{"nf-md-bug", "\U000f00e4", "Bug"},
	{"nf-md-clipboard", "\U000f0147", "Clipboard"},
	{"nf-md-cloud", "\U000f015f", "Cloud"},
	{"nf-md-cog", "\U000f0493", "Gear/settings"},
	{"nf-md-console", "\U000f018d", "Terminal"},
	{"nf-md-database", "\U000f01bc", "Database"},
	{"nf-md-download", "\U000f01da", "Download"},
	{"nf-md-eye", "\U000f0208", "Eye"},
	{"nf-md-file", "\U000f0214", "File"},
	{"nf-md-file_code", "\U000f022e", "File code"},
	{"nf-md-folder", "\U000f024b", "Folder"},
	{"nf-md-folder_open", "\U000f0770", "Folder open"},
	{"nf-md-home", "\U000f02dc", "Home"},
	{"nf-md-lightning_bolt", "\U000f140b", "Lightning bolt"},
	{"nf-md-lock", "\U000f033e", "Lock"},
	{"nf-md-lock_open", "\U000f033f", "Unlock"},
	{"nf-md-magnify", "\U000f0349", "Search"},
	{"nf-md-package", "\U000f03d3", "Package"},
	{"nf-md-play", "\U000f040a", "Play"},
	{"nf-md-refresh", "\U000f0450", "Refresh"},
	{"nf-md-rocket", "\U000f0463", "Rocket"},
	{"nf-md-server", "\U000f048b", "Server"},
	{"nf-md-source_branch", "\U000f062c", "Code branch"},
	{"nf-md-star", "\U000f04ce", "Star"},
	{"nf-md-stop", "\U000f04db", "Stop"},
	{"nf-md-tag", "\U000f04f9", "Tag"},
	{"nf-md-upload", "\U000f0552", "Upload"},
	{"nf-md-wrench", "\U000f05b7", "Wrench"},
}

var registry map[string]string

func init() {
	registry = make(map[string]string, len(entries))
	for _, e := range entries {
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
			return "", fmt.Errorf("unknown icon alias %q; run \"cmdk docs icons\" to see supported aliases", raw)
		}
		return icon, nil
	}

	if n := uniseg.GraphemeClusterCount(raw); n != 1 {
		return "", fmt.Errorf("icon must be a single character, got %q (%d grapheme clusters)", raw, n)
	}
	return raw, nil
}

func All() []Entry {
	return entries
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
	for alias := range registry {
		prefix := commonPrefix(invalid, alias)
		if prefix > bestLen {
			bestLen = prefix
			best = alias
		}
	}
	if bestLen >= 4 {
		return best
	}
	return ""
}

func commonPrefix(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
