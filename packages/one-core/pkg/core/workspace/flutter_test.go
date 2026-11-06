package workspace

import (
	"os"
	"reflect"
	"testing"
)

func TestFlutterDependencies(t *testing.T) {
	projects := []Project{
		{Name: "mobile", RelativeDir: "apps/mobile", Toolchain: "flutter"},
		{Name: "widgets", RelativeDir: "packages/widgets", Toolchain: "flutter"},
		{Name: "design", RelativeDir: "packages/design", Toolchain: "flutter"},
	}
	files := map[string]string{
		"pubspec.yaml":                  "name: _\nworkspace: [apps/mobile, packages/widgets, packages/design]\ndependency_overrides:\n  design_system:\n    path: packages/design\n",
		"apps/mobile/pubspec.yaml":      "name: mobile\nresolution: workspace\ndependencies:\n  shared_widgets: ^1.0.0\ndev_dependencies:\n  design_system: any\n",
		"packages/widgets/pubspec.yaml": "name: shared_widgets\nresolution: workspace\ndependencies:\n  design_system:\n    path: ../design\n",
		"packages/design/pubspec.yaml":  "name: design_system\nresolution: workspace\n",
	}
	read := func(name string) ([]byte, error) {
		if raw, exists := files[name]; exists {
			return []byte(raw), nil
		}
		return nil, os.ErrNotExist
	}
	edges, err := BuildDependencies(t.TempDir(), projects, read)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(edges["mobile"], []string{"design", "widgets"}) || !reflect.DeepEqual(edges["widgets"], []string{"design"}) {
		t.Fatalf("local pub dependencies or root-relative overrides were lost: %+v", edges)
	}
	files["packages/design/pubspec.yaml"] = "name: shared_widgets\n"
	if _, err := BuildDependencies(t.TempDir(), projects, read); err == nil {
		t.Fatal("duplicate pub package names must be rejected")
	}
	files["packages/design/pubspec.yaml"] = "name: design_system\n"
	files["pubspec_overrides.yaml"] = "dependency_overrides:\n  design_system:\n    path: packages/design\n"
	edges, err = BuildDependencies(t.TempDir(), projects, read)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(edges["mobile"], []string{"design", "widgets"}) {
		t.Fatalf("pubspec_overrides.yaml changed workspace resolution: %+v", edges)
	}
	files["pubspec_overrides.yaml"] = "dependency_overrides: []\n"
	if _, err := BuildDependencies(t.TempDir(), projects, read); err == nil {
		t.Fatal("invalid pubspec_overrides.yaml was ignored")
	}
}

func TestParseFlutterProject(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		invalid     bool
	}{
		{"valid", "name: shared_ui\nresolution: workspace\n", false},
		{"invalid name", "name: Shared-UI\n", true},
		{"duplicate field", "name: first\nname: second\n", true},
		{"invalid workspace", "name: ui\nworkspace: broken\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFlutterProject([]byte(tc.input))
			if (err != nil) != tc.invalid {
				t.Fatalf("got error %v, invalid=%v", err, tc.invalid)
			}
		})
	}
}
