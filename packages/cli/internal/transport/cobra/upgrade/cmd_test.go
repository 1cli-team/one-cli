package upgradecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/updatecheck"
	"gopkg.in/yaml.v3"
)

func TestUpgradeEmitsStructuredResults(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	for _, mode := range []output.Mode{output.ModeJSON, output.ModeYAML} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			output.SetMode(mode)
			stdout, err := os.CreateTemp(t.TempDir(), "stdout")
			if err != nil {
				t.Fatal(err)
			}
			defer stdout.Close()
			original := os.Stdout
			os.Stdout = stdout
			defer func() { os.Stdout = original }()
			root := &cobra.Command{Use: "one", Version: "1.0.0"}
			var stderr bytes.Buffer
			root.SetErr(&stderr)
			called := false
			root.AddCommand(command(func(ctx context.Context, version string) (*updatecheck.Result, error) {
				called = true
				if version != "1.0.0" || ctx == nil {
					t.Fatalf("update arguments: %s %v", version, ctx)
				}
				return &updatecheck.Result{Schema: "one-cli/upgrade/v1", CurrentVersion: version,
					LatestVersion: "v1.2.3", TargetPath: "/test/one", Status: "updated"}, nil
			}))
			root.SetArgs([]string{"upgrade"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(stdout.Name())
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if mode == output.ModeJSON {
				err = json.Unmarshal(raw, &result)
			} else {
				err = yaml.Unmarshal(raw, &result)
			}
			if err != nil || !called || stderr.Len() != 0 || result["schema"] != "one-cli/upgrade/v1" || result["status"] != "updated" || result["latest_version"] != "v1.2.3" {
				t.Fatalf("structured update: %q, stderr=%q, err=%v", raw, stderr.String(), err)
			}
		})
	}
}

func TestUpgradeArgumentValidationAndErrors(t *testing.T) {
	output.SetMode(output.ModeJSON)
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	want := errors.New("network unavailable")
	for _, args := range [][]string{{"extra"}, {}} {
		called := false
		cmd := command(func(context.Context, string) (*updatecheck.Result, error) {
			called = true
			return nil, want
		})
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetArgs(args)
		err := cmd.Execute()
		if len(args) > 0 {
			if called || err == nil {
				t.Fatalf("invalid arguments started update: called=%v err=%v", called, err)
			}
		} else if !called || !errors.Is(err, want) {
			t.Fatalf("update error lost: called=%v err=%v", called, err)
		}
	}
}

func TestUpgradeHelpAndResultsRefreshBetweenLocales(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	cmd := Command()
	for _, locale := range []string{"en-US", "zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		i18n.RefreshTree(cmd)
		var help bytes.Buffer
		cmd.SetOut(&help)
		if err := cmd.Help(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(help.String(), "one update") || len(cmd.Aliases) != 0 || cmd.Short != i18n.T("upgrade.short") || cmd.Long != i18n.T("upgrade.long") {
			t.Fatalf("%s help did not refresh: %s", locale, help.String())
		}
		for _, status := range []string{"current", "updated", "pending"} {
			var text bytes.Buffer
			result := upgradeResult{&updatecheck.Result{CurrentVersion: "1.0.0", LatestVersion: "v1.2.3", Status: status}}
			result.RenderTTY(&text)
			containsChinese := strings.ContainsFunc(help.String()+text.String(), func(r rune) bool { return unicode.Is(unicode.Han, r) })
			if text.Len() == 0 || containsChinese != (locale == "zh-CN") || !strings.Contains(text.String(), "v1.2.3") {
				t.Fatalf("%s %s output: %q", locale, status, text.String())
			}
		}
	}
}
