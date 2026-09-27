package workspace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadManifestRejectsRetiredDomainsWithoutRewritingFiles(t *testing.T) {
	for _, domain := range []string{"deploy", "container"} {
		for _, scope := range []string{"workspace", "project"} {
			t.Run(scope+"/"+domain, func(t *testing.T) {
				root := t.TempDir()
				domains := fmt.Sprintf(`{"%s":{"kind":"old"}}`, domain)
				manifest := fmt.Sprintf(`{"version":1,"domains":%s,"projects":[]}`, domains)
				if scope == "project" {
					manifest = fmt.Sprintf(`{"version":1,"projects":[{"name":"web","relativeDir":"apps/web","toolchain":"node","domains":%s}]}`, domains)
				}
				path := filepath.Join(root, ManifestFilename)
				before := []byte(manifest)
				if err := os.WriteFile(path, before, 0o644); err != nil {
					t.Fatal(err)
				}
				artifact := filepath.Join(root, "Dockerfile")
				content := []byte("FROM scratch\n")
				if err := os.WriteFile(artifact, content, 0o644); err != nil {
					t.Fatal(err)
				}
				_, err := ReadManifest(root)
				if err == nil || !strings.Contains(err.Error(), "have been removed") {
					t.Fatalf("error = %v", err)
				}
				coded, ok := err.(interface{ ErrorCode() string })
				if !ok || coded.ErrorCode() != "MANIFEST_INVALID" {
					t.Fatalf("error code = %v", err)
				}
				for name, want := range map[string][]byte{path: before, artifact: content} {
					got, err := os.ReadFile(name)
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("file changed: %s (%v)", name, err)
					}
				}
			})
		}
	}
}
