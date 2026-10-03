package infisical

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gofrs/flock"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
)

const DefaultSharedProject = "shared-credentials"
const DefaultSharedEnvironment = "dev"

// CreateRemoteProject creates only Secret Manager projects. It does not change
// the saved location; the user chooses an environment before binding it.
func CreateRemoteProject(ctx context.Context, name string) (*RemoteProject, error) {
	s, err := session.Require()
	if err != nil {
		return nil, err
	}
	return createProjectFor(ctx, s, name)
}

func createProjectFor(ctx context.Context, s *session.Session, name string) (*RemoteProject, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 || strings.ContainsFunc(name, unicode.IsControl) {
		return nil, i18n.Errorf("infisical.project_name_invalid")
	}
	if s.OrganizationID == "" {
		return nil, i18n.Errorf("infisical.organization_required")
	}
	c, err := NewClient(ctx, &WorkspaceConfig{SiteURL: s.SiteURL}, &Credentials{AccessToken: s.Token})
	if err != nil {
		return nil, err
	}
	id, _, err := c.CreateProjectContext(ctx, name)
	if err != nil {
		return nil, err
	}
	return projectFor(ctx, s, id)
}

// BindDefaultGlobal is the CLI recovery entry point. It applies the same
// automatic project resolution as login, with an optional environment override.
func BindDefaultGlobal(ctx context.Context, environment string) (*GlobalLocation, error) {
	s, err := session.Require()
	if err != nil {
		return nil, err
	}
	return prepareSharedCredentials(ctx, s, environment)
}

// Serialize setup and binding across Dashboard instances on this machine.
func withLocationLock(ctx context.Context, fn func() (*GlobalLocation, error)) (*GlobalLocation, error) {
	p, err := session.ConfigPath("global-env.lock")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return nil, err
	}
	lock := flock.New(p)
	ok, err := lock.TryLockContext(ctx, 50*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, i18n.Errorf("global.configuration_busy")
	}
	defer lock.Unlock()
	return fn()
}
