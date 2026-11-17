package issuetracking

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type bindingStore struct{}
type bindingDocument struct {
	Version   int                        `json:"version"`
	Bindings  map[string]Binding         `json:"bindings"`
	Creations map[string]projectCreation `json:"creations,omitempty"`
}

func (*bindingStore) access(ctx context.Context, change func(*bindingDocument) (bool, error)) error {
	root, err := userdirs.Config()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	path := filepath.Join(root, "issue-bindings.json")
	lock := flock.New(path + ".lock")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return err
	}
	if !ok {
		return Fail("busy")
	}
	defer lock.Unlock()
	doc := bindingDocument{Version: 1, Bindings: map[string]Binding{}}
	raw, err := fsutil.ReadFile(path)
	if err == nil {
		if json.Unmarshal(raw, &doc) != nil || doc.Version != 1 || doc.Bindings == nil {
			return Fail("storage")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	changed, err := change(&doc)
	if err != nil || !changed {
		return err
	}
	raw, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(root, ".issue-bindings-*.json")
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

func (s *bindingStore) get(ctx context.Context, scope Scope) (*Binding, error) {
	var result *Binding
	err := s.access(ctx, func(doc *bindingDocument) (bool, error) {
		if found, ok := doc.Bindings[scope.Key()]; ok {
			result = &found
		}
		return false, nil
	})
	return result, err
}
func (s *bindingStore) put(ctx context.Context, scope Scope, binding *Binding) error {
	return s.access(ctx, func(doc *bindingDocument) (bool, error) {
		if binding == nil {
			delete(doc.Bindings, scope.Key())
		} else {
			doc.Bindings[scope.Key()] = *binding
			delete(doc.Creations, scope.Key())
		}
		return true, nil
	})
}
