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

// EnsureDefaultGlobal is an explicit mutation, never a side effect of GET.
// Reuse the named project after interrupted setup and preserve any saved location.
func EnsureDefaultGlobal(ctx context.Context) (*GlobalLocation, error) {
	return BindDefaultGlobal(ctx, "")
}

// BindDefaultGlobal preserves the saved project, or creates/reuses the default
// project when unbound. An empty environment preserves the saved environment,
// defaulting to dev for a new binding. Explicit environments are validated before
// saving the location.
func BindDefaultGlobal(ctx context.Context, environment string) (*GlobalLocation, error) {
	return withLocationLock(ctx, func() (*GlobalLocation, error) {
		s, err := session.Require()
		if err != nil {
			return nil, err
		}
		location, err := LoadGlobalLocation()
		if err != nil {
			return nil, err
		}
		if location != nil {
			if location.SiteURL != s.SiteURL || location.UserID != s.UserID || (s.OrganizationID != "" && location.OrganizationID != s.OrganizationID) {
				return nil, i18n.Errorf("global.existing_location_mismatch")
			}
			if environment == "" {
				environment = location.DefaultEnvironment
			}
			return bindGlobalFor(ctx, s, location.ProjectID, environment)
		}
		if environment == "" {
			environment = DefaultSharedEnvironment
		}
		if s.OrganizationID == "" {
			return nil, i18n.Errorf("infisical.organization_required")
		}
		projects, err := projectsFor(ctx, s)
		if err != nil {
			return nil, err
		}
		var selected *RemoteProject
		for _, project := range projects {
			if project.Name == DefaultSharedProject {
				if selected != nil {
					return nil, i18n.Errorf("global.duplicate_projects")
				}
				selected = &project
			}
		}
		if selected == nil {
			selected, err = createProjectFor(ctx, s, DefaultSharedProject)
			if err != nil {
				return nil, err
			}
		}
		return bindGlobalFor(ctx, s, selected.ID, environment)
	})
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
