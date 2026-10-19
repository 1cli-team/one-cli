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
)

const (
	// CredentialFile is the only supported client credential storage format.
	CredentialFile    = "file"
	maxCredentialFile = 4 << 20
)

// Add registers a server and saves a supplied token in the private credential file.
// An empty token leaves any existing file credential unchanged.
func (c *Client) Add(ctx context.Context, name, address, token string) error {
	if name == "local" || !workspace.IsValidProjectName(name) {
		return i18n.Errorf("server.name_invalid")
	}
	address, err := NormalizeURL(address)
	if err != nil {
		return err
	}
	if len(token) > 16<<10 || strings.ContainsAny(token, "\r\n") {
		return i18n.Errorf("server.credential_input_invalid")
	}
	return c.UpdateConnections(ctx, func(p *Connections) error {
		if token != "" {
			if err := c.writeCredential(ctx, address, token); err != nil {
				return err
			}
		}
		p.Servers[name] = Profile{URL: address, CredentialStore: CredentialFile}
		return nil
	})
}

func (c *Client) credentialReadError(address string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return i18n.Errorf("server.credential_missing", address)
	}
	return err
}
func (c *Client) serverToken(address string) (string, error) {
	if token := os.Getenv("ONE_SERVER_TOKEN"); token != "" {
		return token, nil
	}
	token, err := c.readCredential(address)
	if err != nil {
		return "", c.credentialReadError(address, err)
	}
	return token, nil
}
func (c *Client) readCredential(address string) (string, error) {
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
func (c *Client) writeCredential(ctx context.Context, address, token string) error {
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

// Desktop uses the same private file as CLI connections, including new addresses
// that do not yet have a named CLI profile.
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
		return "", c.writeCredential(ctx, address, token)
	}
	value, err := c.readCredential(address)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return value, err
}
