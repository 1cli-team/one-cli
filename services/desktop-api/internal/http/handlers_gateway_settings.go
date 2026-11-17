package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	gateway "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

type gatewayConnection struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type gatewayStatus struct {
	URL       string            `json:"url"`
	HasToken  bool              `json:"hasToken"`
	State     string            `json:"state"`
	CheckedAt time.Time         `json:"checkedAt"`
	Snapshot  *gateway.Snapshot `json:"snapshot,omitempty"`
}

func gatewayConnectionPath() (string, error) {
	dir, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gateway.json"), nil
}

func readGatewayConnection() (gatewayConnection, error) {
	var connection gatewayConnection
	path, err := gatewayConnectionPath()
	if err != nil {
		return connection, err
	}
	file, err := os.Open(path)
	if err != nil {
		return connection, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return connection, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<10 ||
		(runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return connection, fmt.Errorf("Gateway connection requires a private file")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&connection); err != nil {
		return gatewayConnection{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return gatewayConnection{}, fmt.Errorf("unexpected data after Gateway connection")
	}
	return connection, nil
}

func desktopGateway() (*gateway.Client, error) {
	connection, err := readGatewayConnection()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return gateway.NewClient(connection.URL, connection.Token)
}

func checkGateway(ctx context.Context, connection gatewayConnection) gatewayStatus {
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

func gatewayConnectionInput(r *http.Request) (gatewayConnection, error) {
	var input gatewayConnection
	if err := decodeJSON(r, &input); err != nil {
		return gatewayConnection{}, i18n.Errorf("gateway.connection_input")
	}
	input.URL, input.Token = strings.TrimSpace(input.URL), strings.TrimSpace(input.Token)
	if input.Token == "" {
		// Keep the saved token only for the exact same endpoint.
		if saved, err := readGatewayConnection(); err == nil && saved.URL == input.URL {
			input.Token = saved.Token
		}
	}
	if len(input.URL)+len(input.Token) > 8<<10 {
		return gatewayConnection{}, i18n.Errorf("gateway.connection_input")
	}
	if _, err := gateway.NewClient(input.URL, input.Token); err != nil {
		return gatewayConnection{}, i18n.Errorf("gateway.connection_input")
	}
	return input, nil
}

func saveGatewayConnection(connection gatewayConnection) error {
	path, err := gatewayConnectionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(connection, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, append(payload, '\n'), 0600)
}

func registerGatewaySettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /gateway/status", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		connection, err := readGatewayConnection()
		switch {
		case errors.Is(err, os.ErrNotExist):
			writeJSON(w, http.StatusOK, gatewayStatus{State: "unconfigured", CheckedAt: time.Now().UTC()})
		case err != nil:
			writeJSON(w, http.StatusOK, gatewayStatus{State: "invalid", CheckedAt: time.Now().UTC()})
		default:
			writeJSON(w, http.StatusOK, checkGateway(r.Context(), connection))
		}
	})
	var saveMu sync.Mutex
	mux.HandleFunc("PUT /gateway/connection", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		saveMu.Lock()
		input, inputErr := gatewayConnectionInput(r)
		var saveErr error
		if inputErr == nil {
			saveErr = saveGatewayConnection(input)
		}
		saveMu.Unlock()
		if inputErr != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, inputErr.Error(), nil)
			return
		}
		if saveErr != nil {
			writeError(w, http.StatusInternalServerError, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_save"), nil)
			return
		}
		writeJSON(w, http.StatusOK, checkGateway(r.Context(), input))
	})
	mux.HandleFunc("POST /gateway/connection/test", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		input, err := gatewayConnectionInput(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, checkGateway(r.Context(), input))
	})
}
