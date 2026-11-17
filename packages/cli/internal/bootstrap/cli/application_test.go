package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/lifecycle"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/plugins/infisical"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"
)

func TestApplicationPluginIsolation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ONE_HOME", root)
	// Construction must not read credentials, even when the saved file is invalid.
	if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "session.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	core, err := NewApplication()
	if err != nil {
		t.Fatal(err)
	}
	full, err := NewApplication(infisical.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"login", "logout", "whoami", "env"} {
		if knownSubcommand(core.root, name) {
			t.Fatalf("core contains plugin command %s", name)
		}
		if !knownSubcommand(full.root, name) {
			t.Fatalf("plugin command missing: %s", name)
		}
	}
	if !knownSubcommand(core.root, "run") || !knownSubcommand(core.root, "exec") {
		t.Fatal("core execution commands missing")
	}
	for _, locale := range []string{"en-US", "zh-CN"} {
		if err := i18n.Init(locale); err != nil {
			t.Fatal(err)
		}
		i18n.RefreshTree(full.root)
		command, _, err := full.root.Find([]string{"login"})
		if err != nil || command.Short != i18n.T("auth.login.short") {
			t.Fatalf("plugin locale was not refreshed: %s", locale)
		}
	}
}

type invocationPlugin struct {
	stopped bool
	result  error
}

func (*invocationPlugin) ID() string                           { return "test" }
func (*invocationPlugin) Contributions() plugins.Contributions { return plugins.Contributions{} }
func (p *invocationPlugin) Register(host lifecycle.Host) error {
	host.Root.AddCommand(&cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return p.result }})
	return nil
}
func (p *invocationPlugin) Start(context.Context, lifecycle.Invocation) (func(context.Context) error, error) {
	return func(context.Context) error { p.stopped = true; return nil }, nil
}

func TestApplicationRunsLifecycleOnCommandError(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	failure := errors.New("probe failed")
	p := &invocationPlugin{result: failure}
	app, err := NewApplication(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Execute(context.Background(), "0.0.0-dev", []string{"probe"}); !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	if !p.stopped {
		t.Fatal("application skipped plugin cleanup")
	}
}

func TestCommandHelpDoesNotStartPlugins(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	p := &invocationPlugin{}
	app, err := NewApplication(p)
	if err != nil {
		t.Fatal(err)
	}
	var help bytes.Buffer
	app.root.SetOut(&help)
	if err := app.Execute(context.Background(), "0.0.0-dev", []string{"probe", "--help"}); err != nil {
		t.Fatal(err)
	}
	if p.stopped {
		t.Fatal("help started plugin resources")
	}
	if !strings.Contains(help.String(), "probe") {
		t.Fatal("command help missing")
	}
}
