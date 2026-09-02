package server

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReportMeta identifies one research report in the reports directory.
type ReportMeta struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// reportTitle returns the first "# " heading of the markdown, or the slug
// when the document has none.
func reportTitle(md, slug string) string {
	for _, line := range strings.Split(md, "\n") {
		if t, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(t)
		}
	}
	return slug
}

func badReportName(name string) bool {
	return name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..")
}

func (s *Server) handleReportsList(w http.ResponseWriter, r *http.Request) {
	out := []ReportMeta{}
	entries, err := os.ReadDir(s.cfg.ReportsDir)
	if err != nil {
		// A bench without a reports directory just has no reports yet.
		writeJSON(w, http.StatusOK, out)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.cfg.ReportsDir, e.Name()))
		if err != nil {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		var mod time.Time
		if info, err := e.Info(); err == nil {
			mod = info.ModTime().UTC()
		}
		out = append(out, ReportMeta{Slug: slug, Title: reportTitle(string(b), slug), UpdatedAt: mod})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleReportGet(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if badReportName(slug) {
		writeErr(w, http.StatusBadRequest, errors.New("bad report slug"))
		return
	}
	b, err := os.ReadFile(filepath.Join(s.cfg.ReportsDir, slug+".md"))
	if err != nil {
		writeErr(w, http.StatusNotFound, errors.New("report not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"slug":     slug,
		"title":    reportTitle(string(b), slug),
		"markdown": string(b),
	})
}

func (s *Server) handleReportImg(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if badReportName(name) {
		writeErr(w, http.StatusBadRequest, errors.New("bad image name"))
		return
	}
	b, err := os.ReadFile(filepath.Join(s.cfg.ReportsDir, "img", name))
	if err != nil {
		writeErr(w, http.StatusNotFound, errors.New("image not found"))
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Write(b)
}
