package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestProjectSettingsReadsLiveBuildTaskWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name, toolchain, file, content, command, status string
	}{
		{"node", "node", "package.json", `{"scripts":{"build":"echo build"}}`, "pnpm run build", "ready"},
		{"node without build", "node", "package.json", `{"scripts":{"dev":"vite"}}`, "", "missing"},
		{"invalid package", "node", "package.json", `{invalid`, "", "invalid"},
		{"missing package", "node", "", "", "", "invalid"},
		{"go", "go", "Taskfile.yml", "version: '3'\ntasks:\n  build:\n    cmds: ['go build ./...']\n", "task build", "ready"},
		{"go without build", "go", "Taskfile.yml", "version: '3'\ntasks: {}\n", "", "missing"},
		{"invalid taskfile", "go", "Taskfile.yml", "tasks: [", "", "invalid"},
		{"missing taskfile", "go", "", "", "", "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := seedProjectSettingsWorkspace(t)
			manifest, err := workspacecore.ReadManifest(root)
			if err != nil {
				t.Fatal(err)
			}
			manifest.Projects[0].Toolchain = tc.toolchain
			if err := workspacecore.WriteManifest(root, manifest); err != nil {
				t.Fatal(err)
			}
			write := func(path, contents string) {
				t.Helper()
				target := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write("package.json", `{"packageManager":"pnpm@12.3.4"}`)
			if tc.file != "" {
				write(filepath.Join("apps/web", tc.file), tc.content)
			}
			before := snapshotWorkspaceTree(t, root)
			service, err := NewService(catalog.Builtin())
			if err != nil {
				t.Fatal(err)
			}
			settings, err := service.ProjectSettings(context.Background(), root, "web", "dev")
			if err != nil {
				t.Fatal(err)
			}
			source := "package.json#scripts.build"
			if tc.toolchain == "go" {
				source = "Taskfile.yml#tasks.build"
			}
			want := ProjectBuildSettings{Command: tc.command, Source: source, Status: tc.status}
			if settings.Project.Build != want {
				t.Fatalf("build = %#v, want %#v", settings.Project.Build, want)
			}
			assertWorkspaceTreeEqual(t, snapshotWorkspaceTree(t, root), before)
			if tc.name == "node" {
				write("apps/web/package.json", `{"scripts":{}}`)
				refreshed, err := service.ProjectSettings(context.Background(), root, "web", "dev")
				if err != nil {
					t.Fatal(err)
				}
				if refreshed.Project.Build.Status != "missing" || refreshed.Project.Build.Command != "" || refreshed.Revision != settings.Revision {
					t.Fatalf("build did not refresh independently of manifest: %#v", refreshed)
				}
			}
		})
	}
}
