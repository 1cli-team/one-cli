package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
)

// Transfer deliberately excludes credentials, sessions, server identity and
// machine-local paths. A destination server is authorized independently.
type Transfer struct {
	Schema        string                `json:"schema"`
	Workspaces    []*workspace.Manifest `json:"workspaces"`
	Preferences   Preferences           `json:"preferences"`
	GlobalBinding json.RawMessage       `json:"globalBinding,omitempty"`
}

func (s *Store) Export(ctx context.Context) (Transfer, error) {
	out := Transfer{Schema: "one-server/export/v1", Workspaces: []*workspace.Manifest{}}
	options := &sql.TxOptions{ReadOnly: true}
	if s.postgres {
		options.Isolation = sql.LevelRepeatableRead
	}
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id FROM workspaces ORDER BY name,id")
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, id := range ids {
		m, _, e := s.load(ctx, tx, id)
		if e != nil {
			return out, e
		}
		out.Workspaces = append(out.Workspaces, m)
	}
	out.Preferences.Locale = "auto"
	err = tx.QueryRowContext(ctx, "SELECT locale,task_ui FROM preferences WHERE scope='owner'").Scan(&out.Preferences.Locale, &out.Preferences.TaskUI)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	var binding string
	err = tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='global-env'").Scan(&binding)
	if err == nil {
		clean, e := transferBinding([]byte(binding))
		if e != nil {
			return out, e
		}
		out.GlobalBinding = clean
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	return out, tx.Commit()
}

// Restore is an atomic import into an empty instance. Existing workspaces are
// never replaced; any validation or identity conflict aborts the whole import.
func (s *Store) Restore(ctx context.Context, in Transfer) error {
	if in.Schema != "one-server/export/v1" {
		return errors.New("unsupported management export schema")
	}
	if in.Preferences.Locale != "auto" && in.Preferences.Locale != "zh-CN" && in.Preferences.Locale != "en-US" {
		return errors.New("invalid imported locale")
	}
	if in.Preferences.TaskUI != "" && in.Preferences.TaskUI != "stream" && in.Preferences.TaskUI != "tui" {
		return errors.New("invalid imported task UI")
	}
	seen := map[string]bool{}
	for _, m := range in.Workspaces {
		if m == nil || m.Workspace == nil {
			return errors.New("missing imported workspace identity")
		}
		if err := workspace.ValidateManifestContext(ctx, m); err != nil {
			return err
		}
		if seen[m.Workspace.ID] {
			return ErrConflict
		}
		seen[m.Workspace.ID] = true
	}
	if len(in.GlobalBinding) > 0 {
		clean, err := transferBinding(in.GlobalBinding)
		if err != nil {
			return err
		}
		in.GlobalBinding = clean
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.postgres {
		if _, err = tx.ExecContext(ctx, "LOCK TABLE workspaces IN EXCLUSIVE MODE"); err != nil {
			return err
		}
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM workspaces").Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, m := range in.Workspaces {
		env, err := json.Marshal(m.Env)
		if err != nil {
			return err
		}
		groups, err := json.Marshal(m.Groups)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, s.SQL("INSERT INTO workspaces(id,name,revision,source_revision,environment,groups_json,registered_at,last_seen_at) VALUES(?,?,1,?,?,?,?,?)"), m.Workspace.ID, m.Workspace.Name, "import", string(env), string(groups), now, now)
		if err != nil {
			return err
		}
		if err = s.writeProjects(ctx, tx, m); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, s.SQL("INSERT INTO preferences(scope,locale,task_ui,revision) VALUES('owner',?,?,1) ON CONFLICT(scope) DO UPDATE SET locale=excluded.locale,task_ui=excluded.task_ui,revision=preferences.revision+1"), in.Preferences.Locale, in.Preferences.TaskUI); err != nil {
		return err
	}
	if len(in.GlobalBinding) > 0 {
		if _, err = tx.ExecContext(ctx, s.SQL("INSERT INTO settings(key,value) VALUES('global-env',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value"), string(in.GlobalBinding)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type transferLocation struct {
	SiteURL            string `json:"siteUrl"`
	UserID             string `json:"userId"`
	OrganizationID     string `json:"organizationId"`
	ProjectID          string `json:"projectId"`
	ProjectName        string `json:"projectName"`
	DefaultEnvironment string `json:"defaultEnvironment"`
}
type transferGlobal struct {
	transferLocation
	Bindings []transferLocation `json:"bindings,omitempty"`
}

func transferBinding(raw []byte) (json.RawMessage, error) {
	if !json.Valid(raw) {
		return nil, errors.New("invalid imported global binding")
	}
	var binding transferGlobal
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding); err != nil {
		return nil, err
	}
	for _, location := range append(binding.Bindings, binding.transferLocation) {
		u, err := url.Parse(location.SiteURL)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || location.ProjectID == "" {
			return nil, errors.New("invalid imported global binding")
		}
		local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
		if u.Scheme != "https" && !(local && u.Scheme == "http") {
			return nil, errors.New("invalid imported global binding URL")
		}
	}
	return json.Marshal(binding)
}
