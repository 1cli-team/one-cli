package template

import (
	"encoding/json"
	"testing"
	"testing/fstest"
)

func TestCompositeDescriptorRejectsUnsafeAndAmbiguousLocations(t *testing.T) {
	base := ProjectSpec{Source: "apps/renderer", Suffix: "-renderer", Category: CategoryFrontend}
	for _, test := range []struct {
		name     string
		projects []ProjectSpec
		shared   []string
	}{
		{"traversal", []ProjectSpec{{Source: "../outside", Suffix: "-main", Category: CategoryBackend}}, nil},
		{"duplicate source", []ProjectSpec{base, {Source: base.Source, Suffix: "-main", Category: CategoryBackend}}, nil},
		{"overlap", []ProjectSpec{base, {Source: base.Source + "/nested", Suffix: "-main", Category: CategoryBackend}}, nil},
		{"duplicate suffix", []ProjectSpec{base, {Source: "services/main", Suffix: base.Suffix, Category: CategoryBackend}}, nil},
		{"invalid suffix", []ProjectSpec{{Source: "apps/renderer", Suffix: "/renderer", Category: CategoryFrontend}}, nil},
		{"invalid category", []ProjectSpec{{Source: "apps/renderer", Suffix: "-renderer", Category: Category("desktop")}}, nil},
		{"shared traversal", []ProjectSpec{base}, []string{"../README.md"}},
		{"shared without projects", nil, []string{"README.md"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, _ := json.Marshal(Spec{SchemaVersion: 1, Projects: test.projects, SharedFiles: test.shared})
			if _, err := readSpec(fstest.MapFS{"template.json": {Data: raw}}); err == nil {
				t.Fatal("accepted invalid composite")
			}
		})
	}
}

func TestCompositeSplitFailsOnMissingSourcesAndSharedCollisions(t *testing.T) {
	spec := Spec{Projects: []ProjectSpec{{Source: "apps/renderer", Suffix: "-renderer", Category: CategoryFrontend}}, SharedFiles: []string{"README.md"}}
	for _, files := range []map[string][]byte{
		{"README.md": []byte("shared")},
		{"apps/renderer/package.json": []byte(`{}`)},
		{"README.md": []byte("shared"), "apps/renderer/README.md": []byte("existing")},
	} {
		if _, err := splitProjects(files, spec, CommonVariables("desktop", "pnpm")); err == nil {
			t.Fatal("accepted incomplete or conflicting component")
		}
	}
}
