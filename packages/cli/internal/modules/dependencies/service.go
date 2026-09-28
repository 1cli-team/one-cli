// Package dependencies prepares application dependencies before development and builds.
// Tool installation belongs to the runtime provider; exec remains a plain runner.
package dependencies

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

type Runner func(context.Context, runtimeport.Command, io.Writer, io.Writer) error

type Service struct {
	Provider runtimeport.Provider
	Run      Runner // optional process boundary for tests
}

type Input struct {
	Root     string
	Manifest *workspace.Manifest
	Project  string
	// Projects selects an explicit set, including projects without a dev command.
	// nil preserves the development selection used by existing callers.
	Projects []string
	Runtime  string
	Log      io.Writer
	// Development allows pnpm to synchronize the lockfile and reuse a manual
	// install after pnpm has verified the workspace dependency state.
	Development bool
}

func (s Service) Prepare(ctx context.Context, in Input) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if in.Log == nil {
		in.Log = io.Discard
	}
	selected := map[string]bool{}
	for _, name := range in.Projects {
		selected[name] = true
	}
	var nodes, goProjects []workspace.ManifestProject
	for _, p := range in.Manifest.Projects {
		if in.Projects != nil {
			if !selected[p.Name] {
				continue
			}
		} else if (in.Project != "" && p.Name != in.Project) || strings.TrimSpace(workspace.ProjectDev(in.Manifest, p.Name)) == "" {
			continue
		}
		switch p.Toolchain {
		case "node":
			nodes = append(nodes, p)
		case "go":
			goProjects = append(goProjects, p)
		}
	}
	if len(nodes) > 0 {
		if err := s.prepareNode(ctx, in, nodes[0].PackageManager); err != nil {
			return err
		}
	}
	for _, p := range goProjects {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.prepareGo(ctx, in, p); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (s Service) run(ctx context.Context, in Input, dir string, args []string, env []string, out, errOut io.Writer) error {
	command := runtimeport.Command{Directory: dir, Argv: args, Env: env}
	if in.Runtime == runtimeport.Mise {
		if s.Provider == nil {
			return i18n.Errorf("dependencies.mise_required")
		}
		var err error
		command, err = s.Provider.Prepare(ctx, command)
		if err != nil {
			return err
		}
	}
	if len(command.Argv) == 0 {
		return i18n.Errorf("dependencies.command_empty")
	}
	run := s.Run
	if run == nil {
		run = runCommand
	}
	return run(ctx, command, out, errOut)
}

func runCommand(ctx context.Context, command runtimeport.Command, out, errOut io.Writer) error {
	child := platformprocess.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	child.Dir, child.Env = command.Directory, command.Env
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, out, errOut
	platformprocess.CancelProcessTree(child)
	err := child.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (s Service) prepareGo(ctx context.Context, in Input, p workspace.ManifestProject) error {
	root, err := filepath.EvalSymlinks(in.Root)
	if err != nil {
		return preparationError(p.Name, in.Root, "resolve Go workspace directory", err, "")
	}
	dir := filepath.Join(root, filepath.FromSlash(p.RelativeDir))
	// Go's workspace loader compares module directory strings. Keep the
	// child cwd, its Unix PWD hint, and GOWORK on the same root spelling.
	// Preserve module-relative paths, which may themselves contain symlinks.
	env := append(os.Environ(), "PWD="+dir)
	var work bytes.Buffer
	if err := s.run(ctx, in, dir, []string{"go", "env", "GOWORK"}, env, &work, in.Log); err != nil {
		return preparationError(p.Name, dir, "resolve Go workspace", err, work.String())
	}
	active := strings.TrimSpace(work.String())
	lockRoot := dir
	if active != "" && active != "off" {
		want := filepath.Join(root, "go.work")
		if filepath.Clean(active) != filepath.Clean(want) {
			// An explicit GOWORK may use a symlink to this workspace file.
			activeInfo, activeErr := os.Stat(active)
			wantInfo, wantErr := os.Stat(want)
			if activeErr != nil || wantErr != nil || !os.SameFile(activeInfo, wantInfo) {
				return i18n.Errorf("dependencies.external_gowork", p.Name, active, want)
			}
		}
		// Resolve the root, not the go.work file itself: relative use paths
		// belong to this workspace even when go.work is a symlink.
		env = append(env, "GOWORK="+want)
		lockRoot = root
	}
	unlock, err := fsutil.WorkspaceLock(ctx, lockRoot, "go-dependencies")
	if err != nil {
		return err
	}
	defer unlock()
	fmt.Fprintf(in.Log, i18n.T("dependencies.go_preparing"), p.Name)
	// In workspace mode, package loading uses the actual Go build graph and
	// writes workspace sums as needed. Expanding `all` can fetch historical
	// versions of local members; only use it for a standalone module.
	commands := [][]string{}
	if active == "" || active == "off" {
		if err := s.downloadModule(ctx, in, p, dir, env); err != nil {
			return err
		}
	}
	commands = append(commands, []string{"go", "list", "-mod=readonly", "-buildvcs=false", "-deps", "./..."})
	for _, args := range commands {
		var detail bytes.Buffer
		out := io.Discard

		if err := s.run(ctx, in, dir, args, env, out, io.MultiWriter(in.Log, &detail)); err != nil {
			failure := preparationError(p.Name, dir, strings.Join(args, " "), err, detail.String())
			if args[1] == "list" {
				return failure.WithRemediation(output.Remediation{Action: "repair-go-module", Hint: i18n.T("dependencies.go_repair_hint"), Command: "one exec " + p.Name + " -- go mod tidy"})
			}
			return failure
		}
	}
	return nil
}

// Go may rewrite go/toolchain directives during mod download. Use an alternate
// module file and publish only checksums after verifying the declarations stayed
// unchanged; never silently repair or upgrade the user's go.mod.
func (s Service) downloadModule(ctx context.Context, in Input, p workspace.ManifestProject, dir string, env []string) error {
	files := fsutil.NewFilePlan(dir)
	module, err := files.Read("go.mod")
	if err != nil {
		return err
	}
	if module == nil {
		return i18n.Errorf("dependencies.go_mod_missing", p.Name)
	}
	sums, err := files.Read("go.sum")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".one-dependencies-*.mod")
	if err != nil {
		return err
	}
	name := f.Name()
	sumName := strings.TrimSuffix(name, ".mod") + ".sum"
	defer os.Remove(name)
	defer os.Remove(sumName)
	_, writeErr := f.Write(module)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if sums != nil {
		if err := os.WriteFile(sumName, sums, 0o600); err != nil {
			return err
		}
	}
	args := []string{"go", "mod", "download", "-modfile=" + name, "all"}
	var detail bytes.Buffer
	if err := s.run(ctx, in, dir, args, env, io.Discard, io.MultiWriter(in.Log, &detail)); err != nil {
		return preparationError(p.Name, dir, "go mod download all", err, detail.String())
	}
	after, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if !bytes.Equal(module, after) {
		return cliErrors.New(cliErrors.ONE_CLI_ERROR, i18n.Tf("dependencies.go_changes_required", p.Name)).WithRemediation(output.Remediation{
			Action: "repair-go-module", Command: "one exec " + p.Name + " -- go mod tidy", Hint: i18n.T("dependencies.go_tidy_hint"),
		})
	}
	afterSums, err := os.ReadFile(sumName)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if afterSums != nil {
		if err := files.Set("go.sum", afterSums, 0o644); err != nil {
			return err
		}
		return files.Apply(ctx)
	}
	return nil
}

func (s Service) prepareNode(ctx context.Context, in Input, fallback string) error {
	unlock, err := fsutil.WorkspaceLock(ctx, in.Root, "node-dependencies")
	if err != nil {
		return err
	}
	defer unlock()
	manager, err := workspace.ResolvePackageManager(in.Root, fallback)
	if err != nil {
		return err
	}
	var versions bytes.Buffer
	for _, args := range [][]string{{manager, "--version"}, {"node", "--version"}} {
		if err := s.run(ctx, in, in.Root, args, os.Environ(), &versions, in.Log); err != nil {
			return preparationError("Node workspace", in.Root, strings.Join(args, " "), err, versions.String())
		}
	}
	// Ask pnpm itself to validate installed dependencies. Its check covers the
	// workspace structure, manifest changes, lockfile, and installation settings.
	// The error policy never installs; stale or unsupported state falls through
	// to the ordinary install below. This also recognizes manual `pnpm install`.
	nativeCheck := in.Development && supportsPNPMDependencyCheck(versions.String())
	if nativeCheck {
		if nodeInstalled(in) {
			args := []string{"pnpm", "--config.verify-deps-before-run=error", "exec", "node", "--eval", ""}
			if err := s.run(ctx, in, in.Root, args, os.Environ(), io.Discard, io.Discard); err == nil {
				return nil
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	before, err := nodeFingerprint(in, versions.String())
	if err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	marker := filepath.Join(cache, "one", "dependencies", fmt.Sprintf("%x", sha256.Sum256([]byte(in.Root))))
	previous, _ := os.ReadFile(marker)
	if !nativeCheck && string(previous) == before && nodeInstalled(in) {
		return nil
	}
	args := NodeInstallCommand(in.Root, manager)
	if in.Development {
		args = []string{manager, "install", "--no-frozen-lockfile"}
	}
	fmt.Fprintf(in.Log, i18n.T("dependencies.node_preparing"), strings.Join(args, " "))
	var detail bytes.Buffer
	if err := s.run(ctx, in, in.Root, args, os.Environ(), in.Log, io.MultiWriter(in.Log, &detail)); err != nil {
		failure := preparationError("Node workspace", in.Root, strings.Join(args, " "), err, detail.String())
		if in.Log != io.Discard {
			// stderr has already been streamed. Keep it in structured context
			// without printing the same diagnostic again in the final summary.
			failure.Message = i18n.Tf("dependencies.failed_summary", "Node workspace", strings.Join(args, " "), err)
			failure.Context["stderr"] = detail.String()
		}
		return failure
	}
	if nativeCheck {
		return nil // pnpm owns the installed-state cache for this path.
	}
	after, err := nodeFingerprint(in, versions.String())
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(marker, []byte(after), 0o600)
}

// verifyDepsBeforeRun=error is supported by the pnpm versions we can verify
// without parsing private lockfile/state formats. Older/unknown versions keep
// the regular installation path and One's fingerprint cache.
func supportsPNPMDependencyCheck(versions string) bool {
	fields := strings.Fields(versions)
	if len(fields) == 0 {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(fields[0], "v"), ".")
	if len(parts) != 3 {
		return false
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	return majorErr == nil && minorErr == nil && (major > 10 || major == 10 && minor >= 14)
}

func nodeInstalled(in Input) bool {
	if !workspace.ProjectDependenciesInstalled(in.Root, in.Root, "node") {
		return false
	}
	for _, p := range in.Manifest.Projects {
		if p.Toolchain == "node" && !workspace.ProjectDependenciesInstalled(in.Root, filepath.Join(in.Root, filepath.FromSlash(p.RelativeDir)), "node") {
			return false
		}
	}
	return true
}

func nodeFingerprint(in Input, versions string) (string, error) {
	h := sha256.New()
	fmt.Fprintln(h, "node-dependencies-v3", in.Development, in.Runtime, versions)
	paths := []string{"package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml", ".npmrc"}
	dirs, err := nodePackageDirs(in)
	if err != nil {
		return "", err
	}
	for _, dir := range dirs {
		paths = append(paths, filepath.Join(dir, "package.json"), filepath.Join(dir, ".npmrc"))
	}
	for _, path := range paths {
		b, err := os.ReadFile(filepath.Join(in.Root, path))
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func NodeInstallCommand(root, manager string) []string {
	if manager != "pnpm" {
		return nil
	}
	return []string{"pnpm", "install", "--frozen-lockfile"}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func preparationError(project, dir, command string, err error, detail string) *output.Error {
	code := cliErrors.ONE_CLI_ERROR
	var missing *exec.Error
	if errors.As(err, &missing) {
		code = cliErrors.RUN_COMMAND_NOT_FOUND
	}
	context := map[string]any{"project": project, "directory": dir, "command": command}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		context["exit_code"] = exit.ExitCode()
	}
	return cliErrors.New(code, i18n.Tf("dependencies.failed", project, command, err, strings.TrimSpace(detail))).WithContext(context)
}

// Internal packages belong to the same install even when only their parent is
// registered in one.manifest.json or selected for development/building.
func nodePackageDirs(in Input) ([]string, error) {
	seen := map[string]bool{}
	var dirs []string
	for _, p := range in.Manifest.Projects {
		if p.Toolchain != "node" {
			continue
		}
		members, err := workspace.NodeProjectPackageDirs(in.Root, p.RelativeDir, nil)
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			if !seen[member] {
				seen[member] = true
				dirs = append(dirs, member)
			}
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}
