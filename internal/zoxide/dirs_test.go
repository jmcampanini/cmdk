package zoxide

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
)

func TestParseDirs_MultiLine(t *testing.T) {
	output := `  42.5 /srv/data/projects
  10.0 /tmp/scratch
 100.0 /srv/data/work`

	items := ParseDirs(output)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	if items[0].Display != "/srv/data/work" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "/srv/data/work")
	}
	if items[1].Display != "/srv/data/projects" {
		t.Errorf("items[1].Display = %q, want %q", items[1].Display, "/srv/data/projects")
	}
	if items[2].Display != "/tmp/scratch" {
		t.Errorf("items[2].Display = %q, want %q", items[2].Display, "/tmp/scratch")
	}
}

func TestParseDirs_ItemFields(t *testing.T) {
	output := "  42.5 /srv/data/projects\n"
	items := ParseDirs(output)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	it := items[0]
	if it.Type != "dir" {
		t.Errorf("Type = %q, want %q", it.Type, "dir")
	}
	if it.Source != "zoxide" {
		t.Errorf("Source = %q, want %q", it.Source, "zoxide")
	}
	if it.Action != item.ActionNextList {
		t.Errorf("Action = %q, want %q", it.Action, item.ActionNextList)
	}
	if it.Display != "/srv/data/projects" {
		t.Errorf("Display = %q, want %q", it.Display, "/srv/data/projects")
	}
	if it.Data["path"] != "/srv/data/projects" {
		t.Errorf("Data[path] = %q, want %q", it.Data["path"], "/srv/data/projects")
	}
	if it.Filter != "" {
		t.Errorf("Filter = %q, want empty (path not under home dir)", it.Filter)
	}
}

func TestParseDirs_SortedByScoreDescending(t *testing.T) {
	output := `   1.0 /low
  50.0 /mid
 200.0 /high`

	items := ParseDirs(output)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Display != "/high" {
		t.Errorf("items[0] = %q, want /high", items[0].Display)
	}
	if items[1].Display != "/mid" {
		t.Errorf("items[1] = %q, want /mid", items[1].Display)
	}
	if items[2].Display != "/low" {
		t.Errorf("items[2] = %q, want /low", items[2].Display)
	}
}

func TestParseDirs_Empty(t *testing.T) {
	items := ParseDirs("")
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestParseDirs_MalformedLinesSkipped(t *testing.T) {
	output := `  42.5 /good/path
not-a-score /bad
just-garbage
  10.0 /another/good`

	items := ParseDirs(output)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "/good/path" {
		t.Errorf("items[0].Display = %q, want %q", items[0].Display, "/good/path")
	}
	if items[1].Display != "/another/good" {
		t.Errorf("items[1].Display = %q, want %q", items[1].Display, "/another/good")
	}
}

func TestParseDirs_PathWithSpaces(t *testing.T) {
	output := "  42.5 /srv/data/my projects/code\n"
	items := ParseDirs(output)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "/srv/data/my projects/code" {
		t.Errorf("Display = %q, want path with spaces preserved", items[0].Display)
	}
	if items[0].Data["path"] != "/srv/data/my projects/code" {
		t.Errorf("Data[path] = %q, want path with spaces preserved", items[0].Data["path"])
	}
}

func TestParseDirs_WhitespaceOnly(t *testing.T) {
	items := ParseDirs("   \n  \n  ")
	if items != nil {
		t.Errorf("expected nil for whitespace-only input, got %v", items)
	}
}
