package serve

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain exactly one JSON object")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func writeServiceError(w http.ResponseWriter, err error) {
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
	case string(cliErrors.ENV_KEY_NOT_FOUND):
		return http.StatusNotFound
	case string(cliErrors.INFISICAL_FOLDER_NOT_FOUND), string(cliErrors.INFISICAL_PROJECT_NOT_FOUND):
		return http.StatusNotFound
	case string(cliErrors.TEMPLATE_NOT_FOUND):
		return http.StatusNotFound
	case string(cliErrors.TARGET_EXISTS):
		return http.StatusConflict
	case string(cliErrors.SERVE_MANIFEST_CONFLICT):
		return http.StatusConflict
	case string(cliErrors.INVALID_NAME), string(cliErrors.TEMPLATE_REQUIRED), string(cliErrors.SUBPROJECT_NAME_REQUIRED):
		return http.StatusBadRequest
	case string(cliErrors.ENV_SET_OVERWRITE_REQUIRED):
		return http.StatusConflict
	case string(cliErrors.ENV_BACKEND_UNCHANGED):
		return http.StatusConflict
	case string(cliErrors.INFISICAL_NOT_CONFIGURED), string(cliErrors.INFISICAL_AUTH_MISSING):
		return http.StatusConflict
	case string(cliErrors.INFISICAL_AUTH_FAILED):
		return http.StatusUnauthorized
	case string(cliErrors.INFISICAL_NETWORK_ERROR), string(cliErrors.INFISICAL_API_ERROR):
		return http.StatusBadGateway
	case string(cliErrors.PREFERENCES_INVALID), string(cliErrors.SERVE_PAYLOAD_INVALID),
		string(cliErrors.ENV_BACKEND_INVALID),
		string(cliErrors.ENV_INVALID_ENV_NAME), string(cliErrors.ENV_INVALID_KEY),
		string(cliErrors.ENV_UNKNOWN_ENVIRONMENT), string(cliErrors.ENV_SET_KEY_REQUIRED):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
