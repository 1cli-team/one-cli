package infisical

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/zalando/go-keyring"
)

func TestGlobalLocationAndListingStayScoped(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var secretsCalled int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "bad token", 401)
			return
		}
		switch {
		case r.URL.Path == "/api/v1/workspace/project-1":
			w.Write([]byte(`{"workspace":{"id":"project-1","name":"Shared","orgId":"org-1","environments":[{"slug":"dev","name":"Development"},{"slug":"prod","name":"Production"}]}}`))
		case strings.Contains(r.URL.Path, "folders"):
			if r.URL.Query().Get("path") != "/docker" {
				t.Errorf("folder request: %s", r.URL)
			}
			w.Write([]byte(`{"folders":[{"id":"child","name":"nested"}]}`))
		case strings.Contains(r.URL.Path, "secrets"):
			secretsCalled++
			if r.URL.Query().Get("recursive") == "true" {
				t.Error("recursive read")
			}
			w.Write([]byte(`{"secrets":[{"secretKey":"PASSWORD","secretValue":"never-list-this","secretComment":"Registry password"}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	store := func(user string) {
		raw, _ := json.Marshal(session.Session{Info: session.Info{SiteURL: upstream.URL, UserID: user, OrganizationID: "org-1", ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token"})
		if e := keyring.Set("one-cli.infisical", "session", string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	store("user-1")
	ctx := context.Background()
	if _, e := BindGlobal(ctx, "project-1", "missing"); e == nil {
		t.Fatal("bound missing env")
	}
	if _, e := BindGlobal(ctx, "project-1", "dev"); e != nil {
		t.Fatal(e)
	}
	listing, e := ListGlobal(ctx, "prod", "/docker")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(listing)
	if strings.Contains(string(raw), "never-list-this") {
		t.Fatal("list leaked value")
	}
	if len(listing.Folders) != 1 || listing.Folders[0] != "/docker/nested" {
		t.Fatal(listing.Folders)
	}
	if _, e := GlobalValues(ctx, "missing", "/docker", nil); e == nil {
		t.Fatal("invalid environment fell back")
	}
	if secretsCalled != 1 {
		t.Fatal("fetched secrets for invalid environment")
	}
	store("user-2")
	if _, e := ListGlobal(ctx, "prod", "/docker"); e == nil {
		t.Fatal("account mismatch accepted")
	}
}
func TestGlobalPathRejectsTraversal(t *testing.T) {
	for _, p := range []string{"relative", "/a/../b", "/a/./b", "/a\\b", "/a\nb"} {
		if _, e := ValidateGlobalPath(p); e == nil {
			t.Errorf("accepted %q", p)
		}
	}
}

func TestGlobalSelectedKeysDoNotReadOtherValuesOrExpandReferences(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	reads := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/workspace/shared":
			w.Write([]byte(`{"workspace":{"id":"shared","orgId":"org","environments":[{"slug":"prod"}]}}`))
		case "/api/v3/secrets/raw/AK":
			reads++
			q := r.URL.Query()
			if q.Get("environment") != "prod" || q.Get("secretPath") != "/oss" || q.Get("expandSecretReferences") == "true" || q.Get("include_imports") == "true" {
				t.Errorf("scope widened: %s", r.URL)
			}
			w.Write([]byte(`{"secret":{"secretKey":"AK","secretValue":"selected-value"}}`))
		default:
			t.Errorf("unexpected value read: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	raw, _ := json.Marshal(session.Session{Info: session.Info{SiteURL: upstream.URL, UserID: "user", OrganizationID: "org", ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err := BindGlobal(context.Background(), "shared", "prod"); err != nil {
		t.Fatal(err)
	}
	values, err := GlobalValues(context.Background(), "prod", "/oss", []string{"AK"})
	if err != nil {
		t.Fatal(err)
	}
	if reads != 1 || len(values) != 1 || values["AK"] != "selected-value" {
		t.Fatalf("values=%v reads=%d", values, reads)
	}
}
