package codehost

import (
	"encoding/base64"
	"errors"
	"net/url"
	"slices"
	"strings"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

// GitEnvironment supplies the saved Gitea credential to one Git invocation.
// The header is scoped to the exact repository URL; redirects and Git tracing
// are disabled for authenticated operations.
func (s Service) GitEnvironment(env []string, remote string) ([]string, error) {
	ref := gitmodule.ParseRepositoryRef(remote)
	u, err := url.Parse(remote)
	if err != nil || !ref.Valid() || u.Scheme != "https" || u.User != nil || ref.Host == gitmodule.GitHubHost {
		return env, nil
	}
	c, err := s.connection(ref.Host)
	if errors.Is(err, ErrMissing) {
		return env, nil
	}
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(c.URL)
	if err != nil || base.Scheme != u.Scheme || webPort(base) != webPort(u) {
		return env, nil
	}
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		return strings.HasPrefix(upper, "GIT_CONFIG_") || strings.HasPrefix(upper, "GIT_TRACE") || upper == "GIT_CURL_VERBOSE"
	})
	authorization := base64.StdEncoding.EncodeToString([]byte(c.Account + ":" + c.Token))
	return append(env,
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=http."+base.JoinPath(strings.TrimPrefix(u.Path, "/")).String()+".extraHeader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+authorization,
		"GIT_CONFIG_KEY_1=http.followRedirects",
		"GIT_CONFIG_VALUE_1=false",
	), nil
}
