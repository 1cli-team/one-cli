package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfisicalDeploymentConfiguration(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "client-secret")
	if err := os.WriteFile(secretPath, []byte("file-credential\n"), 0600); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(t.TempDir(), "empty")
	largePath := filepath.Join(t.TempDir(), "large")
	for path, value := range map[string]string{emptyPath: "\n", largePath: strings.Repeat("x", (16<<10)+1)} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name, source string
		env          map[string]string
		invalid      bool
	}{
		{name: "unconfigured"},
		{name: "empty compose defaults", env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "", "ONE_INFISICAL_CLIENT_SECRET_FILE": ""}},
		{name: "environment", source: "environment", env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET": "environment-credential"}},
		{name: "secret file", source: "secret-file", env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET_FILE": secretPath}},
		{name: "missing secret", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity"}},
		{name: "missing identity", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_SECRET": "environment-credential"}},
		{name: "site only", invalid: true, env: map[string]string{"ONE_INFISICAL_SITE_URL": "https://infisical.example"}},
		{name: "conflicting inputs", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET": "environment-credential", "ONE_INFISICAL_CLIENT_SECRET_FILE": secretPath}},
		{name: "missing file", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET_FILE": secretPath + ".missing"}},
		{name: "empty file", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET_FILE": emptyPath}},
		{name: "large file", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET_FILE": largePath}},
		{name: "directory", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET_FILE": t.TempDir()}},
		{name: "invalid site", invalid: true, env: map[string]string{"ONE_INFISICAL_SITE_URL": "http://infisical.example", "ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET_FILE": secretPath}},
		{name: "multiline secret", invalid: true, env: map[string]string{"ONE_INFISICAL_CLIENT_ID": "identity", "ONE_INFISICAL_CLIENT_SECRET": "private\ncredential"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deployment, err := readInfisicalDeployment(func(name string) string { return tt.env[name] })
			if (err != nil) != tt.invalid {
				t.Fatalf("unexpected validation result: %v", err)
			}
			if err != nil {
				if strings.Contains(err.Error(), "environment-credential") || strings.Contains(err.Error(), "file-credential") {
					t.Fatal("configuration error exposed a credential")
				}
				return
			}
			if tt.source == "" {
				if deployment != nil {
					t.Fatal("empty environment enabled deployment management")
				}
				return
			}
			if deployment == nil || deployment.Source != tt.source || deployment.Machine.ClientID != "identity" || deployment.Machine.SiteURL != "https://app.infisical.com" {
				t.Fatal("deployment metadata was not resolved")
			}
			if tt.source == "secret-file" && deployment.Machine.ClientSecret != "file-credential" {
				t.Fatal("secret file was not read or its trailing newline was not trimmed")
			}
		})
	}
}

func TestInfisicalDeploymentAcceptsProjectedSecretFiles(t *testing.T) {
	root := t.TempDir()
	values := map[string]string{"SITE_URL": "https://infisical.example/", "CLIENT_ID": "identity", "CLIENT_SECRET": "credential", "ORGANIZATION_ID": "org"}
	env := map[string]string{}
	for name, value := range values {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(value+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		env["ONE_INFISICAL_"+name+"_FILE"] = path
	}
	deployment, err := readInfisicalDeployment(func(name string) string { return env[name] })
	if err != nil || deployment == nil || deployment.Source != "secret-file" || deployment.Machine.SiteURL != "https://infisical.example" || deployment.Machine.OrganizationID != "org" {
		t.Fatalf("projected files were not resolved: %v", err)
	}
}
