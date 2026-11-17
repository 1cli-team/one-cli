package execution

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func matchesToken(header, token string) bool {
	actual := sha256.Sum256([]byte(header))
	want := sha256.Sum256([]byte("Bearer " + token))
	return subtle.ConstantTimeCompare(actual[:], want[:]) == 1
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func readBody(w http.ResponseWriter, r *http.Request, limit int64, into any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	return decoder.Decode(into) == nil && decoder.Decode(new(any)) == io.EOF
}

func problem(w http.ResponseWriter, status int, reason string) {
	respond(w, status, map[string]string{"reason": reason})
}

func storeProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNotFound), errors.Is(err, errWorker):
		problem(w, http.StatusNotFound, err.Error())
	case errors.Is(err, errConflict), errors.Is(err, errRepository), errors.Is(err, errKind), errors.Is(err, errModel),
		errors.Is(err, errQuestionConflict), errors.Is(err, errContinueConflict):
		problem(w, http.StatusConflict, err.Error())
	default:
		problem(w, http.StatusInternalServerError, "execution_storage")
	}
}

func Control(token string, s *Store, fallback http.Handler, workerURL string) http.Handler {
	mux := http.NewServeMux()
	handle := func(pattern string, fn http.HandlerFunc) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if !matchesToken(r.Header.Get("Authorization"), token) {
				problem(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			fn(w, r)
		})
	}
	handle("GET /v1/workers", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, http.StatusOK, s.workersList())
	})
	if workerURL != "" {
		handle("POST /v1/workers/invite", func(w http.ResponseWriter, _ *http.Request) {
			invite := s.invite()
			invite.URL = workerURL
			respond(w, http.StatusOK, invite)
		})
	}
	handle("GET /v1/executions", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, http.StatusOK, s.list())
	})
	handle("POST /v1/executions", func(w http.ResponseWriter, r *http.Request) {
		var input api.ExecutionSubmission
		if !readBody(w, r, 64<<10, &input) || !input.Valid() || input.Input.Kind == api.LinearWorkflow || strings.HasPrefix(input.RequestID, "linear/") {
			problem(w, http.StatusBadRequest, "invalid_execution")
			return
		}
		job, err := s.submit(input)
		if err != nil {
			storeProblem(w, err)
			return
		}
		respond(w, http.StatusAccepted, job)
	})
	handle("GET /v1/executions/{id}", func(w http.ResponseWriter, r *http.Request) {
		job, err := s.job(r.PathValue("id"))
		if err != nil {
			storeProblem(w, err)
			return
		}
		respond(w, http.StatusOK, job)
	})
	handle("POST /v1/executions/{id}/answer", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var input api.ExecutionAnswer
		if !api.ValidExecutionID(id) || !readBody(w, r, 64<<10, &input) || !input.Valid() {
			problem(w, http.StatusBadRequest, "invalid_execution_answer")
			return
		}
		input.Answer = strings.TrimSpace(input.Answer)
		question, err := s.answer(id, input)
		if err != nil {
			storeProblem(w, err)
			return
		}
		respond(w, http.StatusOK, question)
	})
	handle("POST /v1/executions/{id}/continue", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var input api.ContinueExecutionInput
		if !api.ValidExecutionID(id) || !readBody(w, r, 1024, &input) || !api.ValidProgressID(input.ProgressID) {
			problem(w, http.StatusBadRequest, "invalid_execution_continue")
			return
		}
		progress, err := s.requestContinue(id, input)
		if err != nil {
			storeProblem(w, err)
			return
		}
		respond(w, http.StatusOK, progress)
	})
	handle("GET /v1/executions/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		offset := 0
		if raw := r.URL.Query().Get("offset"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				problem(w, http.StatusBadRequest, "invalid_log_offset")
				return
			}
			offset = value
		}
		logs, err := s.logs(r.PathValue("id"), offset)
		if err != nil {
			storeProblem(w, err)
			return
		}
		respond(w, http.StatusOK, logs)
	})
	mux.Handle("/", fallback)
	return mux
}
