package trace

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

func WriteTable(w io.Writer, spans []Span) error {
	if len(spans) == 0 {
		return nil
	}

	nameWidth := len("Phase")
	for _, s := range spans {
		if len(s.Name) > nameWidth {
			nameWidth = len(s.Name)
		}
	}

	sep := strings.Repeat("-", nameWidth)
	durSep := "--------"

	p := func(format string, args ...any) error {
		_, err := fmt.Fprintf(w, format, args...)
		return err
	}

	if err := p("%-*s  %s\n", nameWidth, "Phase", "Duration"); err != nil {
		return err
	}
	if err := p("%s  %s\n", sep, durSep); err != nil {
		return err
	}
	for _, s := range spans {
		if err := p("%-*s  %s\n", nameWidth, s.Name, fmtDuration(s.Duration())); err != nil {
			return err
		}
	}
	if err := p("%s  %s\n", sep, durSep); err != nil {
		return err
	}
	return p("%-*s  %s\n", nameWidth, "total", fmtDuration(wallClock(spans)))
}

func toMS(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%.1fms", toMS(d))
}

func wallClock(spans []Span) time.Duration {
	if len(spans) == 0 {
		return 0
	}
	earliest := spans[0].Start
	latest := spans[0].End
	for _, s := range spans[1:] {
		if s.Start.Before(earliest) {
			earliest = s.Start
		}
		if s.End.After(latest) {
			latest = s.End
		}
	}
	return latest.Sub(earliest)
}

type jsonReport struct {
	Phases  []jsonPhase `json:"phases"`
	TotalMS float64     `json:"total_ms"`
}

type jsonPhase struct {
	Name       string  `json:"name"`
	DurationMS float64 `json:"duration_ms"`
}

func WriteJSON(w io.Writer, spans []Span) error {
	phases := make([]jsonPhase, len(spans))
	for i, s := range spans {
		phases[i] = jsonPhase{
			Name:       s.Name,
			DurationMS: toMS(s.Duration()),
		}
	}
	report := jsonReport{
		Phases:  phases,
		TotalMS: toMS(wallClock(spans)),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
