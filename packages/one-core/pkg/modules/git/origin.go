package git

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

type SetOriginInput struct {
	URL              string `json:"url"`
	ExpectedFetchURL string `json:"expectedFetchUrl"`
	ExpectedPushURL  string `json:"expectedPushUrl"`
}

func (s *Service) SetOrigin(ctx context.Context, root string, input SetOriginInput) (*Remote, error) {
	return s.backend.SetOrigin(ctx, root, input)
}

func (s *Service) Origin(ctx context.Context, root string) (Remote, error) {
	return s.backend.Origin(ctx, root)
}

type RepositoryRef struct {
	Host string `json:"host"`
	Slug string `json:"slug"`
}

const GitHubHost = "github.com"

var hostedRepositoryPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

func (r RepositoryRef) Valid() bool {
	owner, name, _ := strings.Cut(r.Slug, "/")
	return r.Host != "" && r.Host == strings.ToLower(r.Host) &&
		!strings.ContainsAny(r.Host, " /@?#\\") && hostedRepositoryPattern.MatchString(r.Slug) &&
		owner != "." && owner != ".." && name != "." && name != ".."
}

func (r RepositoryRef) Matches(other RepositoryRef) bool {
	return r.Valid() && other.Valid() && r.Host == other.Host && strings.EqualFold(r.Slug, other.Slug)
}

// ParseRepositoryRef identifies a hosted repository without retaining credentials
// or conflating the SSH transport port with the host's web API port.
func ParseRepositoryRef(remote string) RepositoryRef {
	if !strings.Contains(remote, "://") {
		host, path, ok := strings.Cut(remote, ":")
		if !ok || strings.ContainsAny(host, "/\\") || strings.HasPrefix(path, "/") {
			return RepositoryRef{}
		}
		remote = "ssh://" + host + "/" + path
	}
	u, err := url.Parse(remote)
	if err != nil || u.ForceQuery || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "ssh") {
		return RepositoryRef{}
	}
	ref := RepositoryRef{
		Host: strings.TrimSuffix(strings.ToLower(u.Hostname()), "."),
		Slug: strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git"),
	}
	if !ref.Valid() {
		return RepositoryRef{}
	}
	return ref
}
