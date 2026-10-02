package creation

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func materializeComposite(ctx context.Context, root string, in ProjectInput, files *fsutil.FilePlan, manifest *workspace.Manifest, outputs []template.ProjectOutput, manager string, miseEnabled bool) (result ProjectResult, err error) {
	result = ProjectResult{Name: in.Name, TargetPath: root, TemplateID: in.Template.ID, Toolchain: string(in.Template.Toolchain), PackageManager: manager}
	for _, project := range manifest.Projects {
		if project.Name == in.Name {
			return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.Tf("creation.group_exists", in.Name))
		}
	}
	for _, group := range manifest.Groups {
		if group.Name == in.Name {
			return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.Tf("creation.group_exists", in.Name))
		}
	}
	group := workspace.ManifestGroup{Name: in.Name}
	// All destination checks precede publication; FilePlan also verifies snapshots.
	for _, output := range outputs {
		category, categoryErr := categoryDirFor(string(output.Category))
		if categoryErr != nil {
			return ProjectResult{}, categoryErr
		}
		rel := filepath.Join(category, output.Name)
		dir := filepath.Join(root, rel)
		if err := fsutil.SafeWritePath(root, dir); err != nil {
			return ProjectResult{}, err
		}
		nonempty, err := dirNonEmpty(dir)
		if err != nil {
			return ProjectResult{}, err
		}
		if nonempty {
			return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.Tf("creation.directory_exists", dir)).WithContext(map[string]any{"subproject_name": output.Name, "target_path": dir})
		}
		for _, current := range manifest.Projects {
			if current.Name == output.Name || current.RelativeDir == filepath.ToSlash(rel) {
				return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.Tf("creation.project_registered", output.Name))
			}
		}
		for _, current := range manifest.Groups {
			if current.Name == output.Name {
				return ProjectResult{}, cliErrors.New(cliErrors.TARGET_EXISTS, i18n.Tf("creation.group_exists", output.Name))
			}
		}
		group.Projects = append(group.Projects, output.Name)
		result.Projects = append(result.Projects, ProjectResult{Name: output.Name, TargetPath: dir, TemplateID: in.Template.ID, Toolchain: string(in.Template.Toolchain), PackageManager: manager})
		manifest.Projects = append(manifest.Projects, workspace.ManifestProject{Name: output.Name, RelativeDir: filepath.ToSlash(rel), Toolchain: string(in.Template.Toolchain)})
	}
	manifest.Groups = append(manifest.Groups, group)
	missing := map[string]bool{}
	defer func() {
		if err == nil {
			return
		}
		dirs := make([]string, 0, len(missing))
		for dir := range missing {
			dirs = append(dirs, dir)
		}
		sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
		for _, dir := range dirs {
			_ = os.Remove(dir)
		}
	}()
	for i, output := range outputs {
		rel, _ := filepath.Rel(root, result.Projects[i].TargetPath)
		for name, raw := range output.Files {
			dest := filepath.Join(rel, filepath.FromSlash(name))
			for dir := filepath.Dir(filepath.Join(root, dest)); dir != root; dir = filepath.Dir(dir) {
				if _, statErr := os.Lstat(dir); os.IsNotExist(statErr) {
					missing[dir] = true
				}
			}
			mode := os.FileMode(0o644)
			if strings.HasSuffix(name, ".sh") {
				mode = 0o755
			}
			if err = files.Set(dest, raw, mode); err != nil {
				return ProjectResult{}, err
			}
		}
		if in.Template.Toolchain == "node" {
			packageName := template.CommonVariables(output.Name, manager)["projectNameKebabCase"]
			if err = configureNodePackage(files, rel, packageName, manager); err != nil {
				return ProjectResult{}, err
			}
		}
	}
	if err = planLanguages(files, manifest); err != nil {
		return ProjectResult{}, err
	}
	if err = validateNodePackageNames(files, manifest); err != nil {
		return ProjectResult{}, err
	}
	if err = finishProjectPlan(ctx, root, files, manifest, miseEnabled); err != nil {
		return ProjectResult{}, err
	}
	result.Warnings = warningMessages(template.CheckAllowedBackends(*in.Template, workspace.SelectionForProject(manifest, nil), ""))
	return result, nil
}
