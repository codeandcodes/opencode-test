//go:build ui

package server

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:ui_dist
var uiFS embed.FS

// registerUI serves the embedded Svelte SPA at /, falling back to
// index.html for unknown non-/api paths so hash-less deep links work.
func (s *Server) registerUI() {
	dist, err := fs.Sub(uiFS, "ui_dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServerFS(dist)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, errors.New("not found"))
			return
		}
		if p := strings.TrimPrefix(r.URL.Path, "/"); p != "" {
			if _, err := fs.Stat(dist, p); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		raw, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, errors.New("embedded index.html missing"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(raw)
	})
}
