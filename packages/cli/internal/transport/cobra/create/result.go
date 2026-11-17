package createcmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/terminalui"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type createResult struct {
	Schema         string   `json:"schema"`
	ProjectName    string   `json:"project_name"`
	CreatedPath    string   `json:"created_path"`
	CreatedInPlace bool     `json:"created_in_place"`
	PackageManager string   `json:"package_manager"`
	SecretsBackend string   `json:"secrets_backend,omitempty"`
	DevEnabled     bool     `json:"dev_enabled"`
	Warnings       []string `json:"warnings,omitempty"`
	displayPath    string
}

// RenderTTY prints a friendly create-success summary.
func (r *createResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	ui := terminalui.New(w)
	ui.Header("one create")
	ui.Success(i18n.Tf("create.success", r.ProjectName))
	ui.Blank()
	fields := []terminalui.Field{{Label: i18n.T("create.location"), Value: compactHomePath(r.CreatedPath)}}
	if r.PackageManager != "" {
		fields = append(fields, terminalui.Field{Label: i18n.T("create.package_manager"), Value: r.PackageManager})
	}
	if r.SecretsBackend == "" {
		fields = append(fields, terminalui.Field{Label: i18n.T("create.env_source"), Value: i18n.T("create.env_local")})
	} else {
		fields = append(fields, terminalui.Field{Label: i18n.T("create.env_source"), Value: r.SecretsBackend})
	}
	if r.DevEnabled {
		fields = append(fields, terminalui.Field{Label: i18n.T("create.dev_enabled"), Value: "one dev"})
	}
	ui.Fields(fields...)
	if len(r.Warnings) > 0 {
		ui.Blank()
	}
	for _, warning := range r.Warnings {
		ui.Warning(warning)
	}
	ui.Section(i18n.T("common.next_steps"))
	cdPath := r.displayPath
	if cdPath == "" {
		cdPath = r.CreatedPath
	}
	ui.Command(fmt.Sprintf("cd %s", cdPath))
	ui.Command("one add")
	ui.Blank()
}

func compactHomePath(path string) string {
	home, err := userdirs.Home()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	if rel == "." {
		return "~"
	}
	return "~/" + filepath.ToSlash(rel)
}

func relativeOrAbs(cwd, targetDir string, useCurrentDir bool) string {
	rel, err := filepath.Rel(resolveDisplayPath(cwd), resolveDisplayPath(targetDir))
	if err == nil && rel != "" {
		return rel
	}
	if useCurrentDir {
		return "."
	}
	return targetDir
}

// resolveDisplayPath resolves symlinks in existing ancestors, even when the
// workspace directory (and some of its parents) has not been created yet.
// Other filesystem errors leave the original path available for display.
func resolveDisplayPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if !os.IsNotExist(err) || parent == path {
		return path
	}
	return filepath.Join(resolveDisplayPath(parent), filepath.Base(path))
}
