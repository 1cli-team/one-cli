package i18n

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCatalogKeysAndFormatParametersMatch(t *testing.T) {
	ensureLoaded()
	format := regexp.MustCompile(`%([-+# 0]*([0-9]+|\*)?(\.([0-9]+|\*))?[a-zA-Z%])`)
	en, zh := catalogs["en-US"], catalogs["zh-CN"]
	for _, catalog := range []map[string]string{en, zh} {
		for key := range catalog {
			if strings.TrimSpace(en[key]) == "" || strings.TrimSpace(zh[key]) == "" {
				t.Errorf("missing translation: %s", key)
				continue
			}
			if !reflect.DeepEqual(format.FindAllString(en[key], -1), format.FindAllString(zh[key], -1)) {
				t.Errorf("format parameters differ for %s: %q / %q", key, en[key], zh[key])
			}
		}
	}
}

func TestLiteralTranslationReferencesExist(t *testing.T) {
	ensureLoaded()
	_, here, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(here), "..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "resources" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "i18n" {
				return true
			}
			index := 0
			switch selector.Sel.Name {
			case "T", "Tf", "Errorf":
			case "MarkShort", "MarkLong":
				index = 1
			case "MarkFlagUsage":
				index = 2
			default:
				return true
			}
			if len(call.Args) <= index {
				return true
			}
			literal, ok := call.Args[index].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			key, err := strconv.Unquote(literal.Value)
			if err != nil {
				return true
			}
			for locale, catalog := range catalogs {
				if catalog[key] == "" {
					t.Errorf("%s: %s missing %q", path, locale, key)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestErrorfPreservesCause(t *testing.T) {
	t.Cleanup(func() { _ = Init(DefaultLocale) })
	cause := errors.New("native error")
	for _, locale := range []string{"en-US", "zh-CN"} {
		_ = Init(locale)
		if err := Errorf("file.read_error", "apps/web/package.json", cause); !errors.Is(err, cause) || !strings.Contains(err.Error(), cause.Error()) {
			t.Fatalf("lost cause: %v", err)
		}
	}
}

func TestLocalizedValuePreservesCustomMetadata(t *testing.T) {
	t.Cleanup(func() { _ = Init(DefaultLocale) })
	_ = Init("zh-CN")
	original := T("template.react-spa.name")
	_ = Init("en-US")
	if got := LocalizedValue("template.react-spa.name", original); got != "React single-page application" {
		t.Fatalf("built-in name: %q", got)
	}
	for _, custom := range []string{"团队的 React 模板", "Our React template"} {
		if got := LocalizedValue("template.react-spa.name", custom); got != custom {
			t.Fatalf("custom name changed: %q", got)
		}
	}
}
