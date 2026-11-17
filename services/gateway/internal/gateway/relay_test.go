package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func TestGatewayOnlyForwardsWorkflowOperations(t *testing.T) {
	const token = "test-local-transport"
	calls := []api.WorkRequest{}
	handler := Handler(token, func(_ context.Context, request api.WorkRequest) (api.WorkReply, error) {
		calls = append(calls, request)
		return api.WorkReply{Status: http.StatusAccepted, Body: json.RawMessage(`{"status":"processing"}`)}, nil
	})
	for _, tc := range []struct {
		method, path, secret, body string
		status                     int
	}{
		{http.MethodPost, "/v1/projects/project/jobs/issue", "bad", "", http.StatusUnauthorized},
		{http.MethodGet, "/v1/code-hosts", token, "", http.StatusNotFound},
		{http.MethodPut, "/v1/projects/project/configuration", token, `{"unexpected":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v1/projects/project/jobs/issue", token, "", http.StatusAccepted},
		{http.MethodPost, "/v1/projects/project/jobs", token, `{"issue":"ONE-1","model":"fixture-model","effort":"xhigh"}`, http.StatusAccepted},
		{http.MethodPost, "/v1/projects/project/jobs", token, `{"issue":"ONE-1","effort":"two words"}`, http.StatusBadRequest},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+tc.secret)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.status || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s %s got %d", tc.method, tc.path, rec.Code)
		}
	}
	if len(calls) != 2 || calls[0].Operation != api.WorkDispatch || calls[0].ProjectID != "project" || calls[0].IssueID != "issue" {
		t.Fatalf("unexpected forwarded operations: %+v", calls)
	}
	if options := calls[1]; options.Operation != api.WorkDispatchWithOptions || options.ProjectID != "project" ||
		options.IssueID != "ONE-1" || options.Codex != (api.CodexOptions{Model: "fixture-model", Effort: "xhigh"}) {
		t.Fatalf("configured dispatch: %+v", options)
	}
}

func TestGatewayRelaysCodexStdio(t *testing.T) {
	cwd, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(cwd, "fake-codex")
	script := "#!/bin/sh\n[ \"$1 $2 $3 $4\" = 'app-server --listen stdio:// --strict-config' ] || exit 3\n[ \"$PWD\" = \"$5\" ] || exit 4\n[ -z \"${ONE_GATEWAY_TOKEN-}\" ] || exit 5\nwhile IFS= read -r line; do printf '%s\\n' \"$line\"; done\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONE_GATEWAY_TOKEN", "fixture-not-forwarded")
	socket, closeSocket, err := OpenCodexSocket(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer closeSocket()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stream, err := api.DialCodex(ctx, socket, api.CodexStart{Directory: cwd, Config: []string{cwd}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	message := `{"id":1,"method":"initialize"}` + "\n"
	if _, err := stream.Input.Write([]byte(message)); err != nil {
		t.Fatal(err)
	}
	if got, err := bufio.NewReader(stream.Output).ReadString('\n'); err != nil || got != message {
		t.Fatalf("relayed data = %q, %v", got, err)
	}
}
