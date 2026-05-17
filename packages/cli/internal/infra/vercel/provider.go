package vercel

// provider.go is the deploy/vercel adapter onto the deploy.Provider
// interface. Apply pulls the API token + team scope from the resolved
// profile, reads the per-project env name from the manifest, and shells
// out via ops.go's Apply.

import (
	"context"
	"path/filepath"
	"sort"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

type providerImpl struct{}

func (providerImpl) ID() string { return workspace.DeployBackendVercel }

func (providerImpl) Apply(ctx context.Context, in deploy.ApplyInput) (*deploy.ApplyResult, error) {
	if in.Resolved == nil || in.Resolved.Profile.Vercel == nil {
		return nil, cliErrors.New(cliErrors.VERCEL_PROFILE_INVALID,
			"deploy/vercel 需要先配置一个 deploy/vercel profile。先 `one configure add deploy/vercel --profile <name> --use`。")
	}
	vp := in.Resolved.Profile.Vercel
	if vp.Credentials == nil || vp.Credentials.APIToken == "" {
		return nil, cliErrors.New(cliErrors.VERCEL_PROFILE_INVALID,
			"deploy/vercel profile 缺 API token。在 vercel.com → Account Settings → Tokens 创建后重新跑 `one configure add deploy/vercel --profile <name>`。")
	}

	projectDir := projectDirFor(in)
	envName := envForProject(in.Manifest, in.Project.Name)

	res, err := Apply(ctx, ApplyInput{
		ProjectDir:  projectDir,
		APIToken:    vp.Credentials.APIToken,
		Team:        vp.Team,
		Env:         envName,
		DryRun:      in.DryRun,
		InjectedEnv: in.InjectedEnv,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &deploy.ApplyResult{
		Schema:            res.Schema,
		Argv:              res.Argv,
		CommandLines:      res.CommandLines,
		DryRun:            res.DryRun,
		InjectedEnvKeys:   sortedKeys(in.InjectedEnv),
		InjectedEnvSource: in.InjectedEnvSource,
	}, nil
}

// sortedKeys returns the map's keys in alphabetical order. Returns nil
// for nil / empty input so the resulting ApplyResult field is omitted
// from the JSON envelope when there is no injection.
func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func init() {
	deploy.Register(providerImpl{})
}

// projectDirFor returns the absolute filesystem dir for the project.
// Project.TargetDir is set by deploycmd; fall through to ProjectRoot +
// RelativeDir for callers that don't pre-resolve it.
func projectDirFor(in deploy.ApplyInput) string {
	if in.Project.TargetDir != "" {
		return in.Project.TargetDir
	}
	return filepath.Join(in.ProjectRoot, filepath.FromSlash(in.Project.RelativeDir))
}

// envForProject reads projects[i].domains.deploy.config.env. Empty when
// the manifest does not pin a value; ops.isProduction treats empty as the
// production tier, preserving the prior "default to production" behaviour.
func envForProject(m *workspace.Manifest, projectName string) string {
	cfg, _ := DecodeProjectConfig(m, projectName)
	if cfg == nil {
		return ""
	}
	return cfg.Env
}
