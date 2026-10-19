package app

import (
	"io"
	"os"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"
)

// Deployment inputs are resolved once at startup. Empty Compose defaults count
// as unset; partially configured identities fail instead of using old DB data.
func readInfisicalDeployment(getenv func(string) string) (*session.Deployment, error) {
	names := []string{"ONE_INFISICAL_SITE_URL", "ONE_INFISICAL_CLIENT_ID", "ONE_INFISICAL_CLIENT_SECRET", "ONE_INFISICAL_ORGANIZATION_ID"}
	values := make([]string, len(names))
	source := "environment"
	configured := false
	for i, name := range names {
		value, path := getenv(name), strings.TrimSpace(getenv(name+"_FILE"))
		if value == "" && path == "" {
			continue
		}
		configured = true
		if value != "" && path != "" {
			return nil, i18n.Errorf("server.infisical_config_conflict", name, name+"_FILE")
		}
		if path != "" {
			source = "secret-file"
			file, err := os.Open(path)
			if err != nil {
				return nil, i18n.Errorf("server.infisical_file_read_failed", name+"_FILE", err)
			}
			info, err := file.Stat()
			if err != nil || !info.Mode().IsRegular() {
				file.Close()
				return nil, i18n.Errorf("server.infisical_config_invalid", name+"_FILE")
			}
			data, err := io.ReadAll(io.LimitReader(file, (16<<10)+1))
			file.Close()
			if err != nil {
				return nil, i18n.Errorf("server.infisical_file_read_failed", name+"_FILE", err)
			}
			value = string(data)
		}
		if len(value) > 16<<10 {
			return nil, i18n.Errorf("server.infisical_config_invalid", name)
		}
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsAny(value, "\r\n") {
			return nil, i18n.Errorf("server.infisical_config_invalid", name)
		}
		values[i] = value
	}
	if !configured {
		return nil, nil
	}
	if values[1] == "" || values[2] == "" {
		return nil, i18n.Errorf("server.infisical_config_incomplete")
	}
	site, err := session.NormalizeSite(values[0])
	if err != nil {
		return nil, err
	}
	return &session.Deployment{Source: source, Machine: session.Machine{SiteURL: site, ClientID: values[1], ClientSecret: values[2], OrganizationID: values[3]}}, nil
}
