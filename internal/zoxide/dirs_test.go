package zoxide

import (
	"testing"

	"github.com/jmcampanini/cmdk/internal/item"
	"github.com/jmcampanini/cmdk/internal/pathfmt"
)

const testHome = "/home/testuser"

func TestParseDirs_MultiLine(t *testing.T) {
	output := `  42.5 /srv/data/projects
  10.0 /tmp/scratch
 100.0 /srv/data/work`

	items := ParseDirs(output, 0, "", "~", nil, pathfmt.Truncation{})
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
	items := ParseDirs(output, 0, "", "~", nil, pathfmt.Truncation{})
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
}

func TestParseDirs_SortedByScoreDescending(t *testing.T) {
	output := `   1.0 /low
  50.0 /mid
 200.0 /high`

	items := ParseDirs(output, 0, "", "~", nil, pathfmt.Truncation{})
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
	items := ParseDirs("", 0, "", "~", nil, pathfmt.Truncation{})
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestParseDirs_MalformedLinesSkipped(t *testing.T) {
	output := `  42.5 /good/path
not-a-score /bad
just-garbage
  10.0 /another/good`

	items := ParseDirs(output, 0, "", "~", nil, pathfmt.Truncation{})
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
	items := ParseDirs(output, 0, "", "~", nil, pathfmt.Truncation{})
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
	items := ParseDirs("   \n  \n  ", 0, "", "~", nil, pathfmt.Truncation{})
	if items != nil {
		t.Errorf("expected nil for whitespace-only input, got %v", items)
	}
}

func TestParseDirs_HomePathGetsTildeDisplay(t *testing.T) {
	path := testHome + "/projects/myapp"
	output := "  42.5 " + path + "\n"
	items := ParseDirs(output, 0, testHome, "~", nil, pathfmt.Truncation{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	it := items[0]
	if it.Display != "~/projects/myapp" {
		t.Errorf("Display = %q, want %q", it.Display, "~/projects/myapp")
	}
	if it.Data["path"] != path {
		t.Errorf("Data[path] = %q, want %q", it.Data["path"], path)
	}
}

func TestParseDirs_MinScoreFilters(t *testing.T) {
	output := `   1.0 /low
  50.0 /mid
 200.0 /high`

	items := ParseDirs(output, 50.0, "", "~", nil, pathfmt.Truncation{})
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Display != "/high" {
		t.Errorf("items[0].Display = %q, want /high", items[0].Display)
	}
	if items[1].Display != "/mid" {
		t.Errorf("items[1].Display = %q, want /mid", items[1].Display)
	}
}

func TestParseDirs_MinScoreZeroKeepsAll(t *testing.T) {
	output := `   0.5 /tiny
   1.0 /low
  50.0 /mid`

	items := ParseDirs(output, 0, "", "~", nil, pathfmt.Truncation{})
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
}

func TestParseDirs_MinScoreIncludesBoundary(t *testing.T) {
	output := `   1.0 /low
  10.0 /mid1
  20.0 /mid2
  30.0 /mid3
  50.0 /high1
 100.0 /high2`

	items := ParseDirs(output, 10.0, "", "~", nil, pathfmt.Truncation{})
	if len(items) != 5 {
		t.Fatalf("got %d items, want 5", len(items))
	}
	if items[0].Display != "/high2" {
		t.Errorf("items[0].Display = %q, want /high2", items[0].Display)
	}
	if items[4].Display != "/mid1" {
		t.Errorf("items[4].Display = %q, want /mid1", items[4].Display)
	}
}

func TestParseDirs_WithDisplayRules(t *testing.T) {
	rules := pathfmt.CompileRules(map[string]string{
		"/srv/data": "/d",
	})
	output := "  42.5 /srv/data/projects\n"
	items := ParseDirs(output, 0, "", "", rules, pathfmt.Truncation{})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "/d/projects" {
		t.Errorf("Display = %q, want %q", items[0].Display, "/d/projects")
	}
	if items[0].Data["path"] != "/srv/data/projects" {
		t.Errorf("Data[path] = %q, want original path preserved", items[0].Data["path"])
	}
}

func TestParseDirs_WithTruncation(t *testing.T) {
	output := "  42.5 /home/testuser/Code/github.com/foo/bar\n"
	items := ParseDirs(output, 0, testHome, "~", nil, pathfmt.Truncation{Length: 2, Symbol: "…"})
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Display != "…/foo/bar" {
		t.Errorf("Display = %q, want %q", items[0].Display, "…/foo/bar")
	}
	if items[0].Data["path"] != "/home/testuser/Code/github.com/foo/bar" {
		t.Errorf("Data[path] = %q, want original path preserved", items[0].Data["path"])
	}
}
