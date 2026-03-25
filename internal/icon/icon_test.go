package icon

import (
	"strings"
	"testing"
)

func TestResolve_Alias(t *testing.T) {
	got, err := Resolve(":nf-dev-github:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "\ue709" {
		t.Errorf("got %q, want \\ue709", got)
	}
}

func TestResolve_AllAliases(t *testing.T) {
	for _, e := range All() {
		got, err := Resolve(":" + e.Alias + ":")
		if err != nil {
			t.Errorf("Resolve(:%s:) error: %v", e.Alias, err)
			continue
		}
		if got != e.Icon {
			t.Errorf("Resolve(:%s:) = %q, want %q", e.Alias, got, e.Icon)
		}
	}
}

func TestResolve_UnknownAlias(t *testing.T) {
	_, err := Resolve(":nf-fake-thing:")
	if err == nil {
		t.Fatal("expected error for unknown alias")
	}
	if !strings.Contains(err.Error(), "unknown icon alias") {
		t.Errorf("error = %q, want 'unknown icon alias'", err.Error())
	}
}

func TestResolve_UnknownAliasWithSuggestion(t *testing.T) {
	_, err := Resolve(":nf-dev-gith:")
	if err == nil {
		t.Fatal("expected error for unknown alias")
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error = %q, want suggestion", err.Error())
	}
}

func TestResolve_RawUnicode(t *testing.T) {
	got, err := Resolve("\ue709")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "\ue709" {
		t.Errorf("got %q, want \\ue709", got)
	}
}

func TestResolve_MultipleGraphemes(t *testing.T) {
	_, err := Resolve("ab")
	if err == nil {
		t.Fatal("expected error for multiple graphemes")
	}
	if !strings.Contains(err.Error(), "single character") {
		t.Errorf("error = %q, want 'single character'", err.Error())
	}
}

func TestResolve_Empty(t *testing.T) {
	_, err := Resolve("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestAll_Sorted(t *testing.T) {
	all := All()
	for i := 1; i < len(all); i++ {
		if all[i].Alias < all[i-1].Alias {
			t.Errorf("entries not sorted: %q comes after %q", all[i].Alias, all[i-1].Alias)
		}
	}
}

func TestAll_NoDuplicates(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range All() {
		if seen[e.Alias] {
			t.Errorf("duplicate alias: %s", e.Alias)
		}
		seen[e.Alias] = true
	}
}

func TestAll_Count(t *testing.T) {
	all := All()
	if len(all) != 51 {
		t.Errorf("got %d entries, want 51", len(all))
	}
}
