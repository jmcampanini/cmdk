package tmux

import "github.com/jmcampanini/cmdk/internal/pathfmt"

// DisplayOptions controls how tmux session identifiers are rendered in item
// display strings. The underlying item data remains unshortened.
type DisplayOptions struct {
	Home              string
	ShortenHome       string
	Rules             []pathfmt.Rule
	SessionTruncation pathfmt.Truncation
}

func (o DisplayOptions) formatSessionValue(value string) string {
	if value == "" {
		return ""
	}
	return pathfmt.DisplayPath(value, o.Home, o.ShortenHome, o.Rules, o.SessionTruncation)
}

func (o DisplayOptions) formatSessionDisplay(sessionName, sessionKey string) string {
	return o.formatSessionValue(sessionDisplayValue(sessionName, sessionKey))
}

func sessionDisplayValue(sessionName, sessionKey string) string {
	if sessionKey != "" {
		return sessionKey
	}
	return sessionName
}
