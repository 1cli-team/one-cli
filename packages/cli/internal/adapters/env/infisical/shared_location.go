package infisical

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gofrs/flock"
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
		return nil, fmt.Errorf("项目名称须为 1–64 个字符，不能包含控制字符")
	}
	if s.OrganizationID == "" {
		return nil, fmt.Errorf("当前登录未选择组织，请在 Infisical 中选择组织后重新登录")
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
				return nil, fmt.Errorf("已有共享凭据位置属于其他账号或组织，请手动选择存放项目")
			}
			return bindGlobalFor(ctx, s, location.ProjectID, location.DefaultEnvironment)
		}
		if s.OrganizationID == "" {
			return nil, fmt.Errorf("当前登录未选择组织，请在 Infisical 中选择组织后重新登录")
		}
		projects, err := projectsFor(ctx, s)
		if err != nil {
			return nil, err
		}
		var selected *RemoteProject
		for _, project := range projects {
			if project.Name == DefaultSharedProject {
				if selected != nil {
					return nil, fmt.Errorf("存在多个同名共享凭据项目，请手动选择存放项目")
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
		return bindGlobalFor(ctx, s, selected.ID, DefaultSharedEnvironment)
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
		return nil, fmt.Errorf("共享凭据位置正在配置，请重试")
	}
	defer lock.Unlock()
	return fn()
}
