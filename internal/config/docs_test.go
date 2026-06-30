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
	if !strings.Contains(output, defaults.Display.ShortenHome) {
		t.Errorf("RenderHelp() should contain default shorten_home %q", defaults.Display.ShortenHome)
	}
}

func TestRenderHelp_ContainsThemeRecipe(t *testing.T) {
	output := RenderHelp()
	for _, want := range []string{
		"To apply an arbitrary palette:",
		"accent = primary",
		"Roles derive",
		"from semantic tokens",
		"cmdk config --validate /path/to/config.toml",
		"Catppuccin Frappe-style semantic mapping",
		"session_icon = \"#81c8be\"",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("RenderHelp() should contain theme recipe text %q", want)
		}
	}
}

func TestRenderHelp_ContainsSessionWindowDocs(t *testing.T) {
	output := RenderHelp()
	for _, want := range []string{
		"cmdk attach",
		"[startup].path",
		"outside tmux",
		"cmdk session window <path> --new",
		"cmdk session window <path> [--name <name>] -- <command> [args...]",
		"@cmdk_session_key",
		"switch-client",
		"Switch to session",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("RenderHelp() should contain %q", want)
		}
	}
}

func TestRenderHelp_ContainsTemplateVars(t *testing.T) {
	output := RenderHelp()
	for _, v := range []string{
		"{{.path}}",
		"{{.pane_id}}",
		"{{sq .path}}",
		"{{.session_attached}}",
		"{{.session_display}}",
		"{{.session_id}}",
		"{{.window_id}}",
		"{{.window_index}}",
		"{{.window_name}}",
		"{{.window_activity}}",
		"{{.launch_path}}",
		"{{.launch_basename}}",
		"{{.session_kind}}",
		"{{.session_name}}",
		"{{.session_windows}}",
		"{{.<key>}}",
	} {
		if !strings.Contains(output, v) {
			t.Errorf("RenderHelp() should contain template variable %q", v)
		}
	}
}
