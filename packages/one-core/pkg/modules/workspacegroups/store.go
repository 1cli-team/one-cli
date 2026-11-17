package workspacegroups

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type Group struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	WorkspaceIDs []string  `json:"workspaceIds"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type document struct {
	Version int     `json:"version"`
	Groups  []Group `json:"groups"`
}

func storagePath() (string, error) {
	root, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "workspace-groups.json"), nil
}

func readDocument(path string) (document, error) {
	result := document{Version: 1, Groups: []Group{}}
	raw, err := fsutil.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, i18n.Errorf("groups.read_failed", path, err)
	}
	result = document{}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, i18n.Errorf("groups.read_failed", path, err)
	}
	if result.Version != 1 {
		return result, i18n.Errorf("groups.version", result.Version)
	}
	seen := make(map[string]bool)
	for index := range result.Groups {
		group := &result.Groups[index]
		if group.ID == "" || group.Name == "" || seen[group.ID] {
			return result, i18n.Errorf("groups.invalid_file", path)
		}
		seen[group.ID] = true
		members := make(map[string]bool)
		for _, id := range group.WorkspaceIDs {
			if id == "" || members[id] {
				return result, i18n.Errorf("groups.invalid_file", path)
			}
			members[id] = true
		}
		if group.WorkspaceIDs == nil {
			group.WorkspaceIDs = []string{}
		}
	}
	if result.Groups == nil {
		result.Groups = []Group{}
	}
	return result, nil
}

func updateDocument(ctx context.Context, update func(*document) error) error {
	path, err := storagePath()
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	lock := flock.New(path + ".lock")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return i18n.Errorf("groups.busy")
	}
	defer lock.Unlock()
	state, err := readDocument(path)
	if err != nil {
		return err
	}
	if err := update(&state); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".workspace-groups-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := fsutil.ReplaceFile(file.Name(), path); err != nil {
		return err
	}
	return fsutil.SyncDir(directory)
}
