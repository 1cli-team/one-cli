package serve

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	gateway "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type gatewayStatus struct {
	URL       string            `json:"url"`
	HasToken  bool              `json:"hasToken"`
	State     string            `json:"state"`
	CheckedAt time.Time         `json:"checkedAt"`
	Snapshot  *gateway.Snapshot `json:"snapshot,omitempty"`
}

func desktopGateway() (*gateway.Client, error) {
	connection, err := gateway.ReadConnection()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return gateway.NewClient(connection.URL, connection.Token)
}

func checkGateway(ctx context.Context, connection gateway.Connection) gatewayStatus {
	result := gatewayStatus{State: "invalid", CheckedAt: time.Now().UTC()}
	client, err := gateway.NewClient(connection.URL, connection.Token)
	if err != nil {
		return result
	}
	result.URL, result.HasToken = connection.URL, true
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	snapshot, err := client.Snapshot(ctx)
	result.CheckedAt = time.Now().UTC()
	if err == nil {
		result.State, result.Snapshot = "connected", &snapshot
		return result
	}
	var response *gateway.ResponseError
	switch {
	case errors.As(err, &response) && (response.Code == http.StatusUnauthorized || response.Code == http.StatusForbidden):
		result.State = "unauthorized"
	case errors.Is(err, gateway.ErrInvalidSnapshot),
		response != nil && response.Code == http.StatusNotFound:
		result.State = "incompatible"
	default:
		result.State = "unreachable"
	}
	return result
}

func registerGatewaySettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /gateway/status", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		connection, err := gateway.ReadConnection()
		switch {
		case errors.Is(err, os.ErrNotExist):
			writeJSON(w, http.StatusOK, gatewayStatus{State: "unconfigured", CheckedAt: time.Now().UTC()})
		case err != nil:
			writeJSON(w, http.StatusOK, gatewayStatus{State: "invalid", CheckedAt: time.Now().UTC()})
		default:
			writeJSON(w, http.StatusOK, checkGateway(r.Context(), connection))
		}
	})
	mux.HandleFunc("PUT /gateway/connection", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		var input gateway.Connection
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_input"), nil)
			return
		}
		connection, err := gateway.SaveConnection(input)
		switch {
		case errors.Is(err, gateway.ErrInvalidConnection):
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_input"), nil)
		case err != nil:
			writeError(w, http.StatusInternalServerError, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_save"), nil)
		default:
			writeJSON(w, http.StatusOK, checkGateway(r.Context(), connection))
		}
	})
	mux.HandleFunc("POST /gateway/connection/test", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		var input gateway.Connection
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_input"), nil)
			return
		}
		connection, err := gateway.ResolveConnection(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_input"), nil)
			return
		}
		writeJSON(w, http.StatusOK, checkGateway(r.Context(), connection))
	})
}
