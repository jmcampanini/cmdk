package trace

import (
	"sync"
	"testing"
	"time"
)

func TestRealTracer_Begin(t *testing.T) {
	tr := New(time.Time{})
	stop := tr.Begin("test-phase")
	time.Sleep(time.Millisecond)
	stop()

	spans := tr.Spans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Name != "test-phase" {
		t.Errorf("name = %q, want %q", spans[0].Name, "test-phase")
	}
	if spans[0].Duration() <= 0 {
		t.Error("duration should be positive")
	}
}

func TestRealTracer_MultipleSpans_ChronologicalOrder(t *testing.T) {
	tr := New(time.Time{})
	stop := tr.Begin("first")
	stop()
	stop = tr.Begin("second")
	stop()

	spans := tr.Spans()
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	if spans[0].Name != "first" {
		t.Errorf("spans[0].Name = %q, want %q", spans[0].Name, "first")
	}
	if spans[1].Name != "second" {
		t.Errorf("spans[1].Name = %q, want %q", spans[1].Name, "second")
	}
}

func TestRealTracer_ConcurrentBegin(t *testing.T) {
	tr := New(time.Time{})
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			stop := tr.Begin("concurrent")
			time.Sleep(time.Millisecond)
			stop()
		})
	}
	wg.Wait()

	spans := tr.Spans()
	if len(spans) != 10 {
		t.Fatalf("got %d spans, want 10", len(spans))
	}
}

func TestRealTracer_ShellToProcess(t *testing.T) {
	processStart := time.Now()
	time.Sleep(time.Millisecond)

	tr := New(processStart)
	stop := tr.Begin("logging")
	stop()

	spans := tr.Spans()
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	if spans[0].Name != "shell-to-process" {
		t.Errorf("spans[0].Name = %q, want %q", spans[0].Name, "shell-to-process")
	}
	if spans[0].Duration() <= 0 {
		t.Error("shell-to-process duration should be positive")
	}
}

func TestRealTracer_NoShellToProcessWhenZero(t *testing.T) {
	tr := New(time.Time{})
	stop := tr.Begin("logging")
	stop()

	spans := tr.Spans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Name == "shell-to-process" {
		t.Error("should not have shell-to-process span when processStart is zero")
	}
}

func TestRealTracer_ProcessStartWithNoSpans(t *testing.T) {
	tr := New(time.Now())
	spans := tr.Spans()
	if len(spans) != 0 {
		t.Fatalf("got %d spans, want 0", len(spans))
	}
}

func TestRealTracer_DoubleStopIsIdempotent(t *testing.T) {
	tr := New(time.Time{})
	stop := tr.Begin("phase")
	stop()
	stop()

	spans := tr.Spans()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1 (double-stop should be idempotent)", len(spans))
	}
}

func TestNoopTracer(t *testing.T) {
	tr := Noop()
	stop := tr.Begin("anything")
	stop() // should not panic

	spans := tr.Spans()
	if spans != nil {
		t.Errorf("noop Spans() should return nil, got %v", spans)
	}
}
