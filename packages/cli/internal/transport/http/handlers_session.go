package serve

import (
	"context"
	"net/http"
	"sync"

	remote "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
)

func registerSessionRoutes(mux *http.ServeMux) {
	var mu sync.Mutex
	var attempt *session.Attempt
	mux.HandleFunc("GET /session", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		info, e := session.Status()
		if e != nil {
			writeServiceError(w, e)
			return
		}
		result := map[string]any{"session": info}
		mu.Lock()
		defer mu.Unlock()
		if attempt != nil {
			_, err, done := attempt.Result()
			state := "waiting"
			if done {
				state = "complete"
			}
			if err != nil {
				state = "failed"
				result["error"] = err.Error()
			}
			result["login"] = map[string]string{"status": state, "url": attempt.URL}
		}
		writeJSON(w, 200, result)
	})
	mux.HandleFunc("POST /session/login", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		var body struct {
			SiteURL string `json:"siteUrl"`
		}
		if e := decodeJSON(r, &body); e != nil {
			writeBadPayload(w, e.Error())
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if attempt != nil {
			_, _, done := attempt.Result()
			if !done {
				writeJSON(w, 200, map[string]string{"url": attempt.URL})
				return
			}
		}
		a, e := session.Start(context.Background(), body.SiteURL)
		if e != nil {
			writeServiceError(w, e)
			return
		}
		attempt = a
		writeJSON(w, 200, map[string]string{"url": a.URL})
	})
	mux.HandleFunc("DELETE /session/login", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if attempt != nil {
			attempt.Cancel()
			attempt = nil
		}
		setNoStore(w)
		writeJSON(w, 200, map[string]bool{"cancelled": true})
	})
	mux.HandleFunc("DELETE /session", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if attempt != nil {
			attempt.Cancel()
			attempt = nil
		}
		if e := session.Logout(); e != nil {
			writeServiceError(w, e)
			return
		}
		setNoStore(w)
		writeJSON(w, 200, map[string]bool{"loggedIn": false})
	})
	mux.HandleFunc("GET /infisical/projects", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		p, e := remote.Projects(r.Context())
		if e != nil {
			writeServiceError(w, e)
			return
		}
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("POST /infisical/projects", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		var body struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		project, err := remote.CreateRemoteProject(r.Context(), body.Name)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, project)
	})
	mux.HandleFunc("GET /infisical/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		p, e := remote.Project(r.Context(), r.PathValue("id"))
		if e != nil {
			writeServiceError(w, e)
			return
		}
		writeJSON(w, 200, p)
	})
}
func registerGlobalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /global-env/location/default", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		location, err := remote.EnsureDefaultGlobal(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"location": location})
	})
	mux.HandleFunc("GET /global-env/location", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		l, e := remote.LoadGlobalLocation()
		if e != nil {
			writeServiceError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"location": l})
	})
	mux.HandleFunc("PUT /global-env/location", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		var b struct {
			ProjectID   string `json:"projectId"`
			Environment string `json:"environment"`
		}
		if e := decodeJSON(r, &b); e != nil {
			writeBadPayload(w, e.Error())
			return
		}
		l, e := remote.BindGlobal(r.Context(), b.ProjectID, b.Environment)
		if e != nil {
			writeServiceError(w, e)
			return
		}
		writeJSON(w, 200, map[string]any{"location": l})
	})
	mux.HandleFunc("GET /global-env/secrets", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		l, e := remote.ListGlobal(r.Context(), r.URL.Query().Get("env"), r.URL.Query().Get("path"))
		if e != nil {
			writeServiceError(w, e)
			return
		}
		writeJSON(w, 200, l)
	})
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		mux.HandleFunc(method+" /global-env/secrets/{key}", func(w http.ResponseWriter, r *http.Request) {
			setNoStore(w)
			action := map[string]string{"GET": "get", "POST": "create", "PUT": "update", "DELETE": "unset"}[r.Method]
			var b struct {
				Value string `json:"value"`
			}
			if r.Method == "POST" || r.Method == "PUT" {
				if e := decodeJSON(r, &b); e != nil {
					writeBadPayload(w, e.Error())
					return
				}
			}
			result, e := remote.GlobalSecret(r.Context(), action, r.URL.Query().Get("env"), r.URL.Query().Get("path"), r.PathValue("key"), b.Value)
			if e != nil {
				writeServiceError(w, e)
				return
			}
			writeJSON(w, 200, result)
		})
	}
	mux.HandleFunc("POST /global-env/folders", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		var b struct {
			Name string `json:"name"`
		}
		if e := decodeJSON(r, &b); e != nil {
			writeBadPayload(w, e.Error())
			return
		}
		if e := remote.CreateGlobalFolder(r.Context(), r.URL.Query().Get("env"), r.URL.Query().Get("path"), b.Name); e != nil {
			writeServiceError(w, e)
			return
		}
		writeJSON(w, 201, map[string]string{"name": b.Name})
	})
}
