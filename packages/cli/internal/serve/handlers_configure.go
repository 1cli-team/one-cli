package serve

// handlers_configure.go is the REST surface that mirrors `one configure
// <verb>`. Each handler is a thin wrapper around
// internal/profile/{store,mutate}; this file owns wire-format concerns
// (JSON in, JSON out, masking, error envelopes) and delegates everything
// else.
//
// Routes (after StripPrefix("/api")):
//
//	GET    /configure                                → full Config (masked unless reveal=1)
//	GET    /configure/{domain}/{backend}             → one section (masked unless reveal=1)
//	POST   /configure/{domain}/{backend}             → upsert {name, profile, use}
//	DELETE /configure/{domain}/{backend}/{name}      → remove
//	PUT    /configure/{domain}/{backend}/default     → set default {name}
//
// "default" is a literal segment, not a path parameter — collisions with a
// profile literally named "default" are method-disambiguated (DELETE
// /configure/.../default deletes the profile, PUT /configure/.../default
// sets it as the default pointer).

import (
	"encoding/json"
	"errors"
	"net/http"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
)

const (
	schemaConfig  = "one-cli/serve-configure-config/v1"
	schemaSection = "one-cli/serve-configure-section/v1"
	schemaUpsert  = "one-cli/serve-configure-upsert/v1"
	schemaRemove  = "one-cli/serve-configure-remove/v1"
	schemaUse     = "one-cli/serve-configure-use/v1"
)

func registerConfigureRoutes(mux *http.ServeMux, _ MuxOpts) {
	mux.HandleFunc("GET /configure", handleGetConfig)
	mux.HandleFunc("GET /configure/{domain}/{backend}", handleGetSection)
	mux.HandleFunc("POST /configure/{domain}/{backend}", handleUpsert)
	mux.HandleFunc("DELETE /configure/{domain}/{backend}/{name}", handleRemove)
	mux.HandleFunc("PUT /configure/{domain}/{backend}/default", handleUse)
}

// handleGetConfig returns every (domain, backend) section in one payload.
// Masks credentials unless reveal=1; the UI uses the unmasked variant only
// when the user explicitly clicks "show" on a credential field.
func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, _, err := profile.Load()
	if err != nil {
		writeProfileError(w, err)
		return
	}
	if !revealRequested(r) {
		c := maskConfig(*cfg)
		cfg = &c
	}
	cfgPath, _ := profile.ConfigPath()
	credPath, _ := profile.CredentialsPath()
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":           schemaConfig,
		"config_path":      cfgPath,
		"credentials_path": credPath,
		"reveal":           revealRequested(r),
		"config":           cfg,
	})
}

// handleGetSection returns one (domain, backend) section. 404 if the pair
// isn't a known (domain, backend) — keeps the URL space honest so the UI
// fails fast on typos.
func handleGetSection(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	if !validPair(domain, backend) {
		writeError(w, http.StatusNotFound, cliErrors.PROFILE_BACKEND_INVALID,
			"unknown (domain, backend) pair",
			map[string]any{"domain": domain, "backend": backend})
		return
	}
	cfg, _, err := profile.Load()
	if err != nil {
		writeProfileError(w, err)
		return
	}
	reveal := revealRequested(r)
	if !reveal {
		c := maskConfig(*cfg)
		cfg = &c
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":  schemaSection,
		"domain":  domain,
		"backend": backend,
		"reveal":  reveal,
		"section": sectionPayload(cfg, domain, backend),
	})
}

// upsertReq mirrors the body of POST /configure/{domain}/{backend}. The
// `profile` field is held as RawMessage so each backend can decode it
// into its specific sub-profile struct (InfisicalProfile, S3Profile, …)
// without falling back to a discriminated-union catch-all.
type upsertReq struct {
	Name    string          `json:"name"`
	Profile json.RawMessage `json:"profile"`
	Use     bool            `json:"use"`
}

func handleUpsert(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	if !validPair(domain, backend) {
		writeError(w, http.StatusNotFound, cliErrors.PROFILE_BACKEND_INVALID,
			"unknown (domain, backend) pair",
			map[string]any{"domain": domain, "backend": backend})
		return
	}
	var body upsertReq
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, err.Error(), nil)
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID,
			"`name` is required.", nil)
		return
	}
	p, err := decodeSubProfile(domain, backend, body.Profile)
	if err != nil {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, err.Error(), nil)
		return
	}
	updated, err := profile.Upsert(profile.Domain(domain), backend, body.Name, p, body.Use)
	if err != nil {
		writeProfileError(w, err)
		return
	}
	status := "completed"
	if updated {
		status = "updated"
	}
	cfg, _, _ := profile.Load()
	isDefault := false
	if cfg != nil {
		_, defaultName := readSection(cfg, domain, backend)
		isDefault = defaultName == body.Name
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"schema":  schemaUpsert,
		"status":  status,
		"domain":  domain,
		"backend": backend,
		"name":    body.Name,
		"default": isDefault,
	})
}

func handleRemove(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	name := r.PathValue("name")
	if !validPair(domain, backend) {
		writeError(w, http.StatusNotFound, cliErrors.PROFILE_BACKEND_INVALID,
			"unknown (domain, backend) pair",
			map[string]any{"domain": domain, "backend": backend})
		return
	}
	if err := profile.Remove(profile.Domain(domain), backend, name); err != nil {
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

func handleUse(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	backend := r.PathValue("backend")
	if !validPair(domain, backend) {
		writeError(w, http.StatusNotFound, cliErrors.PROFILE_BACKEND_INVALID,
			"unknown (domain, backend) pair",
			map[string]any{"domain": domain, "backend": backend})
		return
	}
	var body useReq
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, err.Error(), nil)
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID,
			"`name` is required.", nil)
		return
	}
	if err := profile.SetDefault(profile.Domain(domain), backend, body.Name); err != nil {
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

// validPair reports whether (domain, backend) is one of the eight sections
// the current profile schema knows about. Anything else gets a 404 so the UI doesn't
// have to mirror this list.
func validPair(domain, backend string) bool {
	if domain == "deploy" && profile.IsS3Compatible(backend) {
		return true
	}
	if domain == "container" && profile.IsContainerKind(backend) {
		return true
	}
	switch domain + "/" + backend {
	case "env/infisical", "env/dotenv",
		"deploy/kustomize", "deploy/vercel",
		"deploy/cloudflare", "deploy/edgeone":
		return true
	}
	return false
}

// sectionPayload returns the typed section struct for (domain, backend).
// cfg is expected to already be masked or unmasked per the caller's
// reveal preference.
func sectionPayload(cfg *profile.Config, domain, backend string) any {
	if domain == "deploy" && profile.IsS3Compatible(backend) {
		return *cfg.S3CompatSection(backend)
	}
	if domain == "container" && profile.IsContainerKind(backend) {
		return *cfg.ContainerKindSection(backend)
	}
	switch domain + "/" + backend {
	case "env/infisical":
		return cfg.EnvInfisical
	case "env/dotenv":
		return cfg.EnvDotenv
	case "deploy/kustomize":
		return cfg.DeployKustomize
	case "deploy/vercel":
		return cfg.DeployVercel
	case "deploy/cloudflare":
		return cfg.DeployCloudflare
	case "deploy/edgeone":
		return cfg.DeployEdgeOne
	}
	return nil
}

// readSection mirrors configurecmd.listSection without making it public.
// Returns (names, default) for one (domain, backend) — used by the upsert
// handler to report whether the touched profile is the default one.
func readSection(cfg *profile.Config, domain, backend string) ([]string, string) {
	if domain == "deploy" && profile.IsS3Compatible(backend) {
		sec := cfg.S3CompatSection(backend)
		return mapKeys(sec.Profiles), sec.Default
	}
	if domain == "container" && profile.IsContainerKind(backend) {
		sec := cfg.ContainerKindSection(backend)
		return mapKeys(sec.Profiles), sec.Default
	}
	switch domain + "/" + backend {
	case "env/infisical":
		return mapKeys(cfg.EnvInfisical.Profiles), cfg.EnvInfisical.Default
	case "env/dotenv":
		return mapKeys(cfg.EnvDotenv.Profiles), cfg.EnvDotenv.Default
	case "deploy/kustomize":
		return mapKeys(cfg.DeployKustomize.Profiles), cfg.DeployKustomize.Default
	case "deploy/vercel":
		return mapKeys(cfg.DeployVercel.Profiles), cfg.DeployVercel.Default
	case "deploy/cloudflare":
		return mapKeys(cfg.DeployCloudflare.Profiles), cfg.DeployCloudflare.Default
	case "deploy/edgeone":
		return mapKeys(cfg.DeployEdgeOne.Profiles), cfg.DeployEdgeOne.Default
	}
	return nil, ""
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// decodeSubProfile parses the body's `profile` field into the backend's
// typed sub-profile struct, then assembles the discriminated-union shape
// profile.Upsert expects. Empty body for dotenv is allowed (that backend
// has no fields).
func decodeSubProfile(domain, backend string, raw json.RawMessage) (profile.Profile, error) {
	p := profile.Profile{Backend: backend}
	if domain == "deploy" && profile.IsS3Compatible(backend) {
		var sub profile.S3Profile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.S3 = &sub
		return p, nil
	}
	if domain == "container" && profile.IsContainerKind(backend) {
		var sub profile.ContainerProfile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.Container = &sub
		return p, nil
	}
	switch domain + "/" + backend {
	case "env/infisical":
		var sub profile.InfisicalProfile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.Infisical = &sub
	case "env/dotenv":
		var sub profile.DotenvProfile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.Dotenv = &sub
	case "deploy/kustomize":
		var sub profile.KustomizeProfile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.Kustomize = &sub
	case "deploy/vercel":
		var sub profile.VercelProfile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.Vercel = &sub
	case "deploy/cloudflare":
		var sub profile.CloudflareProfile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.Cloudflare = &sub
	case "deploy/edgeone":
		var sub profile.EdgeOneProfile
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &sub); err != nil {
				return profile.Profile{}, err
			}
		}
		p.EdgeOne = &sub
	default:
		return profile.Profile{}, errors.New("unknown (domain, backend) pair")
	}
	return p, nil
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func revealRequested(r *http.Request) bool {
	return r.URL.Query().Get("reveal") == "1"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

// writeProfileError unwraps a *output.Error and surfaces it through the
// shared error envelope. Falls back to a generic 500 for non-typed errors.
// The status mapping mirrors what the CLI would print: NOT_FOUND-style
// codes get 404, validation errors get 400, everything else is 500.
func writeProfileError(w http.ResponseWriter, err error) {
	var cliErr *output.Error
	if errors.As(err, &cliErr) {
		status := statusForCode(cliErr.Code)
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
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(envelope)
		return
	}
	writeError(w, http.StatusInternalServerError, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
}

func defaultMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func defaultRem(r []output.Remediation) []output.Remediation {
	if r == nil {
		return []output.Remediation{}
	}
	return r
}

// statusForCode maps known CLI error codes to HTTP status. Codes not in
// the table get 500, which is correct for "we tried to do the operation
// and the underlying machinery refused" (filesystem, schema migration).
func statusForCode(code string) int {
	switch code {
	case string(cliErrors.PROFILE_NOT_FOUND):
		return http.StatusNotFound
	case string(cliErrors.PROFILE_ALREADY_EXISTS):
		return http.StatusConflict
	case string(cliErrors.PROFILE_BACKEND_INVALID),
		string(cliErrors.SERVE_PAYLOAD_INVALID):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
