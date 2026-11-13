package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
)

// Runner prepares Node when absent, without adding a Node project or loading
// One credentials. npx's --yes accepts npm's bootstrap prompt, not skills prompts.
type Runner struct{ Runtime runtimeport.Provider }

func (r Runner) Run(ctx context.Context, directory string, args []string, in io.Reader, out, errOut io.Writer) error {
	argv := append([]string{"--yes", "skills@" + CLIVersion}, args...)
	return r.run(ctx, directory, "npx", argv, in, out, errOut, in != nil)
}

// Match skills@1.7.0/src/local-lock.ts exactly. Node's localeCompare ordering
// differs from Go's lexical sorting. Hash captured bytes rather than rereading
// live files, so the baseline check and the eventual transaction agree.
const hashScript = `
const { createHash } = require('node:crypto');
let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  const result = {};
  for (const [name, files] of Object.entries(JSON.parse(input))) {
    const hash = createHash('sha256');
    for (const path of Object.keys(files).sort((a, b) => a.localeCompare(b))) {
      hash.update(path);
      hash.update(Buffer.from(files[path], 'base64'));
    }
    result[name] = hash.digest('hex');
  }
  process.stdout.write(JSON.stringify(result));
});
`

func (r Runner) Hash(ctx context.Context, directory string, files map[string]Files) (map[string]string, error) {
	raw, err := json.Marshal(files)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	diagnostics := &diagnosticLog{}
	if err := r.run(ctx, directory, "node", []string{"-e", hashScript}, bytes.NewReader(raw), &out, diagnostics, false); err != nil {
		return nil, i18n.Errorf("skills.command_failed", platformprocess.ExitCode(err), err, diagnostics.text())
	}
	var hashes map[string]string
	if err := json.Unmarshal(out.Bytes(), &hashes); err != nil {
		return nil, i18n.Errorf("skills.hash_output_invalid", err)
	}
	return hashes, nil
}

func (r Runner) run(ctx context.Context, directory, binary string, args []string, in io.Reader, out, errOut io.Writer, interactive bool) error {
	env := os.Environ()
	argv := append([]string{binary}, args...)
	prepared := runtimeport.Command{Directory: directory, Argv: argv, Env: env}
	if path, err := platformprocess.LookPathIn(binary, directory, env); err == nil {
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
	if !interactive {
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
