package environment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/zalando/go-keyring"
)

func TestWorkspaceBindingRejectsSessionChangesWithoutPublishing(t *testing.T) {
	for _, stage := range []string{"create", "metadata", "verification"} {
		for _, change := range []string{"unchanged", "logout", "account", "organization", "site", "token", "expiry"} {
			t.Run(stage+"/"+change, func(t *testing.T) {
				keyring.MockInit()
				t.Setenv("XDG_CONFIG_HOME", t.TempDir())
				scope := unboundScope(t)
				root := scope.WorkingDirectory()
				manifest, err := workspace.ReadManifest(root)
				if err != nil {
					t.Fatal(err)
				}
				if stage == "create" {
					// An interrupted creation must not back-fill an older manifest's
					// identity before rejecting the changed session.
					manifest.Workspace = nil
				} else {
					manifest.Env = &workspace.EnvironmentConfig{ProjectID: "previous", Environments: []string{"dev"}}
				}
				if err := workspace.WriteManifest(root, manifest); err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(workspace.ManifestPath(root))
				if err != nil {
					t.Fatal(err)
				}
				_, revision, err := workspace.ReadManifestSnapshot(root)
				if err != nil {
					t.Fatal(err)
				}
				var foreignRequests, creates, verifies atomic.Int32
				foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					foreignRequests.Add(1)
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"secrets":[]}`)
				}))
				defer foreign.Close()
				var original session.Session
				store := func(current session.Session) error {
					raw, err := json.Marshal(current)
					if err != nil {
						return err
					}
					return keyring.Set("one-cli.infisical", "session", string(raw))
				}
				var changed atomic.Bool
				changeSession := func() {
					if !changed.CompareAndSwap(false, true) || change == "unchanged" {
						return
					}
					if change == "logout" {
						if err := session.Logout(); err != nil {
							t.Error(err)
						}
						return
					}
					current := original
					switch change {
					case "account":
						current.UserID = "another-user"
					case "organization":
						current.OrganizationID = "another-org"
					case "site":
						current.SiteURL = foreign.URL
					case "token":
						current.Token = "replacement-test-token"
					case "expiry":
						current.ExpiresAt = time.Now().Add(-time.Minute)
					}
					if err := store(current); err != nil {
						t.Error(err)
					}
				}
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Header.Get("Authorization") != "Bearer original-test-token" {
						t.Error("binding used a different session for remote work")
					}
					switch r.URL.Path {
					case "/api/v2/workspace":
						creates.Add(1)
						changeSession()
						fmt.Fprint(w, `{"project":{"id":"remote","name":"demo"}}`)
					case "/api/v1/workspace/remote":
						if stage == "metadata" {
							changeSession()
						}
						fmt.Fprint(w, `{"workspace":{"id":"remote","name":"demo","type":"secret-manager","orgId":"org","environments":[{"slug":"dev"}]}}`)
					case "/api/v3/secrets/raw":
						verifies.Add(1)
						if stage == "verification" {
							changeSession()
						}
						fmt.Fprint(w, `{"secrets":[]}`)
					default:
						t.Errorf("unexpected binding request: %s", r.URL.Path)
						http.NotFound(w, r)
					}
				}))
				defer upstream.Close()
				original = session.Session{
					Info:  session.Info{SiteURL: upstream.URL, UserID: "user", OrganizationID: "org", ExpiresAt: time.Now().Add(time.Hour)},
					Token: "original-test-token",
				}
				if err := store(original); err != nil {
					t.Fatal(err)
				}
				input := BindWorkspaceInput{Scope: scope, Create: stage == "create", Revision: revision}
				if stage != "create" {
					input.ProjectID = "remote"
				}
				service := newTestService(t)
				result, err := service.BindWorkspace(context.Background(), input)
				if foreignRequests.Load() != 0 || (stage != "create" && verifies.Load() != 1) {
					t.Fatal("verification did not stay on the original session and site")
				}
				if change == "unchanged" {
					if err != nil || result.ProjectID != "remote" {
						t.Fatalf("unchanged session failed to bind: %+v %v", result, err)
					}
					return
				}
				wantCode := "INFISICAL_AUTH_FAILED"
				if change == "logout" {
					wantCode = "INFISICAL_AUTH_MISSING"
				}
				var coded *output.Error
				if !errors.As(err, &coded) || coded.Code != wantCode {
					t.Fatalf("session change lost its authentication error: %v", err)
				}
				if strings.Contains(err.Error(), original.Token) || strings.Contains(err.Error(), "replacement-test-token") {
					t.Fatal("session failure exposed a token")
				}
				after, readErr := os.ReadFile(workspace.ManifestPath(root))
				if readErr != nil || string(before) != string(after) {
					t.Fatalf("session change published local changes: %v", readErr)
				}
				if stage == "create" {
					if coded.Context["project_id"] != "remote" || coded.Context["partial_state"] != "project_created_binding_unsaved" {
						t.Fatalf("lost the created project for recovery: %+v", coded.Context)
					}
					if err := store(original); err != nil {
						t.Fatal(err)
					}
					result, err = service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, ProjectID: "remote", Revision: revision})
					if err != nil || result.ProjectID != "remote" || creates.Load() != 1 {
						t.Fatalf("recovery failed or created a duplicate project: %+v %v", result, err)
					}
				}
			})
		}
	}
}
