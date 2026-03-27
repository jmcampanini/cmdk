//go:build darwin

package trace

import (
	"testing"
	"time"
)

func TestProcessStartTime(t *testing.T) {
	before := time.Now()
	got, err := ProcessStartTime()
	if err != nil {
		t.Fatalf("ProcessStartTime() error: %v", err)
	}
	if got.IsZero() {
		t.Fatal("ProcessStartTime() returned zero time")
	}
	if got.After(before) {
		t.Errorf("process start time %v is after test start %v", got, before)
	}
	if time.Since(got) > 60*time.Second {
		t.Errorf("process start time %v is more than 60s ago", got)
	}
}
