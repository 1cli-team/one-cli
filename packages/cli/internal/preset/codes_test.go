package preset_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/preset"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
)

// repoRoot mirrors the helper in internal/cli/e2e_helpers_test.go. We
// don't share it because that file is _test in another package; this
// resolves to the monorepo root regardless of test cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// packages/cli/internal/preset/codes_test.go → four levels up.
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

type goldenCodes struct {
	Templates []struct {
		Code       string `json:"code"`
		TemplateID string `json:"template_id"`
	} `json:"templates"`
	Deploys []struct {
		Code     string `json:"code"`
		DeployID string `json:"deploy_id"`
	} `json:"deploys"`
	Envs []struct {
		Code  string `json:"code"`
		EnvID string `json:"env_id"`
	} `json:"envs"`
	Containers []struct {
		Code        string `json:"code"`
		ContainerID string `json:"container_id"`
	} `json:"containers"`
}

func loadGolden(t *testing.T) goldenCodes {
	t.Helper()
	path := filepath.Join(repoRoot(t), "packages", "cli", "testdata", "preset", "v1_codes.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var g goldenCodes
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return g
}

// TestTemplateCodesMatchGoldenAndRegistry locks down the (code,
// template_id) pairs. Every entry in v1_codes.json must:
//   - exist in registry.json with the same code, AND
//   - have a unique code across the whole registry.
//
// The reverse direction also holds: every template in registry.json
// must appear in the golden file (additions to the registry MUST be
// accompanied by additions to v1_codes.json).
func TestTemplateCodesMatchGoldenAndRegistry(t *testing.T) {
	golden := loadGolden(t)

	reg, err := template.Fetch(t.Context(), "")
	if err != nil {
		t.Fatalf("template.Fetch: %v", err)
	}

	regByCode := map[string]string{}
	regByID := map[string]string{}
	for _, tpl := range reg.Templates {
		if tpl.Code == "" {
			t.Errorf("registry template %q has empty code", tpl.ID)
			continue
		}
		if dup, exists := regByCode[tpl.Code]; exists {
			t.Errorf("duplicate template code %q in registry (used by %s and %s)", tpl.Code, dup, tpl.ID)
		}
		regByCode[tpl.Code] = tpl.ID
		regByID[tpl.ID] = tpl.Code
	}

	// Every golden entry must be in registry with the same code.
	goldenIDs := map[string]bool{}
	for _, e := range golden.Templates {
		goldenIDs[e.TemplateID] = true
		got, ok := regByID[e.TemplateID]
		if !ok {
			t.Errorf("golden template %q (code=%s) is missing from registry.json — removing a frozen code is forbidden", e.TemplateID, e.Code)
			continue
		}
		if got != e.Code {
			t.Errorf("template %q: golden code %q != registry code %q — renaming a frozen code is forbidden", e.TemplateID, e.Code, got)
		}
	}

	// Every registry entry must be in golden (additions to registry must
	// be reflected in golden).
	for id := range regByID {
		if !goldenIDs[id] {
			t.Errorf("registry template %q is missing from testdata/preset/v1_codes.json — add it (codes are append-only)", id)
		}
	}
}

// TestDeployCodesMatchGolden locks the deploy code constants in
// codes.go against the golden file.
func TestDeployCodesMatchGolden(t *testing.T) {
	golden := loadGolden(t)

	goldenPairs := map[byte]string{}
	for _, e := range golden.Deploys {
		if len(e.Code) != 1 {
			t.Errorf("golden deploy code %q is not 1 char", e.Code)
			continue
		}
		goldenPairs[e.Code[0]] = e.DeployID
	}

	got := preset.DeployCodesSnapshot()
	gotPairs := map[byte]string{}
	for _, e := range got {
		gotPairs[e.Code] = e.ID
	}

	// Golden entries must be present in code, with the same id.
	for c, id := range goldenPairs {
		if gotPairs[c] != id {
			t.Errorf("deploy code %q in golden maps to %q but code maps to %q — frozen code drift", string(c), id, gotPairs[c])
		}
	}
	// Code may add new entries (append-only) but must not drop golden ones.
	for c, id := range gotPairs {
		if _, ok := goldenPairs[c]; !ok {
			t.Logf("note: deploy code %q (%s) is in code but not in v1_codes.json — append it to lock", string(c), id)
		}
	}
}

// TestEnvCodesMatchGolden mirrors TestDeployCodesMatchGolden for env
// providers.
func TestEnvCodesMatchGolden(t *testing.T) {
	golden := loadGolden(t)

	goldenPairs := map[byte]string{}
	for _, e := range golden.Envs {
		if len(e.Code) != 1 {
			t.Errorf("golden env code %q is not 1 char", e.Code)
			continue
		}
		goldenPairs[e.Code[0]] = e.EnvID
	}

	got := preset.EnvCodesSnapshot()
	gotPairs := map[byte]string{}
	for _, e := range got {
		gotPairs[e.Code] = e.ID
	}

	for c, id := range goldenPairs {
		if gotPairs[c] != id {
			t.Errorf("env code %q in golden maps to %q but code maps to %q — frozen code drift", string(c), id, gotPairs[c])
		}
	}
}

// TestContainerCodesMatchGolden mirrors TestDeployCodesMatchGolden for
// container backends.
func TestContainerCodesMatchGolden(t *testing.T) {
	golden := loadGolden(t)

	goldenPairs := map[byte]string{}
	for _, e := range golden.Containers {
		if len(e.Code) != 1 {
			t.Errorf("golden container code %q is not 1 char", e.Code)
			continue
		}
		goldenPairs[e.Code[0]] = e.ContainerID
	}

	got := preset.ContainerCodesSnapshot()
	gotPairs := map[byte]string{}
	for _, e := range got {
		gotPairs[e.Code] = e.ID
	}

	for c, id := range goldenPairs {
		if gotPairs[c] != id {
			t.Errorf("container code %q in golden maps to %q but code maps to %q — frozen code drift", string(c), id, gotPairs[c])
		}
	}
	for c, id := range gotPairs {
		if _, ok := goldenPairs[c]; !ok {
			t.Logf("note: container code %q (%s) is in code but not in v1_codes.json — append it to lock", string(c), id)
		}
	}
}

// TestGoldenSortedByCode keeps the golden file readable: each list is
// sorted by code ASCII. Append-only with sort makes review diffs
// minimal (new lines slot in alphabetically).
func TestGoldenSortedByCode(t *testing.T) {
	golden := loadGolden(t)

	codes := func(list any) []string {
		var out []string
		switch xs := list.(type) {
		case []struct {
			Code       string `json:"code"`
			TemplateID string `json:"template_id"`
		}:
			for _, x := range xs {
				out = append(out, x.Code)
			}
		case []struct {
			Code     string `json:"code"`
			DeployID string `json:"deploy_id"`
		}:
			for _, x := range xs {
				out = append(out, x.Code)
			}
		case []struct {
			Code  string `json:"code"`
			EnvID string `json:"env_id"`
		}:
			for _, x := range xs {
				out = append(out, x.Code)
			}
		case []struct {
			Code        string `json:"code"`
			ContainerID string `json:"container_id"`
		}:
			for _, x := range xs {
				out = append(out, x.Code)
			}
		}
		return out
	}

	if !sort.StringsAreSorted(codes(golden.Templates)) {
		t.Error("templates section is not sorted by code (keep golden file readable)")
	}
	if !sort.StringsAreSorted(codes(golden.Deploys)) {
		t.Error("deploys section is not sorted by code")
	}
	if !sort.StringsAreSorted(codes(golden.Envs)) {
		t.Error("envs section is not sorted by code")
	}
	if !sort.StringsAreSorted(codes(golden.Containers)) {
		t.Error("containers section is not sorted by code")
	}
}
