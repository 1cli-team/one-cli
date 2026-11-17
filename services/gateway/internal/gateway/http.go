package gateway

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func Handler(token string, forward func(context.Context, api.WorkRequest) (api.WorkReply, error)) http.Handler {
	digest := sha256.Sum256([]byte("Bearer " + token))
	mux := http.NewServeMux()
	register := func(pattern string, op api.WorkOperation) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			input := api.WorkRequest{
				Operation: op, ProjectID: r.PathValue("project"),
				IssueID: r.PathValue("issue"), RepositoryID: r.PathValue("repository"),
			}
			if op == api.WorkMonitorJobs {
				input.IssueID = r.URL.Query().Get("issue")
				if page := r.URL.Query().Get("page"); page != "" {
					var err error
					input.Page, err = strconv.Atoi(page)
					if err != nil || input.Page < 1 {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
				}
			}
			if op == api.WorkSaveMapping {
				decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
				decoder.DisallowUnknownFields()
				if decoder.Decode(&input.Mapping) != nil || decoder.Decode(new(any)) != io.EOF {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"reason":"invalid_mapping"}`))
					return
				}
			}
			if op == api.WorkDispatchWithOptions {
				var dispatch api.DispatchInput
				decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
				decoder.DisallowUnknownFields()
				if decoder.Decode(&dispatch) != nil || decoder.Decode(new(any)) != io.EOF ||
					!api.ValidIssueReference(dispatch.Issue) || !dispatch.CodexOptions.Valid() {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"reason":"invalid_codex_options"}`))
					return
				}
				input.IssueID, input.Codex = dispatch.Issue, dispatch.CodexOptions
			}
			reply, err := forward(r.Context(), input)
			if err != nil || reply.Status < 200 || reply.Status > 599 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(reply.Status)
			_, _ = w.Write(reply.Body)
		})
	}
	register("GET /v1/status", api.WorkStatus)
	register("GET /v1/codex/models", api.WorkCodexModels)
	register("GET /v1/jobs", api.WorkMonitorJobs)
	register("GET /v1/repositories", api.WorkRepositories)
	register("GET /v1/repositories/{repository}", api.WorkRepository)
	register("GET /v1/projects/{project}/configuration", api.WorkMapping)
	register("PUT /v1/projects/{project}/configuration", api.WorkSaveMapping)
	register("GET /v1/projects/{project}/jobs", api.WorkList)
	register("POST /v1/projects/{project}/jobs", api.WorkDispatchWithOptions)
	register("POST /v1/projects/{project}/jobs/{issue}", api.WorkDispatch)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		received := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(digest[:], received[:]) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
