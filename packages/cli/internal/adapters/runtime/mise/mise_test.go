package mise

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

func TestSupportedVersion(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    bool
	}{
		{"2026.9.7 linux-x64 (2026-09-13)", true}, {"2026.10.0", true}, {"2027.1.0", true},
		{"2026.9.6", false}, {"2026.8.999", false}, {"", false}, {"2026.9.7-dev", false}, {"2026.9.x", false},
	} {
		if got := supportedVersion(tt.version); got != tt.want {
			t.Errorf("%q: got %v want %v", tt.version, got, tt.want)
		}
	}
}

func TestExplicitBinaryIsRespectedAndVersionChecked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake executable")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "private mise")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 2026.9.7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONE_MISE_BINARY", path)
	command := runtimeport.Command{Directory: dir, Argv: []string{"one", "__exec", "--", "a b", ""}, Env: []string{"PATH=/somewhere", "KEEP=unchanged"}}
	prepared, err := (Provider{}).Prepare(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Argv[0] != path || prepared.Argv[len(prepared.Argv)-1] != "" {
		t.Fatalf("argv: %q", prepared.Argv)
	}
	if !strings.HasPrefix(prepared.Env[0], "PATH="+dir+string(os.PathListSeparator)) {
		t.Fatalf("private runtime path missing: %q", prepared.Env)
	}
	if command.Env[0] != "PATH=/somewhere" {
		t.Fatal("mutated caller environment")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 2025.1.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := (Provider{}).Prepare(context.Background(), command); err == nil {
		t.Fatal("unsupported explicit version accepted")
	}
}
