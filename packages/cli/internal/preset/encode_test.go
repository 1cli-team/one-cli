package preset_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/preset"
)

// goldenVector mirrors testdata/preset/v1_vectors.json.
type goldenVector struct {
	ID   string `json:"id"`
	Spec struct {
		Items []struct {
			Kind          string `json:"kind"`
			TemplateCode  string `json:"template_code"`
			DeployCode    string `json:"deploy_code,omitempty"`
			ContainerCode string `json:"container_code,omitempty"`
		} `json:"items"`
		EnvCode string `json:"env_code"`
	} `json:"spec"`
}

func loadVectors(t *testing.T) []goldenVector {
	t.Helper()
	path := filepath.Join(repoRoot(t), "packages", "cli", "testdata", "preset", "v1_vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Vectors []goldenVector `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc.Vectors
}

func toSpec(v goldenVector) preset.Spec {
	spec := preset.Spec{EnvCode: v.Spec.EnvCode}
	for _, it := range v.Spec.Items {
		spec.Items = append(spec.Items, preset.Item{
			Kind:          preset.Kind(it.Kind[0]),
			TemplateCode:  it.TemplateCode,
			DeployCode:    it.DeployCode,
			ContainerCode: it.ContainerCode,
		})
	}
	return spec
}

// TestEncodeVectors asserts Encode(spec) == id for every frozen vector.
func TestEncodeVectors(t *testing.T) {
	for _, v := range loadVectors(t) {
		v := v
		t.Run(v.ID, func(t *testing.T) {
			got, err := preset.Encode(toSpec(v))
			if err != nil {
				t.Fatalf("Encode error: %v", err)
			}
			if got != v.ID {
				t.Errorf("Encode mismatch\n  want: %q\n  got:  %q", v.ID, got)
			}
		})
	}
}

// TestParseEncodeRoundTrip asserts Parse(id) -> Encode produces the
// same id bytes (catches Parse drift even when Encode itself is correct).
func TestParseEncodeRoundTrip(t *testing.T) {
	for _, v := range loadVectors(t) {
		v := v
		t.Run(v.ID, func(t *testing.T) {
			spec, err := preset.Parse(v.ID)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			got, err := preset.Encode(spec)
			if err != nil {
				t.Fatalf("Encode error: %v", err)
			}
			if got != v.ID {
				t.Errorf("round-trip mismatch\n  want: %q\n  got:  %q", v.ID, got)
			}
		})
	}
}

// TestEncodeOrderIndependent shuffles the spec's Items slice and
// asserts Encode produces the same canonical id either way. This is
// the load-bearing property: dashboard / CLI can build the Spec in any
// order; canonicalize sorts it on the way out.
func TestEncodeOrderIndependent(t *testing.T) {
	v := goldenVector{}
	v.ID = "1.bnek.fnav.ei"
	v.Spec.EnvCode = "i"
	v.Spec.Items = []struct {
		Kind          string `json:"kind"`
		TemplateCode  string `json:"template_code"`
		DeployCode    string `json:"deploy_code,omitempty"`
		ContainerCode string `json:"container_code,omitempty"`
	}{
		{Kind: "b", TemplateCode: "ne", DeployCode: "k"},
		{Kind: "f", TemplateCode: "na", DeployCode: "v"},
	}
	forward, err := preset.Encode(toSpec(v))
	if err != nil {
		t.Fatal(err)
	}
	// Reverse Items.
	v.Spec.Items[0], v.Spec.Items[1] = v.Spec.Items[1], v.Spec.Items[0]
	reverse, err := preset.Encode(toSpec(v))
	if err != nil {
		t.Fatal(err)
	}
	if forward != reverse {
		t.Errorf("Encode is order-dependent\n  forward: %q\n  reverse: %q", forward, reverse)
	}
	if forward != v.ID {
		t.Errorf("Encode produced %q, want %q", forward, v.ID)
	}
}

// TestEncodeAcceptsPresetPrefix verifies the parser strips the optional
// `preset:` prefix users may paste in docs.
func TestParseAcceptsPresetPrefix(t *testing.T) {
	got, err := preset.Parse("preset:1.bgo.ei")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	bare, err := preset.Parse("1.bgo.ei")
	if err != nil {
		t.Fatalf("Parse bare error: %v", err)
	}
	if !reflect.DeepEqual(got, bare) {
		t.Errorf("Parse with preset: prefix produced different Spec\n  prefixed: %+v\n  bare:     %+v", got, bare)
	}
}
