package processorch

import (
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func projectWithDev(name, relDir, cmd string) workspace.ManifestProject {
	p := workspace.ManifestProject{
		Name: name, RelativeDir: relDir, TemplateID: "x", Toolchain: "node",
	}
	if cmd != "" {
		p.Domains = &workspace.ProjectDomains{
			Dev: &workspace.ProjectDevOverride{Command: cmd},
		}
	}
	return p
}

func TestBuildEntriesFromManifest_AllProjectsWithDev(t *testing.T) {
	m := &workspace.Manifest{
		Projects: []workspace.ManifestProject{
			projectWithDev("api", "services/api", "pnpm run start:dev"),
			projectWithDev("web", "apps/web", "pnpm run dev"),
		},
	}
	entries := buildEntriesFromManifest(m, "")
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].Name != "api" || entries[1].Name != "web" {
		t.Errorf("order drift: %+v", entries)
	}
	wantCmd := "one run -p services/api -- pnpm run start:dev"
	if entries[0].Cmd != wantCmd {
		t.Errorf("api cmd = %q, want %q", entries[0].Cmd, wantCmd)
	}
}

func TestBuildEntriesFromManifest_SkipsProjectsWithoutDev(t *testing.T) {
	m := &workspace.Manifest{
		Projects: []workspace.ManifestProject{
			projectWithDev("api", "services/api", "pnpm run start:dev"),
			projectWithDev("docs", "apps/docs", ""),
		},
	}
	entries := buildEntriesFromManifest(m, "")
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if entries[0].Name != "api" {
		t.Errorf("expected only api, got %s", entries[0].Name)
	}
}

func TestBuildEntriesFromManifest_SelectorFilters(t *testing.T) {
	m := &workspace.Manifest{
		Projects: []workspace.ManifestProject{
			projectWithDev("api", "services/api", "pnpm run start:dev"),
			projectWithDev("web", "apps/web", "pnpm run dev"),
		},
	}
	entries := buildEntriesFromManifest(m, "web")
	if len(entries) != 1 || entries[0].Name != "web" {
		t.Errorf("selector should pick web only, got %+v", entries)
	}
}

func TestBuildEntriesFromManifest_SelectorWithoutDevYieldsEmpty(t *testing.T) {
	m := &workspace.Manifest{
		Projects: []workspace.ManifestProject{
			projectWithDev("docs", "apps/docs", ""),
		},
	}
	if entries := buildEntriesFromManifest(m, "docs"); len(entries) != 0 {
		t.Errorf("project without dev should yield no entries even when explicitly selected: %+v", entries)
	}
}

func TestBuildEntriesFromManifest_NilManifest(t *testing.T) {
	if entries := buildEntriesFromManifest(nil, ""); entries != nil {
		t.Errorf("nil manifest should yield nil, got %+v", entries)
	}
}
