// Package swagger serves an OpenAPI 3.0 specification that is generated at
// runtime from the actually-registered routes (never stale), exposed through
// the standard swaggo/http-swagger UI at /swagger/index.html.
package swagger

import (
	"encoding/json"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/aeroxe/compliance-hub/backend/internal/permissions"
)

// publicRoutes are excluded from the bearer security requirement in the spec.
var publicRoutes = map[string]bool{
	"GET /health":             true,
	"GET /swagger/doc.json":   true,
	"GET /swagger/index.html": true,
	"GET /ws":                 true,
	"POST /api/v1/auth/login": true,
}

// GenerateSpec builds the OpenAPI 3.0.1 document from the route manifest.
func GenerateSpec(routes []permissions.RouteInfo) []byte {
	paths := map[string]any{}
	for _, r := range routes {
		op := map[string]any{
			"summary":     r.Name,
			"tags":        []string{r.Guard},
			"operationId": operationID(r),
			"responses": map[string]any{
				"200": response("Successful response"),
				"201": response("Created"),
				"400": response("Bad request"),
				"401": response("Unauthorized"),
				"403": response("Forbidden"),
				"404": response("Not found"),
				"500": response("Internal server error"),
			},
		}
		if !publicRoutes[r.Method+" "+r.URL] {
			op["security"] = []map[string][]string{{"bearer_auth": {}}}
		}
		switch r.Method {
		case "POST", "PATCH", "DELETE":
			op["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"type": "object", "additionalProperties": true},
					},
				},
			}
		}
		if paths[r.URL] == nil {
			paths[r.URL] = map[string]any{}
		}
		paths[r.URL].(map[string]any)[lowerMethod(r.Method)] = op
	}

	doc := map[string]any{
		"openapi": "3.0.1",
		"info": map[string]any{
			"title":       "ComplianceHub API",
			"version":     "1.0.0",
			"description": "Compliance tracking and regulatory management platform. All request/response bodies use snake_case; lists use server-side cursor pagination.",
		},
		"servers": []map[string]any{{"url": "/"}},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearer_auth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
		"paths": paths,
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return b
}

// Handler mounts the swagger UI (GET /swagger/*any) loading the runtime spec.
// adaptor.HertzHandler builds a proper *http.Request (including RequestURI,
// which http-swagger's URL parser requires) and a hertz-backed writer.
func Handler() app.HandlerFunc {
	ui := httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json"))
	return adaptor.HertzHandler(ui)
}

func response(desc string) map[string]any {
	return map[string]any{"description": desc}
}

func lowerMethod(m string) string {
	return map[string]string{"POST": "post", "PATCH": "patch", "DELETE": "delete", "GET": "get"}[m]
}

func operationID(r permissions.RouteInfo) string {
	return lowerMethod(r.Method) + "_" + sanitize(r.URL)
}

func sanitize(p string) string {
	out := make([]byte, 0, len(p))
	for i := 0; i < len(p); i++ {
		c := p[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
