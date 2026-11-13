package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

// Match Skills CLI's v1 legacy hash and v2 portable hash. Keep the original
// lock version when registering skills so unrelated hashes stay valid.
// Hash captured bytes so the baseline check and eventual transaction agree.
const hashScript = `
const { createHash } = require('node:crypto');
const version = Number(process.argv[1]);
const included = path => {
  const parts = path.split('/');
  const directories = parts.slice(0, -1);
  if (version === 1) return !directories.some(part => part === '.git' || part === 'node_modules');
  return !parts.includes('metadata.json') &&
    !directories.some(part => ['.git', '__pycache__', '__pypackages__'].includes(part));
};
let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  const result = {};
  for (const [name, files] of Object.entries(JSON.parse(input))) {
    const hash = createHash('sha256');
    const paths = Object.keys(files).filter(included);
    paths.sort(version === 1
      ? (a, b) => a.localeCompare(b)
      : (a, b) => Buffer.compare(Buffer.from(a), Buffer.from(b)));
    for (const path of paths) {
      let content = Buffer.from(files[path], 'base64');
      if (version === 2 && !content.includes(0)) {
        content = Buffer.from(content.toString('latin1').replaceAll('\r\n', '\n'), 'latin1');
      }
      hash.update(path);
      if (version === 2) hash.update('\0' + content.length + '\0');
      hash.update(content);
    }
    result[name] = hash.digest('hex');
  }
  process.stdout.write(JSON.stringify(result));
});
`

func (r Runner) Hash(ctx context.Context, directory string, files map[string]Files, version int) (map[string]string, error) {
	if version != 1 && version != currentLockVersion {
		return nil, i18n.Errorf("skills.lock_unsupported", filepath.Join(directory, "skills-lock.json"))
	}
	raw, err := json.Marshal(files)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	diagnostics := &diagnosticLog{}
	if err := r.run(ctx, directory, "node", []string{"-e", hashScript, strconv.Itoa(version)}, bytes.NewReader(raw), &out, diagnostics, false); err != nil {
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
