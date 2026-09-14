// Package mise prepares commands through an explicitly selected or bundled mise CLI.
package mise

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

type Provider struct{}

func (p Provider) Prepare(ctx context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	command.Argv = append([]string{"exec", "--"}, command.Argv...)
	return p.PrepareCLI(ctx, command)
}

func (Provider) PrepareCLI(ctx context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	path, err := resolveBinary(ctx, command)
	if err != nil {
		return command, err
	}
	command.Argv = append([]string{path}, command.Argv...)
	command.Env = prependRuntimePath(command.Env, filepath.Dir(path))
	if os.Getenv("ONE_MISE_BINARY") == "" {
		command.Env = pinnedRuntimeEnv(command.Env)
	}
	return command, nil
}

func resolveBinary(ctx context.Context, command runtimeport.Command) (string, error) {
	if override := os.Getenv("ONE_MISE_BINARY"); override != "" {
		path, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		if err := checkVersion(ctx, path, command); err != nil {
			return "", err
		}
		return path, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	a, err := currentAsset()
	if err != nil {
		return "", cliErrors.New(cliErrors.MISE_INSTALL_FAILED, err.Error())
	}
	i, err := defaultInstaller()
	if err != nil {
		return "", cliErrors.New(cliErrors.MISE_INSTALL_FAILED, err.Error())
	}
	path, err := i.ensure(ctx, a)
	if err != nil {
		return "", cliErrors.New(cliErrors.MISE_INSTALL_FAILED, "Could not prepare bundled mise: "+err.Error()).WithRemediation(output.Remediation{
			Action: "prepare-mise", Hint: "Ensure the One runtime cache is writable and executable, then retry. Reinstall One if its bundled archive is damaged, or set ONE_MISE_BINARY to a compatible executable. ONE_RUNTIME=builtin uses existing tools for diagnostics.",
		})
	}
	// The executable digest already pins the bundled version. Probing it again
	// would add startup work and mise's default online update check.
	return path, nil
}

func checkVersion(ctx context.Context, path string, command runtimeport.Command) error {
	if _, err := os.Stat(path); err != nil {
		return cliErrors.New(cliErrors.MISE_NOT_FOUND, "Cannot access the mise executable: "+err.Error())
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	probe := exec.CommandContext(probeCtx, path, "--version")
	probe.Dir, probe.Env = command.Directory, pinnedRuntimeEnv(command.Env)
	version, err := probe.Output()
	if err != nil {
		return cliErrors.New(cliErrors.MISE_VERSION_UNSUPPORTED, "Could not read the mise version: "+err.Error())
	}
	if !supportedVersion(string(version)) {
		return cliErrors.New(cliErrors.MISE_VERSION_UNSUPPORTED, fmt.Sprintf("mise %s or newer is required.", runtimeport.MinimumMiseVersion))
	}
	return nil
}

// One owns updates to its bundled runtime. These process-local settings also
// keep a --version probe from consulting the network or replacing the binary.
func pinnedRuntimeEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	result := make([]string, 0, len(env)+2)
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		if key != "MISE_AUTO_UPDATE" && key != "MISE_DISABLE_UPDATE_WARNING" {
			result = append(result, kv)
		}
	}
	return append(result, "MISE_AUTO_UPDATE=false", "MISE_DISABLE_UPDATE_WARNING=true")
}

func prependRuntimePath(env []string, dir string) []string {
	if env == nil {
		env = os.Environ()
	}
	result := append([]string(nil), env...)
	for i, kv := range result {
		key, value, found := strings.Cut(kv, "=")
		if found && (key == "PATH" || runtime.GOOS == "windows" && strings.EqualFold(key, "PATH")) {
			result[i] = key + "=" + dir + string(os.PathListSeparator) + value
			return result
		}
	}
	return append(result, "PATH="+dir)
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
	minimum := []int{2026, 9, 7}
	comparison := 0
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return false
		}
		if comparison == 0 {
			if n < minimum[i] {
				comparison = -1
			}
			if n > minimum[i] {
				comparison = 1
			}
		}
	}
	return comparison >= 0
}
