// Package ci owns the workspace-level CI lifecycle while pkg/ci retains the
// public provider compatibility contract.
package ci

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

type Service struct {
	providers *pkgci.Registry
}

func NewService(providers *pkgci.Registry) (*Service, error) {
	if providers == nil || len(providers.Providers()) == 0 {
		return nil, errors.New("application: CI providers are required")
	}
	return &Service{providers: providers}, nil
}

func (s *Service) selectProjects(
	activeWorkspace execution.Workspace,
	selector string,
	requireNonEmpty bool,
) ([]workspace.Project, error) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		project, ok := activeWorkspace.Project(selector)
		if !ok {
			return nil, cliErrors.New(
				cliErrors.SUBPROJECT_NOT_FOUND,
				i18n.Tf("ci.error.project_not_found", selector),
			).WithContext(map[string]any{
				"selector": selector, "available_projects": activeWorkspace.ProjectNames(),
			})
		}
		return []workspace.Project{*project}, nil
	}
	projects := activeWorkspace.Projects()
	if requireNonEmpty && len(projects) == 0 {
		return nil, cliErrors.New(
			cliErrors.MANIFEST_MISSING_OR_EMPTY,
			i18n.T("ci.error.no_projects"),
		)
	}
	return projects, nil
}

func (s *Service) resolveProviderID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "github-actions" {
		raw = pkgci.DefaultProviderID
	}
	if s.providers.Lookup(raw) != nil {
		return raw, nil
	}
	registered := s.providers.Providers()
	available := make([]string, 0, len(registered))
	for _, provider := range registered {
		available = append(available, provider.ID())
	}
	return "", cliErrors.New(
		cliErrors.CI_PROVIDER_UNKNOWN,
		i18n.Tf("ci.error.provider_unknown", raw),
	).WithContext(map[string]any{
		"provider": raw, "available_providers": available,
	}).WithRemediation(output.Remediation{
		Action: "use-supported-provider", Hint: i18n.T("ci.error.provider_hint"),
		Command: "one ci enable <project> --provider ci/github-actions",
	})
}

type syncResult struct {
	workflowPath string
	created      bool
}

func (s *Service) syncProject(
	root string,
	project workspace.Project,
	providerID string,
) (syncResult, error) {
	provider := s.providers.Lookup(providerID)
	if provider == nil {
		return syncResult{}, cliErrors.New(
			cliErrors.CI_PROVIDER_UNKNOWN,
			fmt.Sprintf("unknown CI provider %q", providerID),
		)
	}
	tc := toolchain.Toolchain(project.Toolchain)
	if tc == "" {
		tc = toolchain.Node
	}
	pm := toolchain.PackageManager(project.PackageManager)
	if pm == "" && tc == toolchain.Node {
		pm = toolchain.PMpnpm
	}
	scripts, err := loadScripts(project.TargetDir)
	if err != nil {
		return syncResult{}, err
	}
	relativeDir, err := filepath.Rel(root, project.TargetDir)
	if err != nil {
		return syncResult{}, err
	}
	relativeDir = filepath.ToSlash(relativeDir)
	input := pkgci.Input{
		ProjectRoot: root, TargetDir: project.TargetDir, RelativeDir: relativeDir,
		ProjectName: project.Name, Toolchain: tc, PackageManager: pm,
		Scripts: scripts, Adapter: toolchain.Get(tc),
	}
	input.WorkflowFilePath = provider.WorkflowFilename(input)
	workflowPath := filepath.Join(root, filepath.FromSlash(input.WorkflowFilePath))
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		return syncResult{}, err
	}
	exists, err := workflowExists(workflowPath)
	if err != nil {
		return syncResult{}, err
	}
	if err := os.WriteFile(workflowPath, []byte(provider.Render(input)), 0o644); err != nil {
		return syncResult{}, err
	}
	return syncResult{workflowPath: workflowPath, created: !exists}, nil
}

func (s *Service) workflowPath(root string, project workspace.Project, providerID string) string {
	provider := s.providers.Lookup(providerID)
	if provider == nil {
		return ""
	}
	relativeDir, err := filepath.Rel(root, project.TargetDir)
	if err != nil {
		relativeDir = project.TargetDir
	}
	relativeDir = filepath.ToSlash(relativeDir)
	relativeWorkflow := provider.WorkflowFilename(pkgci.Input{
		ProjectRoot: root, TargetDir: project.TargetDir, RelativeDir: relativeDir,
		ProjectName: project.Name,
	})
	return filepath.Join(root, filepath.FromSlash(relativeWorkflow))
}

func workflowExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, cliErrors.New(
				cliErrors.CI_RENDER_FAILED,
				fmt.Sprintf("CI workflow path is a directory: %s", path),
			)
		}
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func removeWorkflow(path string) (bool, error) {
	exists, err := workflowExists(path)
	if err != nil || !exists {
		return false, err
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}

func relativeWorkflowPath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func loadScripts(targetDir string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(targetDir, "package.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var value struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]string{}, nil
	}
	if value.Scripts == nil {
		value.Scripts = map[string]string{}
	}
	return value.Scripts, nil
}
