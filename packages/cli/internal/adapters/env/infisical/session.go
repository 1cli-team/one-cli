package infisical

import session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"

func sessionCredentials() (*Credentials, string, error) {
	current, err := session.Require()
	if err != nil {
		return nil, "", err
	}
	return &Credentials{AccessToken: current.Token}, current.SiteURL, nil
}
func sessionAvailable() bool { _, err := session.Require(); return err == nil }
