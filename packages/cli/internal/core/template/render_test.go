package template

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

func TestGoStarterEditsOnlyModuleAndImports(t *testing.T) {
	source := fstest.MapFS{
		"template.json":   {Data: []byte(`{"schemaVersion":1,"go":{"modulePrefix":"example.com/"}}`)},
		"_go.mod":         {Data: []byte("module example.com/starter\n\ngo 1.26.0\n")},
		"go.sum":          {Data: []byte("checksums\n")},
		"main_windows.go": {Data: []byte("//go:build windows\n\npackage main\n\nimport (\n alias \"example.com/starter/lib\"\n _ \"example.com/starter-extra/lib\"\n)\n// example.com/starter remains a comment.\nconst value = \"example.com/starter/lib\"\n")},
		"__tests__/empty": {Data: []byte{}},
		"icon.png":        {Data: []byte{0, 0xff, 1}},
	}
	files, err := prepare(source, CommonVariables("My_API", ""))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(string(source["main_windows.go"].Data), `alias "example.com/starter/lib"`, `alias "example.com/my-api/lib"`, 1)
	if string(files["main_windows.go"]) != want {
		t.Fatalf("unrelated Go content changed:\n%s", files["main_windows.go"])
	}
	if !strings.Contains(string(files["go.mod"]), "module example.com/my-api") {
		t.Fatal(string(files["go.mod"]))
	}
	if !bytes.Equal(files["go.sum"], source["go.sum"].Data) || !bytes.Equal(files["icon.png"], source["icon.png"].Data) {
		t.Fatal("copied files changed")
	}
	target := filepath.Join(t.TempDir(), "output")
	if err := publish(target, files); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(target, "__tests__/empty"))
	if err != nil || info.Size() != 0 {
		t.Fatalf("empty file lost: %v", err)
	}
	source["nested/_go.mod"] = &fstest.MapFile{Data: []byte("module example.com/other")}
	if _, err := prepare(source, CommonVariables("test", "")); err == nil {
		t.Fatal("nested module accepted")
	}
}

func TestTextRulesUseOriginalBytes(t *testing.T) {
	source := fstest.MapFS{
		"template.json": {Data: []byte(`{"schemaVersion":1,"text":[{"files":["README.md"],"from":"FIRST","value":"projectName"},{"files":["README.md"],"from":"SECOND","value":"projectNameKebabCase"}]}`)},
		"README.md":     {Data: []byte("FIRST SECOND\n")},
	}
	files, err := prepare(source, Variables{"projectName": "SECOND", "projectNameKebabCase": "final"})
	if err != nil || string(files["README.md"]) != "SECOND final\n" {
		t.Fatalf("cascading replacement: %s %v", files["README.md"], err)
	}
}

func TestInvalidTemplatesFailDuringPreparation(t *testing.T) {
	tests := []struct {
		name, spec string
		extra      fstest.MapFS
	}{
		{"unknown field", `{"schemaVersion":1,"command":"echo unsafe"}`, nil},
		{"unknown version", `{"schemaVersion":2}`, nil},
		{"trailing JSON", `{"schemaVersion":1} {}`, nil},
		{"duplicate JSON key", `{"schemaVersion":1,"schemaVersion":1}`, nil},
		{"escape", `{"schemaVersion":1,"exclude":["../secret"]}`, nil},
		{"absolute", `{"schemaVersion":1,"exclude":["/secret"]}`, nil},
		{"missing file", `{"schemaVersion":1,"text":[{"files":["missing"],"from":"old","value":"projectName"}]}`, nil},
		{"missing match", `{"schemaVersion":1,"text":[{"files":["README.md"],"from":"absent","value":"projectName"}]}`, nil},
		{"overlap", `{"schemaVersion":1,"text":[{"files":["README.md"],"from":"old","value":"projectName"},{"files":["README.md"],"from":"old","value":"projectNameKebabCase"}]}`, nil},
		{"binary", `{"schemaVersion":1,"text":[{"files":["image"],"from":"old","value":"projectName"}]}`, fstest.MapFS{"image": {Data: []byte{0xff, 0}}}},
		{"collision", `{"schemaVersion":1}`, fstest.MapFS{"go.mod": {Data: []byte("module m")}, "_go.mod": {Data: []byte("module m")}}},
		{"legacy file", `{"schemaVersion":1}`, fstest.MapFS{"file.hbs": {Data: []byte("old")}}},
		{"symlink", `{"schemaVersion":1}`, fstest.MapFS{"link": {Data: []byte("/outside"), Mode: fs.ModeSymlink}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := fstest.MapFS{"template.json": {Data: []byte(tc.spec)}, "README.md": {Data: []byte("old\n")}}
			for name, file := range tc.extra {
				source[name] = file
			}
			if _, err := prepare(source, CommonVariables("new", "pnpm")); err == nil {
				t.Fatal("invalid template accepted")
			}
		})
	}
}

func TestPublishPreservesExistingDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "keep")
	if err := os.WriteFile(file, []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := publish(root, map[string][]byte{"keep": []byte("template")}); err == nil {
		t.Fatal("overwrote existing directory")
	}
	raw, _ := os.ReadFile(file)
	if string(raw) != "user" {
		t.Fatal("user data changed")
	}
}

func TestAllBundledStartersPrepareWithRealNames(t *testing.T) {
	registry, err := Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range registry.Templates {
		for _, name := range []string{"sample", "My_API", "billing-service"} {
			t.Run(entry.ID+"/"+name, func(t *testing.T) {
				source, err := fs.Sub(bundled.TemplatesFS, path.Join(bundled.TemplatesRoot, entry.ID))
				if err != nil {
					t.Fatal(err)
				}
				files, err := prepare(source, CommonVariables(name, "pnpm"))
				if err != nil {
					t.Fatal(err)
				}
				for name := range files {
					if strings.HasSuffix(name, ".hbs") || name == "template.json" || name == "_go.mod" || name == "go.work" {
						t.Fatalf("development file delivered: %s", name)
					}
				}
				if entry.Toolchain == "node" {
					var pkg struct{ Name string }
					if err := json.Unmarshal(files["package.json"], &pkg); err != nil {
						t.Fatal(err)
					}
					if pkg.Name == "" {
						t.Fatal("invalid package")
					}
				}
			})
		}
	}
}

func TestTemplateErrorsFollowLanguageAndRetainCause(t *testing.T) {
	original := i18n.Active()
	t.Cleanup(func() { _ = i18n.Init(original) })
	for _, tc := range []struct{ locale, want string }{{"en-US", "Unsupported template"}, {"zh-CN", "不支持的模板"}} {
		if err := i18n.Init(tc.locale); err != nil {
			t.Fatal(err)
		}
		_, err := prepare(fstest.MapFS{"template.json": {Data: []byte(`{"schemaVersion":2}`)}}, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.locale, err)
		}
		cause := os.ErrPermission
		wrapped := i18n.Errorf("template.prepare_failed", "go-api", cause)
		if !errors.Is(wrapped, cause) {
			t.Fatal("lost error chain")
		}
	}
}

func TestPublishRollsBackFailedFileWrites(t *testing.T) {
	root := t.TempDir()
	err := publish(root, map[string][]byte{
		"a":       []byte("first"),
		"z":       []byte("file blocks the following directory"),
		"z/child": []byte("cannot create"),
	})
	if err == nil {
		t.Fatal("expected write failure")
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("partial files remain: %v %v", entries, readErr)
	}
}
