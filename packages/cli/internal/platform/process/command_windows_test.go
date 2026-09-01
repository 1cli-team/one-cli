//go:build windows

package process

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandContextRunsBatchLauncherWithQuotedArguments(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(dir, "fixture.cmd")
	if err := os.WriteFile(launcher, []byte("@echo off\r\necho [%~1] [%~2]\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := CommandContext(context.Background(), launcher, "hello world", "a&b")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("batch launcher failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "[hello world] [a&b]" {
		t.Fatalf("batch output = %q", got)
	}
}
