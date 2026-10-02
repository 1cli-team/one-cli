package template

import (
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

var projectSuffixRE = regexp.MustCompile(`^-[a-z][a-z0-9-]*$`)

type ProjectOutput struct {
	Name     string
	Category Category
	Files    map[string][]byte
}

func templateSource(id string) (fs.FS, error) {
	if err := validatePaths([]string{id}); err != nil {
		return nil, err
	}
	return fs.Sub(bundled.TemplatesFS, path.Join(bundled.TemplatesRoot, id))
}

// ProjectLayouts exposes only the declarative component locations, without rendering.
func ProjectLayouts(id string) ([]ProjectSpec, error) {
	source, err := templateSource(id)
	if err != nil {
		return nil, err
	}
	spec, err := readSpec(source)
	return spec.Projects, err
}

// PrepareProjects returns nil for ordinary starters. Composite starters are fully
// validated and rendered in memory before creation writes any workspace files.
func PrepareProjects(id string, vars Variables) ([]ProjectOutput, error) {
	source, err := templateSource(id)
	if err != nil {
		return nil, err
	}
	spec, err := readSpec(source)
	if err != nil || len(spec.Projects) == 0 {
		return nil, err
	}
	files, err := prepare(source, vars)
	if err != nil {
		return nil, i18n.Errorf("template.prepare_failed", id, err)
	}
	return splitProjects(files, spec, vars)
}

func splitProjects(files map[string][]byte, spec Spec, vars Variables) ([]ProjectOutput, error) {
	outputs := make([]ProjectOutput, 0, len(spec.Projects))
	for _, project := range spec.Projects {
		output := ProjectOutput{Name: vars["projectName"] + project.Suffix, Category: project.Category, Files: map[string][]byte{}}
		for name, raw := range files {
			if relative, ok := strings.CutPrefix(name, project.Source+"/"); ok {
				output.Files[relative] = raw
			}
		}
		if len(output.Files) == 0 {
			return nil, i18n.Errorf("template.file_missing", project.Source)
		}
		for _, name := range spec.SharedFiles {
			raw, ok := files[name]
			if !ok {
				return nil, i18n.Errorf("template.file_missing", name)
			}
			if _, exists := output.Files[name]; exists {
				return nil, i18n.Errorf("template.file_collision", name)
			}
			output.Files[name] = raw
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}
