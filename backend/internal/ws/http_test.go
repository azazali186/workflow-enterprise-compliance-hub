package ws

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestWsTokenFromAuthorizationHeader(t *testing.T) {
	c := &app.RequestContext{}
	c.Request.Header.Set("Authorization", "Bearer abc.def.ghi")

	token, sub := wsToken(c)
	if token != "abc.def.ghi" || sub != "" {
		t.Fatalf("token=%q sub=%q, want token abc.def.ghi and no subprotocol", token, sub)
	}
}

func TestWsTokenFromSubprotocol(t *testing.T) {
	c := &app.RequestContext{}
	c.Request.Header.Set("Sec-WebSocket-Protocol", "bearer.abc.def.ghi")

	token, sub := wsToken(c)
	if token != "abc.def.ghi" || sub != "bearer.abc.def.ghi" {
		t.Fatalf("token=%q sub=%q, want token abc.def.ghi echoed subprotocol", token, sub)
	}
}

func TestWsTokenSubprotocolListAndCase(t *testing.T) {
	c := &app.RequestContext{}
	// Multiple offered protocols, uppercase prefix: the bearer. entry is found.
	c.Request.Header.Set("Sec-WebSocket-Protocol", "graphql-ws, BEARER.xyz.jwt")

	token, sub := wsToken(c)
	if token != "xyz.jwt" || sub != "BEARER.xyz.jwt" {
		t.Fatalf("token=%q sub=%q, want xyz.jwt", token, sub)
	}
}

func TestWsTokenQueryParamIgnored(t *testing.T) {
	c := &app.RequestContext{}
	// A stale client appending ?token= must be ignored — no JWT in URLs.
	c.Request.URI().SetQueryString("token=evil-token")

	token, sub := wsToken(c)
	if token != "" || sub != "" {
		t.Fatalf("token=%q sub=%q: query parameter must be ignored", token, sub)
	}
}

func TestWsTokenAuthorizationWinsOverSubprotocol(t *testing.T) {
	c := &app.RequestContext{}
	c.Request.Header.Set("Authorization", "Bearer header-token")
	c.Request.Header.Set("Sec-WebSocket-Protocol", "bearer.sub-token")

	token, sub := wsToken(c)
	if token != "header-token" || sub != "" {
		t.Fatalf("token=%q sub=%q, want Authorization header to win", token, sub)
	}
}

func TestWsTokenMissing(t *testing.T) {
	c := &app.RequestContext{}
	token, sub := wsToken(c)
	if token != "" || sub != "" {
		t.Fatalf("token=%q sub=%q, want both empty", token, sub)
	}
}
