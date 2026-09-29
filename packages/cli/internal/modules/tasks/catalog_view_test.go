package tasks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func catalogExample() []Task {
	return []Task{
		{Name: "//:web:dev", Project: "web", Description: "Start the web app", Source: "mise.toml", Interactive: true},
		{Name: "//:build", Project: "web", Description: "Build workspace applications", Source: "mise.toml"},
		{Name: "//:install", Source: "mise.local.toml"},
		{Name: "//apps/api:test", Project: "api", Description: "Run API tests", Source: "apps/api/mise.toml", Cached: true},
		{Name: "//tools:check", Description: "Check tooling", Source: "tools/mise.toml"},
	}
}
func TestCatalogOutputSnapshots(t *testing.T) {
	old := i18n.Active()
	t.Cleanup(func() { i18n.Init(old) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			i18n.Init(locale)
			var out bytes.Buffer
			catalogLayout{width: 90}.render(&out, catalogExample())
			path := filepath.Join("testdata", "catalog-"+locale+".txt")
			if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, out.Bytes(), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != string(want) {
				t.Fatalf("catalog output changed:\n%s", out.String())
			}
		})
	}
}
func TestCatalogDetailsColorsAndNarrowWidths(t *testing.T) {
	old := i18n.Active()
	t.Cleanup(func() { i18n.Init(old) })
	tasks := catalogExample()
	tasks = append(tasks, Task{Name: "//:" + strings.Repeat("编译🧑‍💻", 12), Description: strings.Repeat("中文描述 long-word-without-breaks ", 4), Source: strings.Repeat("long/", 12) + "mise.toml"})
	before, _ := json.Marshal(MarshalCatalog(tasks))
	for _, locale := range []string{"en-US", "zh-CN"} {
		i18n.Init(locale)
		for _, width := range []int{8, 20, 45, 64, 80, 140} {
			for _, verbose := range []bool{false, true} {
				for _, color := range []bool{false, true} {
					var out bytes.Buffer
					catalogLayout{width: width, color: color, verbose: verbose}.render(&out, tasks)
					got := out.String()
					if strings.Contains(got, "\x1b[") != color {
						t.Fatalf("unexpected style color=%v: %q", color, got)
					}
					for _, line := range strings.Split(got, "\n") {
						if ansi.StringWidth(line) > width {
							t.Fatalf("width=%d line=%q", width, line)
						}
					}
					compact := strings.Join(strings.Fields(ansi.Strip(got)), "")
					if strings.Contains(compact, "mise.toml") != verbose {
						t.Fatalf("verbose=%v did not control metadata", verbose)
					}
					if !strings.Contains(compact, strings.Repeat("编译🧑‍💻", 12)) {
						t.Fatal("long task name was lost")
					}
				}
			}
		}
	}
	after, _ := json.Marshal(MarshalCatalog(tasks))
	if !bytes.Equal(before, after) {
		t.Fatal("rendering changed the catalog protocol or task order")
	}
}
func TestCatalogProjectGroupsFollowNamespaces(t *testing.T) {
	groups := groupCatalog(catalogExample())
	if len(groups) != 4 || groups[0].project != "" || groups[0].scope != "" || len(groups[0].tasks) != 2 || groups[0].tasks[0].Name != "//:build" {
		t.Fatal("root entry grouped by its working directory", groups)
	}
	if groups[1].project != "api" || groups[2].project != "web" || groups[3].scope != "//tools" {
		t.Fatal("project or native scope group missing", groups)
	}
}
