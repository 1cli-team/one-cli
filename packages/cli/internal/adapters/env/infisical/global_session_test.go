package infisical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/zalando/go-keyring"
)

func TestGlobalBindingPreservesSavedLocationWhenSessionChanges(t *testing.T) {
	for _, change := range []string{"logout", "account"} {
		t.Run(change, func(t *testing.T) {
			keyring.MockInit()
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			var original session.Session
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/workspace/remote" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("unexpected metadata request: %s", r.URL)
				}
				if change == "logout" {
					if err := session.Logout(); err != nil {
						t.Error(err)
					}
				} else {
					current := original
					current.UserID = "another-user"
					raw, _ := json.Marshal(current)
					if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
						t.Error(err)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"workspace":{"id":"remote","name":"Shared","type":"secret-manager","orgId":"org","environments":[{"slug":"dev"}]}}`)
			}))
			defer upstream.Close()
			original = session.Session{Info: session.Info{SiteURL: upstream.URL, UserID: "user", OrganizationID: "org", ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token"}
			raw, _ := json.Marshal(original)
			if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
				t.Fatal(err)
			}
			path, err := session.ConfigPath("global-env.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			before := []byte(`{"projectId":"previous","defaultEnvironment":"prod"}`)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = BindGlobal(context.Background(), "remote", "dev")
			wantCode := "INFISICAL_AUTH_FAILED"
			if change == "logout" {
				wantCode = "INFISICAL_AUTH_MISSING"
			}
			var coded *output.Error
			if !errors.As(err, &coded) || coded.Code != wantCode {
				t.Fatalf("unexpected session failure: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("changed session overwrote the saved location: %v", err)
			}
		})
	}
}
