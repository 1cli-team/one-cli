package mirrorcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/mirror"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func TestCommandLifecycleWithoutMiseOrWorkspace(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ONE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	output.SetMode(output.ModeJSON)
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	for _, args := range [][]string{{"use", "public", "--dry-run"}, {"use", "public"}, {"status"}, {"list"}, {"use", "huawei"}, {"reset"}} {
		cmd := Command()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		cfg, err := mirror.Load()
		if err != nil {
			t.Fatal(err)
		}
		if args[0] == "reset" || len(args) == 3 {
			if cfg != nil {
				t.Fatalf("unexpected configuration: %+v", cfg)
			}
		} else if args[0] == "use" && (cfg == nil || cfg.Env == nil || cfg.Env.Preset != args[1]) {
			t.Fatalf("selected configuration = %+v, want %s", cfg, args[1])
		}
	}
	path, _ := mirror.Path()
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := Command()
	cmd.SetArgs([]string{"reset"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "preferences.json")); !os.IsNotExist(err) {
		t.Fatal("created unrelated preferences")
	}
}

func TestMirrorHelpRefreshesBothLanguages(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init("en-US") })
	root := &cobra.Command{Use: "one"}
	cmd := Command()
	root.AddCommand(cmd)
	for _, test := range []struct{ locale, short, flag string }{
		{"en-US", "Manage download mirror presets", "Preview without changing"},
		{"zh-CN", "管理下载镜像预设", "预览变更"},
		{"en-US", "Manage download mirror presets", "Preview without changing"},
	} {
		if err := i18n.Init(test.locale); err != nil {
			t.Fatal(err)
		}
		i18n.RefreshTree(root)
		use, _, _ := cmd.Find([]string{"use"})
		check, _, _ := cmd.Find([]string{"test"})
		toolUse, _, _ := cmd.Find([]string{"tools", "use"})
		envUse, _, _ := cmd.Find([]string{"env", "use"})
		if strings.Join(use.ValidArgs, ",") != "public,huawei" || strings.Join(check.ValidArgs, ",") != "public,huawei" {
			t.Fatalf("unexpected presets: use=%v test=%v", use.ValidArgs, check.ValidArgs)
		}
		if strings.Join(toolUse.ValidArgs, ",") != "gh-proxy" || strings.Join(envUse.ValidArgs, ",") != "public,huawei" {
			t.Fatalf("group presets: tools=%v env=%v", toolUse.ValidArgs, envUse.ValidArgs)
		}
		if !strings.Contains(cmd.Short, test.short) || !strings.Contains(use.Flags().Lookup("dry-run").Usage, test.flag) {
			t.Fatalf("help did not refresh to %s", test.locale)
		}
	}
}
