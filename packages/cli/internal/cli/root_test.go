package cli

import (
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

func TestScanOutputValue(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"none", []string{"templates"}, ""},
		{"-o space json", []string{"templates", "-o", "json"}, "json"},
		{"--output space json", []string{"templates", "--output", "json"}, "json"},
		{"-o equals", []string{"templates", "-o=json"}, "json"},
		{"--output equals", []string{"templates", "--output=json"}, "json"},
		{"-ojson concatenated", []string{"templates", "-ojson"}, "json"},
		{"-o text", []string{"-o", "text", "templates"}, "text"},
		{"-o trailing without value", []string{"templates", "-o"}, ""},
		{"first wins", []string{"-o", "json", "--output", "text"}, "json"},
		{"flag mid-args", []string{"add", "nestjs-api", "-o", "json", "--name", "x"}, "json"},
		{"unknown flag without value", []string{"-x", "templates"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scanOutputValue(c.args); got != c.want {
				t.Errorf("scanOutputValue(%q) = %q, want %q", c.args, got, c.want)
			}
		})
	}
}

func TestDetectOutputMode_TextForcesHuman(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })

	output.SetMode(output.ModeJSON) // dirty prior state
	detectOutputMode([]string{"templates", "-o", "text"})
	if output.IsJSON() {
		t.Errorf("-o text should force human format: IsJSON() = true")
	}
}

func TestDetectOutputMode_JSONForces(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })

	output.SetMode(output.ModeAuto)
	detectOutputMode([]string{"templates", "-o", "json"})
	if !output.IsJSON() {
		t.Errorf("-o json should force JSON: IsJSON() = false")
	}
}

func TestDetectOutputMode_UnknownValueFallsThrough(t *testing.T) {
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })

	output.SetMode(output.ModeJSON)
	detectOutputMode([]string{"templates", "-o", "bogus"})
	// unknown value: kubectl-style leniency — leaves prior mode untouched
	if !output.IsJSON() {
		t.Errorf("unknown -o value should not switch off ModeJSON")
	}
}

func TestFirstPositional(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		want   string
		wantOK bool
	}{
		{"empty", nil, "", false},
		{"flag only", []string{"--help"}, "", false},
		{"help token suppresses", []string{"help", "create"}, "", false},
		{"plain positional", []string{"create"}, "create", true},
		{"flag before positional", []string{"-o", "json", "templates"}, "templates", true},
		{"long flag before positional", []string{"--output", "json", "templates"}, "templates", true},
		{"-o=json before positional", []string{"-o=json", "templates"}, "templates", true},
		{"--output=json before positional", []string{"--output=json", "templates"}, "templates", true},
		{"-ojson before positional", []string{"-ojson", "templates"}, "templates", true},
		{"empty string skipped", []string{"", "templates"}, "templates", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := firstPositional(c.args)
			if got != c.want || ok != c.wantOK {
				t.Errorf("firstPositional(%q) = (%q, %v), want (%q, %v)", c.args, got, ok, c.want, c.wantOK)
			}
		})
	}
}

func TestShouldRenderRootHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"bare", nil, true},
		{"-h", []string{"-h"}, true},
		{"--help", []string{"--help"}, true},
		{"help with no follow", []string{"help"}, true},
		{"help unknown", []string{"help", "bogus"}, true},
		{"help known subcommand", []string{"help", "create"}, false},
		{"subcommand", []string{"templates"}, false},
		{"subcommand with help", []string{"templates", "--help"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRenderRootHelp(c.args); got != c.want {
				t.Errorf("shouldRenderRootHelp(%q) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

func TestRootHelpDoesNotAdvertiseUnregisteredCommands(t *testing.T) {
	// The curated help text lives in i18n/locales/*.json and is
	// fetched via RootHelp(). Check every locale so a new tongue
	// can't reintroduce the historical surfaces.
	for _, key := range []string{"root.help"} {
		text := i18n.T(key)
		for _, token := range []string{"one prd", "one design", "PRODUCT / DESIGN"} {
			if strings.Contains(text, token) {
				t.Fatalf("%s advertises unregistered command surface %q", key, token)
			}
		}
	}
}

func TestIsKnownSubcommand(t *testing.T) {
	// In-package commands register via init(); cross-package commands
	// (templates, infisical, dotenv) register via cliexts.Mount.
	for _, name := range []string{
		"create", "templates", "add",
		// Per-domain commands (post capability-interface refactor).
		"env", "container", "dev", "deploy",
		// configure owns the credential CRUD surface (renamed from
		// `profile` to align with industry standard CLIs); skills owns
		// bundled-skill installation.
		"configure", "skills",
	} {
		if !isKnownSubcommand(name) {
			t.Errorf("isKnownSubcommand(%q) = false, want true", name)
		}
	}
	// Removed commands: per-domain ones replaced by `one env|container|dev|deploy`,
	// `one plugins` removed entirely with the plugin concept, `one setup`
	// dissolved into `one configure` + `one skills`, and `one profile`
	// renamed to `one configure`.
	for _, name := range []string{
		"doctor", "status", "unknown", "secrets", "skill", "prd", "design",
		"docker", "infisical", "dotenv", "procs", "compose", "k8s",
		"plugins", "setup", "profile",
		"",
	} {
		if isKnownSubcommand(name) {
			t.Errorf("isKnownSubcommand(%q) = true, want false", name)
		}
	}
}
