package template_test

// JSON-snapshot parity tests. testdata/reference/ holds the expected
// JSON output for `one templates`; these tests fail loudly on any
// structural drift so agents that have already pinned `one-cli/<cmd>/v1`
// schemas keep working.
//
// We compare on a canonicalised structural level (round-trip through
// json + sort keys) rather than byte-for-byte, because Go's
// encoding/json may pick different — but equivalent — escape sequences
// for non-ASCII strings. The agent contract is "same data shape, same
// field set", not "same bytes".

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
)

func TestSnapshot_TemplatesList(t *testing.T) {
	want := loadJSONFixture(t, "templates.json")

	got, err := template.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	gotJSON := mustMarshal(t, got)
	gotMap := mustUnmarshalMap(t, gotJSON)

	if !reflect.DeepEqual(gotMap, want) {
		t.Errorf("templates JSON drift\n  want: %s\n  got:  %s",
			pretty(want), pretty(gotMap))
	}
}

// loadJSONFixture reads testdata/reference/<name> relative to the module root
// and decodes it as a JSON object. Callers compare against this for parity.
func loadJSONFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	path := filepath.Join(moduleRoot(t), "testdata", "reference", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	return v
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func mustUnmarshalMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func pretty(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// moduleRoot finds the repo root via the location of this test file. Robust
// against `go test` being run from any directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// .../internal/template/list_snapshot_test.go → up two levels = repo root.
	return filepath.Join(filepath.Dir(file), "..", "..")
}
