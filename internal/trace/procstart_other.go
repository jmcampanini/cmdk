//go:build !darwin

package trace

import "time"

func ProcessStartTime() (time.Time, error) {
	return time.Time{}, nil
}
