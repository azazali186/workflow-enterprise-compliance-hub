// Package permissions extracts every registered HTTP route from the Hertz
// server and publishes it as an API permission set: it writes routes.json,
// stores the JSON under the `api-gateway-permissions` cache key, and upserts
// one Permission row per route through GORM (no raw SQL).
//
// This mirrors the API-gateway bootstrap pattern: the gateway's
// authorization layer reads `api-gateway-permissions` to know which
// method+path combinations exist and to what they map.
package permissions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/pkg/app/server"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/deps"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

// CacheKey is the Redis/cache key holding the full route permission list.
const CacheKey = "api-gateway-permissions"

// Guard is the authorization scope assigned to every extracted route.
const Guard = "API"

// RouteInfo is a single extracted route record.
type RouteInfo struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Guard  string `json:"guard"`
	Method string `json:"method"`
}

// Generate extracts all routes from the built Hertz server, deduplicates them,
// writes the JSON manifest, stores it in the cache and upserts Permission rows
// in the database. Pass an empty outputPath to skip the file write.
func Generate(h *server.Hertz, d deps.Deps, outputPath string) ([]RouteInfo, error) {
	routes := Extract(h)

	jsonData, err := json.MarshalIndent(routes, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("marshal routes to JSON: %w", err)
	}

	if outputPath != "" {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}
		if err := os.WriteFile(outputPath, jsonData, 0o644); err != nil {
			return nil, fmt.Errorf("write routes JSON to %s: %w", outputPath, err)
		}
		slog.Info("routes manifest written", "path", outputPath)
	}

	if err := d.Cache.Set(context.Background(), CacheKey, string(jsonData), 0); err != nil {
		return nil, fmt.Errorf("store routes in cache: %w", err)
	}
	slog.Info("routes stored in cache", "key", CacheKey, "routes", len(routes))

	if err := storePermissions(d.DB, routes); err != nil {
		return nil, err
	}
	return routes, nil
}

// formatRouteName converts a route path into a human-readable permission name.
// e.g. /api/v1/compliances/:id -> "Compliances :Id"; /ws -> "WebSocket Connection".
func formatRouteName(path string) string {
	if path == "/ws" {
		return "WebSocket Connection"
	}
	cleaned := strings.Replace(path, "/api/v1", "", 1)
	cleaned = strings.ReplaceAll(cleaned, "/", " ")
	titleCaser := cases.Title(language.English)
	return titleCaser.String(strings.TrimSpace(cleaned))
}

// Extract enumerates every registered route as RouteInfo (deduplicated and
// with excluded infra paths removed) without side effects.
func Extract(h *server.Hertz) []RouteInfo {
	var routes []RouteInfo
	seen := make(map[string]struct{})
	for _, r := range h.Routes() {
		routeKey := fmt.Sprintf("%s %s", r.Method, r.Path)
		if _, exists := seen[routeKey]; exists {
			continue
		}
		seen[routeKey] = struct{}{}
		if isExcludedRoute(r.Path) {
			continue
		}
		routes = append(routes, RouteInfo{
			Name:   formatRouteName(r.Path),
			URL:    r.Path,
			Guard:  Guard,
			Method: r.Method,
		})
	}
	return routes
}

// excludedRoutes lists paths that must never become permissions. Health probes
// and internal endpoints never appear in the permission table.
var excludedRoutes = []string{"/health"}

// isExcludedRoute reports whether a route should be skipped. Infrastructure
// endpoints (swagger UI, metrics, health) never become API permissions.
func isExcludedRoute(path string) bool {
	if strings.HasPrefix(path, "/swagger") || path == "/metrics" {
		return true
	}
	for _, p := range excludedRoutes {
		if path == p {
			return true
		}
	}
	return false
}

// storePermissions upserts one Permission row per route (create or update in
// place) and prunes rows whose route is no longer in the manifest, so the
// table always mirrors the registered API surface.
func storePermissions(db *gorm.DB, routes []RouteInfo) error {
	for _, route := range routes {
		uniqueRoute := fmt.Sprintf("%s %s", route.Method, route.URL)

		// Unscoped: a previously soft-deleted row still occupies the unique
		// route slot and must be found instead of causing a duplicate-key error.
		var existing models.Permission
		err := db.Unscoped().Where("route = ?", uniqueRoute).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("query permission %s: %w", uniqueRoute, err)
		}

		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := createPermission(db, route, uniqueRoute); err != nil {
				return err
			}
			continue
		}

		if existing.DeletedAt.Valid {
			// Soft-deleted row: hard-delete it first, then insert fresh so the
			// route becomes visible again without tripping the unique index.
			if err := db.Unscoped().Delete(&models.Permission{}, "id = ?", existing.ID).Error; err != nil {
				return fmt.Errorf("purge soft-deleted permission %s: %w", uniqueRoute, err)
			}
			if err := createPermission(db, route, uniqueRoute); err != nil {
				return err
			}
			continue
		}

		needUpdate := false
		if existing.Name != route.Name {
			existing.Name = route.Name
			needUpdate = true
		}
		if existing.Path != route.URL {
			existing.Path = route.URL
			needUpdate = true
		}
		if existing.Method != route.Method {
			existing.Method = route.Method
			needUpdate = true
		}
		if existing.Service != "api-gateway" {
			existing.Service = "api-gateway"
			needUpdate = true
		}
		if needUpdate {
			if err := db.Save(&existing).Error; err != nil {
				return fmt.Errorf("update permission %s: %w", uniqueRoute, err)
			}
			slog.Info("updated permission", "route", uniqueRoute)
		} else {
			slog.Info("permission unchanged", "route", uniqueRoute)
		}
	}

	// Prune: hard-delete rows for routes that no longer exist in the API.
	keys := make([]string, 0, len(routes))
	for _, r := range routes {
		keys = append(keys, fmt.Sprintf("%s %s", r.Method, r.URL))
	}
	if len(keys) > 0 {
		if err := db.Unscoped().Where("route NOT IN ?", keys).Delete(&models.Permission{}).Error; err != nil {
			return fmt.Errorf("prune stale permissions: %w", err)
		}
		return nil
	}
	// Empty manifest: nothing to keep.
	var all []models.Permission
	if err := db.Unscoped().Find(&all).Error; err != nil {
		return fmt.Errorf("list permissions for prune: %w", err)
	}
	for i := range all {
		if err := db.Unscoped().Delete(&models.Permission{}, "id = ?", all[i].ID).Error; err != nil {
			return fmt.Errorf("prune permission %s: %w", all[i].Route, err)
		}
	}
	return nil
}

func createPermission(db *gorm.DB, route RouteInfo, uniqueRoute string) error {
	newPermission := models.Permission{
		Name:    route.Name,
		Route:   uniqueRoute,
		Path:    route.URL,
		Method:  route.Method,
		Service: "api-gateway",
	}
	if err := db.Create(&newPermission).Error; err != nil {
		return fmt.Errorf("insert permission %s: %w", uniqueRoute, err)
	}
	slog.Info("new permission inserted", "route", uniqueRoute)
	return nil
}
