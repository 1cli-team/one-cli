// Package oneclient is the shared management API client. It never opens a database.
package oneclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/installation"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/updatecheck"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

const APIVersion = 1

// buildVersion is injected into published clients for single-binary upgrade repair.
var buildVersion string

type Connection struct {
	URL         string `json:"url"`
	Token       string `json:"token,omitempty"`
	Generation  string `json:"generation"`
	Port        int    `json:"port"`
	PID         int    `json:"pid"`
	StorageRoot string `json:"storageRoot"`
}
type Info struct {
	APIVersion   int             `json:"apiVersion"`
	Mode         string          `json:"mode"`
	ID           string          `json:"id"`
	Generation   string          `json:"generation"`
	Version      string          `json:"version"`
	Capabilities map[string]bool `json:"capabilities"`
}
type Profile struct {
	URL             string `json:"url"`
	CredentialStore string `json:"credentialStore,omitempty"`
}
type Binding struct {
	Server   string            `json:"server"`
	Projects map[string]string `json:"projects,omitempty"`
}
type Connections struct {
	Version  int                `json:"version"`
	Default  string             `json:"default"`
	Servers  map[string]Profile `json:"servers"`
	Bindings map[string]Binding `json:"bindings"`
}

type Client struct {
	Root   string
	Binary string
	HTTP   *http.Client
}

func New() *Client {
	p, err := userdirs.Resolve()
	root := ""
	if err == nil {
		root = p.Root
	}
	return &Client{Root: root, HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) transport() *http.Client {
	if c.HTTP != nil {
		client := *c.HTTP
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		return &client
	}
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
func RuntimePath(root string) string { return filepath.Join(root, "state", "server.json") }

func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return "", i18n.Errorf("server.url_invalid")
	}
	local := u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", i18n.Errorf("server.https_required")
	}
	return u.String(), nil
}

func (c *Client) ReadConnections() (Connections, error) {
	p := Connections{Version: 1, Default: "local", Servers: map[string]Profile{}, Bindings: map[string]Binding{}}
	if c.Root == "" {
		return p, i18n.Errorf("server.home_invalid")
	}
	data, err := os.ReadFile(filepath.Join(c.Root, "config", "connections.json"))
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	if p.Version != 1 {
		return p, i18n.Errorf("server.connections_newer")
	}
	if p.Servers == nil {
		p.Servers = map[string]Profile{}
	}
	for name, profile := range p.Servers {
		// Keep the metadata field for existing consumers, but storage is always
		// file-based. Legacy markers never enable another credential backend.
		profile.CredentialStore = CredentialFile
		p.Servers[name] = profile
	}
	if p.Bindings == nil {
		p.Bindings = map[string]Binding{}
	}
	if p.Default == "" {
		p.Default = "local"
	}
	return p, nil
}
func (c *Client) UpdateConnections(ctx context.Context, mutate func(*Connections) error) error {
	if c.Root == "" || !filepath.IsAbs(c.Root) {
		return i18n.Errorf("server.home_invalid")
	}
	path := filepath.Join(c.Root, "config", "connections.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	lock := flock.New(path + ".lock")
	ok, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return err
	}
	if !ok {
		return i18n.Errorf("server.busy")
	}
	defer lock.Unlock()
	p, err := c.ReadConnections()
	if err != nil {
		return err
	}
	if err = mutate(&p); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, append(data, '\n'), 0600)
}
func (c *Client) Select(ctx context.Context, name, root, project string) error {
	return c.UpdateConnections(ctx, func(p *Connections) error {
		if name != "local" {
			if _, ok := p.Servers[name]; !ok {
				return i18n.Errorf("server.profile_missing", name)
			}
		}
		if root == "" {
			p.Default = name
			return nil
		}
		root, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		b := p.Bindings[root]
		if project == "" {
			b.Server = name
		} else {
			if b.Projects == nil {
				b.Projects = map[string]string{}
			}
			b.Projects[project] = name
		}
		p.Bindings[root] = b
		return nil
	})
}
func (c *Client) connection(ctx context.Context, root, project string) (Connection, error) {
	if address := os.Getenv("ONE_SERVER_URL"); address != "" {
		address, err := NormalizeURL(address)
		if err != nil {
			return Connection{}, err
		}
		token, err := c.serverToken(address)
		return Connection{URL: address, Token: token}, err
	}
	if root != "" && project != "" {
		var err error
		project, err = ProjectSelector(root, project)
		if err != nil {
			return Connection{}, err
		}
	}
	p, err := c.ReadConnections()
	if err != nil {
		return Connection{}, err
	}
	name := p.Default
	if b, ok := p.Bindings[root]; ok {
		if b.Server != "" {
			name = b.Server
		}
		if override := b.Projects[project]; override != "" {
			name = override
		}
	}
	if name == "local" {
		return c.EnsureLocal(ctx)
	}
	profile, ok := p.Servers[name]
	if !ok {
		return Connection{}, i18n.Errorf("server.profile_missing", name)
	}
	address, err := NormalizeURL(profile.URL)
	if err != nil {
		return Connection{}, err
	}
	token, err := c.serverToken(address)
	if err != nil {
		return Connection{}, err
	}
	return Connection{URL: address, Token: token}, nil
}

func (c *Client) Request(ctx context.Context, conn Connection, method, path string, body, out any) error {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "\\") {
		return i18n.Errorf("server.url_invalid")
	}
	address, err := NormalizeURL(conn.URL)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		data, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, address+"/api/v1"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+conn.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", i18n.Active())
	if conn.Generation != "" {
		req.Header.Set("X-One-Backend-Generation", conn.Generation)
	}
	resp, err := c.transport().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return i18n.Errorf("server.request_failed", address, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error struct {
				Code        string               `json:"code"`
				Message     string               `json:"message"`
				Context     map[string]any       `json:"context"`
				Remediation []output.Remediation `json:"remediation"`
			} `json:"error"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&e) == nil && e.Error.Code != "" {
			return output.NewError(e.Error.Code, e.Error.Message).WithContext(e.Error.Context).WithRemediation(e.Error.Remediation...)
		}
		return i18n.Errorf("server.http_failed", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return i18n.Errorf("server.response_invalid")
	}
	return nil
}

func (c *Client) Info(ctx context.Context) (Info, error) {
	conn, err := c.connection(ctx, "", "")
	if err != nil {
		return Info{}, err
	}
	var info Info
	err = c.Request(ctx, conn, "GET", "/server/info", nil, &info)
	if err == nil && info.APIVersion != APIVersion {
		err = i18n.Errorf("server.api_incompatible")
	}
	return info, err
}
func (c *Client) ReadRuntime(ctx context.Context) (Connection, error) {
	var conn Connection
	data, err := os.ReadFile(RuntimePath(c.Root))
	if err != nil {
		return conn, err
	}
	if err = json.Unmarshal(data, &conn); err != nil {
		return conn, err
	}
	u, err := url.Parse(conn.URL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || conn.StorageRoot != c.Root || len(conn.Token) != 64 || conn.Generation == "" {
		return Connection{}, i18n.Errorf("server.connection_invalid")
	}
	var info Info
	probe, done := context.WithTimeout(ctx, 500*time.Millisecond)
	defer done()
	if err = c.Request(probe, conn, "GET", "/server/info", nil, &info); err != nil {
		return Connection{}, err
	}
	if info.Generation != conn.Generation || info.APIVersion != APIVersion {
		return Connection{}, i18n.Errorf("server.api_incompatible")
	}
	return conn, nil
}
func (c *Client) EnsureLocal(ctx context.Context) (Connection, error) {
	if conn, err := c.ReadRuntime(ctx); err == nil {
		return conn, nil
	}
	binary := c.Binary
	if binary == "" {
		exe, err := os.Executable()
		if err != nil {
			return Connection{}, err
		}
		binary = installation.Companion(exe)
	}
	if stat, err := os.Stat(binary); err != nil || !stat.Mode().IsRegular() {
		if c.Binary == "" && buildVersion != "" && updatecheck.Channel() == "release" {
			repaired, e := updatecheck.EnsureCompanion(ctx, c.Root, buildVersion)
			if e != nil {
				return Connection{}, e
			}
			binary = repaired
		} else {
			return Connection{}, i18n.Errorf("server.binary_missing", binary)
		}
	}
	logPath := filepath.Join(c.Root, "state", "server.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return Connection{}, err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return Connection{}, err
	}
	defer log.Close()
	staged, err := c.stageDaemon(ctx, binary)
	if err != nil {
		return Connection{}, err
	}
	command := exec.Command(staged, "serve", "--local", "--listen", "127.0.0.1:0")
	command.Stderr = log
	command.Env = append(os.Environ(), "ONE_HOME="+c.Root)
	detach(command)
	if err := command.Start(); err != nil {
		return Connection{}, i18n.Errorf("server.start_failed", err)
	}
	go func() { _ = command.Wait() }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return Connection{}, ctx.Err()
		case <-timer.C:
			return Connection{}, i18n.Errorf("server.start_log", logPath)
		case <-ticker.C:
			if conn, err := c.ReadRuntime(ctx); err == nil {
				return conn, nil
			}
		}
	}
}

type Snapshot struct {
	Manifest *workspace.Manifest `json:"manifest"`
	Revision string              `json:"revision"`
	EntryID  string              `json:"entryId"`
}

func (c *Client) Register(ctx context.Context, root, project string) (Connection, Snapshot, error) {
	m, revision, err := workspace.ReadManifestSnapshot(root)
	if err != nil {
		return Connection{}, Snapshot{}, err
	}
	if m.Workspace == nil || m.Workspace.ID == "" {
		return Connection{}, Snapshot{}, i18n.Errorf("server.identity_required")
	}
	conn, err := c.connection(ctx, root, project)
	if err != nil {
		return Connection{}, Snapshot{}, err
	}
	var snapshot Snapshot
	err = c.Request(ctx, conn, "POST", "/workspaces", map[string]any{"manifest": m, "sourceRevision": revision}, &snapshot)
	if err == nil && os.Getenv("ONE_SERVER_URL") == "" {
		err = c.UpdateConnections(ctx, func(p *Connections) error {
			if _, ok := p.Bindings[root]; !ok {
				p.Bindings[root] = Binding{Server: p.Default}
			}
			return nil
		})
	}
	return conn, snapshot, err
}
func (c *Client) Observe(ctx context.Context, root, source string) error {
	_, _, err := c.Register(ctx, root, "")
	return err
}
func (c *Client) Snapshot(ctx context.Context, root string, local *workspace.Manifest) (*workspace.Manifest, error) {
	_, s, err := c.Register(ctx, root, "")
	if err != nil {
		return nil, err
	}
	return s.Manifest, nil
}
func (c *Client) WorkspaceRequest(ctx context.Context, root, project, method, path string, body, out any) error {
	conn, s, err := c.Register(ctx, root, project)
	if err != nil {
		return err
	}
	return c.Request(ctx, conn, method, "/workspaces/"+url.PathEscape(s.EntryID)+path, body, out)
}
func (c *Client) DefaultRequest(ctx context.Context, method, path string, body, out any) error {
	conn, err := c.connection(ctx, "", "")
	if err != nil {
		return err
	}
	return c.Request(ctx, conn, method, path, body, out)
}
func (c *Client) ConnectionFor(ctx context.Context, root, project string) (Connection, error) {
	return c.connection(ctx, root, project)
}

type Preferences struct {
	Schema       string `json:"schema"`
	StoredLocale string `json:"stored_locale"`
	Resolved     string `json:"resolved"`
	ConfigPath   string `json:"config_path"`
	TaskUI       string `json:"taskUI,omitempty"`
}

func (c *Client) Preferences(ctx context.Context) (Preferences, error) {
	var p Preferences
	err := c.DefaultRequest(ctx, "GET", "/preferences", nil, &p)
	return p, err
}
func (c *Client) SetLocale(ctx context.Context, locale string) (Preferences, error) {
	var p Preferences
	err := c.DefaultRequest(ctx, "PUT", "/preferences", map[string]string{"locale": locale}, &p)
	if err == nil {
		err = preferences.CacheAt(c.Root, &preferences.Preferences{Version: 1, Locale: p.StoredLocale, TaskUI: p.TaskUI})
	}
	return p, err
}

// LocalRoot uses only client-owned connection metadata and engineering files.
func (c *Client) LocalRoot(id string) (string, error) {
	p, err := c.ReadConnections()
	if err != nil {
		return "", err
	}
	for root := range p.Bindings {
		m, err := workspace.ReadManifest(root)
		if err == nil && m.Workspace != nil && m.Workspace.ID == id {
			return root, nil
		}
	}
	return "", fmt.Errorf("%s: %s", i18n.T("server.local_path_missing"), id)
}

// RemoteToken lets the trusted native host share the CLI's private credential file.
func RemoteToken(address, token string, save bool) (string, error) {
	return New().remoteToken(address, token, save)
}

// BindAddress connects a checkout selected in Desktop to the server currently
// displayed there. Only bootstrap metadata is written on the client.
func (c *Client) BindAddress(ctx context.Context, root, address string, local bool) error {
	if local {
		return c.Select(ctx, "local", root, "")
	}
	address, err := NormalizeURL(address)
	if err != nil {
		return err
	}
	return c.UpdateConnections(ctx, func(p *Connections) error {
		name := ""
		for candidate, profile := range p.Servers {
			if profile.URL == address {
				name = candidate
				break
			}
		}
		if name == "" {
			sum := sha256.Sum256([]byte(address))
			name = fmt.Sprintf("desktop-%x", sum[:6])
			p.Servers[name] = Profile{URL: address, CredentialStore: CredentialFile}
		}
		binding := p.Bindings[root]
		binding.Server = name
		p.Bindings[root] = binding
		return nil
	})
}
func (c *Client) LocalRootFor(id, address string) (string, error) {
	p, err := c.ReadConnections()
	if err != nil {
		return "", err
	}
	for root, binding := range p.Bindings {
		name := binding.Server
		if name == "" {
			name = p.Default
		}
		matches := p.Servers[name].URL == address
		if name == "local" {
			var local Connection
			if raw, e := os.ReadFile(RuntimePath(c.Root)); e == nil && json.Unmarshal(raw, &local) == nil {
				matches = local.URL == address
			}
		}
		if !matches {
			continue
		}
		m, e := workspace.ReadManifest(root)
		if e == nil && m.Workspace != nil && m.Workspace.ID == id {
			return root, nil
		}
	}
	return "", fmt.Errorf("%s: %s", i18n.T("server.local_path_missing"), id)
}
