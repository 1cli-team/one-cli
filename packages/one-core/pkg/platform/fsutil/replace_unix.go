//go:build !windows

package fsutil

import "os"

func replaceFile(source, target string) error { return os.Rename(source, target) }

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
