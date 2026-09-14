// Package miseconfig produces additive mise fragments without rewriting user TOML.
package miseconfig

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/gowork"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"gopkg.in/yaml.v3"
)

const Filename = workspace.MiseConfigFilename
const header = "# Managed by One CLI mise/v1; sha256="

type Options struct {
	NodeVersion string
	GoVersion   string
}

type Change struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type Plan struct {
	Schema  string   `json:"schema"`
	Root    string   `json:"root"`
	DryRun  bool     `json:"dry_run"`
	Changes []Change `json:"changes"`
	inputs  map[string][]byte
	overlay map[string][]byte
}

// ReadInputs allows creation to preserve this planner's conflict checks when
// publishing mise files together with a future manifest and go.work.
func (p *Plan) ReadInputs() map[string][]byte {
	out := map[string][]byte{}
	for path, b := range p.inputs {
		out[path] = bytes.Clone(b)
	}
	return out
}

type config struct {
	MinVersion   string            `toml:"min_version"`
	MonorepoRoot bool              `toml:"monorepo_root,omitempty"`
	Monorepo     *monorepo         `toml:"monorepo,omitempty"`
	Tools        map[string]string `toml:"tools,omitempty"`
	Tasks        map[string]task   `toml:"tasks,omitempty"`
}
type monorepo struct {
	ConfigRoots []string `toml:"config_roots"`
	Lockfile    bool     `toml:"lockfile"`
}
type task struct {
	Description string `toml:"description"`
	Run         string `toml:"run"`
	RunWindows  string `toml:"run_windows"`
}

func Enabled(root string) bool {
	_, err := os.Lstat(filepath.Join(root, Filename))
	// Let Build report inaccessible or invalid configuration instead of
	// silently skipping synchronization when it cannot be inspected.
	return !os.IsNotExist(err)
}

// Build is entirely static: no mise invocation, hooks, secrets, or network calls.
func Build(root string, opts Options) (*Plan, error) {
	return BuildWithFiles(root, opts, nil)
}

// BuildWithFiles validates the future workspace before creation publishes
// its manifest and language configuration. Overlay paths are root-relative.
func BuildWithFiles(root string, opts Options, files map[string][]byte) (*Plan, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	p := &Plan{Schema: "one-cli/mise-config/v1", Root: root, DryRun: true, Changes: []Change{}, inputs: map[string][]byte{}, overlay: files}
	raw, err := p.read(workspace.ManifestFilename)
	if err != nil {
		return nil, err
	}
	var m workspace.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if _, err := workspace.ReadManifest(root); err != nil {
		return nil, err
	}
	oldRoot, err := p.readOptional(Filename)
	if err != nil {
		return nil, err
	}
	var previous config
	if oldRoot != nil {
		if err := validateManaged(Filename, oldRoot); err != nil {
			return nil, err
		}
		if err := toml.Unmarshal(oldRoot, &previous); err != nil {
			return nil, err
		}
	}
	if opts.GoVersion == "" {
		opts.GoVersion = previous.Tools["go"]
	}
	if opts.NodeVersion == "" {
		opts.NodeVersion = previous.Tools["node"]
	}
	if opts.NodeVersion == "" {
		opts.NodeVersion = "24.15.0"
	}
	if !exactVersion.MatchString(opts.NodeVersion) {
		return nil, fmt.Errorf("--node-version requires an exact version such as 24.15.0")
	}
	if opts.GoVersion != "" && !exactVersion.MatchString(opts.GoVersion) {
		return nil, fmt.Errorf("--go-version requires an exact version such as 1.27.0")
	}
	rootConfig := config{MinVersion: runtimeport.MinimumMiseVersion, MonorepoRoot: true, Monorepo: &monorepo{ConfigRoots: []string{}}, Tools: map[string]string{}}
	hooks, err := p.readOptional(workspace.HooksConfigFilename)
	if err != nil {
		return nil, err
	}
	if hooks != nil {
		rootConfig.Tools[workspace.HKTool] = workspace.HKVersion
	}
	if opts.GoVersion != "" {
		rootConfig.Tools["go"] = opts.GoVersion
	}
	rootPkg, err := p.readOptional("package.json")
	if err != nil {
		return nil, err
	}
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if len(rootPkg) > 0 {
		if err := json.Unmarshal(rootPkg, &pkg); err != nil {
			return nil, err
		}
	}
	if pkg.PackageManager != "" {
		name, version, ok := strings.Cut(pkg.PackageManager, "@")
		version, _, _ = strings.Cut(version, "+") // Corepack integrity suffix is not a mise version.
		if !ok || !exactVersion.MatchString(version) {
			return nil, fmt.Errorf("package.json packageManager must specify an exact version")
		}
		switch name {
		case "pnpm", "npm", "yarn", "bun":
		default:
			return nil, fmt.Errorf("unsupported package manager %q", name)
		}
		rootConfig.Tools[name] = version
		rootConfig.Tools["node"] = opts.NodeVersion
	}
	goWorkspace, err := gowork.Inspect(root, func(path string) ([]byte, error) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			// External user members are read-only inputs, never generated paths.
			b, err := os.ReadFile(path)
			if err == nil {
				p.inputs[path] = b
			}
			return b, err
		}
		return p.readOptional(filepath.ToSlash(rel))
	})
	if err != nil {
		return nil, err
	}
	if opts.GoVersion != "" && goWorkspace.Version != "" && compareVersions(opts.GoVersion, goWorkspace.Version) < 0 {
		return nil, fmt.Errorf("Go %s is below the go.work requirement %s", opts.GoVersion, goWorkspace.Version)
	}
	for _, project := range m.Projects {
		if !workspace.IsValidProjectName(project.Name) {
			return nil, fmt.Errorf("invalid project name %q", project.Name)
		}
		rel := filepath.ToSlash(filepath.Clean(project.RelativeDir))
		if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, conflict(rel, "project path must stay inside the workspace")
		}
		rootConfig.Monorepo.ConfigRoots = append(rootConfig.Monorepo.ConfigRoots, rel)
		pc := config{MinVersion: runtimeport.MinimumMiseVersion, Tasks: map[string]task{}, Tools: map[string]string{}}
		operations := []string{}
		if workspace.ProjectDev(&m, project.Name) != "" {
			operations = append(operations, "dev")
		}
		switch project.Toolchain {
		case "node":
			rootConfig.Tools["node"] = opts.NodeVersion
			raw, err := p.read(rel + "/package.json")
			if err != nil {
				return nil, err
			}
			var scripts struct {
				Scripts map[string]string `json:"scripts"`
			}
			if err := json.Unmarshal(raw, &scripts); err != nil {
				return nil, err
			}
			for _, op := range []string{"build", "test", "lint"} {
				if scripts.Scripts[op] != "" {
					operations = append(operations, op)
				}
			}
		case "go":
			raw, err := p.read(rel + "/go.mod")
			if err != nil {
				return nil, err
			}
			version, err := goVersion(raw, opts.GoVersion)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", project.Name, err)
			}
			if !exactVersion.MatchString(version) {
				return nil, fmt.Errorf("cannot determine Go version for %s; pass --go-version", project.Name)
			}
			if opts.GoVersion == "" {
				if goWorkspace.Version != "" && compareVersions(version, goWorkspace.Version) < 0 {
					version = goWorkspace.Version
				}
				pc.Tools["go"] = version
			}
			rawTask, err := p.readOptional(rel + "/Taskfile.yml")
			if err != nil {
				return nil, err
			}
			if rawTask == nil {
				operations = append(operations, "build", "test", "lint")
			} else {
				pc.Tools["task"] = "3.51.1"
				var tasks struct {
					Tasks map[string]any `yaml:"tasks"`
				}
				if err := yaml.Unmarshal(rawTask, &tasks); err != nil {
					return nil, err
				}
				for _, op := range []string{"build", "test", "lint"} {
					if _, ok := tasks.Tasks[op]; ok {
						operations = append(operations, op)
					}
				}
			}
		}
		for _, op := range operations {
			args := " __exec --protocol 1 --project " + project.Name + " --operation " + op
			pc.Tasks["one:"+op] = task{
				Description: "One " + op + " for " + project.Name,
				Run:         "{{ env.ONE_BINARY_PATH | default(value='one') | quote }}" + args,
				RunWindows:  `if defined ONE_BINARY_PATH ("%ONE_BINARY_PATH%"` + args + `) else (one` + args + `)`,
			}
		}
		if err := p.add(rel+"/"+Filename, pc); err != nil {
			return nil, err
		}
	}
	sort.Strings(rootConfig.Monorepo.ConfigRoots)
	if err := p.add(Filename, rootConfig); err != nil {
		return nil, err
	}
	sort.Slice(p.Changes, func(i, j int) bool { return p.Changes[i].Path < p.Changes[j].Path })
	return p, nil
}

var exactVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
var goDirective = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+(?:\.\d+)?)\s*$`)
var goToolchain = regexp.MustCompile(`(?m)^toolchain\s+go(\d+\.\d+\.\d+)\s*$`)

func compareVersions(a, b string) int {
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range 3 {
		x, _ := strconv.Atoi(aa[i])
		y, _ := strconv.Atoi(bb[i])
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func goVersion(raw []byte, override string) (string, error) {
	minimum := ""
	if match := goDirective.FindSubmatch(raw); len(match) > 1 {
		minimum = string(match[1])
		if strings.Count(minimum, ".") == 1 {
			minimum += ".0"
		}
	}
	if override != "" {
		if minimum != "" && compareVersions(override, minimum) < 0 {
			return "", fmt.Errorf("Go %s is below the go.mod requirement %s", override, minimum)
		}
		return override, nil
	}
	if match := goToolchain.FindSubmatch(raw); len(match) > 1 {
		suggested := string(match[1])
		if minimum == "" || compareVersions(suggested, minimum) > 0 {
			return suggested, nil
		}
	}
	return minimum, nil
}

func (p *Plan) add(rel string, value config) error {
	before, err := p.readOptional(rel)
	if err != nil {
		return err
	}
	if before != nil {
		if err := validateManaged(rel, before); err != nil {
			return err
		}
	}
	body, err := toml.Marshal(value)
	if err != nil {
		return err
	}
	after := []byte(fmt.Sprintf("%s%x\n# Edit user overrides in mise.toml; refresh with one configure mise.\n%s", header, sha256.Sum256(body), body))
	if !bytes.Equal(before, after) {
		p.Changes = append(p.Changes, Change{Path: rel, Before: string(before), After: string(after)})
	}
	return nil
}

func validateManaged(path string, raw []byte) error {
	lines := bytes.SplitN(raw, []byte("\n"), 3)
	if len(lines) != 3 || string(lines[0]) != fmt.Sprintf("%s%x", header, sha256.Sum256(lines[2])) {
		return conflict(path, "file is user-owned or has been modified; preserve it and resolve the conflict before regenerating")
	}
	return nil
}

func (p *Plan) read(rel string) ([]byte, error) {
	if err := p.safePath(rel); err != nil {
		return nil, err
	}
	if raw, ok := p.overlay[rel]; ok {
		return raw, nil
	}
	raw, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(rel)))
	if err == nil {
		if previous, exists := p.inputs[rel]; exists && (previous == nil || !bytes.Equal(previous, raw)) {
			return nil, conflict(rel, "file changed while preparing configuration; retry")
		}
		p.inputs[rel] = raw
	}
	return raw, err
}
func (p *Plan) readOptional(rel string) ([]byte, error) {
	raw, err := p.read(rel)
	if os.IsNotExist(err) {
		if previous, exists := p.inputs[rel]; exists && previous != nil {
			return nil, conflict(rel, "file was removed while preparing configuration; retry")
		}
		p.inputs[rel] = nil
		return nil, nil
	}
	return raw, err
}
func (p *Plan) safePath(rel string) error {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return conflict(rel, "path escapes workspace")
	}
	current := p.Root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return conflict(rel, "symbolic links are not supported in generated configuration paths")
		}
	}
	return nil
}

func (p *Plan) Apply(ctx context.Context) error {
	if p.overlay != nil {
		return fmt.Errorf("a projected mise plan must be applied by its workspace transaction")
	}
	unlock, err := fsutil.WorkspaceLock(ctx, p.Root, "mise")
	if err != nil {
		return err
	}
	defer unlock()
	for rel, expected := range p.inputs {
		path := rel
		if !filepath.IsAbs(rel) {
			if err := p.safePath(rel); err != nil {
				return err
			}
			path = filepath.Join(p.Root, rel)
		}
		actual, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if !bytes.Equal(actual, expected) || (expected == nil) != os.IsNotExist(err) {
			return conflict(rel, "file changed after the plan was prepared; retry")
		}
	}
	applied := []Change{}
	for _, change := range p.Changes {
		if err := writeAtomic(filepath.Join(p.Root, change.Path), []byte(change.After)); err != nil {
			failures := []error{err}
			for i := len(applied) - 1; i >= 0; i-- {
				c := applied[i]
				path := filepath.Join(p.Root, c.Path)
				current, readErr := os.ReadFile(path)
				if readErr != nil || string(current) != c.After {
					failures = append(failures, conflict(c.Path, "could not restore configuration because the file changed during rollback"))
					continue
				}
				var rollbackErr error
				if c.Before == "" {
					rollbackErr = os.Remove(path)
				} else {
					rollbackErr = writeAtomic(path, []byte(c.Before))
				}
				if rollbackErr != nil {
					failures = append(failures, fmt.Errorf("restore %s: %w", c.Path, rollbackErr))
				}
			}
			return errors.Join(failures...)
		}
		applied = append(applied, change)
	}
	p.DryRun = false
	return nil
}

func writeAtomic(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".one-mise-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	_, err = f.Write(content)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return fsutil.ReplaceFile(f.Name(), path)
}

func conflict(path, reason string) error {
	return cliErrors.New(cliErrors.MISE_CONFIG_CONFLICT, fmt.Sprintf("%s: %s", path, reason)).WithContext(map[string]any{"path": path})
}
