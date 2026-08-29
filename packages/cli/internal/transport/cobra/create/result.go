package createcmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type createResult struct {
	Schema         string        `json:"schema"`
	ProjectName    string        `json:"project_name"`
	CreatedPath    string        `json:"created_path"`
	CreatedInPlace bool          `json:"created_in_place"`
	PackageManager string        `json:"package_manager"`
	SecretsBackend string        `json:"secrets_backend,omitempty"`
	CIEnabled      bool          `json:"ci_enabled"`
	DevEnabled     bool          `json:"dev_enabled"`
	Skills         skillsPayload `json:"skills"`
}

// RenderTTY prints a friendly create-success summary.
func (r *createResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, i18n.T("create.success")+"\n", r.ProjectName)
	fmt.Fprintf(w, i18n.T("create.location")+"\n", r.CreatedPath)
	fmt.Fprintf(w, i18n.T("create.package_manager")+"\n", r.PackageManager)
	if r.SecretsBackend == "" || r.SecretsBackend == workspace.EnvBackendDotenv {
		fmt.Fprintln(w, i18n.T("create.env_local"))
	} else {
		fmt.Fprintf(w, i18n.T("create.env_source")+"\n", r.SecretsBackend)
	}
	if r.CIEnabled {
		fmt.Fprintln(w, i18n.T("create.ci_github"))
	}
	if r.DevEnabled {
		fmt.Fprintln(w, i18n.T("create.dev_enabled"))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("common.next_steps"))
	fmt.Fprintf(w, "  cd %s\n", r.CreatedPath)
	fmt.Fprintln(w, "  one add")
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("common.optional"))
	fmt.Fprintln(w, "  one skills install")
}

type skillsPayload struct {
	Status      string       `json:"status"`
	InstalledTo []string     `json:"installed_to,omitempty"`
	SkillCount  int          `json:"skill_count,omitempty"`
	Reason      string       `json:"reason,omitempty"`
	Error       *skillsError `json:"error,omitempty"`
}

type skillsError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func relativeOrAbs(cwd, targetDir string, useCurrentDir bool) string {
	rel, err := filepath.Rel(cwd, targetDir)
	if err == nil && rel != "" {
		return rel
	}
	if useCurrentDir {
		return "."
	}
	return targetDir
}

func parsePresetProjectNames(raw string, want int) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for i, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, cliErrors.New(cliErrors.INVALID_NAME,
				fmt.Sprintf("--project-names 第 %d 项为空。", i+1)).
				WithContext(map[string]any{
					"project_names": raw,
					"index":         i,
				})
		}
		if !workspace.IsValidProjectName(name) {
			return nil, cliErrors.New(cliErrors.INVALID_NAME,
				fmt.Sprintf("子项目名称格式不合法: %q（来自 --project-names）", name)).
				WithContext(map[string]any{
					"project_names": raw,
					"invalid_name":  name,
					"index":         i,
				})
		}
		if seen[name] {
			return nil, cliErrors.New(cliErrors.INVALID_NAME,
				fmt.Sprintf("--project-names 包含重复名称: %q", name)).
				WithContext(map[string]any{
					"project_names":  raw,
					"duplicate_name": name,
					"index":          i,
				})
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) != want {
		return nil, cliErrors.New(cliErrors.PRESET_INVALID,
			fmt.Sprintf("--project-names 数量为 %d，但 preset 会展开 %d 个子项目。", len(names), want)).
			WithContext(map[string]any{
				"project_names": raw,
				"provided":      len(names),
				"expected":      want,
			})
	}
	return names, nil
}
