package trace

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func makeSpans() []Span {
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return []Span{
		{Name: "logging", Start: t0, End: t0.Add(2 * time.Millisecond)},
		{Name: "config", Start: t0.Add(2 * time.Millisecond), End: t0.Add(5 * time.Millisecond)},
		{Name: "tea-ready", Start: t0.Add(5 * time.Millisecond), End: t0.Add(20 * time.Millisecond)},
	}
}

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, makeSpans()); err != nil {
		t.Fatalf("WriteTable error: %v", err)
	}
	out := buf.String()

	for _, phase := range []string{"logging", "config", "tea-ready", "total"} {
		if !strings.Contains(out, phase) {
			t.Errorf("table missing phase %q", phase)
		}
	}
	if !strings.Contains(out, "Phase") {
		t.Error("table missing header")
	}
	if !strings.Contains(out, "20.0ms") {
		t.Errorf("table missing total duration; got:\n%s", out)
	}
}

func TestWriteTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, nil); err != nil {
		t.Fatalf("WriteTable error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil spans, got %q", buf.String())
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, makeSpans()); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var report jsonReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("JSON parse error: %v\nraw: %s", err, buf.String())
	}

	if len(report.Phases) != 3 {
		t.Fatalf("got %d phases, want 3", len(report.Phases))
	}
	if report.Phases[0].Name != "logging" {
		t.Errorf("phases[0].Name = %q, want %q", report.Phases[0].Name, "logging")
	}
	if report.Phases[0].DurationMS != 2.0 {
		t.Errorf("phases[0].DurationMS = %v, want 2.0", report.Phases[0].DurationMS)
	}
	if report.TotalMS != 20.0 {
		t.Errorf("TotalMS = %v, want 20.0", report.TotalMS)
	}
}

func TestWriteJSON_WithShellToProcess(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	spans := []Span{
		{Name: "shell-to-process", Start: t0.Add(-10 * time.Millisecond), End: t0},
		{Name: "logging", Start: t0, End: t0.Add(2 * time.Millisecond)},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, spans); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var report jsonReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if report.Phases[0].Name != "shell-to-process" {
		t.Errorf("first phase = %q, want shell-to-process", report.Phases[0].Name)
	}
	if report.Phases[0].DurationMS != 10.0 {
		t.Errorf("shell-to-process duration = %v, want 10.0", report.Phases[0].DurationMS)
	}
	if report.TotalMS != 12.0 {
		t.Errorf("TotalMS = %v, want 12.0", report.TotalMS)
	}
}
