package envcmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestGlobalBindResultShowsStorageInBothLanguagesAndPreservesJSON(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	location := &environmentmodule.GlobalLocation{
		SiteURL: "https://secrets.example.com", UserID: "user", OrganizationID: "org",
		ProjectID: "shared-project", ProjectName: "shared-credentials", DefaultEnvironment: "dev",
	}
	result := bindOutput{location}
	for _, test := range []struct{ locale, want string }{
		{"zh-CN", "✓ 已绑定共享凭据项目：shared-credentials（shared-project），默认环境：dev。\n"},
		{"en-US", "✓ Bound shared credential project: shared-credentials (shared-project), default environment: dev.\n"},
	} {
		if err := i18n.Init(test.locale); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		result.RenderTTY(&out)
		if out.String() != test.want {
			t.Fatalf("%s binding output = %q, want %q", test.locale, out.String(), test.want)
		}
	}
	want, err := json.Marshal(location)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(result)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("binding JSON changed: %s, error: %v", got, err)
	}
}

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
