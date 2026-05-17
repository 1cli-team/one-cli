package configurecmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
)

// TestBuildContributionsTreeShape locks the verb-first command tree:
// top-level `configure` parent with six verb children. Only `add` has
// per-backend sub-subcommands (because each backend's add takes a
// different flag set); list/current/show/use/remove take pair as a
// positional. If anyone re-domains backends, merges verbs back into a
// per-pair factory, or drops a backend, this test trips.
func TestBuildContributionsTreeShape(t *testing.T) {
	cmds := buildContributions()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(cmds))
	}
	parent := cmds[0]
	if parent.Use != "configure" {
		t.Fatalf("expected top-level Use=%q, got %q", "configure", parent.Use)
	}

	wantVerbs := map[string]bool{
		"add":     true,
		"list":    true,
		"current": true,
		"show":    true,
		"use":     true,
		"remove":  true,
		// `locale` is the first (and so far only) user-global
		// preference under `configure`. Unlike the verbs above it
		// doesn't take a (domain, backend) pair — it just reads /
		// writes preferences.json.
		"locale": true,
	}
	gotVerbs := map[string][]string{}
	for _, child := range parent.Commands() {
		name := child.Name()
		subs := []string{}
		for _, v := range child.Commands() {
			subs = append(subs, v.Use)
		}
		gotVerbs[name] = subs
	}
	for v := range wantVerbs {
		if _, ok := gotVerbs[v]; !ok {
			t.Errorf("missing verb subcommand %q under configure", v)
		}
	}
	for v := range gotVerbs {
		if !wantVerbs[v] {
			t.Errorf("unexpected verb subcommand %q under configure", v)
		}
	}

	// Only `add` should carry per-backend sub-subcommands; the other
	// verbs take pair as a positional and have no children.
	wantAddPairs := map[string]bool{
		"env/infisical [--profile <name>]":       true,
		"deploy/aliyun-oss [--profile <name>]":   true,
		"deploy/tencent-cos [--profile <name>]":  true,
		"deploy/aws-s3 [--profile <name>]":       true,
		"deploy/minio [--profile <name>]":        true,
		"deploy/rustfs [--profile <name>]":       true,
		"deploy/r2 [--profile <name>]":           true,
		"deploy/kustomize [--profile <name>]":    true,
		"deploy/vercel [--profile <name>]":       true,
		"deploy/cloudflare [--profile <name>]":   true,
		"deploy/edgeone [--profile <name>]":      true,
		"container/docker [--profile <name>]":    true,
		"container/dockerhub [--profile <name>]": true,
		"container/ghcr [--profile <name>]":      true,
		"container/acr [--profile <name>]":       true,
	}
	for _, sub := range gotVerbs["add"] {
		if !wantAddPairs[sub] {
			t.Errorf("unexpected add sub-subcommand %q", sub)
		}
		delete(wantAddPairs, sub)
	}
	for sub := range wantAddPairs {
		t.Errorf("missing add sub-subcommand %q", sub)
	}

	for _, v := range []string{"list", "current", "show", "use", "remove", "locale"} {
		if subs := gotVerbs[v]; len(subs) != 0 {
			t.Errorf("verb %q must have no sub-subcommands (pair is positional); got %v", v, subs)
		}
	}
}

// TestAddHelpExamplesAreBackendSpecific verifies each backend's add
// command help text shows the relevant flag set and example, and does
// NOT bleed examples from sibling backends. The path is now
// `configure add <pair> --help` (verb-first); each leaf reaches the
// same per-backend factory but mounted under `add` instead of under
// the pair.
func TestAddHelpExamplesAreBackendSpecific(t *testing.T) {
	tests := []struct {
		path       []string
		want       []string
		doesntWant []string
	}{
		{
			path: []string{"add", "env/infisical", "--help"},
			want: []string{
				"one configure add env/infisical --profile work",
				"--site-url https://infisical.company.com",
				"--client-id",
				"--client-secret",
			},
			doesntWant: []string{
				"--access-key-id",
				"--registry",
				"--kubeconfig-context",
			},
		},
		{
			path: []string{"add", "deploy/aws-s3", "--help"},
			want: []string{
				"one configure add deploy/aws-s3 --profile prod",
				"--endpoint",
				"--access-key-id",
			},
			doesntWant: []string{
				"--client-id",
				"--registry",
				"--kubeconfig-context",
			},
		},
		{
			path: []string{"add", "deploy/aliyun-oss", "--help"},
			want: []string{
				"one configure add deploy/aliyun-oss --profile prod",
				"--endpoint",
				"--access-key-id",
			},
			doesntWant: []string{
				"--client-id",
				"--registry",
				"--kubeconfig-context",
				"registry.cn-hangzhou.aliyuncs.com",
			},
		},
		{
			path: []string{"add", "deploy/minio", "--help"},
			want: []string{
				"one configure add deploy/minio --profile prod",
				"--endpoint",
				"--force-path-style",
			},
			doesntWant: []string{
				"--client-id",
				"--registry",
				"--kubeconfig-context",
			},
		},
		{
			path: []string{"add", "deploy/kustomize", "--help"},
			want: []string{
				"one configure add deploy/kustomize --profile prod-k8s",
				"--kubeconfig-context",
			},
			doesntWant: []string{
				"--client-id",
				"--access-key-id",
				"--registry",
			},
		},
		{
			path: []string{"add", "deploy/cloudflare", "--help"},
			want: []string{
				"one configure add deploy/cloudflare --profile work",
				"--token",
				"--account-id",
			},
			doesntWant: []string{
				"--client-id",
				"--access-key-id",
				"--registry",
				"--kubeconfig-context",
			},
		},
		{
			path: []string{"add", "deploy/edgeone", "--help"},
			want: []string{
				"one configure add deploy/edgeone --profile work",
				"--token",
			},
			doesntWant: []string{
				"--client-id",
				"--registry",
				"--kubeconfig-context",
				"--secret-id",
				"--secret-key",
			},
		},
		{
			path: []string{"add", "container/docker", "--help"},
			want: []string{
				"one configure add container/docker",
				"--registry",
				"--username",
				"--password",
			},
			doesntWant: []string{
				"--client-id",
				"--access-key-id",
				"--kubeconfig-context",
			},
		},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.path, "/"), func(t *testing.T) {
			parent := buildContributions()[0]
			var out bytes.Buffer
			parent.SetOut(&out)
			parent.SetErr(&out)
			parent.SetArgs(tt.path)
			if err := parent.Execute(); err != nil {
				t.Fatalf("help failed: %v", err)
			}
			got := out.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("help missing %q:\n%s", want, got)
				}
			}
			for _, unwanted := range tt.doesntWant {
				if strings.Contains(got, unwanted) {
					t.Errorf("help has stale fragment %q:\n%s", unwanted, got)
				}
			}
		})
	}
}

func TestConfigureWizardDispatchClearsOriginalArgs(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() {
		os.Args = oldArgs
		output.SetMode(output.ModeAuto)
	})
	os.Args = []string{"one", "configure", "add"}
	output.SetMode(output.ModeJSON)

	parent := buildContributions()[0]
	var out bytes.Buffer
	parent.SetOut(&out)
	parent.SetErr(&out)

	err := runSelectedAddBackend(parent, profile.DomainEnv, "infisical")
	if err == nil {
		t.Fatal("expected missing non-interactive profile fields error, got nil")
	}
	coded, ok := err.(interface{ ErrorCode() string })
	if !ok {
		t.Fatalf("expected structured error, got %T: %v", err, err)
	}
	if coded.ErrorCode() == string(cliErrors.UNKNOWN_COMMAND) {
		t.Fatalf("selected backend replayed os.Args into cobra: %v", err)
	}
	if coded.ErrorCode() != string(cliErrors.PROFILE_BACKEND_INVALID) {
		t.Fatalf("expected %s, got %s: %v", cliErrors.PROFILE_BACKEND_INVALID, coded.ErrorCode(), err)
	}
}

// TestParsePairValidatesInput exercises the positional-pair parser
// used by list / current / show / use / remove. The wire format is
// always `<domain>/<backend>`; anything else returns
// PROFILE_BACKEND_INVALID with the valid-pair list inlined in the
// message so the user can copy-paste the right form.
func TestParsePairValidatesInput(t *testing.T) {
	for _, ok := range []string{
		"env/infisical",
		"deploy/aliyun-oss", "deploy/tencent-cos", "deploy/aws-s3",
		"deploy/minio", "deploy/rustfs", "deploy/r2",
		"deploy/kustomize", "deploy/vercel", "deploy/cloudflare", "deploy/edgeone",
		"container/docker",
	} {
		if _, _, err := parsePair(ok); err != nil {
			t.Errorf("parsePair(%q) unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "infisical", "env", "env/", "/infisical", "envinfisical", "env/typo", "env/dotenv"} {
		if _, _, err := parsePair(bad); err == nil {
			t.Errorf("parsePair(%q) want error, got nil", bad)
		}
	}
}

func TestMaskCredentialsMasksTokenDeployBackends(t *testing.T) {
	got := maskCredentials(profile.Profile{
		Cloudflare: &profile.CloudflareProfile{
			Credentials: &profile.CloudflareCredentials{APIToken: "cloudflare-token"},
		},
		EdgeOne: &profile.EdgeOneProfile{
			Credentials: &profile.EdgeOneCredentials{APIToken: "edgeone-token"},
		},
	})

	if got.Cloudflare == nil || got.Cloudflare.Credentials == nil || got.Cloudflare.Credentials.APIToken != "********" {
		t.Fatalf("cloudflare token was not masked: %#v", got.Cloudflare)
	}
	if got.EdgeOne == nil || got.EdgeOne.Credentials == nil || got.EdgeOne.Credentials.APIToken != "********" {
		t.Fatalf("edgeone token was not masked: %#v", got.EdgeOne)
	}
}
