package serve

import (
	"net/http"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

const schemaCatalog = "one-cli/catalog/v1"

type catalogResponse struct {
	Schema   string                `json:"schema"`
	Backends []catalog.BackendSpec `json:"backends"`
}

func registerCatalogRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("GET /catalog", func(w http.ResponseWriter, _ *http.Request) {
		backendCatalog := opts.Catalog
		if backendCatalog == nil {
			backendCatalog = catalog.Builtin()
		}
		writeJSON(w, http.StatusOK, catalogResponse{
			Schema:   schemaCatalog,
			Backends: backendCatalog.All(),
		})
	})
}
