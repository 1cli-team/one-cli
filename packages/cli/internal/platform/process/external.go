// Package process contains shared helpers for invoking child processes.
package process

import (
	"fmt"
	"os"
	"os/exec"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// RunExternal forwards stdin, stdout, and stderr to an external process.
// It checks binary availability first so callers receive a structured error
// with a useful remediation hint.
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
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = workdir
	return cmd.Run()
}
