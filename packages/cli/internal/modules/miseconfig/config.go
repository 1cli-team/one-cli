// Package miseconfig maintains the workspace-root mise.toml.
package miseconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/gowork"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/configedit"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

const Filename = workspace.MiseConfigFilename

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
	Schema    string   `json:"schema"`
	Root      string   `json:"root"`
	DryRun    bool     `json:"dry_run"`
	Changes   []Change `json:"changes"`
	inputs    map[string][]byte
	overlay   map[string][]byte
	generated []Change
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
	Tasks        map[string]Task   `toml:"tasks,omitempty"`
}
type monorepo struct {
	ConfigRoots []string `toml:"config_roots"`
	Lockfile    bool     `toml:"lockfile"`
}

// Task is the generated mise declaration. User overrides remain in mise.toml.
type Task struct {
	Directory   string   `toml:"dir,omitempty" json:"dir,omitempty"`
	Description string   `toml:"description,omitempty" json:"description,omitempty"`
	Run         string   `toml:"run,omitempty" json:"run,omitempty"`
	RunWindows  string   `toml:"run_windows,omitempty" json:"run_windows,omitempty"`
	RawArgs     bool     `toml:"raw_args,omitempty" json:"raw_args,omitempty"`
	Interactive bool     `toml:"interactive,omitempty" json:"interactive,omitempty"`
	Depends     []string `toml:"depends,omitempty" json:"depends,omitempty"`
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
	m, err := workspace.ParseManifest(raw)
	if err != nil {
		return nil, err
	}
	ignore, err := p.readOptional(".gitignore")
	if err != nil {
		return nil, err
	}
	if !slices.Contains(strings.Split(strings.ReplaceAll(string(ignore), "\r\n", "\n"), "\n"), ".one-run-*/") {
		after := strings.TrimRight(string(ignore), "\r\n") + "\n.one-run-*/\n"
		p.Changes = append(p.Changes, Change{Path: ".gitignore", Before: string(ignore), After: after})
	}
	oldRoot, err := p.readOptional(Filename)
	if err != nil {
		return nil, err
	}
	var previous struct {
		Tools map[string]any `toml:"tools"`
	}
	if oldRoot != nil {
		if err := toml.Unmarshal(oldRoot, &previous); err != nil {
			return nil, err
		}
	}
	if opts.GoVersion == "" {
		if version, ok := previous.Tools["go"].(string); ok && exactVersion.MatchString(version) {
			opts.GoVersion = version
		}
	}
	if opts.NodeVersion == "" {
		if version, ok := previous.Tools["node"].(string); ok && exactVersion.MatchString(version) {
			opts.NodeVersion = version
		}
	}
	if opts.NodeVersion == "" {
		opts.NodeVersion = "24.21.0"
	}
	if !exactVersion.MatchString(opts.NodeVersion) {
		return nil, i18n.Errorf("miseconfig.node_version")
	}
	if opts.GoVersion != "" && !exactVersion.MatchString(opts.GoVersion) {
		return nil, i18n.Errorf("miseconfig.go_version")
	}
	rootConfig := config{MinVersion: runtimeport.MinimumMiseVersion, MonorepoRoot: true, Monorepo: &monorepo{ConfigRoots: []string{}}, Tools: map[string]string{}, Tasks: map[string]Task{}}
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
			return nil, i18n.Errorf("miseconfig.package_version")
		}
		switch name {
		case "pnpm":
		default:
			return nil, i18n.Errorf("workspace.package_manager_unsupported", name)
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
		return nil, i18n.Errorf("miseconfig.go_work_version", opts.GoVersion, goWorkspace.Version)
	}
	configs := map[string]config{}
	projects := []workspace.Project{}
	for _, mp := range m.Projects {
		projects = append(projects, workspace.Project{Name: mp.Name, RelativeDir: mp.RelativeDir, TargetDir: filepath.Join(root, mp.RelativeDir), Toolchain: mp.Toolchain})
	}
	for _, project := range m.Projects {
		if !workspace.IsValidProjectName(project.Name) {
			return nil, i18n.Errorf("miseconfig.project_name", project.Name)
		}
		rel := filepath.ToSlash(filepath.Clean(project.RelativeDir))
		if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, conflict(rel, i18n.T("config.project_path"))
		}
		pc := config{MinVersion: runtimeport.MinimumMiseVersion, Tasks: map[string]Task{}, Tools: map[string]string{}}
		nativeProject := workspace.Project{Name: project.Name, RelativeDir: rel, TargetDir: filepath.Join(root, rel), Toolchain: project.Toolchain}
		operations, err := workspace.DiscoverTasks(root, nativeProject, p.readOptional)
		if err != nil {
			return nil, err
		}
		switch project.Toolchain {
		case "node":
			rootConfig.Tools["node"] = opts.NodeVersion
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
				return nil, i18n.Errorf("miseconfig.go_version_unknown", project.Name)
			}
			if opts.GoVersion == "" {
				if goWorkspace.Version != "" && compareVersions(version, goWorkspace.Version) < 0 {
					version = goWorkspace.Version
				}
				if current := rootConfig.Tools["go"]; current == "" || compareVersions(version, current) > 0 {
					rootConfig.Tools["go"] = version
				}
			}
			rootConfig.Tools["task"] = "3.51.1"
		}
		for _, op := range operations {
			argv := append([]string(nil), op.Argv...)
			// Task accepts CLI arguments after --; pnpm forwards script arguments.
			if project.Toolchain == "go" {
				argv = append(argv, "--")
			}
			task := Task{Directory: rel, Description: op.Description, Run: nativeCommand(argv, false), RawArgs: true, Interactive: op.Interactive}
			if windows := nativeCommand(argv, true); windows != task.Run {
				task.RunWindows = windows
			}
			pc.Tasks[op.Name] = task
			if op.Name == "build" || op.Name == "check" || op.Name == "test" || op.Name == "dev" {
				aggregate := rootConfig.Tasks[op.Name]
				aggregate.Depends = append(aggregate.Depends, "//:"+project.Name+":"+op.Name)
				rootConfig.Tasks[op.Name] = aggregate
			}
		}
		configs[project.Name] = pc

	}
	edges, err := workspace.BuildDependencies(root, projects, p.readOptional)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		pc := configs[project.Name]
		upstreamNames, err := dependencyClosure(project.Name, edges)
		if err != nil {
			return nil, err
		}
		for name, task := range pc.Tasks {
			for _, dep := range upstreamNames {
				if name == "build" || name == "dev" || name == "check" || name == "test" || name == "typecheck" {
					if _, ok := configs[dep].Tasks["build"]; ok {
						task.Depends = append(task.Depends, "//:"+dep+":build")
					}
				}
			}
			rootConfig.Tasks[project.Name+":"+name] = task
		}
	}
	for _, group := range m.Groups {
		operations := map[string][]string{}
		var builds []string
		for _, member := range group.Projects {
			for name := range configs[member].Tasks {
				operations[name] = append(operations[name], "//:"+member+":"+name)
			}
			if _, ok := configs[member].Tasks["build"]; ok {
				builds = append(builds, "//:"+member+":build")
			}
		}
		for name, targets := range operations {
			rootConfig.Tasks[group.Name+":"+name] = Task{Depends: targets}
			// Packaging consumes every component's output, including renderer assets.
			if name == "pack" || name == "release" {
				for _, member := range group.Projects {
					key := member + ":" + name
					if task, ok := rootConfig.Tasks[key]; ok {
						for _, build := range builds {
							if !slices.Contains(task.Depends, build) {
								task.Depends = append(task.Depends, build)
							}
						}
						rootConfig.Tasks[key] = task
					}
				}
			}
		}
	}
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
			return "", i18n.Errorf("miseconfig.go_mod_version", override, minimum)
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
	body, err := toml.Marshal(value)
	if err != nil {
		return err
	}
	after, err := configedit.TOML(before, body)
	if err != nil {
		return conflict(rel, err.Error())
	}
	if !bytes.Equal(before, after) {
		p.Changes = append(p.Changes, Change{Path: rel, Before: string(before), After: string(after)})
	}
	// Only complete configurations matching our generated values are eligible
	// for automatic trust. User additions and overrides keep mise's own policy.
	var actual, generated map[string]any
	if err := toml.Unmarshal(after, &actual); err != nil {
		return err
	}
	if err := toml.Unmarshal(body, &generated); err != nil {
		return err
	}
	if reflect.DeepEqual(actual, generated) {
		p.generated = append(p.generated, Change{Path: rel, After: string(after)})
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
			return nil, conflict(rel, i18n.T("config.prepare_changed"))
		}
		p.inputs[rel] = raw
	}
	return raw, err
}
func (p *Plan) readOptional(rel string) ([]byte, error) {
	raw, err := p.read(rel)
	if os.IsNotExist(err) {
		if previous, exists := p.inputs[rel]; exists && previous != nil {
			return nil, conflict(rel, i18n.T("config.prepare_removed"))
		}
		p.inputs[rel] = nil
		return nil, nil
	}
	return raw, err
}
func (p *Plan) safePath(rel string) error {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return conflict(rel, i18n.T("config.path_escape"))
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
			return conflict(rel, i18n.T("config.symlink"))
		}
	}
	return nil
}

func (p *Plan) Apply(ctx context.Context) error {
	if p.overlay != nil {
		return i18n.Errorf("miseconfig.transaction_required")
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
			return conflict(rel, i18n.T("config.plan_changed"))
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
					failures = append(failures, conflict(c.Path, i18n.T("config.rollback_changed")))
					continue
				}
				var rollbackErr error
				if c.Before == "" {
					rollbackErr = os.Remove(path)
				} else {
					rollbackErr = writeAtomic(path, []byte(c.Before))
				}
				if rollbackErr != nil {
					failures = append(failures, i18n.Errorf("file.restore_failed", c.Path, rollbackErr))
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
