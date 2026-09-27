package preset_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/preset"
)

// repoRoot mirrors the helper in tests/e2e/e2e_helpers_test.go. We
// don't share it because that file is _test in another package; this
// resolves to the monorepo root regardless of test cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// packages/cli/internal/modules/preset/codes_test.go → five levels up.
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
}

type goldenCodes struct {
	Templates []struct {
		Code       string `json:"code"`
		TemplateID string `json:"template_id"`
	} `json:"templates"`
	Envs []struct {
		Code  string `json:"code"`
		EnvID string `json:"env_id"`
	} `json:"envs"`
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

// TestEnvCodesMatchGolden locks the environment provider codes.
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
			Code  string `json:"code"`
			EnvID string `json:"env_id"`
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
	if !sort.StringsAreSorted(codes(golden.Envs)) {
		t.Error("envs section is not sorted by code")
	}
}
