package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func seedInfisicalProfiles(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := Upsert(DomainEnv, "infisical", name, Profile{
			Backend: "infisical",
			Infisical: &InfisicalProfile{
				SiteURL:     "https://app.infisical.com",
				Credentials: &InfisicalCredentials{ClientID: "id-" + name, ClientSecret: "secret-" + name},
			},
		}, false); err != nil {
			t.Fatalf("seed profile %q: %v", name, err)
		}
	}
}

func resolveEnvironmentProfile(
	t *testing.T, root, project, environment string,
) *Resolved {
	t.Helper()
	resolved, err := Resolve(ResolveInput{
		Domain:        DomainEnv,
		Backend:       "infisical",
		WorkspaceID:   "same-shared-id",
		WorkspaceRoot: root,
		ProjectName:   project,
		Environment:   environment,
	})
	if err != nil {
		t.Fatalf("resolve %s/%s: %v", environment, project, err)
	}
	return resolved
}

func TestEnvironmentBindingsKeepThreeEnvironmentsIndependent(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "default", "development", "previewing", "production")

	for environment, name := range map[string]string{
		"dev": "development", "preview": "previewing", "prod": "production",
	} {
		if err := BindEnvironmentProfile(
			"same-shared-id", "demo", root, "", environment,
			DomainEnv, "infisical", name,
		); err != nil {
			t.Fatalf("bind %s: %v", environment, err)
		}
	}

	for environment, want := range map[string]string{
		"dev": "development", "preview": "previewing", "prod": "production",
	} {
		resolved := resolveEnvironmentProfile(t, root, "", environment)
		if resolved.Name != want || resolved.Source != "workspace-environment" {
			t.Fatalf("resolve %s = %q (%s), want %q (workspace-environment)",
				environment, resolved.Name, resolved.Source, want)
		}
	}
}

func TestEnvironmentBindingPrecedenceAndLegacyFallback(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "default", "legacy-workspace", "legacy-project", "environment-workspace", "environment-project", "flag")
	if err := BindWorkspaceProfile(
		"same-shared-id", "demo", root, "", DomainEnv, "infisical", "legacy-workspace",
	); err != nil {
		t.Fatal(err)
	}
	if err := BindWorkspaceProfile(
		"same-shared-id", "demo", root, "web", DomainEnv, "infisical", "legacy-project",
	); err != nil {
		t.Fatal(err)
	}
	if err := BindEnvironmentProfile(
		"same-shared-id", "demo", root, "", "dev", DomainEnv, "infisical", "environment-workspace",
	); err != nil {
		t.Fatal(err)
	}

	resolved := resolveEnvironmentProfile(t, root, "web", "dev")
	if resolved.Name != "environment-workspace" || resolved.Source != "workspace-environment" {
		t.Fatalf("environment workspace did not beat legacy project: %#v", resolved)
	}
	if err := BindEnvironmentProfile(
		"same-shared-id", "demo", root, "web", "dev", DomainEnv, "infisical", "environment-project",
	); err != nil {
		t.Fatal(err)
	}
	resolved = resolveEnvironmentProfile(t, root, "web", "dev")
	if resolved.Name != "environment-project" || resolved.Source != "workspace-project-environment" {
		t.Fatalf("environment project did not win: %#v", resolved)
	}
	flagged, err := Resolve(ResolveInput{
		Domain: DomainEnv, Backend: "infisical", FlagOverride: "flag",
		WorkspaceID: "same-shared-id", WorkspaceRoot: root, ProjectName: "web", Environment: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flagged.Name != "flag" || flagged.Source != "flag" {
		t.Fatalf("flag did not win: %#v", flagged)
	}

	if err := UnbindEnvironmentProfile(root, "web", "dev", DomainEnv, "infisical"); err != nil {
		t.Fatal(err)
	}
	if err := UnbindEnvironmentProfile(root, "", "dev", DomainEnv, "infisical"); err != nil {
		t.Fatal(err)
	}
	resolved = resolveEnvironmentProfile(t, root, "web", "dev")
	if resolved.Name != "legacy-project" || resolved.Source != "workspace-project" {
		t.Fatalf("legacy project fallback lost: %#v", resolved)
	}
}

func TestEnvironmentBindingsUseCanonicalRootInsteadOfSharedWorkspaceID(t *testing.T) {
	withIsolatedConfig(t)
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	seedInfisicalProfiles(t, "default", "first-copy", "second-copy")

	for _, binding := range []struct{ root, name string }{
		{firstRoot, "first-copy"}, {secondRoot, "second-copy"},
	} {
		if err := BindEnvironmentProfile(
			"same-shared-id", "demo", binding.root, "", "preview",
			DomainEnv, "infisical", binding.name,
		); err != nil {
			t.Fatal(err)
		}
	}
	if got := resolveEnvironmentProfile(t, firstRoot, "", "preview").Name; got != "first-copy" {
		t.Fatalf("first checkout resolved %q", got)
	}
	if got := resolveEnvironmentProfile(t, secondRoot, "", "preview").Name; got != "second-copy" {
		t.Fatalf("second checkout resolved %q", got)
	}
}

func TestEnvironmentBindingUnbindPrunesEmptyHierarchy(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "default", "workspace", "project")
	if err := BindEnvironmentProfile(
		"workspace-id", "demo", root, "", "prod", DomainEnv, "infisical", "workspace",
	); err != nil {
		t.Fatal(err)
	}
	if err := BindEnvironmentProfile(
		"workspace-id", "demo", root, "web", "prod", DomainEnv, "infisical", "project",
	); err != nil {
		t.Fatal(err)
	}
	if err := UnbindEnvironmentProfile(root, "web", "prod", DomainEnv, "infisical"); err != nil {
		t.Fatal(err)
	}
	if got := resolveEnvironmentProfile(t, root, "web", "prod"); got.Name != "workspace" {
		t.Fatalf("project unbind did not fall back to workspace: %#v", got)
	}
	if err := UnbindEnvironmentProfile(root, "", "prod", DomainEnv, "infisical"); err != nil {
		t.Fatal(err)
	}

	path, err := BindingsPath()
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := loadBindingsAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if bindings.Version != bindingsSchemaVersion || len(bindings.Workspaces) != 0 {
		t.Fatalf("empty hierarchy was not pruned: %#v", bindings)
	}
	// Repeating the operation remains a no-op.
	if err := UnbindEnvironmentProfile(root, "", "prod", DomainEnv, "infisical"); err != nil {
		t.Fatalf("idempotent unbind: %v", err)
	}
}

func TestEnvironmentProfileBindingReadsStaleDirectSelectionUntilExplicitUnbind(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "stale", "fallback")
	if err := SetDefault(DomainEnv, "infisical", "fallback"); err != nil {
		t.Fatal(err)
	}
	if err := BindEnvironmentProfile(
		"workspace-id", "demo", root, "web", "preview",
		DomainEnv, "infisical", "stale",
	); err != nil {
		t.Fatal(err)
	}

	// Simulate an existing stale store produced by an older client or a manual
	// config edit. The current Remove path has a separate cleanup test below.
	cfg, _, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	delete(cfg.EnvInfisical.Profiles, "stale")
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	got, err := EnvironmentProfileBinding(
		root, "web", "preview", DomainEnv, "infisical",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "stale" {
		t.Fatalf("raw direct binding = %q, want stale", got)
	}
	if _, err := Resolve(ResolveInput{
		Domain: DomainEnv, Backend: "infisical", WorkspaceRoot: root,
		ProjectName: "web", Environment: "preview",
	}); err == nil {
		t.Fatal("stale direct binding unexpectedly resolved as a usable Profile")
	}

	if err := UnbindEnvironmentProfile(
		root, "web", "preview", DomainEnv, "infisical",
	); err != nil {
		t.Fatal(err)
	}
	got, err = EnvironmentProfileBinding(
		root, "web", "preview", DomainEnv, "infisical",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("raw direct binding after unbind = %q", got)
	}
	resolved := resolveEnvironmentProfile(t, root, "web", "preview")
	if resolved.Name != "fallback" || resolved.Source != "default" {
		t.Fatalf("unbind did not restore fallback: %#v", resolved)
	}
}

func TestEnvironmentBindRevalidatesProfileAfterWaitingForStoreLock(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "removed-before-commit")
	bindingsPath, err := BindingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(bindingsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	externalLock := flock.New(bindingsPath + ".lock")
	if err := externalLock.Lock(); err != nil {
		t.Fatal(err)
	}

	bindResult := make(chan error, 1)
	go func() {
		bindResult <- BindEnvironmentProfile(
			"workspace-id", "demo", root, "web", "preview",
			DomainEnv, "infisical", "removed-before-commit",
		)
	}()

	// Remove the definition while the binding publication is unable to acquire
	// its file lock. Once unblocked, Bind must reload config and reject instead
	// of publishing a stale name based on an earlier validation.
	cfg, _, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	delete(cfg.EnvInfisical.Profiles, "removed-before-commit")
	cfg.EnvInfisical.Default = ""
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := externalLock.Unlock(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-bindResult:
		var coded interface{ ErrorCode() string }
		if !errors.As(err, &coded) || coded.ErrorCode() != "PROFILE_NOT_FOUND" {
			t.Fatalf("bind error = %v, want PROFILE_NOT_FOUND", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("binding did not finish after the file lock was released")
	}
	got, err := EnvironmentProfileBinding(
		root, "web", "preview", DomainEnv, "infisical",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("queued binding published stale Profile %q", got)
	}
}

func TestEnvironmentBindingWritesOnlyPrivateMachineLocalFile(t *testing.T) {
	configHome := withIsolatedConfig(t)
	root := t.TempDir()
	manifestPath := filepath.Join(root, "one.manifest.json")
	manifest := []byte(`{"schema":"one-cli/manifest/v1","workspace":{"id":"workspace-id"}}`)
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	seedInfisicalProfiles(t, "default", "selected")
	configPath, credentialsPath := cfgPaths(configHome)
	configBefore, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	credentialsBefore, err := os.ReadFile(credentialsPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := BindEnvironmentProfile(
		"workspace-id", "demo", root, "", "dev", DomainEnv, "infisical", "selected",
	); err != nil {
		t.Fatal(err)
	}
	manifestAfter, _ := os.ReadFile(manifestPath)
	configAfter, _ := os.ReadFile(configPath)
	credentialsAfter, _ := os.ReadFile(credentialsPath)
	if string(manifestAfter) != string(manifest) {
		t.Fatal("binding mutated one.manifest.json")
	}
	if string(configAfter) != string(configBefore) {
		t.Fatal("binding mutated config.json")
	}
	if string(credentialsAfter) != string(credentialsBefore) {
		t.Fatal("binding mutated credentials.json")
	}

	path, err := BindingsPath()
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(configHome, "one", "profile-bindings.json")
	if path != wantPath {
		t.Fatalf("bindings path = %q, want %q", path, wantPath)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("bindings mode = %04o, want 0600", info.Mode().Perm())
	}
	var disk map[string]any
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatalf("invalid bindings JSON: %v", err)
	}
	if disk["version"] != float64(1) {
		t.Fatalf("bindings version = %#v", disk["version"])
	}
	if _, err := os.Stat(filepath.Join(root, "profile-bindings.json")); !os.IsNotExist(err) {
		t.Fatalf("binding file was written inside workspace: %v", err)
	}
}

func TestEnvironmentBindingValidationRejectsAmbiguousKeys(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "default", "selected")
	tests := []struct {
		name        string
		root        string
		environment string
		profile     string
	}{
		{name: "missing root", root: filepath.Join(root, "missing"), environment: "dev", profile: "selected"},
		{name: "padded root", root: " " + root, environment: "dev", profile: "selected"},
		{name: "padded environment", root: root, environment: " dev", profile: "selected"},
		{name: "path-like environment", root: root, environment: "team/dev", profile: "selected"},
		{name: "leading dash environment", root: root, environment: "-dev", profile: "selected"},
		{name: "empty profile", root: root, environment: "dev", profile: ""},
		{name: "path-like profile", root: root, environment: "dev", profile: "team/selected"},
		{name: "spaced profile", root: root, environment: "dev", profile: "team selected"},
		{name: "leading dash profile", root: root, environment: "dev", profile: "-selected"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := BindEnvironmentProfile(
				"workspace-id", "demo", test.root, "", test.environment,
				DomainEnv, "infisical", test.profile,
			)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestEnvironmentBindingAcceptsCustomSafeEnvironmentID(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "selected")
	if err := BindEnvironmentProfile(
		"workspace-id", "demo", root, "", "staging_us2",
		DomainEnv, "infisical", "selected",
	); err != nil {
		t.Fatalf("bind custom environment: %v", err)
	}
	resolved := resolveEnvironmentProfile(t, root, "", "staging_us2")
	if resolved.Name != "selected" || resolved.Source != "workspace-environment" {
		t.Fatalf("custom environment resolution: %#v", resolved)
	}
}

func TestEnvironmentBindingMutexPreventsLostInProcessUpdates(t *testing.T) {
	withIsolatedConfig(t)
	root := t.TempDir()
	seedInfisicalProfiles(t, "selected")
	const count = 24
	var wait sync.WaitGroup
	errs := make(chan error, count)
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			errs <- BindEnvironmentProfile(
				"workspace-id", "demo", root, "project-"+string(rune('a'+index)), "dev",
				DomainEnv, "infisical", "selected",
			)
		}(index)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	path, _ := BindingsPath()
	bindings, err := loadBindingsAt(path)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := canonicalBindingRoot(root)
	if got := len(bindings.Workspaces[canonical].Environments["dev"].Projects); got != count {
		t.Fatalf("project bindings = %d, want %d", got, count)
	}
}

func TestBindingFileLockPreventsLostIndependentTransactionUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-bindings.json")
	const count = 16
	start := make(chan struct{})
	errs := make(chan error, count)
	var wait sync.WaitGroup

	// updateBindingsAt intentionally has no dependency on bindingsMu. Each call
	// creates its own flock value, matching independent one serve processes.
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			errs <- updateBindingsAt(context.Background(), path, func(bindings *bindingsFile) (bool, error) {
				// Widen the RMW window: without the file lock, every writer can
				// load the same version and publish over another writer.
				time.Sleep(2 * time.Millisecond)
				if bindings.Workspaces == nil {
					bindings.Workspaces = make(map[string]bindingsWorkspace)
				}
				key := fmt.Sprintf("/workspace/copy-%02d", index)
				bindings.Workspaces[key] = bindingsWorkspace{Name: key}
				return true, nil
			})
		}(index)
	}
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	bindings, err := loadBindingsAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(bindings.Workspaces); got != count {
		t.Fatalf("workspace bindings = %d, want %d", got, count)
	}
}

func TestBindingFileLockSerializesIndependentBindAndUnbindTransactions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-bindings.json")
	rootKey := "/workspace/demo"
	sectionKey := SectionKey(DomainEnv, "infisical")
	if err := saveBindingsAt(&bindingsFile{
		Version: bindingsSchemaVersion,
		Workspaces: map[string]bindingsWorkspace{
			rootKey: {
				Environments: map[string]bindingsEnvironment{
					"dev": {
						Projects: map[string]bindingsProjectProfile{
							"old-project": {Profiles: map[string]string{sectionKey: "old"}},
						},
					},
				},
			},
		},
	}, path); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		errs <- updateBindingsAt(context.Background(), path, func(bindings *bindingsFile) (bool, error) {
			workspace := bindings.Workspaces[rootKey]
			selection := workspace.Environments["dev"]
			time.Sleep(20 * time.Millisecond)
			if selection.Projects == nil {
				selection.Projects = make(map[string]bindingsProjectProfile)
			}
			selection.Projects["new-project"] = bindingsProjectProfile{
				Profiles: map[string]string{sectionKey: "new"},
			}
			workspace.Environments["dev"] = selection
			bindings.Workspaces[rootKey] = workspace
			return true, nil
		})
	}()
	go func() {
		defer wait.Done()
		<-start
		errs <- updateBindingsAt(context.Background(), path, func(bindings *bindingsFile) (bool, error) {
			workspace := bindings.Workspaces[rootKey]
			selection := workspace.Environments["dev"]
			time.Sleep(20 * time.Millisecond)
			delete(selection.Projects, "old-project")
			workspace.Environments["dev"] = selection
			bindings.Workspaces[rootKey] = workspace
			return true, nil
		})
	}()
	close(start)
	wait.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	bindings, err := loadBindingsAt(path)
	if err != nil {
		t.Fatal(err)
	}
	projects := bindings.Workspaces[rootKey].Environments["dev"].Projects
	if _, ok := projects["new-project"]; !ok {
		t.Fatal("concurrent unbind lost the independent bind update")
	}
	if _, ok := projects["old-project"]; ok {
		t.Fatal("concurrent bind lost the independent unbind update")
	}
}

func TestBindingFileReadsShareLockWhileWriterHonorsDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-bindings.json")
	if err := saveBindingsAt(&bindingsFile{Version: bindingsSchemaVersion}, path); err != nil {
		t.Fatal(err)
	}

	// This separate flock value represents a reader in another process.
	externalReader := flock.New(path + ".lock")
	if err := externalReader.RLock(); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			_ = externalReader.Unlock()
		}
	}()

	// Another shared reader must not be blocked by the external reader.
	if _, err := readBindingsAt(context.Background(), path); err != nil {
		t.Fatalf("shared read behind shared lock: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	mutated := false
	err := updateBindingsAt(ctx, path, func(*bindingsFile) (bool, error) {
		mutated = true
		return true, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("exclusive write error = %v, want context deadline exceeded", err)
	}
	if mutated {
		t.Fatal("writer mutated state without acquiring its exclusive lock")
	}
	if err := externalReader.Unlock(); err != nil {
		t.Fatal(err)
	}
	locked = false

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path + ".lock")
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("bindings lock mode = %04o, want 0600", info.Mode().Perm())
		}
	}
}

func TestAtomicBindingMarshalFailurePreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-bindings.json")
	original := []byte(`{"version":1}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteSynced(make(chan int), path); err == nil {
		t.Fatal("expected marshal failure")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("failed atomic write changed destination: %q", after)
	}
}

func TestAtomicBindingRenameFailureCleansSiblingTempFile(t *testing.T) {
	dir := t.TempDir()
	// A non-empty destination directory makes rename fail after the sibling
	// temp file has been written and synced, without relying on file modes (the
	// test suite may run as a privileged user).
	destination := filepath.Join(dir, "profile-bindings.json")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "keep"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteSynced(&bindingsFile{Version: 1}, destination); err == nil {
		t.Fatal("expected rename failure")
	}
	if raw, err := os.ReadFile(filepath.Join(destination, "keep")); err != nil || string(raw) != "keep" {
		t.Fatalf("rename failure changed destination: raw=%q err=%v", raw, err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".profile-bindings-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary binding files leaked after failure: %v", matches)
	}
}
