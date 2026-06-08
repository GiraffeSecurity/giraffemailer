// Package ui embeds the compiled Next.js static export into the binary.
// Run `make build-ui` before `go build` to populate the dist/ directory.
package ui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

// Next.js puts assets under dist/_next; plain //go:embed dist skips names starting with _.
//go:embed all:dist
var distFS embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("ui: dist embed is missing — run `make build-ui` first: " + err.Error())
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		resolved, err := resolve(sub, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(sub, resolved)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if ct := mime.TypeByExtension(path.Ext(resolved)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	})
}

func resolve(fsys fs.FS, name string) (string, error) {
	if fileExists(fsys, name) {
		return name, nil
	}
	if fileExists(fsys, name+"/index.html") {
		return name + "/index.html", nil
	}
	if fileExists(fsys, name+".html") {
		return name + ".html", nil
	}
	if !strings.Contains(path.Base(name), ".") {
		if fileExists(fsys, "index.html") {
			return "index.html", nil
		}
	}
	return "", fs.ErrNotExist
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
