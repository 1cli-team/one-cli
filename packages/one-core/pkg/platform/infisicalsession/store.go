package infisicalsession

import (
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
)

const sessionFile = "session.json"

func readSessionSecret() ([]byte, error) {
	path, err := ConfigPath(sessionFile)
	if err != nil {
		return nil, err
	}
	return fsutil.ReadFile(path)
}

// The caller holds sessionLock. Atomic replacement lets concurrent readers see
// either complete session, and keeps a failed write from truncating credentials.
func writeSessionSecret(raw []byte) error {
	path, err := ConfigPath(sessionFile)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".session-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return fsutil.ReplaceFile(tmp.Name(), path)
}

func removeSessionSecret() error {
	path, err := ConfigPath(sessionFile)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
