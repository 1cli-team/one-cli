// Package codehost implements One's Git hosting configuration and pull requests.
package codehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

const gitea = "gitea"

type Connection struct {
	Provider string `json:"provider"`
	Host     string `json:"host"`
	URL      string `json:"url"`
	Account  string `json:"account,omitempty"`
}

type Input struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type Failure string

func (f Failure) Error() string { return i18n.T("git.code_host." + string(f)) }

const (
	ErrInvalid         Failure = "invalid"
	ErrMissing         Failure = "missing"
	ErrAuth            Failure = "auth"
	ErrTest            Failure = "test"
	ErrAddressConflict Failure = "address_conflict"
	ErrSave            Failure = "save"
	ErrLoad            Failure = "load"
)

var githubHost = Connection{Provider: "github", Host: gitmodule.GitHubHost, URL: "https://github.com"}

type credential struct {
	Connection
	Token string `json:"token"`
}

// Service shares the local One configuration between Desktop and CLI processes.
type Service struct{}

func configPath() (string, error) {
	dir, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "code-hosts.json"), nil
}

func readCredentials(path string) (map[string]credential, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]credential), nil
	}
	if err != nil {
		return nil, ErrLoad
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 ||
		(runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return nil, ErrLoad
	}
	values := make(map[string]credential)
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&values) != nil || decoder.Decode(new(any)) != io.EOF || values == nil {
		return nil, ErrLoad
	}
	for host, saved := range values {
		parsed, valid := parseInput(Input{URL: saved.URL, Token: saved.Token})
		if !valid || host != parsed.Host || saved.Host != host || saved.URL != parsed.URL || saved.Provider != gitea {
			return nil, ErrLoad
		}
	}
	return values, nil
}

func parseInput(input Input) (credential, bool) {
	address, token := strings.TrimSpace(input.URL), strings.TrimSpace(input.Token)
	u, err := url.Parse(address)
	if err != nil || len(address)+len(token) > 8<<10 || u.Hostname() == "" || u.User != nil ||
		u.ForceQuery || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" ||
		token == "" || strings.ContainsFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return credential{}, false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	loopback := net.ParseIP(host).IsLoopback() || host == "localhost"
	if host == "" || host == gitmodule.GitHubHost || (u.Scheme != "https" && !(u.Scheme == "http" && loopback)) {
		return credential{}, false
	}
	if port := u.Port(); port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	u.Path, u.RawPath = "", ""
	return credential{Connection: Connection{Provider: gitea, Host: host, URL: u.String()}, Token: token}, true
}

func (Service) List() ([]Connection, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	values, err := readCredentials(path)
	if err != nil {
		return nil, err
	}
	result := []Connection{githubHost}
	for _, host := range slices.Sorted(maps.Keys(values)) {
		result = append(result, values[host].Connection)
	}
	return result, nil
}

func (Service) connection(host string) (credential, error) {
	path, err := configPath()
	if err != nil {
		return credential{}, err
	}
	values, err := readCredentials(path)
	if err != nil {
		return credential{}, err
	}
	c, exists := values[host]
	if !exists {
		return credential{}, ErrMissing
	}
	return c, nil
}

func update(ctx context.Context, change func(map[string]credential) (bool, error)) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	release, err := fsutil.WorkspaceLock(ctx, filepath.Dir(path), "code-hosts")
	if err != nil {
		return ErrSave
	}
	defer release()
	values, err := readCredentials(path)
	if err != nil {
		return err
	}
	changed, err := change(values)
	if err != nil || !changed {
		return err
	}
	payload, err := json.MarshalIndent(values, "", "  ")
	if err != nil || len(payload) >= 1<<20 || fsutil.WriteAtomic(path, append(payload, '\n'), 0600) != nil {
		return ErrSave
	}
	return nil
}

func (Service) Save(ctx context.Context, input Input) (Connection, error) {
	c, ok := parseInput(input)
	if !ok {
		return Connection{}, ErrInvalid
	}
	account, err := c.currentUser(ctx)
	if err != nil {
		return Connection{}, connectionFailure(err)
	}
	c.Account = account
	err = update(ctx, func(values map[string]credential) (bool, error) {
		if old, exists := values[c.Host]; exists && old.URL != c.URL {
			return false, ErrAddressConflict
		}
		values[c.Host] = c
		return true, nil
	})
	return c.Connection, err
}

func (s Service) Test(ctx context.Context, host string) (Connection, error) {
	if host == githubHost.Host {
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		var user userView
		if err := gh(ctx, "user", nil, &user); err != nil || user.Login == "" {
			return Connection{}, ErrTest
		}
		view := githubHost
		view.Account = user.Login
		return view, nil
	}
	c, err := s.connection(host)
	if err != nil {
		return Connection{}, err
	}
	c.Account, err = c.currentUser(ctx)
	if err != nil {
		return Connection{}, connectionFailure(err)
	}
	return c.Connection, nil
}

func (s Service) Supports(host string) bool {
	if host == githubHost.Host {
		return true
	}
	_, err := s.connection(host)
	return err == nil
}

func (s Service) MatchesOrigin(ref gitmodule.RepositoryRef, remote string) bool {
	if !ref.Matches(gitmodule.ParseRepositoryRef(remote)) {
		return false
	}
	if ref.Host == githubHost.Host {
		return true
	}
	c, err := s.connection(ref.Host)
	if err != nil {
		return false
	}
	origin, err := url.Parse(remote)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") {
		return true
	}
	apiURL, err := url.Parse(c.URL)
	return err == nil && webPort(origin) == webPort(apiURL)
}

func webPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// ImportLegacy preserves credentials from pre-split workflow installations.
// Existing One settings take precedence, including after a token rotation.
func (Service) ImportLegacy(ctx context.Context, stateDir string) error {
	old := filepath.Join(stateDir, "code-hosts.json")
	path, err := configPath()
	if err != nil || old == path {
		return err
	}
	legacy, err := readCredentials(old)
	if err != nil || len(legacy) == 0 {
		return err
	}
	return update(ctx, func(values map[string]credential) (bool, error) {
		before := len(values)
		for host, value := range legacy {
			if _, exists := values[host]; !exists {
				values[host] = value
			}
		}
		return len(values) != before, nil
	})
}

type userView struct {
	Login string `json:"login"`
}

func connectionFailure(err error) error {
	var response *giteaHTTPError
	if errors.As(err, &response) && (response.code == http.StatusUnauthorized || response.code == http.StatusForbidden) {
		return ErrAuth
	}
	return ErrTest
}

func (s Service) VerifyRepository(ref gitmodule.RepositoryRef) error {
	if !ref.Valid() {
		return i18n.Errorf("git.pr_origin")
	}
	if ref.Host != githubHost.Host {
		if _, err := s.connection(ref.Host); err != nil {
			return fmt.Errorf("%s: %w", ref.Host, err)
		}
	}
	return nil
}
