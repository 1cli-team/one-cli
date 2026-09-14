package mise

import (
	"runtime"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/runtime/mise/miserelease"
)

const managedVersion = miserelease.Version

type releaseAsset = miserelease.Asset

func currentAsset() (releaseAsset, error) {
	return miserelease.ForPlatform(runtime.GOOS, runtime.GOARCH)
}
func assetFor(goos, goarch string) (releaseAsset, error) {
	return miserelease.ForPlatform(goos, goarch)
}
