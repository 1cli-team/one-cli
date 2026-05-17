// Package dotenv is the filesystem-backed secrets provider. It reads
// (and only reads — writes happen via `one infisical pull` or hand
// editing) <subproject>/.env files for `one run` injection, and
// exposes `one dotenv list` / `one dotenv get` so users can inspect
// the local file without a third-party tool.
//
// The point of having this as a real backend (and not just a
// hardcoded fallback inside `one run`) is to validate the secrets
// Loader contract under a second implementation. Without that
// validation, the contract would silently shape itself around
// Infisical and break for any future backend.
package dotenv

import (
	"context"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
)

type runLoader struct{}

func (runLoader) ID() string { return "dotenv" }

// Priority is "filesystem": checked last in --from auto, after every
// remote backend has declined. A workspace with no remote secrets
// backend configured will resolve to dotenv on every `one run`.
func (runLoader) Priority() secrets.Priority { return secrets.PriorityFilesystem }

// Available is true unconditionally: every workspace can have a .env,
// and missing files are not a failure mode. We deliberately don't probe
// for file existence here — that would let --from auto fall through to
// "no backend available" when only the .env happens to be absent, which
// is a worse signal than "dotenv was picked and produced zero vars".
func (runLoader) Available(projectRoot string) bool { return true }

// Load reads the dotenv overlay for <projectRoot>/<relativeDir>. When
// envName is non-empty, the chain is:
//
//	.env  →  .env.<env>  →  .env.<env>.local
//
// (with `.env.local` inserted between .env and .env.<env>.local when
// envName is empty). Later layers override earlier ones; any subset may
// exist. Missing files are not an error — they downgrade to an empty
// map and the caller decides what to do (one run prints a friendly
// notice; one deploy proceeds silently). The previous strict mode
// (RUN_DOTENV_MISSING on absent file) interrupted first-time users who
// hadn't yet learned that .env is opt-in.
func (runLoader) Load(_ context.Context, projectRoot, relativeDir, envName string) (map[string]string, error) {
	subDir := filepath.Join(projectRoot, relativeDir)
	chain := overlayChain(subDir, envName)
	merged, _, err := loadOverlay(chain)
	if err != nil {
		return nil, err
	}
	return merged, nil
}

func init() {
	secrets.Register(runLoader{})
}
