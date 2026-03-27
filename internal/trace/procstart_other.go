//go:build !darwin

package trace

import "time"

// ProcessStartTime is unavailable on this platform; the zero time causes
// the shell-to-process span to be silently omitted via the IsZero guard in Spans.
func ProcessStartTime() (time.Time, error) {
	return time.Time{}, nil
}
