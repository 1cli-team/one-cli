package execution

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

const inviteLifetime = 10 * time.Minute

var workerIDPattern = regexp.MustCompile(`^w-[a-z2-7]{26}$`)
var errCredentials = errors.New("worker_credentials_invalid")

type joinRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

type heartbeatRequest struct {
	Session   string              `json:"session"`
	Inventory api.ExecutionWorker `json:"inventory"`
}

type claimRequest struct {
	Session string `json:"session"`
}

func (s *Store) invite() api.WorkerInvite {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for code, expiry := range s.invites {
		if !now.Before(expiry) {
			delete(s.invites, code)
		}
	}
	invite := api.WorkerInvite{Code: rand.Text(), ExpiresAt: now.Add(inviteLifetime)}
	s.invites[digest(invite.Code)] = invite.ExpiresAt
	return invite
}

func validWorkerName(name string) bool {
	return name != "" && len(name) <= 120 && name == strings.TrimSpace(name) && !strings.ContainsFunc(name, unicode.IsControl)
}

func (s *Store) join(input joinRequest, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := digest(token)
	if worker, exists := s.workers[input.ID]; exists {
		if worker.Name != input.Name || subtle.ConstantTimeCompare([]byte(worker.TokenHash), []byte(hash)) != 1 {
			return errCredentials
		}
		return nil
	}
	code := digest(input.Code)
	expires, ok := s.invites[code]
	if !ok || !time.Now().Before(expires) {
		return errCredentials
	}
	worker := registeredWorker{
		ExecutionWorker: api.ExecutionWorker{ID: input.ID, Name: input.Name},
		TokenHash:       hash,
	}
	if err := s.saveWorker(worker); err != nil {
		return err
	}
	s.workers[input.ID] = worker
	delete(s.invites, code)
	return nil
}

func (s *Store) authorizedWorker(id, token string) bool {
	if !workerIDPattern.MatchString(id) || !api.ValidToken(token) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.workers[id]
	return subtle.ConstantTimeCompare([]byte(w.TokenHash), []byte(digest(token))) == 1
}

func WorkerAPI(s *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /worker/v1/join", func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		var input joinRequest
		if !ok || !api.ValidToken(token) || !readBody(w, r, 2<<10, &input) ||
			!workerIDPattern.MatchString(input.ID) || !validWorkerName(input.Name) {
			problem(w, http.StatusBadRequest, "invalid_worker")
			return
		}
		if err := s.join(input, token); err != nil {
			if errors.Is(err, errCredentials) {
				problem(w, http.StatusUnauthorized, err.Error())
			} else {
				storeProblem(w, err)
			}
			return
		}
		respond(w, http.StatusOK, struct{}{})
	})

	handle := func(operation string, fn func(http.ResponseWriter, *http.Request, string)) {
		mux.HandleFunc("POST /worker/v1/workers/{id}/"+operation, func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || !s.authorizedWorker(id, token) {
				problem(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			fn(w, r, id)
		})
	}
	handle("heartbeat", func(w http.ResponseWriter, r *http.Request, id string) {
		var input heartbeatRequest
		if !readBody(w, r, 256<<10, &input) || !validSession(input.Session) {
			problem(w, http.StatusBadRequest, "invalid_worker")
			return
		}
		for _, repo := range input.Inventory.Repositories {
			if !repo.Valid() {
				problem(w, http.StatusBadRequest, "invalid_worker")
				return
			}
		}
		workerResponse(w, s.heartbeat(id, input.Session, input.Inventory))
	})
	handle("claim", func(w http.ResponseWriter, r *http.Request, id string) {
		var input claimRequest
		if !readBody(w, r, 1024, &input) || !validSession(input.Session) {
			problem(w, http.StatusBadRequest, "invalid_worker")
			return
		}
		job, err := s.claim(id, input.Session)
		if err != nil {
			storeProblem(w, err)
			return
		}
		respond(w, http.StatusOK, job)
	})
	handle("complete", func(w http.ResponseWriter, r *http.Request, id string) {
		var input report
		if !readBody(w, r, 4<<20, &input) || len(input.Logs) > api.MaxExecutionLogs {
			problem(w, http.StatusBadRequest, "invalid_worker_report")
			return
		}
		workerResponse(w, s.complete(id, input))
	})
	handle("logs", func(w http.ResponseWriter, r *http.Request, id string) {
		var input report
		if !readBody(w, r, 400<<10, &input) || len(input.Logs) > api.MaxExecutionLogs {
			problem(w, http.StatusBadRequest, "invalid_worker_report")
			return
		}
		workerResponse(w, s.upload(id, input))
	})
	handle("question", func(w http.ResponseWriter, r *http.Request, id string) {
		var input questionReport
		if !readBody(w, r, 64<<10, &input) || !input.valid() {
			problem(w, http.StatusBadRequest, "invalid_worker_report")
			return
		}
		question, err := s.exchangeQuestion(id, input)
		if err != nil {
			storeProblem(w, err)
			return
		}
		respond(w, http.StatusOK, question)
	})
	return mux
}

var sessionPattern = regexp.MustCompile(`^[A-Z2-7]{26}$`)

func validSession(value string) bool { return sessionPattern.MatchString(value) }

func workerResponse(w http.ResponseWriter, err error) {
	if err != nil {
		storeProblem(w, err)
		return
	}
	respond(w, http.StatusOK, struct{}{})
}
