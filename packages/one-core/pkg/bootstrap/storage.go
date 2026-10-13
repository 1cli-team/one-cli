package bootstrap

import (
	"os"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

// InitializeStorage creates One's current directories without importing,
// inspecting, or cleaning up data from earlier layouts. Existing files stay put.
func InitializeStorage() (userdirs.Paths, error) {
	p, err := userdirs.Resolve()
	if err != nil {
		return p, i18n.Errorf("storage.failed", err)
	}
	for _, dir := range []string{p.Root, p.Config, p.Data, p.State, p.Cache, p.Temp} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return p, i18n.Errorf("storage.failed", err)
		}
	}
	return p, nil
}
