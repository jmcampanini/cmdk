package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestConfigDocs_CoversAllFields(t *testing.T) {
	documented := make(map[string]bool)
	for _, section := range ConfigDocs() {
		for _, field := range section.Fields {
			documented[section.Name+"."+field.Name] = true
		}
	}

	structPaths := collectTOMLPaths(reflect.TypeFor[Config](), "")
	for _, path := range structPaths {
		if !documented[path] {
			t.Errorf("config field %q has no doc entry in ConfigDocs()", path)
		}
	}
}

func collectTOMLPaths(t reflect.Type, prefix string) []string {
	var paths []string
	for f := range t.Fields() {
		tomlKey := f.Tag.Get("toml")
		if tomlKey == "" || tomlKey == "-" {
			continue
		}

		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		switch {
		case ft.Kind() == reflect.Struct && ft != reflect.TypeFor[Config]():
			paths = append(paths, collectTOMLPaths(ft, tomlKey)...)
		case ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Struct:
			paths = append(paths, collectTOMLPaths(ft.Elem(), tomlKey)...)
		case ft.Kind() == reflect.Map && ft.Elem().Kind() == reflect.Struct:
			paths = append(paths, collectTOMLPaths(ft.Elem(), tomlKey)...)
		case ft.Kind() == reflect.Map:
			fallthrough
		default:
			if prefix != "" {
				paths = append(paths, prefix+"."+tomlKey)
			} else {
				paths = append(paths, tomlKey)
			}
		}
	}
	return paths
}

func TestRenderHelp_ContainsAllSections(t *testing.T) {
	output := RenderHelp()
	for _, section := range ConfigDocs() {
		if !strings.Contains(output, strings.ToUpper(section.Name)) {
			t.Errorf("RenderHelp() missing section %q", section.Name)
		}
	}
}

func TestRenderHelp_ContainsLiveDefaults(t *testing.T) {
	output := RenderHelp()
	defaults := DefaultConfig()

	if !strings.Contains(output, defaults.Timeout.Fetch.String()) {
		t.Errorf("RenderHelp() should contain default timeout %q", defaults.Timeout.Fetch)
	}
	if !strings.Contains(output, *defaults.Display.ShortenHome) {
		t.Errorf("RenderHelp() should contain default shorten_home %q", *defaults.Display.ShortenHome)
	}
}

func TestRenderHelp_ContainsTemplateVars(t *testing.T) {
	output := RenderHelp()
	for _, v := range []string{"{{.path}}", "{{.pane_id}}", "{{sq .path}}", "{{.<key>}}"} {
		if !strings.Contains(output, v) {
			t.Errorf("RenderHelp() should contain template variable %q", v)
		}
	}
}
