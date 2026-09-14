// Package dependencies prepares application dependencies before development.
// Tool installation belongs to the runtime provider; run remains a plain runner.
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
	"strings"
	"syscall"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"gopkg.in/yaml.v3"
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
	Runtime  string
	Log      io.Writer
}

func (s Service) Prepare(ctx context.Context, in Input) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	if in.Log == nil {
		in.Log = io.Discard
	}
	var nodes, goProjects []workspace.ManifestProject
	for _, p := range in.Manifest.Projects {
		if (in.Project != "" && p.Name != in.Project) || strings.TrimSpace(workspace.ProjectDev(in.Manifest, p.Name)) == "" {
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
			return fmt.Errorf("mise provider is required")
		}
		var err error
		command, err = s.Provider.Prepare(ctx, command)
		if err != nil {
			return err
		}
	}
	if len(command.Argv) == 0 {
		return fmt.Errorf("dependency runtime returned an empty command")
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
	dir := filepath.Join(in.Root, filepath.FromSlash(p.RelativeDir))
	var work bytes.Buffer
	if err := s.run(ctx, in, dir, []string{"go", "env", "GOWORK"}, os.Environ(), &work, in.Log); err != nil {
		return preparationError(p.Name, dir, "resolve Go workspace", err, work.String())
	}
	active := strings.TrimSpace(work.String())
	lockRoot := dir
	if active != "" && active != "off" {
		want := filepath.Join(in.Root, "go.work")
		if filepath.Clean(active) != filepath.Clean(want) {
			// Go may return the physical path while the workspace root uses
			// a symlink (for example /var and /private/var on macOS).
			activeInfo, activeErr := os.Stat(active)
			wantInfo, wantErr := os.Stat(want)
			if activeErr != nil || wantErr != nil || !os.SameFile(activeInfo, wantInfo) {
				return fmt.Errorf("%s uses external GOWORK=%s; use %s or GOWORK=off explicitly", p.Name, active, want)
			}
		}
		lockRoot = in.Root
	}
	unlock, err := fsutil.WorkspaceLock(ctx, lockRoot, "go-dependencies")
	if err != nil {
		return err
	}
	defer unlock()
	fmt.Fprintf(in.Log, "[one] %s: preparing Go dependencies\n", p.Name)
	// In workspace mode, package loading uses the actual Go build graph and
	// writes workspace sums as needed. Expanding `all` can fetch historical
	// versions of local members; only use it for a standalone module.
	commands := [][]string{}
	if active == "" || active == "off" {
		if err := s.downloadModule(ctx, in, p, dir); err != nil {
			return err
		}
	}
	commands = append(commands, []string{"go", "list", "-mod=readonly", "-buildvcs=false", "-deps", "./..."})
	for _, args := range commands {
		var detail bytes.Buffer
		out := io.Discard

		if err := s.run(ctx, in, dir, args, os.Environ(), out, io.MultiWriter(in.Log, &detail)); err != nil {
			failure := preparationError(p.Name, dir, strings.Join(args, " "), err, detail.String())
			if args[1] == "list" {
				return failure.WithRemediation(output.Remediation{Action: "repair-go-module", Hint: "Inspect the Go error. If module declarations need repair, run tidy explicitly.", Command: "one run " + p.Name + " -- go mod tidy"})
			}
			return failure
		}
	}
	return nil
}

// Go may rewrite go/toolchain directives during mod download. Use an alternate
// module file and publish only checksums after verifying the declarations stayed
// unchanged; never silently repair or upgrade the user's go.mod.
func (s Service) downloadModule(ctx context.Context, in Input, p workspace.ManifestProject, dir string) error {
	files := fsutil.NewFilePlan(dir)
	module, err := files.Read("go.mod")
	if err != nil {
		return err
	}
	if module == nil {
		return fmt.Errorf("missing go.mod for %s", p.Name)
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
	if err := s.run(ctx, in, dir, args, os.Environ(), io.Discard, io.MultiWriter(in.Log, &detail)); err != nil {
		return preparationError(p.Name, dir, "go mod download all", err, detail.String())
	}
	after, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if !bytes.Equal(module, after) {
		return cliErrors.New(cliErrors.ONE_CLI_ERROR, "Go dependencies require changes to "+p.Name+"/go.mod; automatic preparation preserved the original file").WithRemediation(output.Remediation{
			Action: "repair-go-module", Command: "one run " + p.Name + " -- go mod tidy", Hint: "Review the module declarations and run tidy explicitly.",
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
	manager := PackageManager(in.Root, fallback)
	var versions bytes.Buffer
	for _, args := range [][]string{{manager, "--version"}, {"node", "--version"}} {
		if err := s.run(ctx, in, in.Root, args, os.Environ(), &versions, in.Log); err != nil {
			return preparationError("Node workspace", in.Root, strings.Join(args, " "), err, versions.String())
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
	if string(previous) == before && nodeInstalled(in) {
		return nil
	}
	args := NodeInstallCommand(in.Root, manager)
	fmt.Fprintf(in.Log, "[one] Preparing Node workspace dependencies: %s\n", strings.Join(args, " "))
	var detail bytes.Buffer
	if err := s.run(ctx, in, in.Root, args, os.Environ(), in.Log, io.MultiWriter(in.Log, &detail)); err != nil {
		return preparationError("Node workspace", in.Root, strings.Join(args, " "), err, detail.String())
	}
	after, err := nodeFingerprint(in, versions.String())
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(marker, []byte(after), 0o600)
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
	fmt.Fprintln(h, versions)
	paths := []string{"package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml", "package-lock.json", "yarn.lock", "bun.lock", "bun.lockb", ".npmrc", ".yarnrc.yml"}
	for _, p := range in.Manifest.Projects {
		if p.Toolchain == "node" {
			paths = append(paths, filepath.Join(p.RelativeDir, "package.json"))
		}
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

func PackageManager(root, fallback string) string {
	manager := strings.TrimSpace(fallback)
	if pkg, err := workspace.ReadPackageJSON(root); err == nil && pkg != nil && pkg.PackageManager != "" {
		manager = pkg.PackageManager
	}
	manager, _, _ = strings.Cut(manager, "@")
	if manager != "" {
		return manager
	}
	for _, item := range []struct{ file, manager string }{{"bun.lock", "bun"}, {"bun.lockb", "bun"}, {"yarn.lock", "yarn"}, {"package-lock.json", "npm"}} {
		if exists(filepath.Join(root, item.file)) {
			return item.manager
		}
	}
	return "pnpm"
}

func NodeInstallCommand(root, manager string) []string {
	switch manager {
	case "pnpm":
		if pnpmDependencyLock(filepath.Join(root, "pnpm-lock.yaml")) {
			return []string{manager, "install", "--frozen-lockfile"}
		}
		return []string{manager, "install", "--no-frozen-lockfile"}
	case "npm":
		if exists(filepath.Join(root, "package-lock.json")) {
			return []string{manager, "ci"}
		}
	case "yarn":
		if exists(filepath.Join(root, "yarn.lock")) {
			pkg, _ := workspace.ReadPackageJSON(root)
			if pkg != nil && strings.HasPrefix(pkg.PackageManager, "yarn@1.") {
				return []string{manager, "install", "--frozen-lockfile"}
			}
			return []string{manager, "install", "--immutable"}
		}
	case "bun":
		if exists(filepath.Join(root, "bun.lock")) || exists(filepath.Join(root, "bun.lockb")) {
			return []string{manager, "install", "--frozen-lockfile"}
		}
	}
	return []string{manager, "install"}
}

// pnpm 12 may write an environment-only YAML document even for --version.
// That records packageManagerDependencies, not installed application packages.
// Read every document; keep malformed/unknown existing locks frozen so pnpm
// reports the problem rather than silently regenerating them.
func pnpmDependencyLock(path string) bool {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	documents := 0
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err == io.EOF {
			return documents == 0
		}
		if err != nil {
			return true
		}
		// pnpm separates the not-yet-created application document with a
		// trailing `---`; the YAML decoder returns a nil document for it.
		if doc == nil {
			continue
		}
		documents++
		importers, ok := doc["importers"].(map[string]any)
		if !ok || len(importers) == 0 {
			return true
		}
		for _, value := range importers {
			fields, ok := value.(map[string]any)
			if !ok || len(fields) == 0 {
				return true
			}
			for key := range fields {
				if key != "configDependencies" && key != "packageManagerDependencies" {
					return true
				}
			}
		}
	}
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
	return cliErrors.New(code, fmt.Sprintf("%s dependency preparation failed (%s): %v\n%s", project, command, err, strings.TrimSpace(detail))).WithContext(context)
}
