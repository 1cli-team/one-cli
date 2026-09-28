package template

import (
	"bytes"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type byteEdit struct {
	start, end int
	value      string
}

func editBytes(raw []byte, edits []byteEdit, name string) ([]byte, error) {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out bytes.Buffer
	pos := 0
	for _, edit := range edits {
		if edit.start < pos {
			return nil, i18n.Errorf("template.text_overlap", name)
		}
		out.Write(raw[pos:edit.start])
		out.WriteString(edit.value)
		pos = edit.end
	}
	out.Write(raw[pos:])
	return out.Bytes(), nil
}

func applyText(files map[string][]byte, rules []TextRule, vars Variables) error {
	edits := map[string][]byteEdit{}
	for _, rule := range rules {
		value, ok := vars[rule.Value]
		if !ok {
			return i18n.Errorf("template.variable_missing", rule.Value)
		}
		for _, name := range rule.Files {
			raw, ok := files[name]
			if !ok {
				return i18n.Errorf("template.file_missing", name)
			}
			if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
				return i18n.Errorf("template.text_invalid", name)
			}
			count := 0
			for start := 0; start < len(raw); {
				i := bytes.Index(raw[start:], []byte(rule.From))
				if i < 0 {
					break
				}
				i += start
				edits[name] = append(edits[name], byteEdit{i, i + len(rule.From), value})
				start = i + len(rule.From)
				count++
			}
			minimum := 1
			if rule.MinMatches != nil {
				minimum = *rule.MinMatches
			}
			if count < minimum {
				return i18n.Errorf("template.text_matches", name, rule.From, minimum, count)
			}
		}
	}
	for _, name := range sortedFiles(files) {
		if len(edits[name]) == 0 {
			continue
		}
		after, err := editBytes(files[name], edits[name], name)
		if err != nil {
			return err
		}
		files[name] = after
	}
	return nil
}

func rewriteGo(files map[string][]byte, spec GoSpec, vars Variables) error {
	raw, ok := files["go.mod"]
	if !ok {
		return i18n.Errorf("template.file_missing", "go.mod")
	}
	mod, err := modfile.Parse("go.mod", raw, nil)
	if err != nil {
		return i18n.Errorf("template.transform_failed", "go.mod", err)
	}
	if mod.Module == nil {
		return i18n.Errorf("template.spec_invalid", "go.mod: module")
	}
	old := mod.Module.Mod.Path
	target := spec.ModulePrefix + vars["projectNameKebabCase"]
	if err := module.CheckPath(target); err != nil {
		return i18n.Errorf("template.transform_failed", "go.mod", err)
	}
	if err := mod.AddModuleStmt(target); err != nil {
		return err
	}
	files["go.mod"], err = mod.Format()
	if err != nil {
		return err
	}
	for _, name := range sortedFiles(files) {
		if strings.HasSuffix(name, "/go.mod") {
			return i18n.Errorf("template.nested_module", name)
		}
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		raw := files[name]
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, name, raw, parser.ImportsOnly)
		if err != nil {
			return i18n.Errorf("template.transform_failed", name, err)
		}
		var edits []byteEdit
		for _, imp := range parsed.Imports {
			value, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if value != old && !strings.HasPrefix(value, old+"/") {
				continue
			}
			start := fset.Position(imp.Path.Pos()).Offset
			end := fset.Position(imp.Path.End()).Offset
			edits = append(edits, byteEdit{start, end, strconv.Quote(target + strings.TrimPrefix(value, old))})
		}
		if len(edits) > 0 {
			files[name], err = editBytes(raw, edits, name)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
