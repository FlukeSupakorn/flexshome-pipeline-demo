package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"sync/atomic"
)

type app struct {
	version     string
	environment string
	ready       atomic.Bool
}

func newApp(version, environment string) *app {
	a := &app{version: version, environment: environment}
	a.ready.Store(true)
	return a
}

func (a *app) drain() { a.ready.Store(false) }

var homePage = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FlexsHome pipeline demo</title></head>
<body><main><h1>FlexsHome</h1><p>Deployment pipeline demo</p><dl><dt>Release</dt><dd>{{.Version}}</dd><dt>Environment</dt><dd>{{.Environment}}</dd></dl><p><a href="/version">Release details</a></p></main></body></html>`))

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = homePage.Execute(w, struct{ Version, Environment string }{a.version, a.environment})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !a.ready.Load() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": a.version, "environment": a.environment})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}
