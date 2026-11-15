package mise

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/mirror"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
)

func TestProviderAppliesMirrorBeforePreparingChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX executable")
	}
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	binary := filepath.Join(dir, "mise")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho 2026.9.7\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONE_MISE_BINARY", binary)
	if _, err := mirror.Select(t.Context(), mirror.EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	command, err := (Provider{}).Prepare(t.Context(), runtimeport.Command{Directory: dir, Argv: []string{"npm", "install"}, Env: []string{"PATH=/bin"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(command.Argv, " ") != binary+" exec -- npm install" {
		t.Fatalf("argv=%q", command.Argv)
	}
	for key, want := range map[string]string{"MISE_NODE_MIRROR_URL": "https://npmmirror.com/mirrors/node/", "npm_config_registry": "https://registry.npmmirror.com/", "MISE_BACKENDS_PNPM": "npm:pnpm"} {
		if got := envValue(command.Env, key); got != want {
			t.Errorf("%s=%q, want %q", key, got, want)
		}
	}
	if strings.Contains(envValue(command.Env, "MISE_URL_REPLACEMENTS"), "sha256") {
		t.Fatal("Go checksum redirected")
	}
}
