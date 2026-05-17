package profile

import (
	"testing"
)

// File-source profile resolves and surfaces CredSource="file".
func TestResolve_FileSource(t *testing.T) {
	withIsolatedConfig(t)
	if _, err := Upsert(DomainEnv, "infisical", "work", Profile{
		Backend: "infisical",
		Infisical: &InfisicalProfile{
			SiteURL:     "https://app.infisical.com",
			Credentials: &InfisicalCredentials{ClientID: "x", ClientSecret: "y"},
		},
	}, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	resolved, err := Resolve(ResolveInput{Domain: DomainEnv, Backend: "infisical"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Name != "work" {
		t.Errorf("name: %q", resolved.Name)
	}
	if resolved.Source != "default" {
		t.Errorf("source: %q want default", resolved.Source)
	}
	if resolved.CredSource != SourceFile {
		t.Errorf("credSource: %q want file", resolved.CredSource)
	}
	if resolved.Profile.Infisical.Credentials == nil ||
		resolved.Profile.Infisical.Credentials.ClientSecret != "y" {
		t.Errorf("credentials not populated: %+v", resolved.Profile.Infisical)
	}
}

// Empty CredentialSource is treated as "file".
func TestResolve_EmptySourceTreatedAsFile(t *testing.T) {
	withIsolatedConfig(t)
	if _, err := Upsert(DomainEnv, "infisical", "work", Profile{
		Backend: "infisical",
		Infisical: &InfisicalProfile{
			// CredentialSource left blank.
			SiteURL:     "https://app.infisical.com",
			Credentials: &InfisicalCredentials{ClientID: "x", ClientSecret: "y"},
		},
	}, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	resolved, err := Resolve(ResolveInput{Domain: DomainEnv, Backend: "infisical"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.CredSource != SourceFile {
		t.Errorf("expected file fallback, got %q", resolved.CredSource)
	}
}

// Non-file credentialSource surfaces PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED.
func TestResolve_UnsupportedSource(t *testing.T) {
	withIsolatedConfig(t)
	if _, err := Upsert(DomainEnv, "infisical", "work", Profile{
		Backend: "infisical",
		Infisical: &InfisicalProfile{
			SiteURL:          "https://app.infisical.com",
			CredentialSource: SourceKeyring,
			Credentials:      &InfisicalCredentials{ClientID: "x", ClientSecret: "y"},
		},
	}, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := Resolve(ResolveInput{Domain: DomainEnv, Backend: "infisical"})
	if err == nil {
		t.Fatalf("expected unsupported error")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok ||
		cliErr.ErrorCode() != "PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED" {
		t.Errorf("expected PROFILE_CREDENTIAL_SOURCE_UNSUPPORTED, got %T %v", err, err)
	}
}

// PROFILE_NONE_CONFIGURED when no profile / flag / env / manifest provides anything.
func TestResolve_NoneConfigured(t *testing.T) {
	withIsolatedConfig(t)
	_, err := Resolve(ResolveInput{Domain: DomainEnv, Backend: "infisical"})
	if err == nil {
		t.Fatalf("expected PROFILE_NONE_CONFIGURED")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok ||
		cliErr.ErrorCode() != "PROFILE_NONE_CONFIGURED" {
		t.Errorf("expected PROFILE_NONE_CONFIGURED, got %T %v", err, err)
	}
}

// --profile flag wins over default.
func TestResolve_FlagOverridesDefault(t *testing.T) {
	withIsolatedConfig(t)
	for _, n := range []string{"work", "personal"} {
		if _, err := Upsert(DomainEnv, "infisical", n, Profile{
			Backend: "infisical",
			Infisical: &InfisicalProfile{
				SiteURL:     "https://app.infisical.com",
				Credentials: &InfisicalCredentials{ClientID: "x", ClientSecret: n},
			},
		}, false); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
	}
	// "work" was added first → default. Flag picks "personal".
	resolved, err := Resolve(ResolveInput{
		Domain:       DomainEnv,
		Backend:      "infisical",
		FlagOverride: "personal",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Source != "flag" || resolved.Name != "personal" {
		t.Errorf("flag did not win: source=%q name=%q", resolved.Source, resolved.Name)
	}
}

func TestResolve_WorkspaceBindingOverridesDefault(t *testing.T) {
	withIsolatedConfig(t)
	for _, n := range []string{"default", "workspace"} {
		if _, err := Upsert(DomainEnv, "infisical", n, Profile{
			Backend: "infisical",
			Infisical: &InfisicalProfile{
				SiteURL:     "https://app.infisical.com",
				Credentials: &InfisicalCredentials{ClientID: "x", ClientSecret: n},
			},
		}, n == "default"); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
	}
	if err := BindWorkspaceProfile("ws-demo", "demo", "/tmp/demo", "", DomainEnv, "infisical", "workspace"); err != nil {
		t.Fatalf("bind workspace: %v", err)
	}
	resolved, err := Resolve(ResolveInput{
		Domain:      DomainEnv,
		Backend:     "infisical",
		WorkspaceID: "ws-demo",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Source != "workspace" || resolved.Name != "workspace" {
		t.Errorf("workspace binding did not win: source=%q name=%q", resolved.Source, resolved.Name)
	}
}

func TestResolve_ProjectBindingOverridesWorkspaceBinding(t *testing.T) {
	withIsolatedConfig(t)
	for _, n := range []string{"default", "workspace", "project"} {
		if _, err := Upsert(DomainDeploy, "vercel", n, Profile{
			Backend: "vercel",
			Vercel: &VercelProfile{
				Team:        n,
				Credentials: &VercelCredentials{APIToken: n},
			},
		}, n == "default"); err != nil {
			t.Fatalf("seed %s: %v", n, err)
		}
	}
	if err := BindWorkspaceProfile("ws-demo", "demo", "/tmp/demo", "", DomainDeploy, "vercel", "workspace"); err != nil {
		t.Fatalf("bind workspace: %v", err)
	}
	if err := BindWorkspaceProfile("ws-demo", "demo", "/tmp/demo", "web", DomainDeploy, "vercel", "project"); err != nil {
		t.Fatalf("bind project: %v", err)
	}
	resolved, err := Resolve(ResolveInput{
		Domain:      DomainDeploy,
		Backend:     "vercel",
		WorkspaceID: "ws-demo",
		ProjectName: "web",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Source != "workspace-project" || resolved.Name != "project" {
		t.Errorf("project binding did not win: source=%q name=%q", resolved.Source, resolved.Name)
	}
}
