package serve

import (
	"context"
	"errors"
	"net/http"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func registerGatewayOpenVikingSyncRoute(mux *http.ServeMux, saved func(context.Context) (knowledge.ConnectionInput, error), loadGateway func() (gateway.Connection, error)) {
	mux.HandleFunc("POST /gateway/workers/{workerId}/openviking", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		var input struct {
			GatewayURL string `json:"gatewayUrl"`
		}
		if err := decodeJSON(r, &input); err != nil || input.GatewayURL == "" {
			writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, i18n.T("openviking.invalid_body"), nil)
			return
		}
		current, err := loadGateway()
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.unavailable"), nil)
			return
		}
		if current.URL != input.GatewayURL {
			writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, i18n.T("openviking.sync_gateway_changed"), nil)
			return
		}
		client, err := gateway.NewClient(current.URL, current.Token)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.unavailable"), nil)
			return
		}
		connection, err := saved(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		var result knowledge.Connection
		err = client.Setup(r.Context(), r.PathValue("workerId"), gateway.SetupInput{
			Operation: gateway.SetupOpenViking, OpenViking: &connection,
		}, &result)
		if err != nil {
			writeOpenVikingSyncError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func writeOpenVikingSyncError(w http.ResponseWriter, err error) {
	var response *gateway.ResponseError
	switch {
	case errors.Is(err, gateway.ErrInvalidSetupWorker):
		writeError(w, http.StatusNotFound, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.setup_error.worker_not_found"), nil)
	case errors.As(err, &response) && response.Code >= 400 && response.Code <= 599:
		message := "openviking.sync_failed"
		switch response.Reason {
		case "setup_pair_https", "worker_offline", "worker_not_found", "setup_busy",
			"setup_timeout", "setup_https_required", "setup_openviking":
			message = "gateway.setup_error." + response.Reason
		}
		writeError(w, response.Code, cliErrors.ONE_CLI_ERROR, i18n.T(message), nil)
	default:
		writeError(w, http.StatusBadGateway, cliErrors.ONE_CLI_ERROR, i18n.T("openviking.sync_failed"), nil)
	}
}
