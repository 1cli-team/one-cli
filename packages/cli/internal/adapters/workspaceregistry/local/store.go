// Package local persists the machine-local Workspace registry in an
// XDG-aware workspaces.json file.
package local

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

const lockRetryDelay = 10 * time.Millisecond
const lockTimeout = 5 * time.Second

type renameFile func(string, string) error

// Store is a file-backed Workspace registry repository. The process-local
// lock is needed in addition to flock: repeated locking through the same flock
// value is intentionally re-entrant, which is not suitable for concurrent RMW
// calls from goroutines sharing a Store. A channel is used so context deadlines
// also bound waiting for this in-process lock.
type Store struct {
	path       string
	lock       *flock.Flock
	localLock  chan struct{}
	renameFile renameFile
}

// Path returns the default XDG-aware workspaces.json path.
func Path() (string, error) {
	var configRoot string
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		configRoot = xdg
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configRoot = filepath.Join(home, ".config")
	}
	return filepath.Join(configRoot, "one", "workspaces.json"), nil
}

// New constructs a Store at the default XDG-aware path.
func New() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return NewAt(path), nil
}

// NewAt constructs a Store at path. It is primarily useful for tests and
// callers that intentionally isolate the One CLI configuration directory.
func NewAt(path string) *Store {
	store := &Store{
		path:       path,
		lock:       flock.New(path + ".lock"),
		localLock:  make(chan struct{}, 1),
		renameFile: os.Rename,
	}
	store.localLock <- struct{}{}
	return store
}

// Path returns this Store's workspaces.json path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load returns the complete registry under a cross-process shared lock.
// A missing file is a valid empty v1 registry.
func (s *Store) Load(ctx context.Context) (registry workspace.Registry, err error) {
	release, err := s.acquire(ctx, false)
	if err != nil {
		return workspace.Registry{}, err
	}
	defer func() {
		err = errors.Join(err, release())
	}()
	return s.loadUnlocked()
}

// Update performs a locked read-modify-write transaction. Parse errors and
// unsupported future versions stop before mutate and publication, ensuring a
// newer or damaged registry is never overwritten by this binary.
func (s *Store) Update(ctx context.Context, mutate func(*workspace.Registry) error) (err error) {
	if mutate == nil {
		return errors.New("workspace registry: mutate function is required")
	}
	release, err := s.acquire(ctx, true)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, release())
	}()

	registry, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	if err := mutate(&registry); err != nil {
		return err
	}
	registry.Version = workspace.RegistrySchemaVersion
	if registry.Workspaces == nil {
		registry.Workspaces = []workspace.RegistryEntry{}
	}
	if err := validateRegistry(registry); err != nil {
		return err
	}
	return s.writeUnlocked(registry)
}

func (s *Store) acquire(ctx context.Context, exclusive bool) (func() error, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, errors.New("workspace registry: path is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lockContext := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > lockTimeout {
		lockContext, cancel = context.WithTimeout(ctx, lockTimeout)
	}
	defer cancel()
	select {
	case <-s.localLock:
	case <-lockContext.Done():
		return nil, fmt.Errorf("lock workspace registry in process: %w", lockContext.Err())
	}
	releaseProcessLock := true
	defer func() {
		if releaseProcessLock {
			s.localLock <- struct{}{}
		}
	}()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create workspace registry directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("secure workspace registry directory: %w", err)
	}

	var locked bool
	var err error
	if exclusive {
		locked, err = s.lock.TryLockContext(lockContext, lockRetryDelay)
	} else {
		locked, err = s.lock.TryRLockContext(lockContext, lockRetryDelay)
	}
	if err != nil {
		return nil, fmt.Errorf("lock workspace registry: %w", err)
	}
	if !locked {
		lockErr := lockContext.Err()
		if lockErr == nil {
			lockErr = errors.New("lock was not acquired")
		}
		return nil, fmt.Errorf("lock workspace registry: %w", lockErr)
	}
	if err := os.Chmod(s.lock.Path(), 0o600); err != nil {
		_ = s.lock.Unlock()
		return nil, fmt.Errorf("secure workspace registry lock: %w", err)
	}

	releaseProcessLock = false
	return func() error {
		unlockErr := s.lock.Unlock()
		s.localLock <- struct{}{}
		if unlockErr != nil {
			return fmt.Errorf("unlock workspace registry: %w", unlockErr)
		}
		return nil
	}, nil
}

func (s *Store) loadUnlocked() (workspace.Registry, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return workspace.Registry{
				Version:    workspace.RegistrySchemaVersion,
				Workspaces: []workspace.RegistryEntry{},
			}, nil
		}
		return workspace.Registry{}, fmt.Errorf("read workspace registry: %w", err)
	}

	var registry workspace.Registry
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return workspace.Registry{}, fmt.Errorf("decode workspace registry: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return workspace.Registry{}, fmt.Errorf("decode workspace registry: %w", err)
	}
	if registry.Version != workspace.RegistrySchemaVersion {
		return workspace.Registry{}, fmt.Errorf(
			"workspace registry: unsupported version %d (current %d)",
			registry.Version,
			workspace.RegistrySchemaVersion,
		)
	}
	if registry.Workspaces == nil {
		registry.Workspaces = []workspace.RegistryEntry{}
	}
	if err := validateRegistry(registry); err != nil {
		return workspace.Registry{}, err
	}
	return registry, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateRegistry(registry workspace.Registry) error {
	if registry.Version != workspace.RegistrySchemaVersion {
		return fmt.Errorf(
			"workspace registry: unsupported version %d (current %d)",
			registry.Version,
			workspace.RegistrySchemaVersion,
		)
	}
	seenEntryIDs := make(map[string]struct{}, len(registry.Workspaces))
	for index, entry := range registry.Workspaces {
		entryID := strings.TrimSpace(entry.EntryID)
		if entryID == "" {
			return fmt.Errorf("workspace registry: workspaces[%d].entryId is required", index)
		}
		if _, exists := seenEntryIDs[entryID]; exists {
			return fmt.Errorf("workspace registry: duplicate entryId %q", entryID)
		}
		seenEntryIDs[entryID] = struct{}{}
		if strings.TrimSpace(entry.Root) == "" {
			return fmt.Errorf("workspace registry: workspaces[%d].root is required", index)
		}
	}
	return nil
}

func (s *Store) writeUnlocked(registry workspace.Registry) error {
	raw, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace registry: %w", err)
	}
	raw = append(raw, '\n')

	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".workspaces-*.json")
	if err != nil {
		return fmt.Errorf("create workspace registry temp file: %w", err)
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("secure workspace registry temp file: %w", err)
	}
	written, err := tmp.Write(raw)
	if err != nil {
		return fmt.Errorf("write workspace registry temp file: %w", err)
	}
	if written != len(raw) {
		return fmt.Errorf("write workspace registry temp file: %w", io.ErrShortWrite)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync workspace registry temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close workspace registry temp file: %w", err)
	}
	closed = true
	if err := s.renameFile(tmpPath, s.path); err != nil {
		return fmt.Errorf("publish workspace registry atomically: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("secure workspace registry file: %w", err)
	}
	if err := syncDirectory(dir); err != nil {
		return err
	}
	return nil
}

func syncDirectory(path string) error {
	// Windows does not support syncing a directory handle through os.File.
	// The temp file itself was synced before rename, so retain the same
	// publication guarantees the rest of the CLI uses on that platform.
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open workspace registry directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("sync workspace registry directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close workspace registry directory after sync: %w", err)
	}
	return nil
}
