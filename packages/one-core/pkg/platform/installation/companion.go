package installation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
)

// Companion resolves an immutable helper selected by an installation receipt.
// Archives and development builds use a sibling one-api. Keeping old helpers
// allows an already running local server to survive CLI/Desktop upgrades.
func Companion(executable string) string {
	if real, err := filepath.EvalSymlinks(executable); err == nil {
		executable = real
	}
	dir := filepath.Dir(executable)
	var receipt struct {
		ServerFile string `json:"serverFile"`
	}
	if raw, err := os.ReadFile(executable + ".installation.json"); err == nil && json.Unmarshal(raw, &receipt) == nil {
		path := filepath.FromSlash(receipt.ServerFile)
		if !filepath.IsAbs(path) && strings.HasPrefix(filepath.ToSlash(path), ".one-api/") && !strings.Contains(path, "..") {
			return filepath.Join(dir, path)
		}
	}
	return filepath.Join(dir, "one-api"+filepath.Ext(executable))
}

// ReplacePair is called under the installation lock, after both candidates
// have been verified. Helpers are content addressed and never replaced while
// running. A failed CLI replacement restores the previous receipt.
func ReplacePair(candidate, helper, target, version string) error {
	digest, err := Digest(helper)
	if err != nil {
		return err
	}
	relative := filepath.Join(".one-api", digest, "one-api"+filepath.Ext(target))
	destination := filepath.Join(filepath.Dir(target), relative)
	if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	existing, err := Digest(destination)
	if errors.Is(err, os.ErrNotExist) {
		stage, e := os.CreateTemp(filepath.Dir(destination), ".stage-*")
		if e != nil {
			return e
		}
		name := stage.Name()
		stage.Close()
		defer os.Remove(name)
		if e = copyFile(helper, name); e != nil {
			return e
		}
		if e = os.Chmod(name, 0755); e != nil {
			return e
		}
		actual, e := Digest(name)
		if e != nil {
			return e
		}
		if actual != digest {
			return errors.New("companion checksum changed while installing")
		}
		if e = fsutil.ReplaceFile(name, destination); e != nil {
			return e
		}
	} else if err != nil {
		return err
	} else if existing != digest {
		return errors.New("installed companion checksum mismatch")
	}
	receiptPath := target + ".installation.json"
	previous, readErr := os.ReadFile(receiptPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	data, err := json.Marshal(map[string]string{"version": version, "serverFile": filepath.ToSlash(relative)})
	if err != nil {
		return err
	}
	if err = fsutil.WriteAtomic(receiptPath, data, 0600); err != nil {
		return err
	}
	if err = fsutil.ReplaceFile(candidate, target); err != nil {
		if readErr == nil {
			_ = fsutil.WriteAtomic(receiptPath, previous, 0600)
		} else {
			_ = os.Remove(receiptPath)
		}
		return err
	}
	return fsutil.SyncDir(filepath.Dir(target))
}
