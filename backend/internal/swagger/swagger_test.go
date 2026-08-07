package swagger

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/aeroxe/compliance-hub/backend/internal/permissions"
)

func specRoutes() []permissions.RouteInfo {
	return []permissions.RouteInfo{
		{Name: "Compliances", URL: "/api/v1/compliances", Guard: "API", Method: "POST"},
		{Name: "Compliances Search", URL: "/api/v1/compliances/search", Guard: "API", Method: "POST"},
		{Name: "Login", URL: "/api/v1/auth/login", Guard: "API", Method: "POST"},
	}
}

func TestGenerateSpecShape(t *testing.T) {
	spec := GenerateSpec(specRoutes())
	var doc map[string]any
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatalf("spec is not valid JSON: %v", err)
	}
	if doc["openapi"] != "3.0.1" {
		t.Errorf("openapi = %v, want 3.0.1", doc["openapi"])
	}
	paths, _ := doc["paths"].(map[string]any)
	if len(paths) != 3 {
		t.Errorf("paths = %d, want 3", len(paths))
	}
	// Protected routes carry the bearer security requirement; login does not.
	create, _ := paths["/api/v1/compliances"].(map[string]any)
	postOp, _ := create["post"].(map[string]any)
	if postOp["security"] == nil {
		t.Error("POST /api/v1/compliances should require bearer auth in the spec")
	}
	if _, ok := postOp["requestBody"]; !ok {
		t.Error("POST operation should declare a requestBody")
	}
	login, _ := paths["/api/v1/auth/login"].(map[string]any)
	loginOp, _ := login["post"].(map[string]any)
	if loginOp["security"] != nil {
		t.Error("login must not require auth in the spec")
	}
}

// TestSwaggerUIServesThroughRealServer verifies the http-swagger UI over a
// real listener (the ut harness cannot drive hertz's chunked response writer
// that the adaptor uses). This mirrors the live server path used by /ws.
func TestSwaggerUIServesThroughRealServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // give the port back; hertz binds it immediately after

	h := server.New(server.WithHostPorts(addr))
	h.GET("/swagger/doc.json", func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.SetContentType("application/json")
		c.Response.SetStatusCode(200)
		c.Response.SetBody(GenerateSpec(specRoutes()))
	})
	h.GET("/swagger/*any", Handler())

	go h.Spin()
	t.Cleanup(func() { _ = h.Shutdown(context.Background()) })

	// Wait for the listener to accept.
	client := &http.Client{Timeout: 3 * time.Second}
	var resp *http.Response
	for i := 0; i < 50; i++ {
		resp, err = client.Get("http://" + addr + "/swagger/index.html")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET /swagger/index.html: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("swagger index status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("swagger index returned an empty body")
	}

	// The doc spec must load through the UI's configured URL.
	docResp, err := client.Get("http://" + addr + "/swagger/doc.json")
	if err != nil {
		t.Fatalf("GET /swagger/doc.json: %v", err)
	}
	defer docResp.Body.Close()
	if docResp.StatusCode != 200 {
		t.Fatalf("swagger doc status = %d, want 200", docResp.StatusCode)
	}
}
