// Package process contains shared helpers for invoking child processes.
package process

import (
	"os"
	"os/exec"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// RunExternal forwards stdin, stdout, and stderr to an external process.
// It checks binary availability first so callers receive a structured error
// with a useful remediation hint.
func RunExternal(workdir string, args []string, missingHint string) error {
	if len(args) == 0 {
		return cliErrors.New(cliErrors.ONE_CLI_ERROR, i18n.T("process.command_empty"))
	}
	if _, err := exec.LookPath(args[0]); err != nil {
		msg := i18n.Tf("process.binary_missing", args[0])
		if missingHint != "" {
			msg += "；" + missingHint
		}
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND, msg)
	}
	cmd := Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = workdir
	return cmd.Run()
}
