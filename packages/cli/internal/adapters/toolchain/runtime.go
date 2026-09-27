package adapters

import (
	"regexp"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

func resolvePackageManager(pm toolchain.PackageManager) toolchain.PackageManager {
	if pm == "" {
		return toolchain.PMpnpm
	}
	return pm
}

// resolveNodeInstallCommand mirrors the TS helper: pnpm/npm/yarn with an
// optional --frozen-lockfile (for CI installs).
func resolveNodeInstallCommand(pm toolchain.PackageManager, frozen bool) string {
	switch pm {
	case toolchain.PMnpm:
		if frozen {
			return "npm ci"
		}
		return "npm install"
	case toolchain.PMyarn:
		if frozen {
			return "yarn install --frozen-lockfile"
		}
		return "yarn install"
	default: // pnpm
		if frozen {
			return "pnpm install --frozen-lockfile"
		}
		return "pnpm install"
	}
}

// resolveLockfileByPM returns the lockfile name for a given package manager.
// Used to wire actions/setup-node's `cache-dependency-path`.
func resolveLockfileByPM(pm toolchain.PackageManager) string {
	switch pm {
	case toolchain.PMnpm:
		return "package-lock.json"
	case toolchain.PMyarn:
		return "yarn.lock"
	default:
		return "pnpm-lock.yaml"
	}
}

// resolveRunScriptCommand assembles `<pm> run <script> [-- args]`. yarn does
// not need the `--` separator before args.
func resolveRunScriptCommand(pm toolchain.PackageManager, scriptName, forwardedArgs string) string {
	args := strings.TrimSpace(forwardedArgs)
	switch pm {
	case toolchain.PMnpm:
		base := "npm run " + scriptName
		if args != "" {
			return base + " -- " + args
		}
		return base
	case toolchain.PMyarn:
		base := "yarn " + scriptName
		if args != "" {
			return base + " " + args
		}
		return base
	default: // pnpm
		base := "pnpm run " + scriptName
		if args != "" {
			return base + " -- " + args
		}
		return base
	}
}

// watchScriptRE matches test scripts that include --watch / watchAll. We
// disable watch mode in CI by appending --watchAll=false --runInBand.
var watchScriptRE = regexp.MustCompile(`(?i)(^|\s)--watch|watch(All)?`)

func resolveTestCommand(scripts map[string]string, pm toolchain.PackageManager) string {
	test, ok := scripts["test"]
	if !ok || test == "" {
		return ""
	}
	if watchScriptRE.MatchString(test) {
		return resolveRunScriptCommand(pm, "test", "--watchAll=false --runInBand")
	}
	return resolveRunScriptCommand(pm, "test", "")
}

// resolveNodeCiCommands picks the sequence of script invocations the GitHub
// Actions workflow should run. Prefer `check` if defined, else `lint` /
// `format`, then `test`, then `build`. If nothing is wired, emit a
// placeholder so the workflow still shows up.
func resolveNodeCiCommands(scripts map[string]string, pm toolchain.PackageManager) []string {
	cmds := []string{}
	if _, ok := scripts["check"]; ok {
		cmds = append(cmds, resolveRunScriptCommand(pm, "check", ""))
	} else {
		if _, ok := scripts["lint"]; ok {
			cmds = append(cmds, resolveRunScriptCommand(pm, "lint", ""))
		}
		if _, ok := scripts["format"]; ok {
			cmds = append(cmds, resolveRunScriptCommand(pm, "format", ""))
		}
	}
	if t := resolveTestCommand(scripts, pm); t != "" {
		cmds = append(cmds, t)
	}
	if _, ok := scripts["build"]; ok {
		cmds = append(cmds, resolveRunScriptCommand(pm, "build", ""))
	}
	if len(cmds) == 0 {
		cmds = append(cmds, `echo "No CI scripts configured for this subproject."`)
	}
	return cmds
}
