// Package store owns management persistence. Clients never open these databases.
package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
)

var (
	ErrNotFound = errors.New("management record not found")
	ErrConflict = errors.New("management revision conflict")
)

type Config struct {
	SQLite, Postgres string
	Key              []byte
}
type Store struct {
	db       *sql.DB
	postgres bool
	aead     cipher.AEAD
}

func Open(ctx context.Context, cfg Config) (*Store, error) {
	if (cfg.SQLite == "") == (cfg.Postgres == "") {
		return nil, errors.New("exactly one database must be configured")
	}
	var err error
	s := &Store{postgres: cfg.Postgres != ""}
	if len(cfg.Key) > 0 {
		block, e := aes.NewCipher(cfg.Key)
		if e != nil {
			return nil, e
		}
		s.aead, err = cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
	}
	driver, dsn := "pgx", cfg.Postgres
	if !s.postgres {
		if !filepath.IsAbs(cfg.SQLite) {
			return nil, errors.New("SQLite path must be absolute")
		}
		if err = os.MkdirAll(filepath.Dir(cfg.SQLite), 0700); err != nil {
			return nil, err
		}
		driver, dsn = "sqlite", cfg.SQLite
	}
	s.db, err = sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if !s.postgres {
		s.db.SetMaxOpenConns(1)
	} else {
		s.db.SetMaxOpenConns(10)
	}
	if err = s.db.PingContext(ctx); err != nil {
		s.Close()
		return nil, fmt.Errorf("connect management database: %w", err)
	}
	if !s.postgres {
		for _, q := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000", "PRAGMA journal_mode = WAL"} {
			if _, err = s.db.ExecContext(ctx, q); err != nil {
				s.Close()
				return nil, err
			}
		}
		if err = os.Chmod(cfg.SQLite, 0600); err != nil {
			s.Close()
			return nil, err
		}
	}
	if err = s.migrate(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) SQL(q string) string {
	if !s.postgres {
		return q
	}
	n := 0
	return replaceQuestions(q, &n)
}
func replaceQuestions(q string, n *int) string {
	var b strings.Builder
	for _, r := range q {
		if r == '?' {
			*n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(*n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.postgres {
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(763263661)"); err != nil {
			return err
		}
	}
	queries := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, revision INTEGER NOT NULL, source_revision TEXT NOT NULL, environment TEXT NOT NULL, groups_json TEXT NOT NULL, registered_at TEXT NOT NULL, last_seen_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS projects (workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, name TEXT NOT NULL, relative_dir TEXT NOT NULL, toolchain TEXT NOT NULL, PRIMARY KEY(workspace_id,name), UNIQUE(workspace_id,relative_dir))`,
		`CREATE TABLE IF NOT EXISTS preferences (scope TEXT PRIMARY KEY, locale TEXT NOT NULL, task_ui TEXT NOT NULL, revision INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS credentials (key TEXT PRIMARY KEY, encrypted_value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS sessions (token_hash TEXT PRIMARY KEY, expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS imports (source TEXT PRIMARY KEY, digest TEXT NOT NULL, completed_at TEXT NOT NULL)`,
	}
	if _, err = tx.ExecContext(ctx, queries[0]); err != nil {
		return err
	}
	var version int
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&version); err != nil {
		return err
	}
	if version > 2 {
		return errors.New("database schema is newer than this server")
	}
	for _, q := range queries[1:] {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES(1) ON CONFLICT(version) DO NOTHING"); err != nil {
		return err
	}
	if version < 2 {
		if _, err = tx.ExecContext(ctx, "ALTER TABLE workspaces ADD COLUMN archived INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES(2)"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func (s *Store) Load(ctx context.Context, id string) (*workspace.Manifest, string, error) {
	return s.load(ctx, s.db, id)
}
func (s *Store) load(ctx context.Context, q queryer, id string) (*workspace.Manifest, string, error) {
	m := &workspace.Manifest{Version: workspace.ManifestVersion, Workspace: &workspace.ManifestWorkspace{ID: id}, Projects: []workspace.ManifestProject{}}
	var revision int64
	var env, groups string
	err := q.QueryRowContext(ctx, s.SQL("SELECT name,revision,environment,groups_json FROM workspaces WHERE id=?"), id).Scan(&m.Workspace.Name, &revision, &env, &groups)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if err = json.Unmarshal([]byte(env), &m.Env); err != nil {
		return nil, "", err
	}
	if err = json.Unmarshal([]byte(groups), &m.Groups); err != nil {
		return nil, "", err
	}
	rows, err := q.QueryContext(ctx, s.SQL("SELECT name,relative_dir,toolchain FROM projects WHERE workspace_id=? ORDER BY relative_dir"), id)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		var p workspace.ManifestProject
		if err = rows.Scan(&p.Name, &p.RelativeDir, &p.Toolchain); err != nil {
			return nil, "", err
		}
		m.Projects = append(m.Projects, p)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	return m, strconv.FormatInt(revision, 10), nil
}

// Register imports engineering declarations while preserving server-owned
// environment settings, even when an old checkout still contains a binding.
func (s *Store) Register(ctx context.Context, m *workspace.Manifest, sourceRevision string) (string, error) {
	if err := workspace.ValidateManifestContext(ctx, m); err != nil {
		return "", err
	}
	if m.Workspace == nil {
		return "", errors.New("workspace identity is required")
	}
	env, err := json.Marshal(m.Env)
	if err != nil {
		return "", err
	}
	groups, err := json.Marshal(m.Groups)
	if err != nil {
		return "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, s.SQL(`INSERT INTO workspaces(id,name,revision,source_revision,environment,groups_json,registered_at,last_seen_at) VALUES(?,?,1,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`), m.Workspace.ID, m.Workspace.Name, sourceRevision, string(env), string(groups), now, now)
	if err != nil {
		return "", err
	}
	query := "SELECT source_revision,revision FROM workspaces WHERE id=?"
	if s.postgres {
		query += " FOR UPDATE"
	}
	var oldSource string
	var revision int64
	if err = tx.QueryRowContext(ctx, s.SQL(query), m.Workspace.ID).Scan(&oldSource, &revision); err != nil {
		return "", err
	}
	// New records need their projects; unchanged registrations only update last seen.
	var count int
	if err = tx.QueryRowContext(ctx, s.SQL("SELECT COUNT(*) FROM projects WHERE workspace_id=?"), m.Workspace.ID).Scan(&count); err != nil {
		return "", err
	}
	if oldSource != sourceRevision || count != len(m.Projects) {
		if err = s.writeProjects(ctx, tx, m); err != nil {
			return "", err
		}
		if oldSource != sourceRevision {
			revision++
		}
		_, err = tx.ExecContext(ctx, s.SQL("UPDATE workspaces SET name=?,source_revision=?,groups_json=?,revision=?,last_seen_at=?,archived=0 WHERE id=?"), m.Workspace.Name, sourceRevision, string(groups), revision, now, m.Workspace.ID)
	} else {
		_, err = tx.ExecContext(ctx, s.SQL("UPDATE workspaces SET last_seen_at=?,archived=0 WHERE id=?"), now, m.Workspace.ID)
	}
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return strconv.FormatInt(revision, 10), nil
}

func (s *Store) writeProjects(ctx context.Context, tx *sql.Tx, m *workspace.Manifest) error {
	if _, err := tx.ExecContext(ctx, s.SQL("DELETE FROM projects WHERE workspace_id=?"), m.Workspace.ID); err != nil {
		return err
	}
	for _, p := range m.Projects {
		if _, err := tx.ExecContext(ctx, s.SQL("INSERT INTO projects(workspace_id,name,relative_dir,toolchain) VALUES(?,?,?,?)"), m.Workspace.ID, p.Name, p.RelativeDir, p.Toolchain); err != nil {
			return err
		}
	}
	return nil
}

// Save changes only managed settings. Engineering declarations are registered by clients.
func (s *Store) Save(ctx context.Context, id string, m *workspace.Manifest, expected string) (string, error) {
	if err := workspace.ValidateManifestContext(ctx, m); err != nil {
		return "", err
	}
	revision, err := strconv.ParseInt(expected, 10, 64)
	if err != nil {
		return "", ErrConflict
	}
	env, err := json.Marshal(m.Env)
	if err != nil {
		return "", err
	}
	r, err := s.db.ExecContext(ctx, s.SQL("UPDATE workspaces SET environment=?,revision=revision+1 WHERE id=? AND revision=?"), string(env), id, revision)
	if err != nil {
		return "", err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return "", err
	}
	if n != 1 {
		return "", ErrConflict
	}
	return strconv.FormatInt(revision+1, 10), nil
}

type Entry struct {
	ID, Name, RegisteredAt, LastSeenAt string
	ProjectCount                       int
}

func (s *Store) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT w.id,w.name,w.registered_at,w.last_seen_at,COUNT(p.name) FROM workspaces w LEFT JOIN projects p ON p.workspace_id=w.id WHERE w.archived=0 GROUP BY w.id,w.name,w.registered_at,w.last_seen_at ORDER BY w.name,w.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err = rows.Scan(&e.ID, &e.Name, &e.RegisteredAt, &e.LastSeenAt, &e.ProjectCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Store) Forget(ctx context.Context, id string) error {
	r, err := s.db.ExecContext(ctx, s.SQL("UPDATE workspaces SET archived=1 WHERE id=?"), id)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type Preferences struct {
	Locale string `json:"locale"`
	TaskUI string `json:"taskUI,omitempty"`
}

func (s *Store) Preferences(ctx context.Context) (Preferences, error) {
	p := Preferences{Locale: "auto"}
	err := s.db.QueryRowContext(ctx, "SELECT locale,task_ui FROM preferences WHERE scope='owner'").Scan(&p.Locale, &p.TaskUI)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return p, err
}
func (s *Store) SavePreferences(ctx context.Context, p Preferences) error {
	_, err := s.db.ExecContext(ctx, s.SQL("INSERT INTO preferences(scope,locale,task_ui,revision) VALUES('owner',?,?,1) ON CONFLICT(scope) DO UPDATE SET locale=excluded.locale,task_ui=excluded.task_ui,revision=preferences.revision+1"), p.Locale, p.TaskUI)
	return err
}
func (s *Store) Setting(ctx context.Context, key string) ([]byte, error) {
	var value string
	err := s.db.QueryRowContext(ctx, s.SQL("SELECT value FROM settings WHERE key=?"), key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return []byte(value), err
}
func (s *Store) PutSetting(ctx context.Context, key string, value []byte) error {
	_, err := s.db.ExecContext(ctx, s.SQL("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value"), key, string(value))
	return err
}

func (s *Store) PutSecret(ctx context.Context, key string, value []byte) error {
	if s.aead == nil {
		return errors.New("server encryption key is not configured")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := s.aead.Seal(nonce, nonce, value, []byte(key))
	_, err := s.db.ExecContext(ctx, s.SQL("INSERT INTO credentials(key,encrypted_value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET encrypted_value=excluded.encrypted_value"), key, base64.StdEncoding.EncodeToString(sealed))
	return err
}
func (s *Store) Secret(ctx context.Context, key string) ([]byte, error) {
	if s.aead == nil {
		return nil, errors.New("server encryption key is not configured")
	}
	var value string
	err := s.db.QueryRowContext(ctx, s.SQL("SELECT encrypted_value FROM credentials WHERE key=?"), key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sealed, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(sealed) < s.aead.NonceSize() {
		return nil, errors.New("invalid encrypted credential")
	}
	return s.aead.Open(nil, sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():], []byte(key))
}
func (s *Store) DeleteSecret(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, s.SQL("DELETE FROM credentials WHERE key=?"), key)
	return err
}
func (s *Store) AddSession(ctx context.Context, hash string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, s.SQL("INSERT INTO sessions(token_hash,expires_at) VALUES(?,?)"), hash, expires.UTC().Format(time.RFC3339))
	return err
}
func (s *Store) Session(ctx context.Context, hash string) bool {
	var expires string
	err := s.db.QueryRowContext(ctx, s.SQL("SELECT expires_at FROM sessions WHERE token_hash=?"), hash).Scan(&expires)
	if err != nil {
		return false
	}
	t, err := time.Parse(time.RFC3339, expires)
	return err == nil && time.Now().Before(t)
}
func (s *Store) RevokeSession(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, s.SQL("DELETE FROM sessions WHERE token_hash=?"), hash)
	return err
}

func (s *Store) Imported(ctx context.Context, source string) (bool, error) {
	var digest string
	err := s.db.QueryRowContext(ctx, s.SQL("SELECT digest FROM imports WHERE source=?"), source).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func (s *Store) CompleteImport(ctx context.Context, source, digest string) error {
	_, err := s.db.ExecContext(ctx, s.SQL("INSERT INTO imports(source,digest,completed_at) VALUES(?,?,?) ON CONFLICT(source) DO NOTHING"), source, digest, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
