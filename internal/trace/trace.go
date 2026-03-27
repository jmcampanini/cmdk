package trace

import (
	"slices"
	"sync"
	"time"
)

type Span struct {
	Name  string
	Start time.Time
	End   time.Time
}

func (s Span) Duration() time.Duration { return s.End.Sub(s.Start) }

type Tracer interface {
	Begin(name string) func()
	Spans() []Span
}

type realTracer struct {
	processStart time.Time
	mu           sync.Mutex
	spans        []Span
}

func New(processStart time.Time) Tracer {
	return &realTracer{processStart: processStart}
}

func (t *realTracer) Begin(name string) func() {
	start := time.Now()
	return func() {
		span := Span{Name: name, Start: start, End: time.Now()}
		t.mu.Lock()
		t.spans = append(t.spans, span)
		t.mu.Unlock()
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

func Noop() Tracer { return noopTracer{} }

func (noopTracer) Begin(string) func() { return noopStop }
func (noopTracer) Spans() []Span       { return nil }
