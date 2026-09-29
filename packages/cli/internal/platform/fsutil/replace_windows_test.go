//go:build windows

package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestReplaceFileWaitsForWindowsReaderAndPublishes(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "state.json")
	source := filepath.Join(dir, "state.tmp")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = reader.Close()
	}()
	if err := ReplaceFile(source, target); err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "new" {
		t.Fatalf("target = %q, want new", raw)
	}
}

func TestReadFilePreservesWindowsPathError(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name  string
		path  string
		cause error
	}{
		{name: "missing", path: filepath.Join(dir, "missing.json"), cause: windows.ERROR_FILE_NOT_FOUND},
		{name: "invalid", path: filepath.Join(dir, "invalid\x00.json"), cause: syscall.EINVAL},
		{name: "directory", path: dir, cause: windows.ERROR_ACCESS_DENIED},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadFile(tc.path)
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("ReadFile error = %T (%v), want *os.PathError", err, err)
			}
			if pathErr.Op != "open" || pathErr.Path != tc.path {
				t.Fatalf("PathError = %+v, want open %q", pathErr, tc.path)
			}
			if !errors.Is(err, tc.cause) {
				t.Fatalf("ReadFile error = %v, want cause %v", err, tc.cause)
			}
		})
	}
}
