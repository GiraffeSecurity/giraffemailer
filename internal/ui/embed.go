// Package ui embeds the compiled Next.js static export into the binary.
// Run `make build-ui` before `go build` to populate the dist/ directory.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var distFS embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("ui: dist embed is missing — run `make build-ui` first: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := sub.Open(path); err != nil {
			if _, err := sub.Open(path + "/index.html"); err == nil {
				r.URL.Path = "/" + path + "/index.html"
			} else if _, err := sub.Open(path + ".html"); err == nil {
				r.URL.Path = "/" + path + ".html"
			} else {
				r.URL.Path = "/index.html"
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}
