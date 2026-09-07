// Package trace records startup phases and renders their timing reports.
package trace

import (
	"slices"
	"sync"
	"time"
)

// Span records the start and end of a named startup phase.
type Span struct {
	Name  string
	Start time.Time
	End   time.Time
}

// Duration returns the elapsed time between the phase boundaries.
func (s Span) Duration() time.Duration { return s.End.Sub(s.Start) }

// Tracer records named phases and returns their completed timing spans.
type Tracer interface {
	Begin(name string) func()
	Spans() []Span
}

type realTracer struct {
	processStart time.Time
	mu           sync.Mutex
	spans        []Span
}

// New creates a concurrent-safe tracer that can include shell-to-process startup time.
func New(processStart time.Time) Tracer {
	return &realTracer{processStart: processStart}
}

func (t *realTracer) Begin(name string) func() {
	start := time.Now()
	var once sync.Once
	return func() {
		once.Do(func() {
			span := Span{Name: name, Start: start, End: time.Now()}
			t.mu.Lock()
			t.spans = append(t.spans, span)
			t.mu.Unlock()
		})
	}
}

func (t *realTracer) Spans() []Span {
	t.mu.Lock()
	spans := slices.Clone(t.spans)
	t.mu.Unlock()

	slices.SortFunc(spans, func(a, b Span) int {
		return a.Start.Compare(b.Start)
	})

	if !t.processStart.IsZero() && len(spans) > 0 {
		shell := Span{
			Name:  "shell-to-process",
			Start: t.processStart,
			End:   spans[0].Start,
		}
		spans = slices.Insert(spans, 0, shell)
	}

	return spans
}

var noopStop = func() {}

type noopTracer struct{}

// Noop returns a tracer that ignores phases and produces no spans.
func Noop() Tracer { return noopTracer{} }

func (noopTracer) Begin(string) func() { return noopStop }
func (noopTracer) Spans() []Span       { return nil }
