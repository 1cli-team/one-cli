package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallUnixCreatesOneSymlink(t *testing.T) {
	homeDir := t.TempDir()
	targetPath := writeTestBinary(t, homeDir, "build/one")
	var linkedTarget, linkedPath string
	installer := localInstaller{
		goos:    "linux",
		homeDir: homeDir,
		makeSymlink: func(target, path string) error {
			linkedTarget, linkedPath = target, path
			return os.WriteFile(path, []byte(target), 0o755)
		},
	}

	result, err := installer.install(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(homeDir, ".local", "bin", "one")
	if result.launcherPath != wantPath || linkedPath != wantPath {
		t.Fatalf("launcher path = %q, linked path = %q, want %q", result.launcherPath, linkedPath, wantPath)
	}
	if linkedTarget != result.targetPath {
		t.Fatalf("linked target = %q, resolved target = %q", linkedTarget, result.targetPath)
	}
	if result.usedCommandShim {
		t.Fatal("Unix install unexpectedly used a command shim")
	}
}

func TestInstallWindowsPrefersExeSymlink(t *testing.T) {
	homeDir := t.TempDir()
	targetPath := writeTestBinary(t, homeDir, "build/one.exe")
	installer := localInstaller{
		goos:    "windows",
		homeDir: homeDir,
		makeSymlink: func(target, path string) error {
			return os.WriteFile(path, []byte(target), 0o755)
		},
	}

	result, err := installer.install(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(homeDir, ".local", "bin", "one.exe")
	if result.launcherPath != wantPath {
		t.Fatalf("launcher path = %q, want %q", result.launcherPath, wantPath)
	}
	if result.usedCommandShim {
		t.Fatal("Windows install used a command shim even though symlink creation succeeded")
	}
}

func TestInstallWindowsFallsBackToCommandShim(t *testing.T) {
	homeDir := t.TempDir()
	targetPath := writeTestBinary(t, homeDir, "build/one.exe")
	installer := localInstaller{
		goos:        "windows",
		homeDir:     homeDir,
		makeSymlink: func(string, string) error { return os.ErrPermission },
	}

	result, err := installer.install(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !result.usedCommandShim || !errors.Is(result.symlinkErr, os.ErrPermission) {
		t.Fatalf("fallback result = %+v", result)
	}
	if filepath.Base(result.launcherPath) != "one.cmd" {
		t.Fatalf("launcher path = %q, want one.cmd", result.launcherPath)
	}
	body, err := os.ReadFile(result.launcherPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "\""+result.targetPath+"\" %*") {
		t.Fatalf("command shim does not forward arguments to target:\n%s", text)
	}
}

func TestInstallWindowsRefusesOrdinaryLegacyLauncher(t *testing.T) {
	homeDir := t.TempDir()
	targetPath := writeTestBinary(t, homeDir, "build/one.exe")
	legacyPath := filepath.Join(homeDir, ".local", "bin", "one")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte("user-managed"), 0o644); err != nil {
		t.Fatal(err)
	}
	installer := localInstaller{
		goos:        "windows",
		homeDir:     homeDir,
		makeSymlink: os.Symlink,
	}

	_, err := installer.install(targetPath)
	if err == nil || !strings.Contains(err.Error(), "not a symlink") {
		t.Fatalf("expected a safe refusal for an ordinary legacy launcher, got %v", err)
	}
	body, readErr := os.ReadFile(legacyPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != "user-managed" {
		t.Fatalf("legacy launcher was modified: %q", body)
	}
}

func TestWriteWindowsShimEscapesPercentCharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.cmd")
	if err := writeWindowsShim(path, "C:\\work%20\\one.exe"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "C:\\work%%20\\one.exe") {
		t.Fatalf("percent sign was not escaped:\n%s", body)
	}
}

func TestVerifyWindowsCommandShim(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cmd.exe verification is Windows-specific")
	}
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "fake one.cmd")
	if err := os.WriteFile(targetPath, []byte("@echo off\r\nexit /b 0\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcherPath := filepath.Join(dir, "one.cmd")
	if err := writeWindowsShim(launcherPath, targetPath); err != nil {
		t.Fatal(err)
	}
	if err := verifyLauncher(localInstallResult{
		launcherPath:    launcherPath,
		targetPath:      targetPath,
		usedCommandShim: true,
	}); err != nil {
		t.Fatalf("verify command shim: %v", err)
	}
}

func writeTestBinary(t *testing.T, homeDir, relativePath string) string {
	t.Helper()
	path := filepath.Join(homeDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
