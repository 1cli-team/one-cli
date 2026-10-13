package userdirs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

// Paths describes One-owned storage. Resolution has no filesystem side effects;
// entry points initialize the current directories before opening any stores.
type Paths struct {
	Root, Config, Data, State, Cache, Temp string
}

func Resolve() (Paths, error) {
	root := strings.TrimSpace(os.Getenv("ONE_HOME"))
	if root == "" {
		home, err := Home()
		if err != nil {
			return Paths{}, err
		}
		root = filepath.Join(home, ".one")
	}
	if !filepath.IsAbs(root) {
		return Paths{}, i18n.Errorf("path.absolute_required", "ONE_HOME")
	}
	root = filepath.Clean(root)
	return Paths{Root: root, Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), State: filepath.Join(root, "state"), Cache: filepath.Join(root, "cache"), Temp: filepath.Join(root, "tmp")}, nil
}

func Config() (string, error) { p, err := Resolve(); return p.Config, err }
func Cache() (string, error)  { p, err := Resolve(); return p.Cache, err }

// Temp and Locks also serve callers outside the CLI entry point. Task files are
// private to a run; atomic replacement files still live beside their target.
func Temp() (string, error) {
	p, err := Resolve()
	if err == nil {
		err = os.MkdirAll(p.Temp, 0o700)
	}
	return p.Temp, err
}

func Locks() (string, error) {
	p, err := Resolve()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(p.State, "locks")
	return dir, os.MkdirAll(dir, 0o700)
}
