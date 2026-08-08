package server

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"

	"github.com/aeroxe/compliance-hub/backend/internal/config"
)

func TestCORSAllowlist(t *testing.T) {
	cfg := &config.Config{CORSAllowedOrigins: []string{"https://app.example.com"}}
	h := server.New()
	h.Use(cors(cfg))
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.String(200, "pong")
	})

	cases := []struct {
		name       string
		origin     string
		wantACAO   string
		wantCreds  string
		wantStatus int
	}{
		{"allowed origin reflected", "https://app.example.com", "https://app.example.com", "true", 200},
		{"disallowed origin gets no CORS", "https://evil.example.com", "", "", 200},
		{"no origin (curl) no CORS", "", "", "", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var headers []ut.Header
			if tc.origin != "" {
				headers = append(headers, ut.Header{Key: "Origin", Value: tc.origin})
			}
			w := ut.PerformRequest(h.Engine, "GET", "/ping", nil, headers...)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if got := string(w.Header().Get("Access-Control-Allow-Origin")); got != tc.wantACAO {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tc.wantACAO)
			}
			if got := string(w.Header().Get("Access-Control-Allow-Credentials")); got != tc.wantCreds {
				t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, tc.wantCreds)
			}
		})
	}
}

func TestCORSPreflight(t *testing.T) {
	cfg := &config.Config{CORSAllowedOrigins: []string{"http://localhost:3000"}}
	h := server.New()
	h.Use(cors(cfg))
	h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
		c.String(200, "pong")
	})

	// Allowed preflight is short-circuited with 204 + the allowlist headers.
	w := ut.PerformRequest(h.Engine, "OPTIONS", "/ping", nil,
		ut.Header{Key: "Origin", Value: "http://localhost:3000"},
		ut.Header{Key: "Access-Control-Request-Method", Value: "POST"})
	if w.Code != 204 {
		t.Fatalf("preflight status = %d, want 204", w.Code)
	}
	if got := string(w.Header().Get("Access-Control-Allow-Origin")); got != "http://localhost:3000" {
		t.Errorf("preflight ACAO = %q", got)
	}

	// Disallowed preflight returns no allow-origin, so the browser refuses it.
	w2 := ut.PerformRequest(h.Engine, "OPTIONS", "/ping", nil,
		ut.Header{Key: "Origin", Value: "https://evil.example.com"})
	if got := string(w2.Header().Get("Access-Control-Allow-Origin")); got != "" {
		t.Errorf("disallowed preflight ACAO = %q, want empty", got)
	}
}
