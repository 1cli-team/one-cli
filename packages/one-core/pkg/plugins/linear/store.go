package linear

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

const defaultRedirect = "http://127.0.0.1:39127/oauth/linear/callback"

type credential struct {
	tracking.Connection
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ClientID     string    `json:"clientId"`
}

type operationRecord struct {
	tracking.Operation
	ConnectionID string `json:"connectionId"`
	ProjectID    string `json:"projectId"`
	IssueID      string `json:"issueId,omitempty"`
	Digest       string `json:"digest"`
}

type document struct {
	Version     int                        `json:"version"`
	Config      tracking.OAuthConfig       `json:"config"`
	Connections map[string]credential      `json:"connections"`
	Operations  map[string]operationRecord `json:"operations"`
}

func (p *Plugin) transaction(ctx context.Context, action func(*document) (bool, error)) error {
	root := p.root
	if root == "" {
		var err error
		root, err = userdirs.Config()
		if err != nil {
			return err
		}
		root = filepath.Join(root, "linear")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	path := filepath.Join(root, "connections.json")
	lock := flock.New(path + ".lock")
	wait, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(wait, 20*time.Millisecond)
	if err != nil {
		return err
	}
	if !ok {
		return tracking.Fail("busy")
	}
	defer lock.Unlock()
	doc := document{Version: 1, Config: tracking.OAuthConfig{RedirectURI: defaultRedirect}, Connections: map[string]credential{}, Operations: map[string]operationRecord{}}
	raw, err := fsutil.ReadFile(path)
	if err == nil {
		if json.Unmarshal(raw, &doc) != nil || doc.Version != 1 || doc.Connections == nil || doc.Operations == nil {
			return tracking.Fail("storage")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	changed, err := action(&doc)
	if err != nil || !changed {
		return err
	}
	raw, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(root, ".connections-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err = tmp.Chmod(0600); err != nil {
		return err
	}
	if _, err = tmp.Write(raw); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = fsutil.ReplaceFile(tmp.Name(), path); err != nil {
		return err
	}
	return fsutil.SyncDir(root)
}
