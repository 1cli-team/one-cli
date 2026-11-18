package openviking

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
	ov "github.com/volcengine/OpenViking/sdk/go"
)

type connection struct {
	URL     string `json:"url"`
	APIKey  string `json:"apiKey,omitempty"`
	Account string `json:"account,omitempty"`
	User    string `json:"user,omitempty"`
	Scope   string `json:"scope"`
}

type workspaceBinding struct {
	URI   string                  `json:"uri"`
	Scope string                  `json:"scope,omitempty"`
	Agent knowledge.AgentSettings `json:"agent,omitzero"`
	// Older releases stored their own connection alongside the URI.
	URL    string `json:"url,omitempty"`
	APIKey string `json:"apiKey,omitempty"`
}

func (c connection) public() knowledge.Connection {
	return knowledge.Connection{URL: c.URL, HasAPIKey: c.APIKey != "", Account: c.Account, User: c.User}
}

func (c connection) sameCredential(other connection) bool {
	return c.URL == other.URL && c.APIKey == other.APIKey
}

func (c connection) sameIdentity(other connection) bool {
	return c.URL == other.URL && c.Account != "" && c.User != "" &&
		c.Account == other.Account && c.User == other.User
}

func (c connection) scope() string {
	if c.Account != "" && c.User != "" {
		return digest(c.URL + "\x00identity\x00" + c.Account + "\x00" + c.User)
	}
	return digest(c.URL + "\x00credential\x00" + c.APIKey)
}

func digest(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }

func configDirectory() (string, error) {
	config, err := userdirs.Config()
	return filepath.Join(config, "openviking"), err
}

func connectionPath(dir string) string { return filepath.Join(dir, "connection.json") }

func lockSettings(ctx context.Context, dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return fsutil.WorkspaceLock(ctx, dir, "openviking")
}

func bindingPath(dir, workspaceID string) string {
	return filepath.Join(dir, digest(workspaceID)+".json")
}

func readConfig[T any](path string) (T, error) {
	var value T
	raw, err := fsutil.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return value, nil
	}
	if err != nil {
		return value, err
	}
	if json.Unmarshal(raw, &value) != nil {
		return value, i18n.Errorf("openviking.storage")
	}
	return value, nil
}

func writeConfig(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := fsutil.WriteAtomic(path, data, 0600); err != nil {
		return err
	}
	return fsutil.SyncDir(dir)
}

type legacyBinding struct {
	file string
	workspaceBinding
}

var workspaceFileName = regexp.MustCompile("^[0-9a-f]{64}[.]json$")

func legacyBindings(dir string) ([]legacyBinding, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var matches []legacyBinding
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !workspaceFileName.MatchString(entry.Name()) {
			continue
		}
		file := filepath.Join(dir, entry.Name())
		saved, err := readConfig[workspaceBinding](file)
		if err != nil {
			return nil, err
		}
		if saved.URL != "" {
			matches = append(matches, legacyBinding{file: file, workspaceBinding: saved})
		}
	}
	return matches, nil
}

func migrateBindings(saved connection, legacy []legacyBinding) error {
	for _, entry := range legacy {
		if entry.URL != saved.URL || entry.APIKey != saved.APIKey {
			continue
		}
		if err := writeConfig(entry.file, workspaceBinding{URI: entry.URI, Scope: saved.Scope}); err != nil {
			return err
		}
	}
	return nil
}

// Adopt the old connection only when all workspaces had the same credentials.
func loadConnection(ctx context.Context, dir string) (connection, error) {
	file := connectionPath(dir)
	saved, err := readConfig[connection](file)
	if err != nil || saved.URL != "" {
		return saved, err
	}
	legacy, err := legacyBindings(dir)
	if err != nil || len(legacy) == 0 {
		return connection{}, err
	}
	unlock, err := lockSettings(ctx, dir)
	if err != nil {
		return connection{}, err
	}
	defer unlock()
	saved, err = readConfig[connection](file)
	if err != nil || saved.URL != "" {
		return saved, err
	}
	legacy, err = legacyBindings(dir)
	if err != nil || len(legacy) == 0 {
		return connection{}, err
	}
	candidate := connection{URL: legacy[0].URL, APIKey: legacy[0].APIKey}
	for _, entry := range legacy[1:] {
		if !candidate.sameCredential(connection{URL: entry.URL, APIKey: entry.APIKey}) {
			return connection{}, nil
		}
	}
	candidate.Scope = candidate.scope()
	if err := writeConfig(file, candidate); err != nil {
		return connection{}, err
	}
	return candidate, migrateBindings(candidate, legacy)
}

func (*Plugin) Connection(ctx context.Context) (*knowledge.Connection, error) {
	dir, err := configDirectory()
	if err != nil {
		return nil, err
	}
	saved, err := loadConnection(ctx, dir)
	if err != nil || saved.URL == "" {
		return nil, err
	}
	public := saved.public()
	return &public, nil
}

// ConnectionForSync returns credentials for server-side forwarding to an
// explicitly selected execution machine. Client-facing status uses Connection.
func (*Plugin) ConnectionForSync(ctx context.Context) (knowledge.ConnectionInput, error) {
	dir, err := configDirectory()
	if err != nil {
		return knowledge.ConnectionInput{}, err
	}
	saved, err := loadConnection(ctx, dir)
	if err != nil {
		return knowledge.ConnectionInput{}, err
	}
	if err := requireConnection(saved); err != nil {
		return knowledge.ConnectionInput{}, err
	}
	return knowledge.ConnectionInput{URL: saved.URL, APIKey: &saved.APIKey}, nil
}

func configured(saved connection, input knowledge.ConnectionInput) (connection, error) {
	address, err := parseAddress(input.URL)
	if err != nil {
		return connection{}, err
	}
	next := connection{URL: address.String()}
	if input.APIKey != nil {
		next.APIKey = strings.TrimSpace(*input.APIKey)
	} else if saved.URL == next.URL {
		next.APIKey = saved.APIKey
	}
	if len(next.APIKey) > 8192 || strings.ContainsFunc(next.APIKey, unicode.IsControl) {
		return connection{}, cliErrors.New(cliErrors.OPENVIKING_INVALID_INPUT, i18n.T("openviking.invalid_key"))
	}
	return next, nil
}

func requireConnection(saved connection) error {
	if saved.URL == "" {
		return cliErrors.New(cliErrors.OPENVIKING_UNBOUND, i18n.T("openviking.connection_required"))
	}
	return nil
}

func probe(ctx context.Context, saved connection) (connection, error) {
	// Check an authenticated data API because liveness may be public.
	client, err := sdk(saved, 8*time.Second)
	if err != nil {
		return connection{}, err
	}
	if entries, err := client.List(ctx, "viking://resources", &ov.ListOptions{Limit: 1}); err != nil {
		return connection{}, sdkError(err)
	} else if entries == nil {
		return connection{}, invalidResponse()
	}
	// Older deployments can accept a key without returning identity from health.
	payload, err := fetch(ctx, saved, http.MethodGet, "/health", nil, nil, "", 4*time.Second)
	if err == nil {
		var identity struct {
			Account string `json:"account_id"`
			User    string `json:"user_id"`
		}
		if json.Unmarshal(payload, &identity) == nil && identity.Account != "" && identity.User != "" {
			saved.Account, saved.User = identity.Account, identity.User
		}
	}
	if ctx.Err() != nil {
		return connection{}, ctx.Err()
	}
	return saved, nil
}

func (*Plugin) TestConnection(ctx context.Context, input knowledge.ConnectionInput) (knowledge.Connection, error) {
	dir, err := configDirectory()
	if err != nil {
		return knowledge.Connection{}, err
	}
	saved, err := loadConnection(ctx, dir)
	if err != nil {
		return knowledge.Connection{}, err
	}
	if input.URL != "" || input.APIKey != nil {
		saved, err = configured(saved, input)
		if err != nil {
			return knowledge.Connection{}, err
		}
	}
	if err := requireConnection(saved); err != nil {
		return knowledge.Connection{}, err
	}
	checked, err := probe(ctx, saved)
	return checked.public(), err
}

func (*Plugin) Configure(ctx context.Context, input knowledge.ConnectionInput) (knowledge.Connection, error) {
	dir, err := configDirectory()
	if err != nil {
		return knowledge.Connection{}, err
	}
	if _, err := loadConnection(ctx, dir); err != nil {
		return knowledge.Connection{}, err
	}
	file := connectionPath(dir)
	unlock, err := lockSettings(ctx, dir)
	if err != nil {
		return knowledge.Connection{}, err
	}
	defer unlock()
	saved, err := readConfig[connection](file)
	if err != nil {
		return knowledge.Connection{}, err
	}
	next, err := configured(saved, input)
	if err != nil {
		return knowledge.Connection{}, err
	}
	next, err = probe(ctx, next)
	if err != nil {
		return knowledge.Connection{}, err
	}
	switch {
	case next.sameIdentity(saved):
		next.Scope = saved.Scope
	case next.sameCredential(saved) && (saved.Account == "" || next.Account == ""):
		next.Scope = saved.Scope
		if next.Account == "" {
			next.Account, next.User = saved.Account, saved.User
		}
	default:
		next.Scope = next.scope()
	}
	legacy, err := legacyBindings(dir)
	if err != nil {
		return knowledge.Connection{}, err
	}
	if err := writeConfig(file, next); err != nil {
		return knowledge.Connection{}, err
	}
	return next.public(), migrateBindings(next, legacy)
}
