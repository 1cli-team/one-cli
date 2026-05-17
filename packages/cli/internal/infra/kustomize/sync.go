package kustomize

// sync.go is the package-local Sync entry point: scaffolds the
// per-workload Kustomize tree (base/<workload>.yaml + kustomization.yaml
// + overlays/dev|staging|prod). plugin.go's Sync method delegates here.

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/internalcommon"
	"gopkg.in/yaml.v3"
)

// Sync scaffolds the Kustomize tree under <workspaceRoot>/kustomize for
// the given workload. Idempotent: re-running on an already-configured
// workspace is a no-op.
//
// containerPort is the port the workload listens on inside the container;
// pass 0 to use the default (8080).
func Sync(workspaceRoot, workloadName string, containerPort int) error {
	if workloadName == "" {
		return nil
	}
	root := filepath.Join(workspaceRoot, "kustomize")
	baseDir := filepath.Join(root, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	port := containerPort
	if port == 0 {
		port = 8080
	}

	// 1. write per-workload manifest under base/<workload>.yaml — never
	//    overwrite existing (preserve user edits).
	workloadFile := filepath.Join(baseDir, workloadName+".yaml")
	if err := writeIfMissing(workloadFile, deploymentTpl, struct {
		WorkloadName  string
		ContainerPort int
	}{
		WorkloadName:  workloadName,
		ContainerPort: port,
	}); err != nil {
		return err
	}

	// 2. ensure base/kustomization.yaml lists this workload between
	//    sentinel markers.
	if err := appendBaseResource(baseDir, workloadName); err != nil {
		return err
	}

	// 3. write overlays (dev / staging / prod) — created once, never touched.
	if err := writeRawIfMissing(filepath.Join(root, "overlays", "dev", "kustomization.yaml"), overlayDevRaw); err != nil {
		return err
	}
	if err := writeRawIfMissing(filepath.Join(root, "overlays", "staging", "kustomization.yaml"), overlayStagingRaw); err != nil {
		return err
	}
	if err := writeRawIfMissing(filepath.Join(root, "overlays", "prod", "kustomization.yaml"), overlayProdRaw); err != nil {
		return err
	}
	return nil
}

func appendBaseResource(baseDir, workload string) error {
	path := filepath.Join(baseDir, "kustomization.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		raw = []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n\nresources:\n" + startMarker + "\n" + endMarker + "\n")
	}
	existing := internalcommon.NormalizeNewlines(string(raw))
	withMarkers := ensureResourceMarkers(existing)

	entry := "  - " + workload + ".yaml"
	if hasResource(withMarkers, entry) {
		if withMarkers != existing {
			return os.WriteFile(path, []byte(internalcommon.EnsureTrailingNewline(withMarkers)), 0o644)
		}
		return nil
	}

	updated := withMarkers
	if strings.Contains(updated, endMarker) {
		updated = strings.Replace(updated, endMarker, entry+"\n"+endMarker, 1)
	} else {
		updated = strings.TrimRight(updated, "\n") + "\n" + entry + "\n"
	}
	return os.WriteFile(path, []byte(internalcommon.EnsureTrailingNewline(updated)), 0o644)
}

func ensureResourceMarkers(content string) string {
	normalized := internalcommon.NormalizeNewlines(content)
	if strings.Contains(normalized, startMarker) && strings.Contains(normalized, endMarker) {
		return normalized
	}
	lines := strings.Split(normalized, "\n")
	resourcesIdx := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "resources:" {
			resourcesIdx = i
			break
		}
	}
	if resourcesIdx >= 0 {
		newLines := append([]string{}, lines[:resourcesIdx+1]...)
		newLines = append(newLines, startMarker, endMarker)
		newLines = append(newLines, lines[resourcesIdx+1:]...)
		return internalcommon.EnsureTrailingNewline(strings.Join(newLines, "\n"))
	}
	base := strings.TrimRight(normalized, "\n")
	if base == "" {
		return "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n\nresources:\n" + startMarker + "\n" + endMarker + "\n"
	}
	return internalcommon.EnsureTrailingNewline(base + "\n\nresources:\n" + startMarker + "\n" + endMarker)
}

func hasResource(content, entry string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimRight(line, " \t") == entry {
			return true
		}
	}
	return false
}

func writeIfMissing(path string, tpl *template.Template, data any) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func writeRawIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// SyncOverlayTarget updates the selected overlay with the namespace and
// image overrides resolved by deploy/container flow. It intentionally
// touches only kustomization.yaml fields managed by one-cli and leaves
// resources / user patches intact.
func SyncOverlayTarget(workspaceRoot, overlayRelPath, namespace string, images map[string]string) error {
	if strings.TrimSpace(overlayRelPath) == "" {
		overlayRelPath = defaultOverlay
	}
	path := overlayRelPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspaceRoot, filepath.FromSlash(path))
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		path = filepath.Join(path, "kustomization.yaml")
	} else if err != nil && errors.Is(err, fs.ErrNotExist) && filepath.Ext(path) == "" {
		path = filepath.Join(path, "kustomization.yaml")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var doc kustomizationDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if ns := strings.TrimSpace(namespace); ns != "" {
		doc.Namespace = ns
	}
	for name, image := range images {
		name = strings.TrimSpace(name)
		image = strings.TrimSpace(image)
		if name == "" || image == "" {
			continue
		}
		newName, newTag := splitImageRef(image)
		doc.setImage(name, newName, newTag)
	}
	updated, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, updated, 0o644)
}

type kustomizationDoc struct {
	APIVersion string              `yaml:"apiVersion,omitempty"`
	Kind       string              `yaml:"kind,omitempty"`
	Namespace  string              `yaml:"namespace,omitempty"`
	Resources  []string            `yaml:"resources,omitempty"`
	NamePrefix string              `yaml:"namePrefix,omitempty"`
	Images     []kustomizeImageRef `yaml:"images,omitempty"`
	Extra      map[string]any      `yaml:",inline,omitempty"`
}

type kustomizeImageRef struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName,omitempty"`
	NewTag  string `yaml:"newTag,omitempty"`
}

func (d *kustomizationDoc) setImage(name, newName, newTag string) {
	for i := range d.Images {
		if d.Images[i].Name == name {
			d.Images[i].NewName = newName
			d.Images[i].NewTag = newTag
			return
		}
	}
	d.Images = append(d.Images, kustomizeImageRef{
		Name:    name,
		NewName: newName,
		NewTag:  newTag,
	})
}

func splitImageRef(ref string) (string, string) {
	if idx := strings.LastIndex(ref, ":"); idx > -1 {
		slash := strings.LastIndex(ref, "/")
		if idx > slash {
			return ref[:idx], ref[idx+1:]
		}
	}
	return ref, ""
}
