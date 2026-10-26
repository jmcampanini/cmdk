package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDocs_CoversAllFields(t *testing.T) {
	documented := make(map[string]bool)
	for _, section := range Docs() {
		for _, field := range section.Fields {
			documented[section.Name+"."+field.Name] = true
		}
	}

	structPaths := collectTOMLPaths(reflect.TypeFor[Config](), "")
	for _, path := range structPaths {
		if !documented[path] {
			t.Errorf("config field %q has no doc entry in Docs()", path)
		}
	}
}

func collectTOMLPaths(t reflect.Type, prefix string) []string {
	var paths []string
	for f := range t.Fields() {
		tomlTag := f.Tag.Get("toml")
		tomlKey := strings.SplitN(tomlTag, ",", 2)[0]
		if tomlKey == "" || tomlKey == "-" {
			continue
		}

		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		path := joinTOMLPath(prefix, tomlKey)

		switch {
		case ft.Kind() == reflect.Struct && ft != reflect.TypeFor[Config]():
			paths = append(paths, collectTOMLPaths(ft, path)...)
		case ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct:
			paths = append(paths, collectTOMLPaths(ft.Elem(), tomlKey)...)
		case ft.Kind() == reflect.Map && ft.Elem().Kind() == reflect.Struct:
			paths = append(paths, collectTOMLPaths(ft.Elem(), path)...)
		case ft.Kind() == reflect.Map:
			fallthrough
		default:
			paths = append(paths, path)
		}
	}
	return paths
}

func joinTOMLPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// unwrapped collapses the wrapped reference into one line so a test can look
// for a phrase without knowing where the renderer broke it.
func unwrapped(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func TestRenderReference_ContainsEveryTableAndKey(t *testing.T) {
	output := RenderReference()
	flat := unwrapped(output)
	for _, section := range Docs() {
		if !strings.Contains(output, section.Table+"\n") {
			t.Errorf("RenderReference() missing table header %q", section.Table)
		}
		for _, field := range section.Fields {
			if !strings.Contains(output, "  "+field.Name+"  "+field.Type) {
				t.Errorf("RenderReference() missing key %s.%s", section.Name, field.Name)
			}
			if field.Validation != "" && !strings.Contains(flat, "Constraint: "+field.Validation) {
				t.Errorf("RenderReference() missing constraint for %s.%s", section.Name, field.Name)
			}
		}
	}
}

func TestRenderReference_ContainsLiveDefaults(t *testing.T) {
	output := RenderReference()
	defaults := DefaultConfig()

	for _, want := range []string{
		`fetch  duration (default "` + defaults.Timeout.Fetch.String() + `")`,
		`shorten_home  string (default "` + defaults.Display.ShortenHome + `")`,
		`window_name_max_length  int (default 20)`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("RenderReference() should contain %q", want)
		}
	}
}

func TestRenderReference_KeepsThemeRecipeSteps(t *testing.T) {
	output := RenderReference()
	for _, want := range []string{"\n  1. Pick the table", "\n     palettes"} {
		if !strings.Contains(output, want) {
			t.Errorf("RenderReference() should contain %q", want)
		}
	}
	flat := unwrapped(output)
	for _, want := range []string{
		"accent = primary accent",
		"Roles derive from the tokens",
		`"cmdk config --validate <path>"`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("RenderReference() should contain %q", want)
		}
	}
}

func TestRenderReference_FitsEightyColumns(t *testing.T) {
	for i, line := range strings.Split(RenderReference(), "\n") {
		if width := ansi.StringWidth(line); width > referenceWidth {
			t.Errorf("line %d is %d columns, want at most %d: %q", i+1, width, referenceWidth, line)
		}
	}
}

func TestWrapWords(t *testing.T) {
	got := wrapWords("one two three four", 9)
	want := []string{"one two", "three", "four"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("wrapWords() = %q, want %q", got, want)
	}
	if got := wrapWords("extraordinarily", 5); len(got) != 1 || got[0] != "extraordinarily" {
		t.Errorf("wrapWords() split a single long word: %q", got)
	}
}
