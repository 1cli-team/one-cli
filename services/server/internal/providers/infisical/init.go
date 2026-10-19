package infisical

import "context"
import session "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisicalsession"

// CreateBoundProject creates provider metadata, without publishing local files.
func CreateBoundProject(ctx context.Context, name string) (*RemoteProject, error) {
	current, err := session.Require(ctx)
	if err != nil {
		return nil, err
	}
	return createProjectFor(ctx, current, name)
}
