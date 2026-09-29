package secrets

import (
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestEnvironmentSelectionUsesFixedDefaultAndRemoteSlugs(t *testing.T) {
	root := t.TempDir()
	m := &workspace.Manifest{Version: 2, Env: &workspace.EnvironmentConfig{ProjectID: "remote", Environments: []string{"prod", "dev", "qa"}}}
	if err := workspace.WriteManifest(root, m); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		flag, want     string
		allow, invalid bool
	}{
		{"", "dev", false, false}, {"qa", "qa", false, false}, {"production", "", false, true}, {"new-env", "new-env", true, false},
	} {
		got, _, err := ResolveEnvName(root, tc.flag, tc.allow)
		if (err != nil) != tc.invalid || got != tc.want {
			t.Errorf("flag %q = %q, %v", tc.flag, got, err)
		}
	}
}
