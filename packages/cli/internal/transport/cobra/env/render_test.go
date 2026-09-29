package envcmd

import (
	"bytes"
	"strings"
	"testing"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestSetResultIdentifiesActualRemoteProjectInBothLanguages(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		var out bytes.Buffer
		result := setOutput{&environmentmodule.SetResult{Key: "API_TOKEN", Environment: "dev", Path: "/", Binding: &environmentmodule.BindingResult{ProjectID: "remote-new", ProjectName: "demo-a3f2", Created: true, RequestedName: "demo"}}}
		result.RenderTTY(&out)
		for _, want := range []string{"demo-a3f2", "remote-new", "API_TOKEN", "dev"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%s missing %q: %s", locale, want, out.String())
			}
		}
		if strings.Contains(out.String(), "env.project_") {
			t.Fatalf("unresolved translation: %s", out.String())
		}
	}
}
