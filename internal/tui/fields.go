package tui

import "strings"

// extractField returns a single field from a delimited line.
// Index is 1-based; 0 returns the whole line.
// Out-of-bounds indices fall back to the whole line.
func extractField(line, delimiter string, index int) string {
	if index < 1 {
		return line
	}
	parts := strings.Split(line, delimiter)
	if index > len(parts) {
		return line
	}
	return parts[index-1]
}
