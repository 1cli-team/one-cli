package serve

import (
	"errors"
	"net/http"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func registerCodeHostRoutes(mux *http.ServeMux) {
	service := codehost.Service{}
	reply := func(w http.ResponseWriter, result any, err error) {
		setNoStore(w)
		if err == nil {
			writeJSON(w, http.StatusOK, result)
			return
		}
		status, key := http.StatusInternalServerError, "git.code_host.load"
		var failure codehost.Failure
		if errors.As(err, &failure) {
			key = "git.code_host." + string(failure)
			switch failure {
			case codehost.ErrInvalid:
				status = http.StatusBadRequest
			case codehost.ErrMissing:
				status = http.StatusNotFound
			case codehost.ErrAuth, codehost.ErrTest, codehost.ErrRepositoryAccess:
				status = http.StatusBadGateway
			case codehost.ErrAddressConflict:
				status = http.StatusConflict
			}
		}
		writeError(w, status, cliErrors.ONE_CLI_ERROR, i18n.T(key), nil)
	}
	mux.HandleFunc("GET /code-hosts", func(w http.ResponseWriter, _ *http.Request) {
		result, err := service.List()
		reply(w, result, err)
	})
	mux.HandleFunc("PUT /code-hosts", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		var input codehost.Input
		if err := decodeJSON(r, &input); err != nil {
			reply(w, nil, codehost.ErrInvalid)
			return
		}
		result, err := service.Save(r.Context(), input)
		reply(w, result, err)
	})
	mux.HandleFunc("POST /code-hosts/{host}/test", func(w http.ResponseWriter, r *http.Request) {
		result, err := service.Test(r.Context(), r.PathValue("host"))
		reply(w, result, err)
	})
}
