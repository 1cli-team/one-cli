package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckNodeWorkspace(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "apps/web/package.json", "{\"name\":\"web\"}")
	if err := checkNodeWorkspace(root); err != nil {
		t.Fatalf("clean workspace: %v", err)
	}
	writeTestFile(t, root, "apps/web/pnpm-lock.yaml", "lockfileVersion: 9\n")
	if err := checkNodeWorkspace(root); err == nil || !strings.Contains(err.Error(), "pnpm-lock.yaml") {
		t.Fatalf("duplicate lock should fail, got %v", err)
	}
}

func TestCheckFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "apps/docs/content/docs/en/good.md", "---\ntitle: Good\ndescription: Fine\n---\n")
	if err := checkFrontmatter(root); err != nil {
		t.Fatalf("valid frontmatter: %v", err)
	}
	writeTestFile(t, root, "apps/docs/content/docs/en/bad.mdx", "---\ntitle: Bad\n---\n")
	if err := checkFrontmatter(root); err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("missing description should fail, got %v", err)
	}
}

func TestCheckGoFormatAcceptsCRLF(t *testing.T) {
	root := t.TempDir()
	formattedCRLF := "package sample\r\n\r\nfunc Value() int {\r\n\treturn 1\r\n}\r\n"
	writeTestFile(t, root, "packages/kernel/sample.go", formattedCRLF)
	writeTestFile(t, root, "packages/cli/sample.go", formattedCRLF)

	if err := checkGoFormat(root); err != nil {
		t.Fatalf("gofmt-compatible CRLF source should pass: %v", err)
	}
}

func writeTestFile(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
