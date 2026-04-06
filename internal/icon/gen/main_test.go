package main

import "testing"

func TestDescriptionFromAlias(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"cod-terminal_tmux", "Terminal tmux"},
		{"dev-go", "Go"},
		{"oct-git_branch", "Git branch"},
		{"cod-arrow_circle_down", "Arrow circle down"},
		{"dev-a", "A"},
	}
	for _, tt := range tests {
		got := descriptionFromAlias(tt.key)
		if got != tt.want {
			t.Errorf("descriptionFromAlias(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestDescriptionFromAlias_NoDash(t *testing.T) {
	got := descriptionFromAlias("nodash")
	if got != "" {
		t.Errorf("descriptionFromAlias(\"nodash\") = %q, want empty", got)
	}
}

func TestHasAllowedPrefix(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"cod-terminal", true},
		{"dev-go", true},
		{"oct-git_branch", true},
		{"md-folder", false},
		{"fa-terminal", false},
		{"METADATA", false},
	}
	for _, tt := range tests {
		got := hasAllowedPrefix(tt.key)
		if got != tt.want {
			t.Errorf("hasAllowedPrefix(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}
