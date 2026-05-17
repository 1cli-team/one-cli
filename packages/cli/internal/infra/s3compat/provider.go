package s3compat

// provider.go adapts s3compat to the deploy.Provider interface. One
// parameterised providerImpl per backend id; all six share the same
// Apply implementation because the underlying upload protocol is
// identical — only the user-visible id, prompts, and defaults differ
// (those live in configurecmd, not here).

import (
	"context"
	"fmt"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

type providerImpl struct {
	kind string
}

func (p providerImpl) ID() string { return p.kind }

func (p providerImpl) Apply(ctx context.Context, in deploy.ApplyInput) (*deploy.ApplyResult, error) {
	if in.Resolved == nil || in.Resolved.Profile.S3 == nil {
		return nil, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
			fmt.Sprintf("deploy/%s 缺少 profile；先 `one configure add deploy/%s <name> --use`。", p.kind, p.kind))
	}
	ep, creds := endpointAndCredsFromProfile(in.Resolved, in.Manifest, in.Project.Name)
	res, err := Apply(ctx, ApplyInput{
		ProjectRoot: in.ProjectRoot,
		Subproject: &Subproject{
			Name:        in.Project.Name,
			RelativeDir: in.Project.RelativeDir,
			Toolchain:   in.Toolchain,
		},
		Endpoint:    ep,
		Credentials: creds,
		DryRun:      in.DryRun,
		Kind:        p.kind,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &deploy.ApplyResult{
		Schema: res.Schema,
		Argv:   res.Argv,
		DryRun: res.DryRun,
	}, nil
}

func init() {
	for _, kind := range []string{
		workspace.DeployBackendAliyunOSS,
		workspace.DeployBackendTencentCOS,
		workspace.DeployBackendAWSS3,
		workspace.DeployBackendMinIO,
		workspace.DeployBackendRustFS,
		workspace.DeployBackendR2,
	} {
		deploy.Register(providerImpl{kind: kind})
	}
}

// endpointAndCredsFromProfile pulls the s3 endpoint + credentials out
// of the resolved profile, and threads in the per-project bucket from
// the manifest. The profile holds endpoint + credentials (machine-
// level); the manifest holds the per-project bucket override.
func endpointAndCredsFromProfile(resolved *profile.Resolved, m *workspace.Manifest, projectName string) (*Endpoint, *Credentials) {
	if resolved == nil || resolved.Profile.S3 == nil {
		return nil, nil
	}
	src := resolved.Profile.S3
	ep := &Endpoint{
		Endpoint:       src.Endpoint,
		Region:         src.Region,
		Bucket:         workspace.DeployBucketForProject(m, projectName),
		ForcePathStyle: src.ForcePathStyle,
	}
	var creds *Credentials
	if src.Credentials != nil {
		creds = &Credentials{
			AccessKeyID:     src.Credentials.AccessKeyID,
			AccessKeySecret: src.Credentials.AccessKeySecret,
		}
	}
	return ep, creds
}
