package template

import (
	"regexp"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

var pythonModuleRE = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func rewritePython(files map[string][]byte, spec PythonSpec, vars Variables) error {
	module := vars["projectNameSnakeCase"]
	if !pythonModuleRE.MatchString(module) {
		return i18n.Errorf("template.spec_invalid", "projectNameSnakeCase")
	}
	prefix := "src/" + spec.Module + "/"
	if _, ok := files[prefix+"__init__.py"]; !ok {
		return i18n.Errorf("template.file_missing", prefix+"__init__.py")
	}
	for _, file := range sortedFiles(files) {
		if !strings.HasPrefix(file, prefix) {
			continue
		}
		target := "src/" + module + "/" + strings.TrimPrefix(file, prefix)
		if target == file {
			continue
		}
		if _, exists := files[target]; exists {
			return i18n.Errorf("template.file_collision", target)
		}
		files[target] = files[file]
		delete(files, file)
	}
	return nil
}
