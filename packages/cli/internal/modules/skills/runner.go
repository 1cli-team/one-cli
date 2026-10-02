package skills

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// Runner prepares Node when absent, without adding a Node project or loading
// One credentials. npx's --yes accepts npm's bootstrap prompt, not skills prompts.
type Runner struct{ Runtime runtimeport.Provider }

func (r Runner) Run(ctx context.Context, directory string, args []string, in io.Reader, out, errOut io.Writer) error {
	env := os.Environ()
	argv := append([]string{"npx", "--yes", "skills@" + CLIVersion}, args...)
	prepared := runtimeport.Command{Directory: directory, Argv: argv, Env: env}
	if path, err := platformprocess.LookPathIn("npx", directory, env); err == nil {
		prepared.Argv[0] = path
	} else {
		if r.Runtime == nil {
			return i18n.Errorf("skills.node_required", err)
		}
		prepared.Argv = append([]string{"exec", "node@24.21.0", "--"}, argv...)
		var prepareErr error
		prepared, prepareErr = r.Runtime.PrepareCLI(ctx, prepared)
		if prepareErr != nil {
			return i18n.Errorf("skills.prepare_failed", prepareErr)
		}
	}
	child := platformprocess.CommandContext(ctx, prepared.Argv[0], prepared.Argv[1:]...)
	child.Dir, child.Env = prepared.Directory, prepared.Env
	child.Stdin, child.Stdout, child.Stderr = in, out, errOut
	if in == nil {
		platformprocess.CancelProcessTree(child)
	} else {
		// Keep interactive children in the foreground terminal group. A
		// cancelled request still stops npm, node, git and their descendants.
		child.Cancel = func() error {
			if child.Process == nil {
				return os.ErrProcessDone
			}
			return platformprocess.StopTree(child.Process)
		}
		child.WaitDelay = 5 * time.Second
	}
	return platformprocess.RunForwarded(ctx, child)
}
