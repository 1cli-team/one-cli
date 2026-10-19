package oneclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/zalando/go-keyring"
)

type CredentialStore string

const (
	CredentialFile    CredentialStore = "file"
	CredentialKeyring CredentialStore = "keyring"
	maxCredentialFile                 = 4 << 20
)

type keyringBackend interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
}
type systemKeyring struct{}

func (systemKeyring) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}
func (systemKeyring) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}
func (c *Client) systemCredentials() keyringBackend {
	if c.keyring != nil {
		return c.keyring
	}
	return systemKeyring{}
}

func validCredentialStore(store CredentialStore) bool {
	return store == CredentialFile || store == CredentialKeyring
}
func profileCredentialStore(profile Profile) CredentialStore {
	if profile.CredentialStore == "" {
		return CredentialKeyring // Profiles written before store selection used the keyring.
	}
	return profile.CredentialStore
}
func credentialStoreForAddress(profiles Connections, address string, fallback CredentialStore) (CredentialStore, bool) {
	for _, profile := range profiles.Servers {
		if profile.URL == address && profileCredentialStore(profile) == CredentialFile {
			return CredentialFile, true
		}
	}
	for _, profile := range profiles.Servers {
		if profile.URL == address {
			return profileCredentialStore(profile), true
		}
	}
	return fallback, false
}

// AddWithCredentialStore selects storage explicitly. Re-adding an existing URL
// without a token copies its credential when changing stores. Other installations
// may share the old keyring item, so migration leaves that item intact.
func (c *Client) AddWithCredentialStore(ctx context.Context, name, address, token string, store CredentialStore) error {
	if name == "local" || !workspace.IsValidProjectName(name) {
		return i18n.Errorf("server.name_invalid")
	}
	address, err := NormalizeURL(address)
	if err != nil {
		return err
	}
	if !validCredentialStore(store) {
		return i18n.Errorf("server.credential_store_invalid", store)
	}
	if len(token) > 16<<10 || strings.ContainsAny(token, "\r\n") {
		return i18n.Errorf("server.credential_input_invalid")
	}
	return c.UpdateConnections(ctx, func(p *Connections) error {
		oldStore, exists := credentialStoreForAddress(*p, address, store)
		if token == "" && exists && oldStore != store {
			token, err = c.readCredential(address, oldStore)
			if err != nil {
				return c.credentialReadError(address, err)
			}
		}
		if token != "" {
			if err := c.writeCredential(ctx, address, token, store); err != nil {
				return err
			}
		}
		for alias, profile := range p.Servers {
			if profile.URL == address {
				profile.CredentialStore = store
				p.Servers[alias] = profile
			}
		}
		p.Servers[name] = Profile{URL: address, CredentialStore: store}
		return nil
	})
}

func (c *Client) credentialReadError(address string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return i18n.Errorf("server.credential_missing", address)
	}
	return err
}
func (c *Client) serverToken(address string, store CredentialStore) (string, error) {
	if token := os.Getenv("ONE_SERVER_TOKEN"); token != "" {
		return token, nil
	}
	token, err := c.readCredential(address, store)
	if err != nil {
		return "", c.credentialReadError(address, err)
	}
	return token, nil
}
func (c *Client) readCredential(address string, store CredentialStore) (string, error) {
	if store == CredentialKeyring {
		token, err := c.systemCredentials().Get("one-cli.server", address)
		if errors.Is(err, keyring.ErrNotFound) || (err == nil && token == "") {
			return "", os.ErrNotExist
		}
		if err != nil {
			return "", i18n.Errorf("server.credential_keyring_read_failed", err)
		}
		return token, nil
	}
	if store != CredentialFile {
		return "", i18n.Errorf("server.credential_store_invalid", store)
	}
	path, err := c.credentialPath()
	if err != nil {
		return "", err
	}
	credentials, err := readCredentialFile(path)
	if err != nil {
		return "", err
	}
	if token := credentials.Tokens[address]; token != "" {
		return token, nil
	}
	return "", os.ErrNotExist
}
func (c *Client) writeCredential(ctx context.Context, address, token string, store CredentialStore) error {
	if store == CredentialKeyring {
		if err := c.systemCredentials().Set("one-cli.server", address, token); err != nil {
			return i18n.Errorf("server.credential_keyring_write_failed", err)
		}
		return nil
	}
	path, err := c.credentialPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return i18n.Errorf("server.credential_file_write_failed", path, err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return i18n.Errorf("server.credential_file_write_failed", path, err)
	}
	if err := fsutil.SafeWritePath(c.Root, path+".lock"); err != nil {
		return err
	}
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return i18n.Errorf("server.busy")
	}
	defer lock.Unlock()
	credentials, err := readCredentialFile(path)
	if errors.Is(err, os.ErrNotExist) {
		credentials = credentialFile{Version: 1, Tokens: map[string]string{}}
	} else if err != nil {
		return err
	}
	credentials.Tokens[address] = token
	data, err := json.MarshalIndent(credentials, "", "  ")
	if len(data)+1 > maxCredentialFile {
		return i18n.Errorf("server.credential_file_full", path)
	}
	if err == nil {
		err = fsutil.WriteAtomic(path, append(data, '\n'), 0600)
	}
	if err != nil {
		return i18n.Errorf("server.credential_file_write_failed", path, err)
	}
	return nil
}

type credentialFile struct {
	Version int               `json:"version"`
	Tokens  map[string]string `json:"tokens"`
}

func (c *Client) credentialPath() (string, error) {
	if c.Root == "" || !filepath.IsAbs(c.Root) {
		return "", i18n.Errorf("server.home_invalid")
	}
	path := filepath.Join(c.Root, "config", "server-credentials.json")
	if err := fsutil.SafeWritePath(c.Root, path); err != nil {
		return "", err
	}
	for _, dir := range []string{c.Root, filepath.Dir(path)} {
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", i18n.Errorf("server.credential_file_read_failed", path, err)
		}
		if !info.IsDir() || !privateCredentialOwner(info, true) {
			return "", i18n.Errorf("server.credential_directory_unsafe", dir)
		}
	}
	return path, nil
}
func readCredentialFile(path string) (credentialFile, error) {
	var credentials credentialFile
	f, err := openCredentialFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return credentials, err
		}
		return credentials, i18n.Errorf("server.credential_file_read_failed", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return credentials, i18n.Errorf("server.credential_file_read_failed", path, err)
	}
	if !info.Mode().IsRegular() || !privateCredentialOwner(info, false) {
		return credentials, i18n.Errorf("server.credential_file_unsafe", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxCredentialFile+1))
	if err != nil {
		return credentials, i18n.Errorf("server.credential_file_read_failed", path, err)
	}
	if len(data) > maxCredentialFile || json.Unmarshal(data, &credentials) != nil || credentials.Version < 1 {
		// Decoder errors may contain token fragments; do not include file contents.
		return credentials, i18n.Errorf("server.credential_file_invalid", path)
	}
	if credentials.Version > 1 {
		return credentials, i18n.Errorf("server.credential_file_newer", path)
	}
	if credentials.Tokens == nil {
		return credentials, i18n.Errorf("server.credential_file_invalid", path)
	}
	for address, token := range credentials.Tokens {
		normalized, err := NormalizeURL(address)
		if err != nil || normalized != address || token == "" || len(token) > 16<<10 || strings.ContainsAny(token, "\r\n") {
			return credentials, i18n.Errorf("server.credential_file_invalid", path)
		}
	}
	return credentials, nil
}

// Desktop defaults to the keyring for new addresses, and respects an existing
// CLI profile's selected store so both clients can share and rotate credentials.
func (c *Client) remoteToken(address, token string, save bool) (string, error) {
	address, err := NormalizeURL(address)
	if err != nil {
		return "", err
	}
	if save {
		if token == "" || len(token) > 16<<10 || strings.ContainsAny(token, "\r\n") {
			return "", i18n.Errorf("server.credential_input_invalid")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return "", c.UpdateConnections(ctx, func(p *Connections) error {
			store, _ := credentialStoreForAddress(*p, address, CredentialKeyring)
			return c.writeCredential(ctx, address, token, store)
		})
	}
	profiles, err := c.ReadConnections()
	if err != nil {
		return "", err
	}
	store, _ := credentialStoreForAddress(profiles, address, CredentialKeyring)
	value, err := c.readCredential(address, store)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return value, err
}
