// Package workspace owns read-only Workspace metadata used by the local
// Dashboard HTTP boundary.
package workspace

import (
	"errors"
	"strings"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

var (
	ErrInvalidInput    = errors.New("workspace: invalid input")
	ErrProjectNotFound = errors.New("workspace: project not found")
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Overview(root string, environments ...string) (workspacecore.Overview, error) {
	environment := ""
	if len(environments) > 0 {
		environment = strings.TrimSpace(environments[0])
	}
	return workspacecore.BuildOverview(root, environment)
}

func findProject(manifest *workspacecore.Manifest, name string) *workspacecore.ManifestProject {
	if manifest == nil {
		return nil
	}
	for index := range manifest.Projects {
		if manifest.Projects[index].Name == name {
			return &manifest.Projects[index]
		}
	}
	return nil
}
