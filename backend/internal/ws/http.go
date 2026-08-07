package ws

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/coder/websocket"

	"github.com/aeroxe/compliance-hub/backend/internal/auth"
	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/config"
)

// HandleHTTP upgrades a GET /ws request into a WebSocket connection.
// Optional ?topic=comma,separated,event,types pre-subscribes the client;
// without it the client receives every event type.
//
// The upgrade is done inside a standard http.Handler wrapped with
// adaptor.HertzHandler: hertz's adaptor supplies an http.ResponseWriter that
// implements http.Hijacker (required by coder/websocket) and blocks until the
// hijacked connection closes, which keeps the request context alive for the
// lifetime of the socket.
func (h *Hub) HandleHTTP(ctx context.Context, c *app.RequestContext) {
	adaptor.HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An empty OriginPatterns list means "allow all origins" in
		// coder/websocket (browser clients always send an Origin header), which
		// keeps the hub consistent with the permissive CORS middleware.
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			slog.Warn("websocket upgrade failed", "error", err, "url", r.URL.String())
			http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
			return
		}

		topics := splitTopics(r.URL.Query().Get("topic"))
		if err := h.ServeWS(ctx, conn, topics); err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}))(ctx, c)
}

// HandleHTTPAuth upgrades a /ws connection only after validating the caller's
// bearer token (JWT signature + active single-session fingerprint), so the
// event stream is protected by the same RBAC identity as the REST API. The
// token is read from the ?token= query parameter (browser clients cannot set
// headers on the WebSocket handshake) or the Authorization header.
func (h *Hub) HandleHTTPAuth(ctx context.Context, c *app.RequestContext, cfg *config.Config, cache cache.Cache) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tokenStr := wsToken(c)
	if tokenStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{"error": "missing or invalid token"})
		return
	}
	claims, err := auth.ParseToken(cfg.JWTSecret, tokenStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{"error": "invalid token"})
		return
	}
	expected, ok := cache.Get(ctx, auth.SessionKey(claims.UserID))
	if !ok || auth.SessionHash(tokenStr, claims.UserID) != expected {
		c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{"error": "session expired"})
		return
	}

	// Stash the identity so downstream audit/hub logging can attribute events.
	c.Set("auth_user_id", claims.UserID)
	h.HandleHTTP(ctx, c)
}

// wsToken extracts the bearer token from the query string or Authorization
// header of a WebSocket handshake.
func wsToken(c *app.RequestContext) string {
	if t := string(c.Query("token")); t != "" {
		return t
	}
	authorization := string(c.GetHeader("Authorization"))
	if len(authorization) > 7 && strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return strings.TrimSpace(authorization[7:])
	}
	return ""
}

func splitTopics(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, t := range strings.Split(raw, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}
