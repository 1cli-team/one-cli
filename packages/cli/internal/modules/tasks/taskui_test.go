package tasks

import (
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/taskui"
)

func TestTaskGraphResolvesScopesGlobsAndAliases(t *testing.T) {
	p := &Plan{Entries: []string{"//:start"}, Tasks: []Task{
		{Name: "//:prepare", nativeName: "prepare"},
		{Name: "//apps/api:prepare", nativeName: "//apps/api:prepare"},
		{Name: "//apps/api:dev", nativeName: "//apps/api:dev", Dependencies: []string{"prepare", "//:prepare"}},
		{Name: "//:web:dev", nativeName: "web:dev", Dependencies: []string{"prepare"}},
		{Name: "//:start", nativeName: "dev", Dependencies: []string{"//apps/api:dev", "web:*", "web:dev"}},
		{Name: "//:dev", nativeName: "dev", Dependencies: []string{"//apps/api:dev", "web:*", "web:dev"}},
	}}
	want := taskui.Graph{Entries: []string{"//:dev"}, Tasks: []taskui.Task{
		{Name: "//:prepare"},
		{Name: "//apps/api:prepare"},
		{Name: "//apps/api:dev", Dependencies: []string{"//apps/api:prepare", "//:prepare"}},
		{Name: "//:web:dev", Dependencies: []string{"//:prepare"}},
		{Name: "//:dev", Dependencies: []string{"//apps/api:dev", "//:web:dev"}},
	}}
	if got := taskGraph(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("graph=%+v want=%+v", got, want)
	}
	if p.Tasks[4].Dependencies[1] != "web:*" || p.Entries[0] != "//:start" {
		t.Fatal("display graph changed scheduler plan")
	}
}
