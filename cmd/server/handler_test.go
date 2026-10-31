package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func request(a *app, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, r)
	return w
}

func TestHealth(t *testing.T) {
	w := request(newApp("abc123", "dev"), http.MethodGet, "/healthz")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("health = %d %q", w.Code, w.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("health body = %q (%v)", w.Body.String(), err)
	}
}

func TestReadinessDuringDrain(t *testing.T) {
	a := newApp("abc123", "dev")
	if w := request(a, http.MethodGet, "/readyz"); w.Code != http.StatusOK {
		t.Fatalf("ready before drain = %d", w.Code)
	}
	a.drain()
	if w := request(a, http.MethodGet, "/readyz"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready during drain = %d", w.Code)
	}
	if w := request(a, http.MethodGet, "/healthz"); w.Code != http.StatusOK {
		t.Fatalf("draining process should remain alive; health = %d", w.Code)
	}
}

func TestVersion(t *testing.T) {
	w := request(newApp("source-abc", "uat"), http.MethodGet, "/version")
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || body["version"] != "source-abc" || body["environment"] != "uat" || len(body) != 2 {
		t.Fatalf("version response = %d %#v", w.Code, body)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("version response must not be cached")
	}
}

func TestHomeEscapesReleaseText(t *testing.T) {
	w := request(newApp("<script>alert(1)</script>", "dev"), http.MethodGet, "/")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "FlexsHome") || strings.Contains(w.Body.String(), "<script>") {
		t.Fatalf("home response = %d %q", w.Code, w.Body.String())
	}
}

func TestRoutesAndMethods(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/missing", http.StatusNotFound},
		{http.MethodGet, "/version/extra", http.StatusNotFound},
		{http.MethodPost, "/version", http.StatusMethodNotAllowed},
		{http.MethodHead, "/healthz", http.StatusOK},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := request(newApp("abc", "dev"), tc.method, tc.path)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
		})
	}
}

func TestResponseSafetyHeaders(t *testing.T) {
	w := request(newApp("abc", "dev"), http.MethodGet, "/")
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("missing safety headers: %v", w.Header())
	}
}
