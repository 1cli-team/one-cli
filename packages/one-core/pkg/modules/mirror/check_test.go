package mirror

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckRejectsLoginPagesAndValidatesRealContent(t *testing.T) {
	for _, tc := range []struct {
		name, target, kind, filename, content string
		code                                  int
		want                                  string
	}{
		{"login instead of archive", "go", "artifact", "", "<html>login</html>", 200, "failed"},
		{"archive", "go", "artifact", "", "\x1f\x8b\x08\x00data", 206, "ok"},
		{"bad checksum", "go", "checksum", "", "<html>login</html>", 200, "failed"},
		{"checksum", "go", "checksum", "", strings.Repeat("a", 64), 200, "ok"},
		{"node checksum filename", "node", "checksum", "node.tar.xz", strings.Repeat("a", 64) + "  wrong.tar.xz", 200, "failed"},
		{"node checksum", "node", "checksum", "node.tar.xz", strings.Repeat("a", 64) + "  node.tar.xz", 200, "ok"},
		{"mise checksum relative path", "mise", "checksum", "mise.tar.gz", strings.Repeat("a", 64) + "  ./mise.tar.gz", 200, "ok"},
		{"registry", "npm", "metadata", "", `{"name":"pnpm","version":"12.3.4"}`, 200, "ok"},
		{"empty registry", "npm", "metadata", "", `{}`, 200, "failed"},
		{"missing", "npm", "metadata", "", "missing", 404, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.kind == "artifact" && r.Header.Get("Range") == "" {
					t.Error("archive request is unbounded")
				}
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.content)
			}))
			defer server.Close()
			result := checkEndpoint(context.Background(), server.Client(), probe{tc.target, tc.kind, server.URL, tc.filename})
			if result.Status != tc.want {
				t.Fatalf("got %+v, want %s", result, tc.want)
			}
		})
	}
}

func TestCheckCancellationAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client := server.Client()
	client.Timeout = 20 * time.Millisecond
	r := checkEndpoint(t.Context(), client, probe{target: "npm", kind: "metadata", url: server.URL})
	if r.Status != "failed" || r.Detail == "" {
		t.Fatalf("timeout = %+v", r)
	}
}
