package git

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	registryport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/workspaceregistry"
)

type RepositoryEntry = registryport.LegacyRepository

type repositoryRegistry struct {
	Version      int               `json:"version"`
	Repositories []RepositoryEntry `json:"repositories"`
}

func repositoryRegistryPath() (string, error) {
	root, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "git", "repositories.json"), nil
}

func readRepositoryRegistry(path string) (repositoryRegistry, error) {
	registry := repositoryRegistry{Version: 1, Repositories: []RepositoryEntry{}}
	data, err := fsutil.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return registry, nil
	}
	if err != nil {
		return registry, i18n.Errorf("git.repositories_read_failed", path, err)
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return registry, i18n.Errorf("git.repositories_read_failed", path, err)
	}
	if registry.Version != 1 {
		return registry, i18n.Errorf("git.repositories_version", registry.Version)
	}
	seenIDs := make(map[string]bool)
	seenPaths := make(map[string]bool)
	for _, entry := range registry.Repositories {
		if entry.ID == "" || !filepath.IsAbs(entry.Path) || !filepath.IsAbs(entry.CommonDir) || seenIDs[entry.ID] || seenPaths[entry.CommonDir] {
			return registry, i18n.Errorf("git.repositories_invalid", path)
		}
		seenIDs[entry.ID] = true
		seenPaths[entry.CommonDir] = true
	}
	if registry.Repositories == nil {
		registry.Repositories = []RepositoryEntry{}
	}
	return registry, nil
}

// LegacyRepositories is a read-only migration source. New registrations are
// owned exclusively by the workspace registry.
func LegacyRepositories() ([]registryport.LegacyRepository, error) {
	path, err := repositoryRegistryPath()
	if err != nil {
		return nil, err
	}
	registry, err := readRepositoryRegistry(path)
	return registry.Repositories, err
}
