package mise

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// Copies of this test executable act as portable PATH mise fixtures, including
// on Windows where shell scripts cannot be launched as native executables.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		path, err := os.Executable()
		if err == nil {
			if version, err := os.ReadFile(path + ".version"); err == nil {
				fmt.Print(string(version))
				os.Exit(0)
			}
		}
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func isolatedRuntimeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ONE_MISE_BINARY", "")
	for _, key := range []string{"XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(key, filepath.Join(home, key))
	}
	return home
}

func fakeMise(t *testing.T, dir, version string) string {
	t.Helper()
	name := "mise"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".version", []byte(version), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSystemMiseUsesCommandPathWithoutInstalling(t *testing.T) {
	home := isolatedRuntimeHome(t)
	dir := filepath.Join(t.TempDir(), "system mise")
	want := fakeMise(t, dir, "2026.9.7")
	command := runtimeport.Command{Directory: home, Argv: []string{"--version"}, Env: []string{"PATH=" + dir, "KEEP=yes"}}
	got, err := (Provider{}).PrepareCLI(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if got.Argv[0] != want {
		t.Fatalf("selected %s, want system %s", got.Argv[0], want)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("system resolution wrote managed files: %v %v", entries, err)
	}
}

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
	if !strings.HasPrefix(envValue(prepared.Env, "PATH"), dir+string(os.PathListSeparator)) {
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

func TestResolverRechecksSystemDeletionAndManagedRepair(t *testing.T) {
	home := isolatedRuntimeHome(t)
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	a, archive := archiveFixture(t, "tar.gz", []byte("managed fixture"))
	d, server, requests := serveArchive(t, archive)
	i := installer{root: paths.runtimeRoot(), legacyRoot: paths.legacyRoot(), downloader: d}
	r := binaryResolver{paths: paths, asset: func() (releaseAsset, error) { return a, nil }, install: i.ensure}
	systemDir := filepath.Join(t.TempDir(), "system")
	system := fakeMise(t, systemDir, "2026.9.7")
	command := runtimeport.Command{Directory: home, Env: []string{"PATH=" + systemDir}}
	resolve := func(source string, downloads int32) resolvedBinary {
		t.Helper()
		got, err := r.resolve(context.Background(), command)
		if err != nil || got.source != source || requests.Load() != downloads {
			t.Fatalf("resolve: %+v err=%v requests=%d; want %s/%d", got, err, requests.Load(), source, downloads)
		}
		return got
	}
	resolve("system", 0)
	if err := os.Remove(system); err != nil {
		t.Fatal(err)
	}
	managed := resolve("managed", 1)
	resolve("managed", 1)
	if err := os.Remove(managed.path); err != nil {
		t.Fatal(err)
	}
	resolve("managed", 2)
	fakeMise(t, systemDir, "2026.10.0")
	resolve("system", 2)
	if err := os.WriteFile(system+".version", []byte("2025.1.0"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve("managed", 2)
	server.Close()
	resolve("managed", 2)
	if err := os.Remove(managed.path); err != nil {
		t.Fatal(err)
	}
	if _, err := r.resolve(context.Background(), command); err == nil || !strings.Contains(err.Error(), "Could not prepare mise") {
		t.Fatalf("offline empty install must fail: %v", err)
	}
}

func TestResolverExplicitFailureAndCancellationDoNotInstall(t *testing.T) {
	isolatedRuntimeHome(t)
	r := binaryResolver{install: func(context.Context, releaseAsset) (string, error) {
		t.Fatal("unexpected installation")
		return "", nil
	}}
	path := fakeMise(t, t.TempDir(), "2025.1.0")
	t.Setenv("ONE_MISE_BINARY", path)
	if _, err := r.resolve(context.Background(), runtimeport.Command{}); err == nil {
		t.Fatal("unsupported explicit version accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := r.resolve(context.Background(), runtimeport.Command{}); err == nil {
		t.Fatal("missing explicit version accepted")
	}
	t.Setenv("ONE_MISE_BINARY", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.resolve(ctx, runtimeport.Command{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestResolverSkipsManagedPathAndAcceptsExternalWithoutManagedDirectories(t *testing.T) {
	isolatedRuntimeHome(t)
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	managedDir := filepath.Join(paths.runtimeRoot(), managedVersion, "fixture")
	managed := fakeMise(t, managedDir, "2026.9.7")
	externalDir := t.TempDir()
	external := fakeMise(t, externalDir, "2026.9.7")
	r := binaryResolver{paths: paths, pathsErr: errors.New("managed directory unavailable")}
	command := runtimeport.Command{Env: []string{"PATH=" + managedDir + string(os.PathListSeparator) + externalDir}}
	got, err := r.resolve(context.Background(), command)
	if err != nil || got.source != "system" || got.path != external {
		t.Fatalf("discovery: %+v %v", got, err)
	}
	// A direct override inside One's tree must be verified BEFORE execution.
	t.Setenv("ONE_MISE_BINARY", managed)
	r.pathsErr = nil
	r.asset = func() (releaseAsset, error) { return releaseAsset{BinarySHA256: strings.Repeat("0", 64)}, nil }
	if _, err := r.resolve(context.Background(), command); err == nil {
		t.Fatal("unverified managed override accepted")
	}
	raw, err := os.ReadFile(managed)
	if err != nil {
		t.Fatal(err)
	}
	r.asset = func() (releaseAsset, error) {
		return releaseAsset{BinarySHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}, nil
	}
	got, err = r.resolve(context.Background(), command)
	if err != nil || !got.managed || got.source != "explicit" {
		t.Fatalf("managed override: %+v %v", got, err)
	}
}

func TestExternalProviderPreservesMiseDirectories(t *testing.T) {
	isolatedRuntimeHome(t)
	dir := t.TempDir()
	fakeMise(t, dir, "2026.9.7")
	env := []string{"PATH=" + dir, "MISE_DATA_DIR=/external-data", "MISE_CONFIG_DIR=/external-config", "MISE_STATE_DIR=/external-state", "MISE_CACHE_DIR=/external-cache", "MISE_AUTO_UPDATE=true"}
	got, err := (Provider{}).PrepareCLI(context.Background(), runtimeport.Command{Env: env})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range env[1:] {
		key, expected, _ := strings.Cut(value, "=")
		if envValue(got.Env, key) != expected {
			t.Errorf("external setting changed: %s", key)
		}
	}
}
