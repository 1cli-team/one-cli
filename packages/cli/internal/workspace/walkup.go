package workspace

// walkup.go centralises "given a possibly-empty -d / --dir flag, find
// the workspace root by walking upward looking for one.manifest.json".
//
// Before this lived as 28-line private helpers in internal/cli/run.go,
// internal/cmdgate/cmdgate.go (formerly internal/infra/internalcommon/),
// and internal/secrets/dotenv/cmd.go. Each copy claimed the duplication
// was unavoidable to dodge import cycles. Now there's one canonical
// implementation everyone imports.

import (
	"os"
	"path/filepath"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// WalkUpToManifest resolves a possibly-empty --dir flag to an absolute
// workspace root, walking upward from the resolved start directory
// until a one.manifest.json is found. Returns the absolute path on
// success, or a NOT_ONE_PROJECT structured error if the walk reaches
// the filesystem root without finding the manifest.
//
// dirFlag semantics:
//   - "" or whitespace → walk up from cwd
//   - absolute path    → walk up from that path
//   - relative path    → join with cwd, walk up from there
//
// The walk-up behaviour is deliberate: domain commands and `one run` should
// work whether the user is at the workspace root or somewhere inside a
// project. We only stop at filesystem boundaries; we don't enforce a depth
// limit.
func WalkUpToManifest(dirFlag string) (string, error) {
	start := strings.TrimSpace(dirFlag)
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = cwd
	} else if !filepath.IsAbs(start) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = filepath.Join(cwd, start)
	}
	cur := filepath.Clean(start)
	for {
		if HasManifest(cur) {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", cliErrors.New(cliErrors.NOT_ONE_PROJECT,
				"找不到 one.manifest.json：从 "+start+" 向上每一级都没找到。")
		}
		cur = parent
	}
}
