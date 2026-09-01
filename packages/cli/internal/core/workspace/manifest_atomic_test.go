package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestWriteManifestAtomicallyPreservesPermissionsAndFormat(t *testing.T) {
	root := t.TempDir()
	path := ManifestPath(root)
	if err := os.WriteFile(path, []byte(`{"version":1,"projects":[]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{
		Version: ManifestVersion,
		Projects: []ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
		}},
	}
	if err := WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0o600 {
		t.Fatalf("manifest permissions = %o, want 600", got)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), "\n") || !strings.Contains(string(raw), `  "projects":`) {
		t.Fatalf("manifest formatting changed: %s", raw)
	}
	assertNoManifestTemps(t, root)
}

func TestAtomicManifestRenameFailurePreservesPublishedFile(t *testing.T) {
	root := t.TempDir()
	path := ManifestPath(root)
	original := []byte(`{"version":1,"projects":[]}` + "\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	replacement := []byte(`{"version":1,"projects":[{"name":"web"}]}` + "\n")
	publishErr := errors.New("injected rename failure")
	renameCalled := false
	err := atomicWriteManifestWithRename(path, replacement, 0o644, func(tempPath, targetPath string) error {
		renameCalled = true
		if filepath.Dir(tempPath) != filepath.Dir(targetPath) {
			t.Fatalf("temp and target are not siblings: %q, %q", tempPath, targetPath)
		}
		if targetPath != path {
			t.Fatalf("rename target = %q, want %q", targetPath, path)
		}
		raw, readErr := os.ReadFile(tempPath)
		if readErr != nil {
			t.Fatalf("temp file was not readable after close: %v", readErr)
		}
		if string(raw) != string(replacement) {
			t.Fatalf("temp contents = %q, want %q", raw, replacement)
		}
		info, statErr := os.Stat(tempPath)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0o640 {
			t.Fatalf("temp permissions = %o, want existing mode 640", got)
		}
		return publishErr
	})
	if !renameCalled || !errors.Is(err, publishErr) {
		t.Fatalf("atomic write error = %v; renameCalled = %v", err, renameCalled)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(original) {
		t.Fatalf("failed publication changed existing manifest: %q", raw)
	}
	assertNoManifestTemps(t, root)
}

func TestWriteManifestMarshalFailurePreservesPublishedFile(t *testing.T) {
	root := t.TempDir()
	path := ManifestPath(root)
	original := []byte(`{"version":1,"projects":[]}` + "\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{
		Version: ManifestVersion,
		Domains: &WorkspaceDomains{Env: &BackendRef{
			Kind: EnvBackendInfisical, Config: []byte(`{"broken"`),
		}},
		Projects: []ManifestProject{},
	}
	if err := WriteManifest(root, manifest); err == nil {
		t.Fatal("expected malformed raw config to fail marshaling")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(original) {
		t.Fatalf("marshal failure changed existing manifest: %q", raw)
	}
	assertNoManifestTemps(t, root)
}

func TestAtomicManifestConcurrentReadersNeverObservePartialJSON(t *testing.T) {
	root := t.TempDir()
	if err := WriteManifest(root, &Manifest{Version: ManifestVersion, Projects: []ManifestProject{}}); err != nil {
		t.Fatal(err)
	}
	const readerCount = 4
	const writes = 20
	const projectsPerWrite = 80
	stop := make(chan struct{})
	errorsFound := make(chan error, 1024)
	recordError := func(err error) {
		select {
		case errorsFound <- err:
		default:
		}
	}
	var readers sync.WaitGroup
	for reader := 0; reader < readerCount; reader++ {
		readers.Add(1)
		go func(reader int) {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				manifest, err := ReadManifest(root)
				if err != nil {
					recordError(fmt.Errorf("reader %d: %w", reader, err))
					continue
				}
				if count := len(manifest.Projects); count != 0 && count != projectsPerWrite {
					recordError(fmt.Errorf("reader %d observed %d projects", reader, count))
				}
			}
		}(reader)
	}
	for iteration := 0; iteration < writes; iteration++ {
		projects := make([]ManifestProject, 0, projectsPerWrite)
		for index := 0; index < projectsPerWrite; index++ {
			projects = append(projects, ManifestProject{
				Name:         fmt.Sprintf("service-%03d", index),
				RelativeDir:  fmt.Sprintf("services/service-%03d", index),
				TemplateID:   "go-api",
				Toolchain:    "go",
				BuildVersion: fmt.Sprintf("1.%d.%d", iteration, index),
			})
		}
		if err := WriteManifest(root, &Manifest{Version: ManifestVersion, Projects: projects}); err != nil {
			close(stop)
			readers.Wait()
			t.Fatal(err)
		}
	}
	close(stop)
	readers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
	assertNoManifestTemps(t, root)
}

func assertNoManifestTemps(t *testing.T, root string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "."+ManifestFilename+"-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("manifest temp files were not cleaned up: %v", matches)
	}
}
