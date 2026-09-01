//go:build windows

package fsutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
