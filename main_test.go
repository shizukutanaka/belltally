package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"belltally/internal/tally"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	return (&server{store: &tally.Store{Path: filepath.Join(t.TempDir(), "belltally.json")}}).routes()
}

func serveRequest(t *testing.T, h http.Handler, method, target, host, origin, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func requireStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, want, rr.Body.String())
	}
}

func TestGuardAllowsLocalhostGET(t *testing.T) {
	rr := serveRequest(t, testHandler(t), http.MethodGet, "/api/state", "127.0.0.1:8781", "", "")
	requireStatus(t, rr, http.StatusOK)
}

func TestGuardRejectsForeignHost(t *testing.T) {
	rr := serveRequest(t, testHandler(t), http.MethodGet, "/api/state", "evil.example.com", "", "")
	requireStatus(t, rr, http.StatusForbidden)
}

func TestGuardRejectsForeignOrigin(t *testing.T) {
	body := `{"company":7,"points":1,"delta":1}`
	rr := serveRequest(t, testHandler(t), http.MethodPost, "/api/count", "127.0.0.1:8781", "http://evil.example.com", body)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestGuardRejectsMalformedOrigin(t *testing.T) {
	body := `{"company":7,"points":1,"delta":1}`
	rr := serveRequest(t, testHandler(t), http.MethodPost, "/api/count", "127.0.0.1:8781", "http://[::1", body)
	requireStatus(t, rr, http.StatusForbidden)
}

func TestGuardAllowsMatchingOrigin(t *testing.T) {
	body := `{"company":7,"points":1,"delta":1}`
	rr := serveRequest(t, testHandler(t), http.MethodPost, "/api/count", "127.0.0.1:8781", "http://127.0.0.1:8781", body)
	requireStatus(t, rr, http.StatusOK)
}

func TestGuardAllowsMissingOrigin(t *testing.T) {
	body := `{"company":7,"points":1,"delta":1}`
	rr := serveRequest(t, testHandler(t), http.MethodPost, "/api/count", "127.0.0.1:8781", "", body)
	requireStatus(t, rr, http.StatusOK)
}
