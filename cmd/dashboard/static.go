package main

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:web_dist
var embeddedWebFS embed.FS

//go:embed placeholder.html
var placeholderHTML []byte

func webFS() (fs.FS, bool) {
	sub, err := fs.Sub(embeddedWebFS, "web_dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return sub, true
}

func spaHandler(static fs.FS) http.HandlerFunc {
	index, err := fs.ReadFile(static, "index.html")
	if err != nil {
		index = placeholderHTML
	}
	server := http.FileServer(http.FS(static))

	return func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(r.URL.Path)
		if clean == "/" {
			writeHTML(w, index)
			return
		}

		rel := strings.TrimPrefix(clean, "/")
		f, err := static.Open(rel)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				writeHTML(w, index)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = f.Close()
		server.ServeHTTP(w, r)
	}
}

func placeholderHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeHTML(w, placeholderHTML)
	}
}

func writeHTML(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}
