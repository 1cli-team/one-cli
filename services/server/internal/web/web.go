package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed assets/*
var assets embed.FS

// Handler serves the same React bundle used by the desktop application.
func Handler(root string) http.Handler {
	var files fs.FS
	if root != "" {
		files = os.DirFS(root)
	} else {
		files, _ = fs.Sub(assets, "assets")
	}
	server := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(files, name); err != nil {
			if path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		server.ServeHTTP(w, r)
	})
}
