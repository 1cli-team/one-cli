package infisical

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
)

// Resolve messages when displayed so a cached preparation failure follows a
// Dashboard language change without issuing another remote mutation.
type bindingError struct {
	key  string
	args []any
}

func (e *bindingError) Error() string                  { return i18n.Tf(e.key, e.args...) }
func sharedBindingError(key string, args ...any) error { return &bindingError{key: key, args: args} }

// PrepareSharedCredentials resolves storage using the captured login session.
// Ownership of an old local record is not a remote access check: projects
// accessible to another member of the current organization can be reused.
func PrepareSharedCredentials(ctx context.Context, expected *session.Session) (*GlobalLocation, error) {
	return prepareSharedCredentials(ctx, expected, "")
}

func prepareSharedCredentials(ctx context.Context, expected *session.Session, override string) (*GlobalLocation, error) {
	return withLocationLock(ctx, func() (*GlobalLocation, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := session.WithUnchanged(expected, func() error { return nil }); err != nil {
			return nil, err
		}
		if expected.OrganizationID == "" {
			return nil, sharedBindingError("infisical.organization_required")
		}
		config, err := loadGlobalConfig()
		if err != nil {
			return nil, err
		}
		saved := config.forSession(expected)
		var preferred *GlobalLocation
		if saved != nil {
			preferred = saved
		} else if config != nil {
			preferred = &config.GlobalLocation
		}
		// Only a successful scoped listing can establish that storage is absent.
		projects, err := projectsFor(ctx, expected)
		if err != nil {
			return nil, err
		}
		candidates := []RemoteProject{}
		for _, project := range projects {
			if project.Name == DefaultSharedProject {
				candidates = append(candidates, project)
			}
		}
		var selected *RemoteProject
		for _, candidate := range candidates {
			if preferred != nil && preferred.SiteURL == expected.SiteURL && preferred.OrganizationID == expected.OrganizationID && preferred.ProjectID == candidate.ID {
				selected = &candidate
				break
			}
		}
		if selected == nil {
			switch len(candidates) {
			case 0:
				selected, err = createProjectFor(ctx, expected, DefaultSharedProject)
				if err != nil {
					return nil, err
				}
			case 1:
				selected = &candidates[0]
			default:
				return nil, sharedBindingError("global.duplicate_projects")
			}
		}
		project, err := projectFor(ctx, expected, selected.ID)
		if err != nil {
			return nil, err
		}
		if project.Name != DefaultSharedProject {
			return nil, sharedBindingError("global.project_changed")
		}
		environment := DefaultSharedEnvironment
		if saved != nil && saved.ProjectID == project.ID {
			for _, candidate := range project.Environments {
				if candidate.Slug == saved.DefaultEnvironment {
					environment = saved.DefaultEnvironment
					break
				}
			}
		}
		if override != "" {
			environment = override
		}
		return saveGlobalBinding(ctx, expected, project, environment)
	})
}
