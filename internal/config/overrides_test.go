package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesOverridesBeforeValidationAndResolution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[display]\ntruncation_length = -1\ntruncation_symbol = ':nf-invalid:'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name       string
		length     int
		symbol     string
		wantSymbol string
		wantError  string
	}{
		{name: "resolve alias", length: 2, symbol: ":nf-cod-ellipsis:", wantSymbol: "\uea7c"},
		{name: "explicit zero and empty"},
		{name: "negative length", length: -1, wantError: "display.truncation_length"},
		{name: "unknown alias", length: 2, symbol: ":nf-invalid:", wantError: "display.truncation_symbol"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, report, err := Load(path, Overrides{TruncationLength: &test.length, TruncationSymbol: &test.symbol})
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("Load() error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Display.TruncationLength != test.length || cfg.Display.TruncationSymbol != test.wantSymbol {
				t.Errorf("Load().Display = %+v, want length %d and symbol %q", cfg.Display, test.length, test.wantSymbol)
			}
			for field, source := range map[string]string{
				"display.truncationlength": "--truncate",
				"display.truncationsymbol": "--truncate-symbol",
			} {
				if report.Updates[field] != source {
					t.Errorf("Load() source for %s = %q, want %q", field, report.Updates[field], source)
				}
			}
		})
	}
}

func TestLoadWithoutOverridesKeepsFileValuesAndSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[display]\ntruncation_length = 3\ntruncation_symbol = '...'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, report, err := Load(path, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Display.TruncationLength != 3 || cfg.Display.TruncationSymbol != "..." {
		t.Errorf("Load().Display = %+v, want file truncation values", cfg.Display)
	}
	for _, field := range []string{"display.truncationlength", "display.truncationsymbol"} {
		if report.Updates[field] != path {
			t.Errorf("Load() source for %s = %q, want %q", field, report.Updates[field], path)
		}
	}
}
