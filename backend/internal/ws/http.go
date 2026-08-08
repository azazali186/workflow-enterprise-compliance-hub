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
// accept performs the HTTP->WebSocket upgrade. When subprotocol is non-empty
// it is offered to coder/websocket so the client's Sec-WebSocket-Protocol
// handshake is echoed back — browsers abort the upgrade when they send a
// subprotocol and the server does not echo one.
//
// There is intentionally no public (unauthenticated) upgrade path: every
// registered /ws route goes through HandleHTTPAuth, so the event stream is
// always identity-checked.
func (h *Hub) accept(ctx context.Context, c *app.RequestContext, subprotocol string) {
	adaptor.HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An empty OriginPatterns list means "allow all origins" in
		// coder/websocket (browser clients always send an Origin header), which
		// keeps the hub consistent with the permissive CORS middleware.
		var opts *websocket.AcceptOptions
		if subprotocol != "" {
			opts = &websocket.AcceptOptions{Subprotocols: []string{subprotocol}}
		}
		conn, err := websocket.Accept(w, r, opts)
		if err != nil {
			// Log only the path, never the raw URL: a stale client that still
			// appends ?token= must not be able to push a JWT into the logs.
			slog.Warn("websocket upgrade failed", "error", err, "path", r.URL.Path)
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
// event stream is protected by the same RBAC identity as the REST API.
//
// The token is carried either in the Authorization header (non-browser
// clients) or in the Sec-WebSocket-Protocol subprotocol as "bearer.<token>"
// (browser clients cannot set headers on the WebSocket handshake). The
// ?token= query parameter is deliberately NOT supported: tokens in URLs end
// up in access logs.
func (h *Hub) HandleHTTPAuth(ctx context.Context, c *app.RequestContext, cfg *config.Config, cache cache.Cache) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tokenStr, subprotocol := wsToken(c)
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
	h.accept(ctx, c, subprotocol)
}

// wsAuthPrefix marks the Sec-WebSocket-Protocol subprotocol that carries the
// bearer token for browser clients.
const wsAuthPrefix = "bearer."

// wsToken extracts the bearer token from a WebSocket handshake and returns it
// together with the exact subprotocol value that must be echoed back on
// Accept so the browser handshake completes.
//
// Precedence: Authorization: Bearer <token> first, then the first
// "bearer.<token>" entry in Sec-WebSocket-Protocol. Query parameters are
// never inspected — tokens must not ride in URLs.
func wsToken(c *app.RequestContext) (token, subprotocol string) {
	authorization := string(c.GetHeader("Authorization"))
	if len(authorization) > 7 && strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return strings.TrimSpace(authorization[7:]), ""
	}
	for _, sp := range strings.Split(string(c.GetHeader("Sec-WebSocket-Protocol")), ",") {
		sp = strings.TrimSpace(sp)
		token, ok := strings.CutPrefix(strings.ToLower(sp), wsAuthPrefix)
		if ok && token != "" {
			// Echo the client's exact (case-preserved) value so coder/websocket
			// matches and the browser handshake completes.
			return sp[len(wsAuthPrefix):], sp
		}
	}
	return "", ""
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
