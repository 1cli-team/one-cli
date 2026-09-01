package cli_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallPS1_InstallsVerifiedLocalArchive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("install.ps1 executes only on Windows")
	}
	powerShell, err := exec.LookPath("pwsh")
	if err != nil {
		powerShell, err = exec.LookPath("powershell.exe")
	}
	if err != nil {
		t.Skipf("PowerShell not on PATH: %v", err)
	}

	binary, err := os.ReadFile(binaryPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	entry, err := zipWriter.Create("one.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive.Bytes())
	checksums := fmt.Sprintf("%x  one-cli_windows_amd64.zip\n", sum)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v9.9.9/one-cli_windows_amd64.zip":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive.Bytes())
		case "/v9.9.9/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	installDir := filepath.Join(t.TempDir(), "One CLI", "bin")
	cmd := exec.Command(powerShell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", installPowerShellPath(t))
	cmd.Env = append(os.Environ(),
		"ONE_VERSION=v9.9.9",
		"ONE_RELEASE_BASE_URL="+server.URL,
		"ONE_INSTALL_DIR="+installDir,
		"ONE_FORCE=1",
		"ONE_SKIP_VERIFY=0",
		"ONE_NO_PATH_UPDATE=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.ps1 local install failed:\n%s\n%v", output, err)
	}
	installed, err := os.ReadFile(filepath.Join(installDir, "one.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, binary) {
		t.Fatal("installed one.exe differs from the verified archive payload")
	}
}

func installPowerShellPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "..", "..", "apps", "docs", "public", "install.ps1")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("install.ps1 missing (%v) — skip if running outside the main repo", err)
	}
	return path
}

func TestInstallPS1_SyntaxValid(t *testing.T) {
	executable, err := exec.LookPath("pwsh")
	if err != nil && runtime.GOOS == "windows" {
		executable, err = exec.LookPath("powershell.exe")
	}
	if err != nil {
		t.Skipf("PowerShell not on PATH: %v", err)
	}

	script := installPowerShellPath(t)
	parseOnly := "$tokens = $null; $errors = $null; " +
		"[System.Management.Automation.Language.Parser]::ParseFile($env:ONE_INSTALLER_TEST_PATH, [ref]$tokens, [ref]$errors) | Out-Null; " +
		"if ($errors.Count -gt 0) { $errors | ForEach-Object { Write-Error $_.Message }; exit 1 }"
	cmd := exec.Command(executable, "-NoProfile", "-NonInteractive", "-Command", parseOnly)
	cmd.Env = append(os.Environ(), "ONE_INSTALLER_TEST_PATH="+script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.ps1 syntax check failed:\n%s\n%v", output, err)
	}
}

func TestInstallPS1_RequiredSentinels(t *testing.T) {
	body, err := os.ReadFile(installPowerShellPath(t))
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		"one-cli_windows_amd64.zip",
		"checksums.txt",
		"Get-FileHash",
		"Expand-Archive",
		"ONE_VERSION",
		"ONE_INSTALL_DIR",
		"ONE_FORCE",
		"ONE_SKIP_VERIFY",
		"ONE_NO_PATH_UPDATE",
		"SetEnvironmentVariable(\"Path\"",
		"[IO.File]::Replace",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("install.ps1 is missing required sentinel %q", want)
		}
	}
}
