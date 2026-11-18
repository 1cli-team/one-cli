package execution

import (
	"errors"
	"os"
	"path/filepath"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

const mirrorCapabilityHeader = "X-One-Worker-China-Mirror"

var errWorkerMirror = errors.New("worker_mirror_unavailable")

func (s *Store) downloadSettings() api.DownloadSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.downloads
}

func (s *Store) setDownloadSettings(settings api.DownloadSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeJSON(s.downloadSettingsPath(), settings); err != nil {
		return err
	}
	s.downloads = settings
	return nil
}

func (s *Store) downloadSettingsPath() string {
	return filepath.Join(s.root, "download-settings.json")
}

func (s *Store) loadDownloadSettings() error {
	err := readJSON(s.downloadSettingsPath(), &s.downloads)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
