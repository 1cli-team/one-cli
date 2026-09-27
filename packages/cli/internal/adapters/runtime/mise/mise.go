// Package mise prepares commands using an external or One-managed mise CLI.
package mise

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

type Provider struct{}

func (p Provider) Prepare(ctx context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	command.Argv = append([]string{"exec", "--"}, command.Argv...)
	return p.PrepareCLI(ctx, command)
}

func (Provider) PrepareCLI(ctx context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	paths, pathsErr := defaultPaths()
	i := installer{root: paths.runtimeRoot(), legacyRoot: paths.legacyRoot(), downloader: defaultDownloader(), out: os.Stderr}
	r := binaryResolver{paths: paths, pathsErr: pathsErr, asset: currentAsset, install: i.ensure, out: os.Stderr}
	selected, err := r.resolve(ctx, command)
	if err != nil {
		return command, err
	}
	command.Argv = append([]string{selected.path}, command.Argv...)
	command.Env = prependRuntimePath(command.Env, filepath.Dir(selected.path))
	if selected.managed {
		command.Env = paths.environment(command.Env)
	}
	return command, nil
}

type resolvedBinary struct {
	path    string
	source  string
	managed bool
}

type binaryResolver struct {
	paths    runtimePaths
	pathsErr error
	asset    func() (releaseAsset, error)
	install  func(context.Context, releaseAsset) (string, error)
	out      io.Writer
}

func (r binaryResolver) resolve(ctx context.Context, command runtimeport.Command) (resolvedBinary, error) {
	if err := ctx.Err(); err != nil {
		return resolvedBinary{}, err
	}
	if override := os.Getenv("ONE_MISE_BINARY"); override != "" {
		path, err := filepath.Abs(override)
		if err != nil {
			return resolvedBinary{}, err
		}
		owned := r.paths.owns(path)
		if owned {
			if r.pathsErr != nil {
				return resolvedBinary{}, r.pathsErr
			}
			a, err := r.asset()
			if err != nil {
				return resolvedBinary{}, err
			}
			if err := verifyExecutable(canonicalPath(path), a.BinarySHA256); err != nil {
				return resolvedBinary{}, cliErrors.New(cliErrors.MISE_INSTALL_FAILED, i18n.Tf("mise.explicit_failed", err))
			}
		} else if err := checkVersion(ctx, path, command); err != nil {
			return resolvedBinary{}, err
		}
		return resolvedBinary{path: path, source: "explicit", managed: owned}, nil
	}
	for _, path := range systemCandidates(command.Env) {
		if r.paths.owns(path) {
			continue
		}
		if err := checkVersion(ctx, path, command); err == nil {
			return resolvedBinary{path: path, source: "system"}, nil
		} else if ctx.Err() != nil {
			return resolvedBinary{}, ctx.Err()
		} else if r.out != nil {
			fmt.Fprintf(r.out, i18n.T("mise.skipping"), path, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return resolvedBinary{}, err
	}
	if r.pathsErr != nil {
		return resolvedBinary{}, cliErrors.New(cliErrors.MISE_INSTALL_FAILED, r.pathsErr.Error())
	}
	a, err := r.asset()
	if err != nil {
		return resolvedBinary{}, cliErrors.New(cliErrors.MISE_INSTALL_FAILED, err.Error())
	}
	path, err := r.install(ctx, a)
	if err != nil {
		return resolvedBinary{}, cliErrors.New(cliErrors.MISE_INSTALL_FAILED, i18n.Tf("mise.prepare_failed", managedVersion, a.Platform, r.paths.runtimeRoot(), err)).WithRemediation(output.Remediation{
			Action: "prepare-mise", Hint: i18n.T("mise.prepare_hint"),
		})
	}
	// The executable digest pins the managed version; no online probe is needed.
	return resolvedBinary{path: path, source: "managed", managed: true}, nil
}

func systemCandidates(env []string) []string {
	names := []string{"mise"}
	if runtime.GOOS == "windows" {
		names = nil
		extensions := envValue(env, "PATHEXT")
		if extensions == "" {
			extensions = ".COM;.EXE;.BAT;.CMD"
		}
		for _, ext := range strings.Split(extensions, ";") {
			// Native executables only: no implicit shell or command-script execution.
			if strings.EqualFold(ext, ".exe") || strings.EqualFold(ext, ".com") {
				names = append(names, "mise"+strings.ToLower(ext))
			}
		}
	}
	var paths []string
	seen := make(map[string]bool)
	for _, dir := range filepath.SplitList(envValue(env, "PATH")) {
		// Like exec.ErrDot, do not silently run project-local executables.
		if !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range names {
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			key := canonicalPath(path)
			if runtime.GOOS == "windows" {
				key = strings.ToLower(key)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			paths = append(paths, path)
		}
	}
	return paths
}

func checkVersion(ctx context.Context, path string, command runtimeport.Command) error {
	if _, err := os.Stat(path); err != nil {
		return cliErrors.New(cliErrors.MISE_NOT_FOUND, i18n.Tf("mise.access_failed", err))
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	probe := exec.CommandContext(probeCtx, path, "--version")
	probe.WaitDelay = time.Second
	probe.Dir, probe.Env = command.Directory, pinnedRuntimeEnv(command.Env)
	version, err := probe.Output()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return cliErrors.New(cliErrors.MISE_VERSION_UNSUPPORTED, i18n.Tf("mise.version_failed", err))
	}
	if !supportedVersion(string(version)) {
		return cliErrors.New(cliErrors.MISE_VERSION_UNSUPPORTED, i18n.Tf("mise.version_required", runtimeport.MinimumMiseVersion))
	}
	return nil
}

// One owns updates to its managed runtime. These process-local settings also
// keep a --version probe from consulting the network or replacing the binary.
func pinnedRuntimeEnv(env []string) []string {
	return replaceEnv(env, "MISE_AUTO_UPDATE=false", "MISE_DISABLE_UPDATE_WARNING=true")
}

func prependRuntimePath(env []string, dir string) []string {
	value := envValue(env, "PATH")
	if value != "" {
		dir += string(os.PathListSeparator) + value
	}
	return replaceEnv(env, "PATH="+dir)
}

func supportedVersion(value string) bool {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(fields[0], "v"), ".")
	if len(parts) != 3 {
		return false
	}
	minimum := strings.Split(runtimeport.MinimumMiseVersion, ".")
	comparison := 0
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return false
		}
		if comparison == 0 {
			min, _ := strconv.Atoi(minimum[i])
			if n < min {
				comparison = -1
			}
			if n > min {
				comparison = 1
			}
		}
	}
	return comparison >= 0
}
