package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportsListGetAndImage(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	dir := t.TempDir()
	s.cfg.ReportsDir = dir
	os.MkdirAll(filepath.Join(dir, "img"), 0o755)
	os.WriteFile(filepath.Join(dir, "model-x.md"),
		[]byte("# Model X on the bench\n\nbody ![shot](img/shot.png)\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "img", "shot.png"), []byte("PNGBYTES"), 0o644)

	rr, _ := doJSON(t, s, "GET", "/api/reports", nil)
	if rr.Code != 200 {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	var list []ReportMeta
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Slug != "model-x" || list[0].Title != "Model X on the bench" {
		t.Fatalf("unexpected list: %+v", list)
	}

	rr, out := doJSON(t, s, "GET", "/api/reports/model-x", nil)
	if rr.Code != 200 || out["title"] != "Model X on the bench" {
		t.Fatalf("get: %d %v", rr.Code, out)
	}
	if md, _ := out["markdown"].(string); !strings.Contains(md, "img/shot.png") {
		t.Fatalf("markdown body missing: %v", out)
	}

	rr, _ = doJSON(t, s, "GET", "/api/reports/img/shot.png", nil)
	if rr.Code != 200 || rr.Body.String() != "PNGBYTES" {
		t.Fatalf("img: %d %q", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("img content-type: %q", ct)
	}
}

func TestReportsRejectTraversalAndMissing(t *testing.T) {
	s, _, _ := newTestServer(t, "ok")
	s.cfg.ReportsDir = t.TempDir()

	if rr, _ := doJSON(t, s, "GET", "/api/reports/..%2fsecret", nil); rr.Code != 400 && rr.Code != 404 {
		t.Fatalf("traversal slug not rejected: %d", rr.Code)
	}
	if rr, _ := doJSON(t, s, "GET", "/api/reports/nope", nil); rr.Code != 404 {
		t.Fatalf("missing report: %d", rr.Code)
	}

	// A bench with no reports directory yet serves an empty list, not an error.
	s.cfg.ReportsDir = filepath.Join(s.cfg.ReportsDir, "does-not-exist")
	rr, _ := doJSON(t, s, "GET", "/api/reports", nil)
	if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %q", rr.Code, rr.Body.String())
	}
}
