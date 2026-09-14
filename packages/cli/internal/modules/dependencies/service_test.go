package dependencies

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

func write(t *testing.T, root, path, content string) {
	t.Helper()
	file := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func project(name, dir, language string) workspace.ManifestProject {
	return workspace.ManifestProject{Name: name, RelativeDir: dir, Toolchain: language, Domains: &workspace.ProjectDomains{Dev: &workspace.ProjectDevOverride{Command: "unused-in-preparation"}}}
}

func setupGo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go not installed")
	}
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "")
	t.Setenv("GOFLAGS", "-modcacherw")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOMODCACHE", filepath.Join(t.TempDir(), "modules"))
	return t.TempDir()
}

func TestGoMissingSumsAndEmptyCachePreparedWithoutChangingModule(t *testing.T) {
	root := setupGo(t)
	proxy := filepath.Join(t.TempDir(), "proxy")
	mod := "module example.com/dependency\ngo 1.23.0\n"
	write(t, proxy, "example.com/dependency/@v/v1.0.0.mod", mod)
	write(t, proxy, "example.com/dependency/@v/v1.0.0.info", `{"Version":"v1.0.0","Time":"2024-01-01T00:00:00Z"}`)
	write(t, proxy, "example.com/dependency/@v/list", "v1.0.0\n")
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, content := range map[string]string{"go.mod": mod, "dependency.go": "package dependency\nconst Value = 42\n"} {
		w, err := z.Create("example.com/dependency@v1.0.0/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	write(t, proxy, "example.com/dependency/@v/v1.0.0.zip", buf.String())
	urlPath := filepath.ToSlash(proxy)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	t.Setenv("GOPROXY", "file://"+urlPath)
	module := "module example.com/api\ngo 1.23.0\nrequire example.com/dependency v1.0.0\n"
	write(t, root, "services/api/go.mod", module)
	write(t, root, "services/api/main.go", "package main\nimport (\"fmt\"; \"example.com/dependency\")\nfunc main(){fmt.Println(dependency.Value)}\n")
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("api", "services/api", "go")}}, Runtime: runtimeport.Builtin}
	for i := 0; i < 3; i++ {
		if i == 2 {
			if err := os.RemoveAll(os.Getenv("GOMODCACHE")); err != nil {
				t.Fatal(err)
			}
		}
		if err := (Service{}).Prepare(context.Background(), in); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(filepath.Join(root, "services/api/go.mod"))
		if string(b) != module {
			t.Fatalf("go.mod changed: %s", b)
		}
		sum, _ := os.ReadFile(filepath.Join(root, "services/api/go.sum"))
		if !strings.Contains(string(sum), "example.com/dependency v1.0.0 h1:") {
			t.Fatalf("missing content checksum: %s", sum)
		}
	}
}

func TestGoWorkspaceUsesLocalLibraryWithNoRemoteVersion(t *testing.T) {
	root := setupGo(t)
	write(t, root, "go.work", "go 1.25.0\nuse (\n ./services/api\n \"./packages/shared lib\"\n)\n")
	write(t, root, "services/api/go.mod", "module example.com/api\ngo 1.25.0\nrequire example.com/shared v0.0.0\n")
	write(t, root, "services/api/main.go", "package main\nimport (\"fmt\"; \"example.com/shared\")\nfunc main(){fmt.Println(shared.Value)}\n")
	write(t, root, "packages/shared lib/go.mod", "module example.com/shared\ngo 1.25.0\n")
	write(t, root, "packages/shared lib/shared.go", "package shared\nconst Value = 42\n")
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("api", "services/api", "go")}}}
	if err := (Service{}).Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	write(t, root, "packages/shared lib/shared.go", "package shared\nconst Value = 99\n")
	var out bytes.Buffer
	if err := (Service{}).run(context.Background(), in, filepath.Join(root, "services/api"), []string{"go", "run", "-buildvcs=false", "."}, os.Environ(), &out, &out); err != nil {
		t.Fatalf("%v: %s", err, &out)
	}
	if strings.TrimSpace(out.String()) != "99" {
		t.Fatalf("not using local library: %s", &out)
	}
}

func TestGoUnresolvedImportFailsWithoutTidy(t *testing.T) {
	root := setupGo(t)
	module := "module example.com/api\ngo 1.25.0\n"
	write(t, root, "api/go.mod", module)
	write(t, root, "api/main.go", "package main\nimport _ \"example.invalid/missing\"\nfunc main(){}\n")
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("api", "api", "go")}}}
	err := (Service{}).Prepare(context.Background(), in)
	if err == nil || !strings.Contains(err.Error(), "example.invalid/missing") {
		t.Fatalf("error = %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "api/go.mod"))
	if string(b) != module {
		t.Fatal("implicit tidy changed go.mod")
	}
}

func TestGoWorkspaceAcceptsSymlinkPaths(t *testing.T) {
	for _, mode := range []string{"workspace-root", "gowork", "gowork-parent", "pwd-alias"} {
		t.Run(mode, func(t *testing.T) {
			root := setupGo(t)
			write(t, root, "go.work", "go 1.25.0\nuse ./api\n")
			write(t, root, "api/go.mod", "module example.com/api\ngo 1.25.0\n")
			write(t, root, "api/main.go", "package main\nfunc main(){}\n")
			physicalRoot, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			linkTarget := filepath.Join(physicalRoot, "go.work")
			alias := filepath.Join(physicalRoot, "alias.work")
			if mode != "gowork" {
				linkTarget = physicalRoot
				alias = filepath.Join(t.TempDir(), "workspace-link")
			}
			if err := os.Symlink(linkTarget, alias); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlinks unavailable: %v", err)
				}
				t.Fatal(err)
			}
			active := alias
			switch mode {
			case "workspace-root":
				root, active = alias, filepath.Join(physicalRoot, "go.work")
			case "gowork-parent":
				active = filepath.Join(alias, "go.work")
			case "pwd-alias":
				active = filepath.Join(physicalRoot, "go.work")
				t.Setenv("PWD", filepath.Join(alias, "api"))
			}
			t.Setenv("GOWORK", active)
			in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("api", "api", "go")}}}
			if err := (Service{}).Prepare(context.Background(), in); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGoWorkspaceRejectsExternalFileWithSameContents(t *testing.T) {
	root := setupGo(t)
	work := "go 1.25.0\n"
	write(t, root, "go.work", work)
	write(t, root, "api/go.mod", "module example.com/api\ngo 1.25.0\n")
	external := t.TempDir()
	write(t, external, "go.work", work)
	active := filepath.Join(external, "go.work")
	t.Setenv("GOWORK", active)
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("api", "api", "go")}}}
	if err := (Service{}).Prepare(context.Background(), in); err == nil || !strings.Contains(err.Error(), "uses external GOWORK="+active) {
		t.Fatalf("expected external workspace rejection, got %v", err)
	}
}

type fakeProvider struct{ dir string }

func (p fakeProvider) Prepare(_ context.Context, c runtimeport.Command) (runtimeport.Command, error) {
	c.Argv = append([]string{"wrapped"}, c.Argv...)
	c.Directory = p.dir
	c.Env = append(c.Env, "PREPARED_ENV=present")
	return c, nil
}
func (p fakeProvider) PrepareCLI(ctx context.Context, c runtimeport.Command) (runtimeport.Command, error) {
	return p.Prepare(ctx, c)
}

func TestRuntimePreparedDirectoryArgumentsAndEnvironmentAreForwarded(t *testing.T) {
	dir := t.TempDir()
	s := Service{Provider: fakeProvider{dir}, Run: func(_ context.Context, c runtimeport.Command, _, _ io.Writer) error {
		if c.Directory != dir || strings.Join(c.Argv, " ") != "wrapped go version" || !strings.Contains(strings.Join(c.Env, "\n"), "PREPARED_ENV=present") {
			t.Fatalf("prepared command lost: %+v", c)
		}
		return nil
	}}
	if err := s.run(context.Background(), Input{Runtime: runtimeport.Mise}, "before", []string{"go", "version"}, nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestNodeRootInstallDeduplicatedAndInvalidated(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	write(t, root, "package.json", `{"packageManager":"pnpm@12.3.4"}`)
	write(t, root, "apps/a/package.json", `{"dependencies":{"dependency":"1.0.0"}}`)
	write(t, root, "apps/b/package.json", `{}`)
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("a", "apps/a", "node"), project("b", "apps/b", "node")}}}
	installs := 0
	s := Service{Run: func(_ context.Context, c runtimeport.Command, out, _ io.Writer) error {
		if c.Directory != root {
			t.Fatalf("install outside root: %s", c.Directory)
		}
		if c.Argv[1] == "--version" {
			fmt.Fprintln(out, "12.3.4")
			return nil
		}
		installs++
		if err := os.MkdirAll(filepath.Join(root, "node_modules/dependency"), 0o755); err != nil {
			return err
		}
		return nil
	}}
	for i := 0; i < 2; i++ {
		if err := s.Prepare(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	if installs != 1 {
		t.Fatalf("installs=%d", installs)
	}
	write(t, root, "apps/b/package.json", `{"scripts":{"dev":"updated"}}`)
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if installs != 2 {
		t.Fatal("package change ignored")
	}
	if err := os.RemoveAll(filepath.Join(root, "node_modules")); err != nil {
		t.Fatal(err)
	}
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if installs != 3 {
		t.Fatal("cleared dependencies ignored")
	}
}

func TestCancellationAndFailedPreparationPermitRetry(t *testing.T) {
	root := t.TempDir()
	write(t, root, "api/go.mod", "module example.com/api\ngo 1.25.0\n")
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("api", "api", "go")}}}
	started := make(chan struct{})
	var once sync.Once
	s := Service{Run: func(ctx context.Context, c runtimeport.Command, out, _ io.Writer) error {
		if c.Argv[1] == "env" {
			return nil
		}
		once.Do(func() { close(started) })
		<-ctx.Done()
		return ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Prepare(ctx, in) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("preparation did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel blocked")
	}
	s.Run = func(context.Context, runtimeport.Command, io.Writer, io.Writer) error { return nil }
	if err := s.Prepare(context.Background(), in); err != nil {
		t.Fatalf("retry could not acquire lock: %v", err)
	}
}

func TestNodeLockfileMismatchUsesFrozenInstallAndStops(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	write(t, root, "package.json", `{"packageManager":"pnpm@12.3.4"}`)
	write(t, root, "pnpm-lock.yaml", "existing lock\n")
	in := Input{Root: root, Manifest: &workspace.Manifest{Projects: []workspace.ManifestProject{project("web", "apps/web", "node"), project("api", "services/api", "go")}}}
	s := Service{Run: func(_ context.Context, c runtimeport.Command, out, _ io.Writer) error {
		if c.Argv[0] == "go" {
			t.Fatal("continued after failed install")
		}
		if c.Argv[1] == "--version" {
			fmt.Fprintln(out, "12.3.4")
			return nil
		}
		if strings.Join(c.Argv, " ") != "pnpm install --frozen-lockfile" {
			t.Fatalf("command=%v", c.Argv)
		}
		return errors.New("lockfile mismatch")
	}}
	if err := s.Prepare(context.Background(), in); err == nil {
		t.Fatal("expected install failure")
	}
}

func TestPNPMEnvironmentDocumentDoesNotPretendDependenciesAreLocked(t *testing.T) {
	root := t.TempDir()
	environment := "---\nlockfileVersion: '9.0'\nimporters:\n  .:\n    configDependencies: {}\n    packageManagerDependencies:\n      pnpm: {specifier: 12.3.4, version: 12.3.4}\n"
	graph := "---\nlockfileVersion: '9.0'\nimporters:\n  .: {}\n  apps/web:\n    dependencies:\n      react: {specifier: '19', version: '19.3.0'}\n"
	for _, fixture := range []struct{ body, flag string }{
		{environment, "--no-frozen-lockfile"},
		{environment + "\n---\n", "--no-frozen-lockfile"},
		{environment + graph, "--frozen-lockfile"},
		{graph + environment, "--frozen-lockfile"},
		{"not valid: [", "--frozen-lockfile"},
	} {
		write(t, root, "pnpm-lock.yaml", fixture.body)
		args := NodeInstallCommand(root, "pnpm")
		if args[len(args)-1] != fixture.flag {
			t.Fatalf("lock=%s args=%v", fixture.body, args)
		}
	}
}
