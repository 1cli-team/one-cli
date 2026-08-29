package kustomize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncScaffoldsAllThreeOverlays(t *testing.T) {
	tmp := t.TempDir()
	if err := Sync(tmp, "api", 8080); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	for _, env := range []string{"dev", "staging", "prod"} {
		path := filepath.Join(tmp, "kustomize", "overlays", env, "kustomization.yaml")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing overlay %s: %v", env, err)
		}
		got := string(raw)
		for _, want := range []string{
			"namespace: " + env,
			"namePrefix: " + env + "-",
			"app.kubernetes.io/environment: " + env,
			"resources:\n  - ../../base",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("overlay %s missing %q:\n%s", env, want, got)
			}
		}
	}
}

func TestSyncOverlayTargetPreservesUnknownFields(t *testing.T) {
	tmp := t.TempDir()
	overlay := filepath.Join(tmp, "kustomize", "overlays", "prod")
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(overlay, "kustomization.yaml")
	before := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
commonLabels:
  app.kubernetes.io/managed-by: user
patches:
  - path: deployment-patch.yaml
`
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	err := SyncOverlayTarget(tmp, "kustomize/overlays/prod", "prod", map[string]string{
		"api": "ghcr.io/acme/api:v0.1.1",
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	after := string(raw)
	for _, want := range []string{
		"namespace: prod",
		"commonLabels:",
		"app.kubernetes.io/managed-by: user",
		"patches:",
		"path: deployment-patch.yaml",
		"name: api",
		"newName: ghcr.io/acme/api",
		"newTag: v0.1.1",
	} {
		if !strings.Contains(after, want) {
			t.Fatalf("updated kustomization missing %q:\n%s", want, after)
		}
	}
}
