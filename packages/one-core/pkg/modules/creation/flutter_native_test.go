package creation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/template"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
)

// Opt in on a machine with Flutter installed. No platform signing or user
// project files are touched; pub downloads use the caller's configured cache.
func TestFlutterNativeTemplates(t *testing.T) {
	if os.Getenv("ONE_TEST_FLUTTER_TEMPLATES") != "1" {
		t.Skip("set ONE_TEST_FLUTTER_TEMPLATES=1 to run Flutter SDK smoke tests")
	}
	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, workspace.ManifestFilename), []byte("version = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"flutter-app", "flutter-lib"} {
		for i := range registry.Templates {
			if registry.Templates[i].ID == id {
				if _, err := materializeProject(context.Background(), root, ProjectInput{Template: &registry.Templates[i], Name: id, SkipSkills: true}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	run := func(dir, command string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), command, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", command, args, err, out)
		}
	}
	run(root, "flutter", "pub", "get")
	run(root, "flutter", "pub", "get", "--enforce-lockfile")
	for _, dir := range []string{"apps/flutter-app", "packages/flutter-lib"} {
		run(filepath.Join(root, dir), "task", "format")
		run(filepath.Join(root, dir), "task", "check")
		run(filepath.Join(root, dir), "task", "test")
	}
	app := filepath.Join(root, "apps/flutter-app")
	run(app, "task", "build")
	before, err := os.ReadFile(filepath.Join(app, "lib/main.dart"))
	if err != nil {
		t.Fatal(err)
	}
	run(app, "task", "platforms", "--", "--platforms", "android,ios")
	after, err := os.ReadFile(filepath.Join(app, "lib/main.dart"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("platform generation overwrote application source")
	}
	for _, file := range []string{"android/app/build.gradle.kts", "ios/Runner.xcodeproj/project.pbxproj"} {
		if _, err := os.Stat(filepath.Join(app, file)); err != nil {
			t.Fatal(err)
		}
	}
}
