package cli_test

// Shared helpers for the snapshot_e2e_*_test.go files in this package.
// They build the binary path relative to the test file's location,
// exec it, and compare JSON output against testdata/reference/ fixtures.
//
// Tests that depend on bin/one being present must call binaryPath; it
// t.Skip's if the binary isn't built yet (CI runs `task build` first).

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the repository root, derived
// from this file's location at compile time. Survives `go test` being
// invoked from any working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// e2e_helpers_test.go lives at internal/cli/, so two levels up.
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// binaryPath returns the path to bin/one, skipping the test if the
// binary hasn't been built. Run `task build` first.
func binaryPath(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(repoRoot(t), "bin", "one")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("binary not built; run `task build` first (%v)", err)
	}
	return bin
}

// indexByteString is the test-helper local copy of strings.IndexByte
// (avoids pulling strings just for this).
func indexByteString(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// runBinary execs bin/one with the given args from the current cwd.
func runBinary(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	return runBinaryIn(t, "", args...)
}

// runBinaryIn execs bin/one in the given directory (empty = current).
// Inherits the test process env, so t.Setenv("HOME", ...) propagates.
func runBinaryIn(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := binaryPath(t)
	cmd := exec.Command(bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = prependPath(os.Environ(), filepath.Dir(bin))
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("exec error: %v", err)
	}
	return out.String(), errBuf.String(), exitCode
}

func prependPath(env []string, dir string) []string {
	out := append([]string{}, env...)
	prefix := "PATH="
	for i, kv := range out {
		if strings.HasPrefix(kv, prefix) {
			path := strings.TrimPrefix(kv, prefix)
			if path == "" {
				out[i] = prefix + dir
			} else {
				out[i] = prefix + dir + string(os.PathListSeparator) + path
			}
			return out
		}
	}
	return append(out, prefix+dir)
}

// loadFixture reads a JSON fixture from testdata/reference/<name> and
// decodes it into a map for structural comparison.
func loadFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "reference", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return mustParseJSON(t, string(raw))
}

// mustParseJSON decodes s into a map, failing the test on error.
func mustParseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("decode json (input=%q): %v", s, err)
	}
	return m
}

// firstJSONLine extracts the first complete { … } payload from a possibly
// multi-line buffer. The CLI emits the structured envelope first (now
// pretty-printed across multiple lines), then a plain one-line summary;
// only the JSON envelope participates in the contract. Uses json.Decoder
// to consume exactly one top-level value.
func firstJSONLine(s string) string {
	idx := strings.IndexByte(s, '{')
	if idx < 0 {
		return ""
	}
	dec := json.NewDecoder(strings.NewReader(s[idx:]))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return ""
	}
	return string(raw)
}

// pretty indent-marshals v for use in test failure messages.
func pretty(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

// volatileKeys lists JSON keys whose values change per-run (timestamps,
// absolute paths, machine-specific lists). canonicalize scrubs them
// recursively before snapshot comparison so fixtures stay stable.
//
// Add new entries when a new test surfaces a new volatile key — keep
// the set tight: scrubbing too aggressively hides real regressions.
var volatileKeys = map[string]bool{
	"created_path":     true, // create: absolute path under tempdir
	"installed_to":     true, // setup/create: list of absolute skill paths under $HOME
	"profile_path":     true, // profile-add (legacy v3): absolute path to ~/.config/one/profiles.json
	"config_path":      true, // profile-add (v4): absolute path to ~/.config/one/config.json
	"credentials_path": true, // profile-add (v4): absolute path to ~/.config/one/credentials.json
	"display_path":     true, // error envelopes: relative-or-absolute path, depends on cwd
	"target_path":      true, // create error: absolute target path
	"workspace_path":   true,
	"workspace_root":   true,
	"root":             true, // status: workspace.root is absolute
	"updatedAt":        true, // manifest: ISO timestamp
	"updated_at":       true,
	"timestamp":        true, // status envelopes
	"detected_at":      true,
	"absolute_path":    true,
	"path":             true, // status subproject entries
	"node_version":     true, // depends on local node install
	"go_version":       true,
	"version_actual":   true,
	"git_commit":       true,
	"node_modules":     true,
	"home":             true,
	"message":          true, // error envelopes' message embeds tempdir paths and is i18n-mutable; contract is `code`+`schema`
	"generated_files":  true, // add: list of absolute paths to generated AI guide files
	"files":            true, // add-spec: list of absolute paths to generated spec files
	"written_to":       true, // env init: absolute path to one.manifest.json
	"url":              true, // serve: per-run URL with random port + token
	"port":             true, // serve: kernel-assigned port when --port 0
	"token":            true, // serve: 32-byte random per-run session token
}

// canonicalize returns a deep copy of m with all volatileKeys scrubbed
// to the sentinel string "<volatile>". Maps and slices are walked
// recursively. The original is not mutated.
func canonicalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		// Sort keys for deterministic walk; the resulting map's iteration
		// order doesn't affect DeepEqual but stable order helps when
		// debugging by printing intermediate state.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if volatileKeys[k] {
				out[k] = "<volatile>"
				continue
			}
			out[k] = canonicalize(x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = canonicalize(e)
		}
		return out
	default:
		return v
	}
}

// assertSnapshot compares got (after canonicalize) against the fixture
// at testdata/reference/<name>. On mismatch fails the test with a
// pretty-printed diff.
//
// When UPDATE_SNAPSHOTS=1 is set, writes the canonicalized got to the
// fixture file instead of comparing. Use this to bootstrap new fixtures
// or to update one after a deliberate output change — review the diff
// before committing.
func assertSnapshot(t *testing.T, name string, got map[string]any) {
	t.Helper()
	canon := canonicalize(got)
	path := filepath.Join(repoRoot(t), "testdata", "reference", name)
	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		b, err := json.MarshalIndent(canon, "", "  ")
		if err != nil {
			t.Fatalf("marshal canon for %s: %v", path, err)
		}
		b = append(b, '\n')
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for fixture %s: %v", path, err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
		t.Logf("UPDATE_SNAPSHOTS=1: wrote %s", path)
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v\n  hint: run with UPDATE_SNAPSHOTS=1 to create", path, err)
	}
	var want any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	if !reflect.DeepEqual(canon, want) {
		t.Errorf("snapshot drift vs %s\n  want: %s\n  got:  %s\n  hint: re-run with UPDATE_SNAPSHOTS=1 if this change is intentional",
			path, pretty(want), pretty(canon))
	}
}

// isolateHome redirects HOME to the given dir for the duration of the
// test. The CLI plants skills under $HOME/.<agent>/skills; without
// this the test would mutate the developer's actual agent dirs.
//
// Also clears XDG_CONFIG_HOME because profile.ConfigPath() honours
// it ahead of HOME (Linux convention). On Linux CI XDG_CONFIG_HOME is
// often set inherited from the runner shell, which would bleed
// config.json / credentials.json state across tests despite the
// per-test HOME.
func isolateHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")
}

// fileExists is a tiny convenience wrapper used in tree-shape assertions.
func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// readManifest parses one.manifest.json at the given workspace root
// for spot assertions in tests. Returns the parsed map.
func readManifest(t *testing.T, workspaceRoot string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(workspaceRoot, "one.manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	return mustParseJSON(t, string(raw))
}

// jsonContains reports whether stdout looks like a JSON object that
// contains the given top-level key. Cheap pre-check before mustParseJSON
// when the binary may have printed nothing.
func jsonContains(s, key string) bool {
	return strings.Contains(s, "\""+key+"\":")
}

// bootstrapWorkspace runs `one create <tmp>/<name> -y` to produce a
// fresh workspace under HOME-isolated tempdir, returning the workspace
// root. The caller is expected to have already called isolateHome(t, tmp).
//
// Used by add / status E2E tests that need a real workspace
// to operate on but don't themselves test create's output. Note that
// v0.5+ this writes WorkspaceDefaults (env/dotenv + ci/github-actions +
// dev/process) into manifest.plugins; tests that need the workspace
// without one of those domains should call clearManifestDomain
// after bootstrap.
func bootstrapWorkspace(t *testing.T, tmp, name string) string {
	t.Helper()
	target := filepath.Join(tmp, name)
	stdout, stderr, code := runBinary(t, "create", target, "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("bootstrapWorkspace(%s) failed: exit %d\n  stdout: %s\n  stderr: %s",
			name, code, stdout, stderr)
	}
	return target
}

// patchManifestDomain rewrites one.manifest.json to set the workspace's
// kind for one of the polymorphic domains (env / container) to a specific
// backend. Used by tests that need to switch the selected backend (e.g. flip
// env from dotenv to infisical for init/set/get tests) without going
// through the configure flow (which would trigger SyncSubproject
// side-effects).
//
// The current manifest stores backend selections under m.domains.<domain>.kind. ci / dev are
// always-on and have no on-disk representation, so they're not
// supported here.
func patchManifestDomain(t *testing.T, ws, domain, pluginID string) {
	t.Helper()
	mp := filepath.Join(ws, "one.manifest.json")
	raw, err := os.ReadFile(mp)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	// Strip "<domain>/" prefix to get the bare backend name (kind).
	kind := pluginID
	if i := indexByteString(pluginID, '/'); i > 0 {
		kind = pluginID[i+1:]
	}
	switch domain {
	case "env", "container":
		domains, _ := doc["domains"].(map[string]any)
		if domains == nil {
			domains = map[string]any{}
		}
		entry, _ := domains[domain].(map[string]any)
		if entry == nil {
			entry = map[string]any{}
		}
		entry["kind"] = kind
		domains[domain] = entry
		doc["domains"] = domains
	default:
		t.Fatalf("patchManifestDomain: unsupported domain %q in manifest", domain)
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(mp, out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// clearManifestDomain rewrites one.manifest.json to drop the given domain
// from the current schema. The manifest only persists env / container / deploy under
// m.domains.<domain>; ci / dev are always-on and have no on-disk
// representation, so passing them is a no-op.
func clearManifestDomain(t *testing.T, ws, domain string) {
	t.Helper()
	mp := filepath.Join(ws, "one.manifest.json")
	raw, err := os.ReadFile(mp)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if domains, ok := doc["domains"].(map[string]any); ok {
		delete(domains, domain)
		if len(domains) == 0 {
			delete(doc, "domains")
		} else {
			doc["domains"] = domains
		}
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(mp, out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}
