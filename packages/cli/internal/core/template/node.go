package template

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/jsonedit"
)

var scopeRE = regexp.MustCompile(`^@[a-z0-9][a-z0-9._-]*$`)
var packageTokenRE = regexp.MustCompile(`@[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*`)

// The files already exist in memory here; discover declared workspace members
// from logical paths, without consulting the destination filesystem.
func nodeMembers(files map[string][]byte) ([]string, error) {
	raw, ok := files["package.json"]
	if !ok {
		return nil, i18n.Errorf("template.file_missing", "package.json")
	}
	var root struct{ Workspaces json.RawMessage }
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	var patterns []string
	if err := json.Unmarshal(root.Workspaces, &patterns); err != nil {
		var obj struct{ Packages []string }
		if err := json.Unmarshal(root.Workspaces, &obj); err != nil {
			return nil, i18n.Errorf("template.spec_invalid", "package.json: workspaces")
		}
		patterns = obj.Packages
	}
	members := []string{"package.json"}
	for _, pattern := range patterns {
		p := strings.TrimPrefix(pattern, "!")
		if !fsPathPattern(p) {
			return nil, i18n.Errorf("template.spec_path", pattern)
		}
	}
	for _, name := range sortedFiles(files) {
		if name == "package.json" || path.Base(name) != "package.json" {
			continue
		}
		included, excluded := false, false
		for _, pattern := range patterns {
			negative := strings.HasPrefix(pattern, "!")
			match, err := path.Match(strings.TrimPrefix(pattern, "!"), path.Dir(name))
			if err != nil {
				return nil, err
			}
			if match {
				if negative {
					excluded = true
				} else {
					included = true
				}
			}
		}
		if included && !excluded {
			members = append(members, name)
		}
	}
	if len(members) < 2 {
		return nil, i18n.Errorf("template.spec_invalid", "package.json: workspaces")
	}
	// Explicit member paths must not silently disappear.
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "!") || strings.ContainsAny(pattern, "*?[") {
			continue
		}
		if _, ok := files[path.Join(pattern, "package.json")]; !ok {
			return nil, i18n.Errorf("template.file_missing", path.Join(pattern, "package.json"))
		}
	}
	return members, nil
}

func fsPathPattern(p string) bool {
	if p == "" || strings.ContainsAny(p, `\:{}()`) || strings.Contains(p, "**") || strings.HasPrefix(p, "/") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	_, err := path.Match(p, "")
	return err == nil
}

func rewriteNode(files map[string][]byte, spec NodeSpec, vars Variables) error {
	targetScope := "@" + vars["projectNameKebabCase"]
	if !scopeRE.MatchString(targetScope) {
		return i18n.Errorf("template.spec_invalid", "projectNameKebabCase")
	}
	members, err := nodeMembers(files)
	if err != nil {
		return err
	}
	names := map[string]string{}
	for _, file := range members {
		var pkg struct{ Name string }
		if err := json.Unmarshal(files[file], &pkg); err != nil {
			return i18n.Errorf("template.transform_failed", file, err)
		}
		if pkg.Name == "" {
			return i18n.Errorf("template.spec_invalid", file+": name")
		}
		if _, ok := names[pkg.Name]; ok {
			return i18n.Errorf("template.file_collision", pkg.Name)
		}
		target := vars["projectNameKebabCase"]
		if file != "package.json" {
			if !strings.HasPrefix(pkg.Name, spec.Scope+"/") || packageTokenRE.FindString(pkg.Name) != pkg.Name {
				return i18n.Errorf("template.spec_invalid", file+": name")
			}
			target = targetScope + strings.TrimPrefix(pkg.Name, spec.Scope)
		}
		names[pkg.Name] = target
	}
	for _, file := range members {
		raw, err := jsonedit.RewriteStrings(files[file], func(p []string, key bool, value string) (string, error) {
			if !key && len(p) == 1 && p[0] == "name" {
				return names[value], nil
			}
			if len(p) == 2 {
				switch p[0] {
				case "dependencies", "devDependencies", "peerDependencies", "optionalDependencies":
					if key {
						if target, ok := names[value]; ok {
							return target, nil
						}
					}
				case "scripts":
					if !key {
						return packageTokenRE.ReplaceAllStringFunc(value, func(token string) string {
							if target, ok := names[token]; ok {
								return target
							}
							return token
						}), nil
					}
				}
			}
			return value, nil
		})
		if err != nil {
			return i18n.Errorf("template.transform_failed", file, err)
		}
		// Keep renamed dependency keys in the order required by the formatter.
		var pkg map[string]json.RawMessage
		if err := json.Unmarshal(raw, &pkg); err != nil {
			return err
		}
		updates := map[string]json.RawMessage{}
		for _, field := range []string{"dependencies", "devDependencies", "peerDependencies", "optionalDependencies"} {
			if value, ok := pkg[field]; ok {
				sorted, err := jsonedit.SortKeys(value)
				if err != nil {
					return i18n.Errorf("template.transform_failed", file, err)
				}
				if !bytes.Equal(value, sorted) {
					updates[field] = sorted
				}
			}
		}
		raw, err = jsonedit.UpdateFields(raw, updates)
		if err != nil {
			return err
		}
		files[file] = raw
	}
	for _, file := range spec.SourceFiles {
		raw, ok := files[file]
		if !ok {
			return i18n.Errorf("template.file_missing", file)
		}
		if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
			return i18n.Errorf("template.text_invalid", file)
		}
		from := spec.Scope + "/"
		if !bytes.Contains(raw, []byte(from)) {
			return i18n.Errorf("template.text_matches", file, from, 1, 0)
		}
		files[file] = bytes.ReplaceAll(raw, []byte(from), []byte(targetScope+"/"))
	}
	return nil
}
