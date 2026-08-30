package serve

// handlers_configure.go owns only the REST contract for machine profiles.
// Catalog validation, typed profile decoding, masking, storage mutation and
// section lookup live in the configure application service, shared with Cobra.

import (
	"encoding/json"
	"errors"
	"net/http"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

const (
	schemaConfig  = "one-cli/serve-configure-config/v1"
	schemaSection = "one-cli/serve-configure-section/v1"
	schemaUpsert  = "one-cli/serve-configure-upsert/v1"
	schemaRemove  = "one-cli/serve-configure-remove/v1"
	schemaUse     = "one-cli/serve-configure-use/v1"
)

type configureHandler struct {
	profiles *configureapp.ProfileService
}

func registerConfigureRoutes(mux *http.ServeMux, opts MuxOpts) {
	handler := configureHandler{profiles: opts.ProfileService}
	mux.HandleFunc("GET /configure", handler.getConfig)
	mux.HandleFunc("GET /configure/{domain}/{backend}", handler.getSection)
	mux.HandleFunc("POST /configure/{domain}/{backend}", handler.upsert)
	mux.HandleFunc("DELETE /configure/{domain}/{backend}/{name}", handler.remove)
	mux.HandleFunc("PUT /configure/{domain}/{backend}/default", handler.use)
}

func (h configureHandler) knownPair(w http.ResponseWriter, domain, backend string) bool {
	if _, err := h.profiles.Lookup(profile.Domain(domain), backend); err != nil {
		writeError(
			w,
			http.StatusNotFound,
			cliErrors.PROFILE_BACKEND_INVALID,
			"unknown (domain, backend) pair",
			map[string]any{"domain": domain, "backend": backend},
		)
		return false
	}
	return true
}

func (h configureHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.profiles.Load()
	if err != nil {
		writeProfileError(w, err)
		return
	}
	reveal := revealRequested(r)
	if !reveal {
		masked, err := h.profiles.MaskConfig(*config)
		if err != nil {
			writeProfileError(w, err)
			return
		}
		config = &masked
	}
	configPath, credentialsPath, _ := h.profiles.Paths()
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":           schemaConfig,
		"config_path":      configPath,
		"credentials_path": credentialsPath,
		"reveal":           reveal,
		"config":           config,
	})
}

func (h configureHandler) getSection(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	if !h.knownPair(w, domain, backend) {
		return
	}
	config, err := h.profiles.Load()
	if err != nil {
		writeProfileError(w, err)
		return
	}
	reveal := revealRequested(r)
	if !reveal {
		masked, err := h.profiles.MaskConfig(*config)
		if err != nil {
			writeProfileError(w, err)
			return
		}
		config = &masked
	}
	section, err := h.profiles.Section(config, profile.Domain(domain), backend)
	if err != nil {
		writeProfileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":  schemaSection,
		"domain":  domain,
		"backend": backend,
		"reveal":  reveal,
		"section": section.Payload,
	})
}

type upsertReq struct {
	Name    string          `json:"name"`
	Profile json.RawMessage `json:"profile"`
	Use     bool            `json:"use"`
}

func (h configureHandler) upsert(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	if !h.knownPair(w, domain, backend) {
		return
	}
	var body upsertReq
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, err.Error(), nil)
		return
	}
	if body.Name == "" {
		writeError(
			w,
			http.StatusBadRequest,
			cliErrors.SERVE_PAYLOAD_INVALID,
			"`name` is required.",
			nil,
		)
		return
	}
	value, err := h.profiles.DecodeProfile(profile.Domain(domain), backend, body.Profile)
	if err != nil {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, err.Error(), nil)
		return
	}
	result, err := h.profiles.Upsert(configureapp.UpsertProfileInput{
		Domain:         profile.Domain(domain),
		Backend:        backend,
		Name:           body.Name,
		Profile:        value,
		SetDefault:     body.Use,
		PreserveMasked: true,
	})
	if err != nil {
		writeProfileError(w, err)
		return
	}
	status := "completed"
	if result.Updated {
		status = "updated"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":  schemaUpsert,
		"status":  status,
		"domain":  domain,
		"backend": backend,
		"name":    body.Name,
		"default": result.Default,
	})
}

func (h configureHandler) remove(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	name := r.PathValue("name")
	if !h.knownPair(w, domain, backend) {
		return
	}
	if err := h.profiles.Remove(profile.Domain(domain), backend, name); err != nil {
		writeProfileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":  schemaRemove,
		"status":  "removed",
		"domain":  domain,
		"backend": backend,
		"name":    name,
	})
}

type useReq struct {
	Name string `json:"name"`
}

func (h configureHandler) use(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	if !h.knownPair(w, domain, backend) {
		return
	}
	var body useReq
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, err.Error(), nil)
		return
	}
	if body.Name == "" {
		writeError(
			w,
			http.StatusBadRequest,
			cliErrors.SERVE_PAYLOAD_INVALID,
			"`name` is required.",
			nil,
		)
		return
	}
	if err := h.profiles.SetDefault(profile.Domain(domain), backend, body.Name); err != nil {
		writeProfileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":  schemaUse,
		"domain":  domain,
		"backend": backend,
		"name":    body.Name,
	})
}

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func revealRequested(r *http.Request) bool { return r.URL.Query().Get("reveal") == "1" }

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func writeProfileError(w http.ResponseWriter, err error) {
	var cliErr *output.Error
	if errors.As(err, &cliErr) {
		envelope := map[string]any{
			"schema": "one-cli/error/v1",
			"error": map[string]any{
				"code":        cliErr.Code,
				"message":     cliErr.Message,
				"context":     defaultMap(cliErr.Context),
				"remediation": defaultRem(cliErr.Remediation),
			},
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(statusForCode(cliErr.Code))
		_ = json.NewEncoder(w).Encode(envelope)
		return
	}
	writeError(w, http.StatusInternalServerError, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
}

func defaultMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func defaultRem(value []output.Remediation) []output.Remediation {
	if value == nil {
		return []output.Remediation{}
	}
	return value
}

func statusForCode(code string) int {
	switch code {
	case string(cliErrors.PROFILE_NOT_FOUND):
		return http.StatusNotFound
	case string(cliErrors.PROFILE_ALREADY_EXISTS):
		return http.StatusConflict
	case string(cliErrors.PROFILE_IN_USE):
		return http.StatusConflict
	case string(cliErrors.PROFILE_BACKEND_INVALID), string(cliErrors.SERVE_PAYLOAD_INVALID):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
