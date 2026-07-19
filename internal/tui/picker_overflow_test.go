package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/cmdk/internal/cmdrun"
	"github.com/jmcampanini/cmdk/internal/item"
)

func TestRunPickerSource_OverflowFailsInsteadOfTrimming(t *testing.T) {
	// One byte past the 4 MiB cap: the source must fail with KindOutput and
	// produce no items — never a silently shortened list.
	result, err := runPickerSource(`yes x | head -c 4194305`, 10*time.Second, item.Stage{Key: "test"})
	var cmdErr *cmdrun.CommandError
	if !errors.As(err, &cmdErr) || cmdErr.Kind != cmdrun.KindOutput {
		t.Fatalf("error = %v, want KindOutput CommandError", err)
	}
	if !strings.Contains(cmdErr.Headline(), "output exceeds") {
		t.Errorf("Headline = %q, want output-limit violation", cmdErr.Headline())
	}
	if len(result.Items) != 0 {
		t.Errorf("Items = %d, want none from an overflowing source", len(result.Items))
	}
}

func TestRunPickerSource_AtLimitSucceeds(t *testing.T) {
	result, err := runPickerSource(`yes x | head -c 4194304`, 10*time.Second, item.Stage{Key: "test"})
	if err != nil {
		t.Fatalf("at-limit output should succeed, got %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatal("expected items from an at-limit source")
	}
}
