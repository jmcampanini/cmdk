package cmd

import "testing"

func useTempConfigHome(t *testing.T) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	oldConfigPath := configPath
	configPath = ""
	t.Cleanup(func() { configPath = oldConfigPath })
	return xdg
}
