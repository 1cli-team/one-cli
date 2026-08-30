package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

func catalogRequest(t *testing.T, backendCatalog *catalog.Catalog) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	mux := BuildMux(MuxOpts{
		Token:         testToken,
		ExpectedHosts: map[string]struct{}{"127.0.0.1": {}},
		SelfOrigin:    "http://127.0.0.1",
		UIDisabled:    true,
		Catalog:       backendCatalog,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/catalog?token="+testToken, nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec, rec.Body.Bytes()
}

func TestCatalogEndpointUsesInjectedCatalog(t *testing.T) {
	t.Parallel()

	custom, err := catalog.New(catalog.BackendSpec{
		ID:           catalog.BackendID{Domain: catalog.DomainEnv, Name: "test"},
		Pair:         "env/test",
		Capabilities: []catalog.Capability{catalog.CapabilityEnvGet},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec, raw := catalogRequest(t, custom)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/catalog status = %d, body = %s", rec.Code, raw)
	}
	var payload catalogResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Schema != schemaCatalog {
		t.Fatalf("schema = %q, want %q", payload.Schema, schemaCatalog)
	}
	if len(payload.Backends) != 1 || payload.Backends[0].Pair != "env/test" {
		t.Fatalf("backends = %#v", payload.Backends)
	}
}

func TestCatalogEndpointContainsNoCredentialValues(t *testing.T) {
	t.Parallel()

	rec, raw := catalogRequest(t, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/catalog status = %d, body = %s", rec.Code, raw)
	}
	var payload catalogResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Backends) != 16 {
		t.Fatalf("len(backends) = %d, want 16", len(payload.Backends))
	}
	for _, backend := range payload.Backends {
		for _, field := range backend.Profile.Fields {
			if field.Type == catalog.FieldSecret && field.Default != nil {
				t.Fatalf("secret field %s/%s exposes a default", backend.Pair, field.Path)
			}
		}
	}
}
