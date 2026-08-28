package addcmd

import (
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
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
