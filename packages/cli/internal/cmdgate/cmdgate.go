// Package cmdgate holds the workspace-state gate that per-domain
// commands (one container / dev / deploy) pass through before doing real work.
// It enforces:
//
//  1. cwd / -d resolves to a one-cli workspace (NOT_ONE_PROJECT otherwise)
//  2. manifest.<section> equals the expected backend id
//     (BACKEND_NOT_ENABLED otherwise, with remediation pointing at the
//     manifest section)
//
// Without these helpers, every per-domain cmd.go would copy ~40 lines
// of path math + manifest reading + structured-error construction. This
// package consolidates them so the not-enabled state surfaces with a
// uniform error code and remediation shape.
//
// Sibling helpers — RunExternal — also live here because backends that
// shell out to external tools (docker / kubectl / mprocs) want one
// shared way to wire stdin/stdout/stderr and propagate exit codes.
package cmdgate

import (
	"fmt"
	"os"
	"os/exec"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// DomainAccessor is the function each per-domain command provides to
// extract its currently-configured backend id from the manifest's
// domain section. The accessor is domain-specific (manifest.Env.Backend
// vs projects[i].Container vs projects[i].Deploy.Target); we keep
// it as a callback rather than a string-keyed map because the
// manifest's struct fields are typed in Go, not dynamic.
type DomainAccessor func(*workspace.Manifest) string

// RequireDomainEnabled is the standard gate for per-domain commands.
// Returns the absolute workspace root if and only if:
//
//   - cwd / dirFlag resolves to a directory containing one.manifest.json
//   - read(manifest) == expectedBackendID
//
// otherwise returns a structured error suitable for cli return.
//
// Parameters:
//   - dirFlag:           the value of --dir / -d ("" → cwd)
//   - domain:            human-readable domain name ("container",
//     "deploy", "env", ...) — surfaced in error context
//   - expectedBackendID: namespaced id ("container/docker") that must
//     equal the active backend in this domain
//   - read:              accessor extracting the domain's backend id
//     from the manifest. Accessors are responsible for their own nil
//     checks on the relevant manifest section.
func RequireDomainEnabled(dirFlag, domain, expectedBackendID string, read DomainAccessor) (string, error) {
	root, err := workspace.WalkUpToManifest(dirFlag)
	if err != nil {
		return "", err
	}
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return "", err
	}
	current := read(m)
	if current != expectedBackendID {
		return "", cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
			fmt.Sprintf("%s 未在该工作区启用。请在 one.manifest.json 中配置该域。",
				expectedBackendID)).
			WithContext(map[string]any{
				"workspace":        root,
				"required_backend": expectedBackendID,
				"domain":           domain,
				"current":          current,
			})
	}
	return root, nil
}

// RunExternal forwards stdin / stdout / stderr to an external process
// and propagates the exit code. The bin lookup is checked separately
// so a missing tool surfaces as RUN_COMMAND_NOT_FOUND with the right
// remediation hint, rather than a generic exec error.
func RunExternal(workdir string, args []string, missingHint string) error {
	if len(args) == 0 {
		return cliErrors.New(cliErrors.ONE_CLI_ERROR, "RunExternal: empty argv")
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		msg := fmt.Sprintf("%s 二进制不在 PATH 中", args[0])
		if missingHint != "" {
			msg += "；" + missingHint
		}
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND, msg)
	}
	c := exec.Command(args[0], args[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = workdir
	return c.Run()
}
