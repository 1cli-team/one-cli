package edgeone

// provider.go is the deploy/edgeone adapter onto the deploy.Provider
// interface. Apply pulls the API token from the resolved profile,
// reads the per-project ProjectName + env name from the manifest, and
// shells out via ops.go's Apply.

import (
	"context"
	"path/filepath"
	"sort"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

type providerImpl struct{}

func (providerImpl) ID() string { return workspace.DeployBackendEdgeOne }

func (providerImpl) Apply(ctx context.Context, in deploy.ApplyInput) (*deploy.ApplyResult, error) {
	if in.Resolved == nil || in.Resolved.Profile.EdgeOne == nil {
		return nil, cliErrors.New(cliErrors.EDGEONE_PROFILE_INVALID,
			"deploy/edgeone 需要先配置一个 deploy/edgeone profile。先 `one configure add deploy/edgeone --profile <name> --use`。")
	}
	ep := in.Resolved.Profile.EdgeOne
	apiToken := ""
	if ep.Credentials != nil {
		apiToken = ep.Credentials.APIToken
	}

	projectDir := projectDirFor(in)
	envName := envForProject(in.Manifest, in.Project.Name)
	projectName := projectNameForProject(in.Manifest, in.Project.Name)

	res, err := Apply(ctx, ApplyInput{
		ProjectDir:  projectDir,
		AssetDir:    defaultOutputDir(in.Project.TemplateID),
		APIToken:    apiToken,
		Region:      ep.Region,
		ProjectName: projectName,
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

// projectNameForProject reads projects[i].domains.deploy.config.projectName.
// Empty when no manifest pin is set; the edgeone CLI then falls back to
// edgeone.json or interactive prompt.
func projectNameForProject(m *workspace.Manifest, projectName string) string {
	cfg, _ := DecodeProjectConfig(m, projectName)
	if cfg == nil {
		return ""
	}
	return cfg.ProjectName
}
