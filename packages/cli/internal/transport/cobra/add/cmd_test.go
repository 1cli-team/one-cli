package addcmd

import (
	"bytes"
	"encoding/json"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
)

func TestProjectKindForUsesDirectoryCategory(t *testing.T) {
	tests := []struct {
		name string
		tpl  template.Template
		want projectKind
	}{
		{
			name: "documentation site stays with applications",
			tpl: template.Template{
				Category: template.CategoryFrontend,
				Tags:     []string{"docs"},
			},
			want: kindApplication,
		},
		{
			name: "backend maps to services",
			tpl:  template.Template{Category: template.CategoryBackend},
			want: kindService,
		},
		{
			name: "library maps to packages",
			tpl:  template.Template{Category: template.CategoryLibrary},
			want: kindLibrary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := projectKindFor(tt.tpl); got != tt.want {
				t.Fatalf("projectKindFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompositeAddResult(t *testing.T) {
	original := i18n.Active()
	t.Cleanup(func() { _ = i18n.Init(original) })
	result := addResult{SubprojectName: "desktop", TargetPath: "/workspace", TemplateID: "electron-app", Toolchain: "node", Projects: []addProject{
		{Name: "desktop-renderer", TargetPath: "/workspace/apps/desktop-renderer", Toolchain: "node"},
		{Name: "desktop-main", TargetPath: "/workspace/services/desktop-main", Toolchain: "node"},
		{Name: "desktop-preload", TargetPath: "/workspace/packages/desktop-preload", Toolchain: "node"},
	}}
	for _, locale := range []string{"zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		var out bytes.Buffer
		result.RenderTTY(&out)
		for _, member := range result.Projects {
			if !strings.Contains(out.String(), member.TargetPath) {
				t.Fatal(out.String())
			}
		}
		if !strings.Contains(out.String(), "one run desktop:dev") || strings.Contains(out.String(), "one dev -p desktop") {
			t.Fatal(out.String())
		}
		if !strings.Contains(out.String(), i18n.Tf("add.group_success", "desktop")) {
			t.Fatal(out.String())
		}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields["projects"]) == 0 {
		t.Fatalf("%s: %v", raw, err)
	}
	result.Projects = nil
	raw, _ = json.Marshal(result)
	if strings.Contains(string(raw), `"projects"`) {
		t.Fatal("ordinary output changed")
	}
}
