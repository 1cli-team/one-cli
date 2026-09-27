package dependencies

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

func TestPNPMDevelopmentReusesManualInstallAndUpdatesNewProjects(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native pnpm executable test")
	}
	pnpm, err := exec.LookPath("pnpm")
	if err != nil {
		t.Skip("pnpm is not installed")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("CI", "true")
	versionCmd := exec.Command(pnpm, "--version")
	versionCmd.Dir = root
	version, err := versionCmd.Output()
	if err != nil {
		t.Fatalf("pnpm --version: %v", err)
	}
	if !supportsPNPMDependencyCheck(string(version)) {
		t.Skipf("pnpm %s lacks dependency verification", version)
	}
	write(t, root, "package.json", `{"name":"workspace","private":true}`)
	write(t, root, "pnpm-workspace.yaml", "packages:\n  - apps/*\n  - packages/*\n  - apps/web/apps/ui\n")
	write(t, root, "packages/shared/package.json", `{"name":"shared","version":"1.0.0"}`)
	write(t, root, "apps/web/package.json", `{"name":"web","workspaces":["apps/ui"]}`)
	write(t, root, "apps/web/apps/ui/package.json", `{"name":"@web/ui","dependencies":{"shared":"workspace:*"}}`)
	manual := exec.Command(pnpm, "install", "--no-frozen-lockfile", "--offline")
	manual.Dir = root
	if data, err := manual.CombinedOutput(); err != nil {
		t.Fatalf("manual install: %v\n%s", err, data)
	}
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{
		project("web", "apps/web", "node"), project("shared", "packages/shared", "node"),
	}}, Runtime: runtimeport.Builtin, Development: true}
	installs := 0
	s := Service{Run: func(ctx context.Context, c runtimeport.Command, out, errOut io.Writer) error {
		if c.Argv[0] == "pnpm" {
			c.Argv[0] = pnpm
			if c.Argv[1] == "install" {
				installs++
				c.Argv = append(c.Argv, "--offline") // The fixture uses only workspace dependencies.
			}
		}
		return runCommand(ctx, c, out, errOut)
	}}
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if installs != 0 {
		t.Fatal("reinstalled after a successful manual pnpm install")
	}
	write(t, root, "apps/ccc/package.json", `{"name":"ccc","dependencies":{"shared":"workspace:*"}}`)
	in.Manifest.Projects = append(in.Manifest.Projects, project("ccc", "apps/ccc", "node"))
	before, _ := os.ReadFile(filepath.Join(root, "pnpm-lock.yaml"))
	strict := in
	strict.Development = false
	if err := s.Prepare(context.Background(), strict); err == nil {
		t.Fatal("strict preparation accepted an outdated lockfile")
	}
	after, _ := os.ReadFile(filepath.Join(root, "pnpm-lock.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("strict preparation rewrote the lockfile")
	}
	installs = 0
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatalf("new project: %v", err)
	}
	after, _ = os.ReadFile(filepath.Join(root, "pnpm-lock.yaml"))
	if !strings.Contains(string(after), "apps/ccc:") {
		t.Fatalf("new project missing from lockfile: %s", after)
	}
	if !nodeInstalled(in) {
		t.Fatal("new project's dependency was not linked")
	}
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if installs != 1 {
		t.Fatalf("expected one install after adding a project, got %d", installs)
	}
	// The native check must notice a manifest edit even if links still exist.
	write(t, root, "apps/ccc/package.json", `{"name":"ccc","dependencies":{"shared":"workspace:^"}}`)
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if installs != 2 {
		t.Fatal("a changed dependency specifier was ignored")
	}
	// No manifest entry is added for the UI package. Both native validation
	// and the strict-build fingerprint must still notice edits inside it.
	if err := s.Prepare(context.Background(), strict); err != nil {
		t.Fatal(err)
	}
	if err := s.Prepare(context.Background(), strict); err != nil {
		t.Fatal(err)
	}
	before, _ = os.ReadFile(filepath.Join(root, "pnpm-lock.yaml"))
	write(t, root, "apps/web/apps/ui/package.json", `{"name":"@web/ui","dependencies":{"shared":"workspace:^"}}`)
	if err := s.Prepare(context.Background(), strict); err == nil {
		t.Fatal("cached build ignored an internal manifest edit")
	}
	after, _ = os.ReadFile(filepath.Join(root, "pnpm-lock.yaml"))
	if !bytes.Equal(before, after) {
		t.Fatal("strict build changed the lockfile")
	}
	installs = 0
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if installs != 1 {
		t.Fatal("development ignored an internal manifest edit")
	}
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if installs != 1 {
		t.Fatal("reinstalled an unchanged composite project")
	}
}

func TestNodeInstallFailureIsOnlyPrintedOnce(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		_ = i18n.Init(locale)
		root := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		write(t, root, "package.json", `{"packageManager":"pnpm@12.3.4"}`)
		write(t, root, "apps/web/package.json", `{"dependencies":{"missing":"1.0.0"}}`)
		diagnostic := "ERR_PNPM_FETCH_500: registry unavailable\n"
		calls := 0
		service := Service{Run: func(_ context.Context, c runtimeport.Command, out, errOut io.Writer) error {
			if c.Argv[1] == "--version" {
				_, _ = io.WriteString(out, "12.3.4\n")
				return nil
			}
			calls++
			if strings.Join(c.Argv, " ") != "pnpm install --no-frozen-lockfile" {
				t.Fatalf("unexpected install: %v", c.Argv)
			}
			_, _ = io.WriteString(errOut, diagnostic)
			return errors.New("exit status 1")
		}}
		var log bytes.Buffer
		in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("web", "apps/web", "node")}}, Development: true, Log: &log}
		err := service.Prepare(context.Background(), in)
		failure, ok := err.(*output.Error)
		if !ok {
			t.Fatalf("expected structured error, got %v", err)
		}
		if strings.Count(log.String()+err.Error(), strings.TrimSpace(diagnostic)) != 1 {
			t.Fatalf("duplicated diagnostic: %s\n%v", log.String(), err)
		}
		if failure.Context["stderr"] != diagnostic || calls != 1 {
			t.Fatalf("lost diagnostic or retried failed install: %+v, calls=%d", failure.Context, calls)
		}
		in.Log = nil
		err = service.Prepare(context.Background(), in)
		if err == nil || !strings.Contains(err.Error(), strings.TrimSpace(diagnostic)) {
			t.Fatalf("discarded diagnostic when logs were disabled: %v", err)
		}
	}
}

func TestPNPMValidationCancellationDoesNotStartInstall(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"packageManager":"pnpm@12.3.4"}`)
	write(t, root, "apps/web/package.json", `{}`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := Service{Run: func(_ context.Context, c runtimeport.Command, out, _ io.Writer) error {
		if c.Argv[1] == "--version" {
			_, _ = io.WriteString(out, "12.3.4\n")
			return nil
		}
		if c.Argv[1] != "--config.verify-deps-before-run=error" {
			t.Fatalf("installation started after cancellation: %v", c.Argv)
		}
		cancel()
		return ctx.Err()
	}}
	err := s.Prepare(ctx, Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("web", "apps/web", "node")}}, Development: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
