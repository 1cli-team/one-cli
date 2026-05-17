package docker

// provider.go registers the four container.Provider IDs that share
// this package's docker-CLI transport: docker / dockerhub / ghcr /
// acr (Aliyun). Mirrors the parameterized providerImpl{kind} pattern
// in infra/s3compat — the kinds share Build / Push / Info wholesale;
// they only differ in how containercmd's profile.go resolves the
// machine-level profile into a container.Registry (host / namespace
// derivation lives in resolve.go).

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
)

type providerImpl struct{ kind string }

func (p providerImpl) ID() string { return p.kind }

func (providerImpl) Info(ctx context.Context, in container.InfoInput) (*container.InfoResult, error) {
	_ = ctx
	return Info(in)
}

func (providerImpl) Build(ctx context.Context, in container.BuildInput) (*container.BuildResult, error) {
	return Build(ctx, in)
}

func (providerImpl) Push(ctx context.Context, in container.PushInput) (*container.PushResult, error) {
	return Push(ctx, in)
}

func init() {
	for _, kind := range profile.ContainerKinds() {
		container.Register(providerImpl{kind: kind})
	}
}
