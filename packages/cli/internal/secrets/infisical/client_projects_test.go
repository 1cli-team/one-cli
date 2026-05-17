package infisical

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// newTestClient builds a Client wired against an httptest server, bypassing
// the Infisical SDK entirely. accessToken is read from the field directly
// so we don't need to construct (or mock) a real SDK instance.
func newTestClient(t *testing.T, baseURL, token string) *Client {
	t.Helper()
	cfg := &WorkspaceConfig{ProjectID: "ignored", SiteURL: baseURL}
	return &Client{
		cfg:         cfg,
		accessToken: token,
	}
}

func TestCreateProject_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/workspace" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("missing/incorrect Authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"project":{"id":"p_abc","name":"demo"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "tok")
	id, name, err := c.CreateProject("demo")
	if err != nil {
		t.Fatalf("CreateProject = %v", err)
	}
	if id != "p_abc" || name != "demo" {
		t.Errorf("got id=%q name=%q; want p_abc/demo", id, name)
	}
}

func TestCreateProject_NameTaken_409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"project name already exists"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "tok")
	_, _, err := c.CreateProject("demo")
	if err == nil {
		t.Fatalf("expected error; got nil")
	}
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.INFISICAL_PROJECT_NAME_TAKEN) {
		t.Errorf("expected INFISICAL_PROJECT_NAME_TAKEN; got %v", err)
	}
}

func TestCreateProject_NameTaken_400Body(t *testing.T) {
	// Some Infisical versions surface name collisions as 400 with a body
	// containing "already exists". We branch on body text in that case.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"a project with this name already exists"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "tok")
	_, _, err := c.CreateProject("demo")
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.INFISICAL_PROJECT_NAME_TAKEN) {
		t.Errorf("expected INFISICAL_PROJECT_NAME_TAKEN from 400+body; got %v", err)
	}
}

func TestCreateProject_Forbidden_403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"insufficient permissions"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "tok")
	_, _, err := c.CreateProject("demo")
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.INFISICAL_PROJECT_CREATE_FORBIDDEN) {
		t.Errorf("expected INFISICAL_PROJECT_CREATE_FORBIDDEN; got %v", err)
	}
}

func TestCreateProject_Unauthorized_401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "tok")
	_, _, err := c.CreateProject("demo")
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.INFISICAL_AUTH_FAILED) {
		t.Errorf("expected INFISICAL_AUTH_FAILED; got %v", err)
	}
}

func TestCreateProject_OtherError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream blew up"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, "tok")
	_, _, err := c.CreateProject("demo")
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.INFISICAL_API_ERROR) {
		t.Errorf("expected INFISICAL_API_ERROR for 5xx; got %v", err)
	}
}

func TestCreateProject_MissingToken(t *testing.T) {
	c := newTestClient(t, "https://example.invalid", "")
	_, _, err := c.CreateProject("demo")
	var typed *output.Error
	if !errors.As(err, &typed) || typed.Code != string(cliErrors.INFISICAL_AUTH_FAILED) {
		t.Errorf("expected INFISICAL_AUTH_FAILED on empty token; got %v", err)
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("expected message to mention token; got %v", err)
	}
}

func TestCreateProject_TrimsTrailingSlashFromSiteURL(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/api/v2/workspace" {
			t.Errorf("path = %q; want /api/v2/workspace (no double-slash)", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"project":{"id":"p_id","name":"demo"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL+"/", "tok")
	if _, _, err := c.CreateProject("demo"); err != nil {
		t.Fatalf("CreateProject = %v", err)
	}
	if hits != 1 {
		t.Errorf("expected exactly 1 server hit; got %d", hits)
	}
}
