package infisical

import "context"
import session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"

func sessionCredentials(ctx context.Context) (*Credentials, string, error) {
	current, err := session.Require(ctx)
	if err != nil {
		return nil, "", err
	}
	return &Credentials{AccessToken: current.Token}, current.SiteURL, nil
}
