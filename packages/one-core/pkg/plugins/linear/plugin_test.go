package linear

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

func testPlugin(t *testing.T, handler http.HandlerFunc) *Plugin {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	p := New()
	p.root = t.TempDir()
	p.apiURL = server.URL
	p.tokenURL = server.URL
	if err := p.transaction(context.Background(), func(doc *document) (bool, error) {
		doc.Connections["connection"] = credential{Connection: tracking.Connection{ID: "connection", OrganizationID: "org"}, AccessToken: "private-access", RefreshToken: "private-refresh", ClientID: "public-client", ExpiresAt: time.Now().Add(time.Hour)}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	return p
}

func decodeRequest(t *testing.T, r *http.Request) (string, map[string]any) {
	t.Helper()
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Error(err)
	}
	return body.Query, body.Variables
}

func reply(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}
func pageReply(nodes any) map[string]any {
	return map[string]any{"nodes": nodes, "pageInfo": tracking.PageInfo{}}
}
func testBinding() tracking.Binding {
	return tracking.Binding{Scope: tracking.Scope{Kind: "group", ID: "local"}, ConnectionID: "connection", OrganizationID: "org", Project: tracking.Project{Named: tracking.Named{ID: "project"}}, TeamID: "team"}
}

func TestProjectCreationUsesSelectedTeam(t *testing.T) {
	team := tracking.Named{ID: "engineering", Name: "Engineering"}
	const projectID = "719fc370-6a28-4eb4-b9be-5b433e19f09b"
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		query, variables := decodeRequest(t, r)
		switch {
		case strings.Contains(query, "query OneLinearTeams"):
			reply(w, map[string]any{"teams": pageReply([]tracking.Named{team})})
		case strings.Contains(query, "mutation OneLinearCreateProject"):
			input, _ := variables["input"].(map[string]any)
			ids, _ := input["teamIds"].([]any)
			if input["id"] != projectID || input["name"] != "One project" || len(ids) != 1 || ids[0] != team.ID {
				t.Errorf("unexpected project input: %v", input)
			}
			reply(w, map[string]any{"projectCreate": map[string]any{"success": true, "project": map[string]any{
				"id": projectID, "name": "One project", "teams": pageReply([]tracking.Named{team}),
			}}})
		default:
			t.Errorf("unexpected query: %s", query)
		}
	})
	teams, err := p.Teams(context.Background(), "connection")
	if err != nil || len(teams) != 1 || teams[0] != team {
		t.Fatalf("teams = %+v, %v", teams, err)
	}
	project, err := p.CreateProject(context.Background(), projectID, tracking.ProjectInput{ConnectionID: "connection", Name: "One project", TeamID: team.ID})
	if err != nil || project.ID != projectID || len(project.Teams) != 1 || project.Teams[0] != team {
		t.Fatalf("project = %+v, %v", project, err)
	}
}
func issueReply(id string) map[string]any {
	return map[string]any{"id": id, "identifier": "ONE-1", "title": "Test", "updatedAt": "2026-11-18T00:00:00Z", "team": tracking.Named{ID: "team"}, "project": tracking.Named{ID: "project"}, "state": tracking.State{Named: tracking.Named{ID: "state"}}, "labels": pageReply([]any{}), "comments": pageReply([]any{})}
}

func TestGraphErrorsAndScopedCache(t *testing.T) {
	var mode atomic.Int32
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		query, variables := decodeRequest(t, r)
		if !strings.Contains(query, "OneLinearIssues") {
			t.Errorf("unexpected query %s", query)
		}
		filter := variables["filter"].(map[string]any)
		if filter["project"].(map[string]any)["id"].(map[string]any)["eq"] != "project" {
			t.Error("missing project scope")
		}
		switch mode.Load() {
		case 1:
			w.WriteHeader(429)
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"issues": pageReply([]any{issueReply("one")})}, "errors": []any{map[string]any{"extensions": map[string]any{"code": "FORBIDDEN"}}}})
		default:
			reply(w, map[string]any{"issues": pageReply([]any{issueReply("one")})})
		}
	})
	ctx := context.Background()
	list, err := p.Issues(ctx, testBinding(), tracking.IssueFilter{})
	if err != nil || len(list.Nodes) != 1 || list.Stale {
		t.Fatalf("initial list: %+v %v", list, err)
	}
	mode.Store(1)
	list, err = p.Issues(ctx, testBinding(), tracking.IssueFilter{})
	if err != nil || !list.Stale {
		t.Fatalf("cache fallback: %+v %v", list, err)
	}
	_, err = p.Issues(ctx, testBinding(), tracking.IssueFilter{Query: "different"})
	assertCode(t, err, "rate_limit")
	mode.Store(2)
	_, err = p.Issues(ctx, testBinding(), tracking.IssueFilter{})
	assertCode(t, err, "forbidden")
}

func TestCreateRecoveryNeverRepeatsMutation(t *testing.T) {
	var creates atomic.Int32
	var createdID string
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		query, variables := decodeRequest(t, r)
		switch {
		case strings.Contains(query, "OneLinearProject("):
			reply(w, map[string]any{"project": map[string]any{"id": "project", "teams": pageReply([]tracking.Named{{ID: "team"}})}})
		case strings.Contains(query, "OneLinearTeam("):
			reply(w, map[string]any{"team": map[string]any{"id": "team"}})
		case strings.Contains(query, "OneLinearTeamItems"):
			field := "labels"
			if strings.Contains(query, "states(") {
				field = "states"
			}
			if strings.Contains(query, "members(") {
				field = "members"
			}
			reply(w, map[string]any{"team": map[string]any{field: pageReply([]any{})}})
		case strings.Contains(query, "OneLinearCreateIssue"):
			creates.Add(1)
			createdID = variables["input"].(map[string]any)["id"].(string)
			// Linear accepted the issue, but the client did not receive its response.
			w.WriteHeader(http.StatusBadGateway)
		case strings.Contains(query, "OneLinearIssue("):
			reply(w, map[string]any{"issue": issueReply(createdID)})
		default:
			t.Errorf("unexpected query %s", query)
		}
	})
	ctx := context.Background()
	input := tracking.IssueInput{Title: "Test", TeamID: "team", OperationID: "11111111-1111-4111-8111-111111111111"}
	_, err := p.CreateIssue(ctx, testBinding(), input)
	assertCode(t, err, "outcome_unknown")
	recovered, err := p.CreateIssue(ctx, testBinding(), input)
	if err != nil || recovered.ID != input.OperationID || creates.Load() != 1 {
		t.Fatalf("recovery duplicated mutation: %+v %v %d", recovered, err, creates.Load())
	}
	operation, err := p.Operation(ctx, testBinding(), input.OperationID)
	if err != nil || operation.Status != "confirmed" {
		t.Fatalf("operation: %+v %v", operation, err)
	}
	input.Title = "different"
	_, err = p.CreateIssue(ctx, testBinding(), input)
	assertCode(t, err, "operation")
	other := testBinding()
	other.Project.ID = "other"
	_, err = p.Operation(ctx, other, input.OperationID)
	assertCode(t, err, "operation")
}

func TestIssueScopeAndUpdateConflict(t *testing.T) {
	var foreign atomic.Bool
	var updates atomic.Int32
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		query, _ := decodeRequest(t, r)
		if strings.Contains(query, "mutation") {
			updates.Add(1)
		}
		issue := issueReply("issue")
		if foreign.Load() {
			issue["project"] = tracking.Named{ID: "foreign"}
		}
		reply(w, map[string]any{"issue": issue})
	})
	_, err := p.UpdateIssue(context.Background(), testBinding(), "issue", tracking.IssueInput{Title: "Updated", TeamID: "team", ExpectedUpdatedAt: "old"})
	assertCode(t, err, "conflict")
	foreign.Store(true)
	_, err = p.Issue(context.Background(), testBinding(), "issue")
	assertCode(t, err, "scope")
	if updates.Load() != 0 {
		t.Fatal("invalid update reached Linear")
	}
}

func TestRefreshAcrossInstancesAndPrivateStatus(t *testing.T) {
	var refreshes atomic.Int32
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("client_id") != "public-client" || r.Form.Get("client_secret") != "" {
			t.Error("invalid PKCE refresh parameters")
		}
		refreshes.Add(1)
		_ = json.NewEncoder(w).Encode(tokenResult{AccessToken: "rotated-access", RefreshToken: "rotated-refresh", ExpiresIn: 3600})
	})
	err := p.transaction(context.Background(), func(doc *document) (bool, error) {
		c := doc.Connections["connection"]
		c.ExpiresAt = time.Now().Add(-time.Minute)
		doc.Connections["connection"] = c
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	other := New()
	other.root = p.root
	other.tokenURL = p.tokenURL
	var workers sync.WaitGroup
	for _, client := range []*Plugin{p, other} {
		workers.Add(1)
		go func(client *Plugin) {
			defer workers.Done()
			token, err := client.token(context.Background(), "connection")
			if err != nil || token != "rotated-access" {
				t.Errorf("token=%q err=%v", token, err)
			}
		}(client)
	}
	workers.Wait()
	if refreshes.Load() != 1 {
		t.Fatalf("refreshes=%d", refreshes.Load())
	}
	state, err := p.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(state)
	if strings.Contains(string(raw), "rotated") || strings.Contains(string(raw), "private-") {
		t.Fatal("credentials leaked through status")
	}
	info, err := os.Stat(filepath.Join(p.root, "connections.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("credential file must be private")
	}
}

func TestOAuthPKCEStateAndCancellation(t *testing.T) {
	var verifier string
	var exchanges atomic.Int32
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			_ = r.ParseForm()
			verifier = r.Form.Get("code_verifier")
			exchanges.Add(1)
			if r.Form.Get("client_secret") != "" || r.Form.Get("code") != "code" {
				t.Error("unexpected token exchange")
			}
			_ = json.NewEncoder(w).Encode(tokenResult{AccessToken: "oauth-access", RefreshToken: "oauth-refresh", ExpiresIn: 3600, Scope: json.RawMessage(`"read write"`)})
			return
		}
		reply(w, map[string]any{"viewer": map[string]any{"id": "user", "name": "User", "organization": tracking.Named{ID: "workspace", Name: "Workspace"}}})
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	redirect := "http://" + address + "/oauth/linear/callback"
	if err := p.Configure(context.Background(), tracking.OAuthConfig{ClientID: "client", RedirectURI: redirect}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	auth, err := p.AuthorizeProjectCreation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(auth.URL)
	values := parsed.Query()
	if values.Get("scope") != "read,write" {
		t.Fatal("project creation did not request write scope")
	}
	_, err = p.Authorize(ctx)
	assertCode(t, err, "busy")
	response, err := http.Get(redirect + "?code=code&state=wrong")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 400 || exchanges.Load() != 0 {
		t.Fatal("invalid state accepted")
	}
	response, err = http.Get(redirect + "?code=code&state=" + url.QueryEscape(values.Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	challenge := sha256.Sum256([]byte(verifier))
	if values.Get("code_challenge_method") != "S256" || values.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(challenge[:]) {
		t.Fatal("invalid PKCE challenge")
	}
	state, err := p.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Authorization.Status != "connected" || len(state.Connections) != 2 {
		t.Fatalf("authorization=%+v", state)
	}
	for _, account := range state.Connections {
		if account.CanCreateProjects != (account.ID == "workspace:user") {
			t.Fatalf("incorrect write authorization for %s", account.ID)
		}
	}
	p.CancelAuthorization()
	if err := validateConfig(tracking.OAuthConfig{ClientID: "client", RedirectURI: "https://example.com/callback"}); err == nil {
		t.Fatal("remote callback accepted")
	}
}

func TestInvalidStorageIsPreserved(t *testing.T) {
	p := New()
	p.root = t.TempDir()
	path := filepath.Join(p.root, "connections.json")
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := p.Status(context.Background())
	assertCode(t, err, "storage")
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "corrupt" {
		t.Fatal("invalid store was replaced")
	}
}

func TestCommentRecoveryAndPendingOperations(t *testing.T) {
	var writes atomic.Int32
	id := "22222222-2222-4222-8222-222222222222"
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		query, _ := decodeRequest(t, r)
		switch {
		case strings.Contains(query, "OneLinearIssue("):
			reply(w, map[string]any{"issue": issueReply("issue")})
		case strings.Contains(query, "OneLinearCreateComment"):
			writes.Add(1)
			w.WriteHeader(502)
		case strings.Contains(query, "OneLinearComment("):
			reply(w, map[string]any{"comment": map[string]any{"id": id, "body": "Comment", "issue": map[string]any{"id": "issue", "project": tracking.Named{ID: "project"}}}})
		default:
			t.Errorf("unexpected query %s", query)
		}
	})
	ctx := context.Background()
	_, err := p.Comment(ctx, testBinding(), "issue", "Comment", id)
	assertCode(t, err, "outcome_unknown")
	pending, err := p.PendingOperations(ctx, testBinding())
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%+v %v", pending, err)
	}
	comment, err := p.Comment(ctx, testBinding(), "issue", "Comment", id)
	if err != nil || comment.ID != id || writes.Load() != 1 {
		t.Fatalf("comment recovery=%+v %v writes=%d", comment, err, writes.Load())
	}
	pending, err = p.PendingOperations(ctx, testBinding())
	if err != nil || len(pending) != 0 {
		t.Fatalf("confirmed comment remained pending: %+v %v", pending, err)
	}
}

func TestProjectPagination(t *testing.T) {
	var requests atomic.Int32
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		_, variables := decodeRequest(t, r)
		requests.Add(1)
		teams := map[string]any{"nodes": []tracking.Named{{ID: "first"}}, "pageInfo": tracking.PageInfo{EndCursor: "cursor", HasNextPage: true}}
		if variables["after"] == "cursor" {
			teams = pageReply([]tracking.Named{{ID: "second"}})
		}
		reply(w, map[string]any{"project": map[string]any{"id": "project", "teams": teams}})
	})
	project, err := p.Project(context.Background(), "connection", "project")
	if err != nil || len(project.Teams) != 2 || requests.Load() != 2 {
		t.Fatalf("paginated project=%+v %v", project, err)
	}
}

func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var problem *tracking.Error
	if !errors.As(err, &problem) || problem.Code != code {
		t.Fatalf("error=%v, want %s", err, code)
	}
}

func TestTransitionIssueWritesOnlyStateAndReadsItBack(t *testing.T) {
	var reads, writes atomic.Int32
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		query, variables := decodeRequest(t, r)
		if strings.Contains(query, "mutation OneLinearTransitionIssue") {
			input := variables["input"].(map[string]any)
			if len(input) != 1 || input["stateId"] != "review" || variables["id"] != "issue" {
				t.Errorf("unexpected transition: %v", variables)
			}
			writes.Add(1)
			reply(w, map[string]any{"issueUpdate": map[string]any{"success": true}})
			return
		}
		reads.Add(1)
		state := "state"
		if writes.Load() == 1 {
			state = "review"
		}
		issue := issueReply("issue")
		issue["state"] = tracking.State{Named: tracking.Named{ID: state}}
		reply(w, map[string]any{"issue": issue})
	})
	if err := p.TransitionIssue(t.Context(), testBinding(), "issue", "state", "review"); err != nil {
		t.Fatal(err)
	}
	if reads.Load() != 2 || writes.Load() != 1 {
		t.Fatalf("transition did not verify its outcome: reads=%d writes=%d", reads.Load(), writes.Load())
	}
	if err := p.TransitionIssue(t.Context(), testBinding(), "issue", "state", "review"); err != nil || writes.Load() != 1 {
		t.Fatalf("retry did not recognize the state: %v, writes=%d", err, writes.Load())
	}
}

func TestRelayedAuthorizationExchangesValidStateOnceOnTarget(t *testing.T) {
	var exchanges atomic.Int32
	var gotChallenge atomic.Value
	p := testPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer remote-access" {
			reply(w, map[string]any{"viewer": map[string]any{"id": "user", "name": "Remote User", "organization": map[string]string{"id": "remote-org"}}})
			return
		}
		exchanges.Add(1)
		if err := r.ParseForm(); err != nil || r.Form.Get("code") != "test-code" || r.Form.Get("client_id") != "remote-client" {
			t.Error("unexpected OAuth token request")
		}
		challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		gotChallenge.Store(base64.RawURLEncoding.EncodeToString(challenge[:]))
		_, _ = io.WriteString(w, `{"access_token":"remote-access","refresh_token":"remote-refresh","expires_in":3600}`)
	})
	ctx := t.Context()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := tracking.OAuthConfig{ClientID: "remote-client", RedirectURI: "http://" + listener.Addr().String() + "/oauth/linear/callback"}
	if err := p.Configure(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	authorization, err := p.BeginRelayedAuthorization(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, p.CompleteRelayedAuthorization(ctx, "bad-state", "test-code"), "oauth_state")
	if exchanges.Load() != 0 {
		t.Fatal("invalid state reached the token endpoint")
	}
	if err := p.CompleteRelayedAuthorization(ctx, authorization.ID, "test-code"); err != nil {
		t.Fatal(err)
	}
	if err := p.CompleteRelayedAuthorization(ctx, authorization.ID, "test-code"); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authorization.URL)
	if exchanges.Load() != 1 || gotChallenge.Load() != u.Query().Get("code_challenge") {
		t.Fatal("authorization was replayed or PKCE challenge did not match")
	}
	state, err := p.Status(ctx)
	if err != nil || state.Authorization.Status != "connected" || len(state.Connections) != 2 || state.Connections[1].ID != "remote-org:user" {
		t.Fatalf("target authorization was not saved: %+v, %v", state, err)
	}
}
