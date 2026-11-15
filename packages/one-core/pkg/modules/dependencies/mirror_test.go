package dependencies

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/mirror"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
)

func TestDependencyCommandReceivesMirrorsWithoutMise(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	if _, err := mirror.Select(t.Context(), mirror.EnvGroup, "public", false); err != nil {
		t.Fatal(err)
	}
	var got runtimeport.Command
	s := Service{Run: func(_ context.Context, command runtimeport.Command, _, _ io.Writer) error { got = command; return nil }}
	if err := s.run(t.Context(), Input{Runtime: runtimeport.Builtin}, t.TempDir(), []string{"pnpm", "install"}, []string{}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(got.Env, "\n"), "npm_config_registry=https://registry.npmmirror.com/") {
		t.Fatalf("registry missing: %q", got.Env)
	}
}
