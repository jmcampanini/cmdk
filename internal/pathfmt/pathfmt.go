package pathfmt

import (
	"os"
	"strings"
)

func DisplayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	home = strings.TrimRight(home, "/")

	if path == home {
		return "~"
	}

	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}

	return path
}
