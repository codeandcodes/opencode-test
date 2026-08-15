package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	s := New(Config{})
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]bool
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if !body["ok"] {
		t.Fatalf("body = %v, want ok:true", body)
	}
}
