package tasks

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type fixtureLoader struct {
	mu     sync.Mutex
	calls  map[string]int
	values map[string]map[string]string
}

func (l *fixtureLoader) ID() string { return "infisical" }
func (l *fixtureLoader) Load(_ context.Context, _ string, project, _ string) (map[string]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls[project]++
	return l.values[project], nil
}

type nativeProvider struct{ binary string }

func (p nativeProvider) PrepareCLI(_ context.Context, c runtimeport.Command) (runtimeport.Command, error) {
	c.Argv = append([]string{p.binary}, c.Argv...)
	return c, nil
}
func (p nativeProvider) Prepare(ctx context.Context, c runtimeport.Command) (runtimeport.Command, error) {
	c.Argv = append([]string{"exec", "--"}, c.Argv...)
	return p.PrepareCLI(ctx, c)
}
func syntheticLoader() *fixtureLoader {
	return &fixtureLoader{calls: map[string]int{}, values: map[string]map[string]string{
		"apps/web":     {"ONE_TEST_SHARED": "fake-web", "ONE_TEST_WEB": "web-only", "ONE_TEST_COMPLEX": "中文 ' \" $() `x` \\ \n\r\n", "ONE_TEST_EMPTY": ""},
		"packages/lib": {"ONE_TEST_SHARED": "fake-lib", "ONE_TEST_LIB": "lib-only", "ONE_TEST_COMPLEX": "中文 ' \" $() `x` \\ \n\r\n", "ONE_TEST_EMPTY": ""},
	}}
}
func hashValues(v map[string]string) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func TestNativeEnvironmentChild(t *testing.T) {
	if os.Getenv("ONE_NATIVE_CHILD") != "1" {
		return
	}
	if os.Getenv("ONE_NATIVE_ROOT") == "1" {
		fmt.Println("ROOT_RAN")
		return
	}
	values := map[string]string{}
	for _, entry := range os.Environ() {
		k, v, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(k, "ONE_TEST_") && k != "ONE_TEST_MISE_BINARY" {
			values[k] = v
		}
	}
	fmt.Println("ENV_HASH=" + hashValues(values))
	if os.Getenv("ONE_NATIVE_GRANDCHILD") != "1" {
		c := exec.Command(os.Args[0], "-test.run=^TestNativeEnvironmentChild$")
		c.Env = append(os.Environ(), "ONE_NATIVE_GRANDCHILD=1")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			os.Exit(17)
		}
	}
	// The requested raw-log contract: child values are not filtered by One.
	fmt.Println("CHILD_LOG=" + values["ONE_TEST_SHARED"])
	time.Sleep(40 * time.Millisecond)
}
func TestNativeMiseProjectEnvironmentsAndCache(t *testing.T) {
	for _, name := range []string{"direct_workspace", "symlinked_workspace"} {
		t.Run(name, func(t *testing.T) {
			testNativeMiseProjectEnvironmentsAndCache(t, name == "symlinked_workspace")
		})
	}
}

func testNativeMiseProjectEnvironmentsAndCache(t *testing.T, symlinked bool) {
	t.Helper()
	binary, err := exec.LookPath("mise")
	if err != nil {
		t.Skip("mise is not installed")
	}
	if value := os.Getenv("ONE_TEST_MISE_BINARY"); value != "" {
		binary = value
	}
	compose, err := exec.Command(binary, "which", "process-compose", "--tool", "process-compose@1.122.0").Output()
	if err != nil {
		t.Skipf("official pinned Process Compose is required: %v", err)
	}
	t.Setenv("PATH", filepath.Dir(strings.TrimSpace(string(compose)))+string(os.PathListSeparator)+os.Getenv("PATH"))
	w := taskWorkspace(t)
	if symlinked {
		// Reproduce macOS /var -> /private/var path aliases on other platforms.
		link := filepath.Join(t.TempDir(), "workspace")
		if err := os.Symlink(w.Root(), link); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			t.Fatal(err)
		}
		w, err = execution.ResolveWorkspaceScope(execution.NewScope(context.Background(), link))
		if err != nil {
			t.Fatal(err)
		}
		if w.Root() != link {
			t.Fatalf("workspace did not retain symlink path: %s", w.Root())
		}
	}
	w.Manifest().Env = &workspace.EnvironmentConfig{ProjectID: "synthetic", Environments: []string{"dev", "staging", "prod"}}
	isolated := t.TempDir()
	for _, key := range []string{"MISE_DATA_DIR", "MISE_STATE_DIR", "MISE_CACHE_DIR", "MISE_CONFIG_DIR", "MISE_SYSTEM_CONFIG_DIR"} {
		t.Setenv(key, filepath.Join(isolated, key))
	}
	t.Setenv("MISE_GLOBAL_CONFIG_FILE", filepath.Join(isolated, "global.toml"))
	t.Setenv("MISE_TRUSTED_CONFIG_PATHS", w.Root())
	t.Setenv("MISE_AUTO_INSTALL", "0")
	t.Setenv("MISE_EXPERIMENTAL", "0")
	t.Setenv("MISE_TASK_RUN_AUTO_INSTALL", "0")
	t.Setenv("MISE_TASK_CACHE_DIR", filepath.Join(isolated, "task-cache"))
	t.Setenv("ONE_NATIVE_CHILD", "1")
	loader := syntheticLoader()
	quote := func(s string) string {
		if runtime.GOOS == "windows" {
			return `"` + s + `"`
		}
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}
	config := map[string]any{"tasks": map[string]any{"build": map[string]any{"depends": []string{"web:build", "lib:build"}, "run": quote(os.Args[0]) + " -test.run=^TestNativeEnvironmentChild$", "env": map[string]string{"ONE_NATIVE_ROOT": "1"}, "sources": []string{"apps/web/package.json"}, "outputs": []string{"root-dist"}, "cache": map[string]any{"enabled": true}}, "web:build": map[string]any{"dir": "apps/web", "run": quote(os.Args[0]) + " -test.run=^TestNativeEnvironmentChild$", "sources": []string{"package.json"}, "outputs": []string{"dist"}, "cache": map[string]any{"enabled": true}}, "lib:build": map[string]any{"dir": "packages/lib", "run": quote(os.Args[0]) + " -test.run=^TestNativeEnvironmentChild$"}}}
	raw, _ := toml.Marshal(config)
	writeTaskFile(t, w.Root(), "mise.toml", string(raw))
	writeTaskFile(t, w.Root(), "apps/web/dist/existing", "old output")
	writeTaskFile(t, w.Root(), "root-dist/existing", "old output")
	// A local command override must retain both its command and the injected env.
	local, _ := toml.Marshal(map[string]any{"tasks": map[string]any{"web:build": map[string]any{"dir": "apps/web", "run": quote(os.Args[0]) + " -test.run=^TestNativeEnvironmentChild$", "sources": []string{"package.json"}, "outputs": []string{"dist"}, "cache": map[string]any{"enabled": true}}}})
	writeTaskFile(t, w.Root(), "mise.local.toml", string(local))
	var selectedLoader secrets.Loader = loader
	if symlinked {
		selectedLoader = &fixtureBatchLoader{fixtureLoader: loader}
	}
	service := Service{Provider: nativeProvider{binary}, Loaders: secrets.MustRegistry(selectedLoader), Prepare: func(context.Context, dependencies.Input) error { return nil }, WorkerCommand: testWorkerCommand}
	opts := Options{Name: "build", UI: "stream", Cache: "off", Jobs: 2}
	for round := 0; round < 3; round++ {
		if round == 1 {
			loader.values["apps/web"]["ONE_TEST_SHARED"] = "rotated"
		}
		if round == 2 {
			delete(loader.values["apps/web"], "ONE_TEST_EMPTY")
		}
		plan, err := service.Plan(context.Background(), w, opts)
		if err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		result, err := service.Execute(context.Background(), w, plan, opts, nil, &out, &errOut)
		if err != nil {
			t.Fatalf("%v\n%s\n%s", err, out.String(), errOut.String())
		}
		if result.Status != "succeeded" {
			t.Fatal(result)
		}
		log := out.String() + errOut.String()
		if !strings.Contains(log, "ROOT_RAN\n") {
			t.Fatalf("downstream freshness skipped: %s", log)
		}
		for _, values := range loader.values {
			if strings.Count(log, "ENV_HASH="+hashValues(values)) != 2 {
				t.Fatalf("environment or grandchild mismatch: %s", log)
			}
			if !strings.Contains(log, "CHILD_LOG="+values["ONE_TEST_SHARED"]) {
				t.Fatal("raw child log was changed")
			}
		}
		if loader.calls["apps/web"] != round+1 || loader.calls["packages/lib"] != round+1 {
			t.Fatal(loader.calls)
		}
	}
	after, _ := os.ReadFile(filepath.Join(w.Root(), "mise.toml"))
	if !bytes.Equal(raw, after) {
		t.Fatal("run changed mise config")
	}

	t.Run("simultaneous invocations keep independent snapshots", func(t *testing.T) {
		var wg sync.WaitGroup
		failures := make(chan error, 2)
		for _, label := range []string{"first", "second"} {
			wg.Go(func() {
				isolatedLoader := syntheticLoader()
				for _, values := range isolatedLoader.values {
					values["ONE_TEST_SHARED"] = label
				}
				independent := service
				independent.Loaders = secrets.MustRegistry(isolatedLoader)
				plan, err := independent.Plan(context.Background(), w, opts)
				if err != nil {
					failures <- err
					return
				}
				var out, stderr bytes.Buffer
				_, err = independent.Execute(context.Background(), w, plan, opts, nil, &out, &stderr)
				if err != nil {
					failures <- fmt.Errorf("%w: %s %s", err, out.String(), stderr.String())
					return
				}
				for _, values := range isolatedLoader.values {
					if strings.Count(out.String()+stderr.String(), "ENV_HASH="+hashValues(values)) != 2 {
						failures <- fmt.Errorf("cross-invocation values: %s %s", out.String(), stderr.String())
						return
					}
				}
			})
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			t.Error(err)
		}
	})
	t.Run("native task alias retains environment", func(t *testing.T) {
		local, _ := toml.Marshal(map[string]any{"tasks": map[string]any{"web:build": map[string]any{"alias": "compile", "dir": "apps/web", "run": quote(os.Args[0]) + " -test.run=^TestNativeEnvironmentChild$"}}})
		writeTaskFile(t, w.Root(), "mise.local.toml", string(local))
		aliasOpts := opts
		aliasOpts.Name = "compile"
		plan, err := service.Plan(context.Background(), w, aliasOpts)
		if err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		_, err = service.Execute(context.Background(), w, plan, aliasOpts, nil, &out, &stderr)
		if err != nil {
			t.Fatalf("%v: %s %s", err, out.String(), stderr.String())
		}
		if strings.Count(out.String()+stderr.String(), "ENV_HASH="+hashValues(loader.values["apps/web"])) != 2 {
			t.Fatalf("alias lost environment: %s %s", out.String(), stderr.String())
		}
	})
	t.Run("profile replacement retains the project snapshot", func(t *testing.T) {
		t.Setenv("MISE_ENV", "one-test")
		writeTaskFile(t, w.Root(), "mise.one-test.toml", string(local))
		t.Cleanup(func() { os.Remove(filepath.Join(w.Root(), "mise.one-test.toml")) })
		plan, err := service.Plan(context.Background(), w, opts)
		if err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		_, err = service.Execute(context.Background(), w, plan, opts, nil, &out, &stderr)
		if err != nil || !strings.Contains(out.String()+stderr.String(), "ENV_HASH="+hashValues(loader.values["apps/web"])) {
			t.Fatalf("profile lost environment: %v %s %s", err, out.String(), stderr.String())
		}
	})
	t.Run("native project scopes with identical task names", func(t *testing.T) {
		if err := os.Remove(filepath.Join(w.Root(), "mise.local.toml")); err != nil {
			t.Fatal(err)
		}
		writeTaskFile(t, w.Root(), "mise.toml", `monorepo_root=true
[monorepo]
config_roots=["apps/web","packages/lib"]
[tasks.build]
depends=["//apps/web:build","//packages/lib:build"]
`)
		for _, dir := range []string{"apps/web", "packages/lib"} {
			raw, _ := toml.Marshal(map[string]any{"tasks": map[string]any{"build": map[string]any{"run": quote(os.Args[0]) + " -test.run=^TestNativeEnvironmentChild$"}}})
			writeTaskFile(t, w.Root(), dir+"/mise.toml", string(raw))
		}
		plan, err := service.Plan(context.Background(), w, opts)
		if err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		_, err = service.Execute(context.Background(), w, plan, opts, nil, &out, &errOut)
		if err != nil {
			t.Fatalf("%v: %s %s", err, out.String(), errOut.String())
		}
		for _, values := range loader.values {
			if strings.Count(out.String()+errOut.String(), "ENV_HASH="+hashValues(values)) != 2 {
				t.Fatalf("scoped environment mismatch: %s %s", out.String(), errOut.String())
			}
		}
		// Project selection resolves the native scope, without inventing a root alias.
		selected := opts
		selected.Projects = []string{"web"}
		if plan, err = service.Plan(context.Background(), w, selected); err != nil || len(plan.Tasks) != 1 {
			t.Fatalf("%+v %v", plan, err)
		}
	})
	t.Run("file task keeps its native source and project environment", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX executable file task")
		}
		if err := os.Remove(filepath.Join(w.Root(), "apps/web/mise.toml")); err != nil {
			t.Fatal(err)
		}
		writeTaskFile(t, w.Root(), "apps/web/mise.toml", "[tasks.build]\ndescription='native file'\n")
		path := filepath.Join(w.Root(), "apps/web/.mise/tasks/build")
		writeTaskFile(t, w.Root(), "apps/web/.mise/tasks/build", "#!/bin/sh\nexec "+quote(os.Args[0])+" -test.run=^TestNativeEnvironmentChild$\n")
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatal(err)
		}
		selected := opts
		selected.Projects = []string{"web"}
		plan, err := service.Plan(context.Background(), w, selected)
		if err != nil {
			t.Fatal(err)
		}
		var out, stderr bytes.Buffer
		_, err = service.Execute(context.Background(), w, plan, selected, nil, &out, &stderr)
		if err != nil {
			t.Fatalf("%v: %s %s", err, out.String(), stderr.String())
		}
		if strings.Count(out.String()+stderr.String(), "ENV_HASH="+hashValues(loader.values["apps/web"])) != 2 {
			t.Fatalf("file task lost environment: %s %s", out.String(), stderr.String())
		}
	})

}
func TestEnvironmentSessionContainsNoValuesOnDiskAndCleansUp(t *testing.T) {
	w := taskWorkspace(t)
	w.Manifest().Env = &workspace.EnvironmentConfig{ProjectID: "synthetic", Environments: []string{"dev", "staging", "prod"}}
	plan, err := NewPlan(w, Options{Name: "build"})
	if err != nil {
		t.Fatal(err)
	}
	loader := syntheticLoader()
	service := Service{Loaders: secrets.MustRegistry(loader)}
	base := secrets.MergeIntoEnviron(os.Environ(), map[string]string{"MISE_PLUGINS_DIR": t.TempDir()}, true)
	session, err := service.prepareEnvironment(context.Background(), w, plan, base)
	if err != nil {
		t.Fatal(err)
	}
	defer session.close()
	config, err := composeConfig(plan, testWorkerCommand)
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range loader.values {
		for _, value := range values {
			if value != "" && bytes.Contains(config, []byte(value)) {
				t.Fatal("values written into config")
			}
		}
	}
	broker, err := newInvocationBroker(map[string]leafSpec{"task-0": {Task: plan.Tasks[0], Environment: session.env}})
	if err != nil {
		t.Fatal(err)
	}
	defer broker.close()
	endpoint := broker.endpoint + "/task-0"
	r, err := http.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatal("unauthenticated request accepted")
	}
	broker.close()
	client := http.Client{Timeout: time.Second}
	if r, err := client.Get(endpoint); err == nil {
		r.Body.Close()
		t.Fatal("broker remained open")
	}

}

// Load deliberately fails so a regression to single-project loading is visible.
type fixtureBatchLoader struct {
	*fixtureLoader
	batches int
	err     error
	omit    bool
}

func (l *fixtureBatchLoader) Load(context.Context, string, string, string) (map[string]string, error) {
	return nil, errors.New("single-project load called")
}
func (l *fixtureBatchLoader) LoadProjects(ctx context.Context, root string, dirs []string, env string) (map[string]map[string]string, error) {
	l.batches++
	if l.err != nil {
		return nil, l.err
	}
	values := map[string]map[string]string{}
	for _, dir := range dirs {
		if _, exists := values[dir]; exists {
			return nil, errors.New("duplicate batch directory")
		}
		v, err := l.fixtureLoader.Load(ctx, root, dir, env)
		if err != nil {
			return nil, err
		}
		if !l.omit {
			values[dir] = v
		}
	}
	return values, nil
}

func TestEnvironmentBatchFailureLeavesNoSession(t *testing.T) {
	for _, locale := range []string{"en-US", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			if err := i18n.Init(locale); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = i18n.Init("en-US") })
			for _, omit := range []bool{false, true} {
				w := taskWorkspace(t)
				w.Manifest().Env = &workspace.EnvironmentConfig{ProjectID: "fixture"}
				l := &fixtureBatchLoader{fixtureLoader: syntheticLoader(), omit: omit}
				if !omit {
					l.err = errors.New("synthetic permission denied")
				}
				service := Service{Loaders: secrets.MustRegistry(l)}
				before, _ := os.ReadDir(w.Root())
				session, err := service.prepareEnvironment(context.Background(), w, &Plan{Environment: "dev", Tasks: []Task{{Project: "web"}, {Project: "lib"}, {Project: "web"}}}, nil)
				if session != nil || err == nil || (!omit && !errors.Is(err, l.err)) {
					t.Fatalf("unexpected result: %v %v", session, err)
				}
				if omit && !strings.Contains(err.Error(), "web") && !strings.Contains(err.Error(), "lib") {
					t.Fatal("missing project context", err)
				}
				after, _ := os.ReadDir(w.Root())
				if !reflect.DeepEqual(before, after) || l.batches != 1 {
					t.Fatal("failed load created session files or repeated batch")
				}
			}
		})
	}
}

func testWorkerCommand(id string) []string {
	return []string{os.Args[0], "-test.run=^TestProcessLeafChild$", "--", id}
}
func TestProcessLeafChild(t *testing.T) {
	if os.Getenv("ONE_PROCESS_ENDPOINT") == "" {
		return
	}
	err := RunProcessLeaf(context.Background(), os.Args[len(os.Args)-1], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(process.ExitCode(err))
	}
	os.Exit(0)
}
