// Package webui embeds the production interface in the standalone executable.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

// Build the frontend with `npm --prefix web run build` before compiling Go.
//
//go:embed dist
var content embed.FS

func Handler() http.Handler {
	assets, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
