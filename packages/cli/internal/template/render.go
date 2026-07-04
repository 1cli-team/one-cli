package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aymerick/raymond"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bundled"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// Variables is the merged variable bag passed to handlebars + path
// interpolation. Mirrors TemplateVariables in TS — all values are strings;
// users can extend the bag via template.json declarations.
type Variables map[string]string

// CommonVariables returns the base variable bag every template starts with.
// Users (or template.json) can override any of these.
func CommonVariables(projectName, packageManager string) Variables {
	return Variables{
		"projectName":           projectName,
		"projectNameCamelCase":  toCamelCase(projectName),
		"projectNamePascalCase": toPascalCase(projectName),
		"projectNameKebabCase":  toKebabCase(projectName),
		"author":                "",
		"description":           "",
		"license":               "MIT",
		"year":                  fmt.Sprintf("%d", currentYear()),
		"packageManager":        packageManager,
	}
}

// excludedTemplateEntries are filenames the renderer never copies into
// the destination. .git / node_modules are obvious; go.mod / go.sum are
// dev-only module-isolation files for Go templates.
var excludedTemplateEntries = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"AGENTS.md":    {},
	"CLAUDE.md":    {},
	"go.mod":       {},
	"go.sum":       {},
}

// pathVarRE matches the __varName__ placeholder syntax used in
// directory and file names.
var pathVarRE = regexp.MustCompile(`__([a-zA-Z0-9_]+)__`)

// LocalTemplatePrefix is the registry-repo prefix marking a template that
// ships inside the bundled templates tree (vs. a remote tarball).
const LocalTemplatePrefix = "local:"

// Render copies the templateID's tree (from the embedded TemplatesFS) into
// targetDir, applying handlebars to *.hbs files and path-variable substitution
// to filenames. Mirrors renderTemplateIntoTarget in TS.
//
// templateID is the directory name under bundled.TemplatesRoot — e.g. for
// the nestjs-api template, templateID = "nestjs-api". Callers normally derive
// this from a registry entry's Repo field (parseLocalTemplateRepo).
func Render(templateID string, targetDir string, vars Variables) error {
	root := filepath.ToSlash(filepath.Join(bundled.TemplatesRoot, templateID))
	// Sanity check that the directory exists in the embed FS.
	if _, err := fs.Stat(bundled.TemplatesFS, root); err != nil {
		return cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
			fmt.Sprintf("本地模板不存在：%s。请确认 templates/%s 已存在。", templateID, templateID))
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	return renderEmbeddedTree(root, targetDir, vars, true)
}

// renderEmbeddedTree walks an FS subtree and writes the rendered output to
// the host filesystem. isRoot is true only for the top-level invocation and
// triggers special handling of template.json (skipped from copy).
func renderEmbeddedTree(srcRoot string, dstRoot string, vars Variables, isRoot bool) error {
	entries, err := fs.ReadDir(bundled.TemplatesFS, srcRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, skip := excludedTemplateEntries[name]; skip {
			continue
		}
		if isRoot && !entry.IsDir() && name == "template.json" {
			continue
		}
		renderedName := replacePathVariables(name, vars)
		srcPath := filepath.ToSlash(filepath.Join(srcRoot, name))
		if entry.IsDir() {
			dstPath := filepath.Join(dstRoot, renderedName)
			if err := renderEmbeddedTree(srcPath, dstPath, vars, false); err != nil {
				return err
			}
			continue
		}
		// File: handlebars-render if .hbs, else copy as-is.
		isHbs := strings.HasSuffix(renderedName, ".hbs")
		dstName := renderedName
		if isHbs {
			dstName = strings.TrimSuffix(renderedName, ".hbs")
		}
		dstPath := filepath.Join(dstRoot, dstName)
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		raw, err := fs.ReadFile(bundled.TemplatesFS, srcPath)
		if err != nil {
			return err
		}
		var body []byte
		if isHbs {
			rendered, rerr := renderHandlebars(string(raw), vars)
			if rerr != nil {
				return cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND,
					fmt.Sprintf("模板渲染失败 %s: %v", srcPath, rerr))
			}
			body = []byte(rendered)
		} else {
			body = raw
		}
		if err := os.WriteFile(dstPath, body, copyMode(entry)); err != nil {
			return err
		}
	}
	return nil
}

// copyMode preserves the executable bit when an embedded file is .sh / hook.
// embed.FS doesn't surface stat info, so we approximate by extension.
func copyMode(d fs.DirEntry) os.FileMode {
	name := d.Name()
	if strings.HasSuffix(name, ".sh") || strings.Contains(name, "/.husky/") {
		return 0o755
	}
	return 0o644
}

func renderHandlebars(template string, vars Variables) (string, error) {
	tpl, err := raymond.Parse(template)
	if err != nil {
		return "", err
	}
	// raymond expects map[string]interface{}; convert.
	ctx := make(map[string]interface{}, len(vars))
	for k, v := range vars {
		ctx[k] = v
	}
	return tpl.Exec(ctx)
}

// replacePathVariables expands __varName__ placeholders in directory / file
// names. Unknown variables pass through unchanged so users can include
// double-underscore strings in template paths if they really need to.
func replacePathVariables(name string, vars Variables) string {
	return pathVarRE.ReplaceAllStringFunc(name, func(segment string) string {
		match := pathVarRE.FindStringSubmatch(segment)
		if len(match) != 2 {
			return segment
		}
		if v, ok := vars[match[1]]; ok {
			return v
		}
		return segment
	})
}

// helpers below.

func toCamelCase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}
	out := words[0]
	for _, w := range words[1:] {
		out += capFirst(w)
	}
	return out
}

func toPascalCase(s string) string {
	words := splitWords(s)
	out := ""
	for _, w := range words {
		out += capFirst(w)
	}
	return out
}

func toKebabCase(s string) string {
	return strings.Join(splitWords(s), "-")
}

var (
	camelToWordsRE = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	nonAlphaNumRE  = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

func splitWords(s string) []string {
	s = strings.TrimSpace(s)
	s = camelToWordsRE.ReplaceAllString(s, "$1 $2")
	parts := nonAlphaNumRE.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, strings.ToLower(p))
	}
	return out
}

func capFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// currentYear is a package-level seam for tests; production calls time.Now().
var currentYear = func() int {
	return _now().Year()
}
