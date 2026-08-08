// Package auditlog writes structured audit-trail entries: every mutating
// operation records the actor, IP, user agent, the full before/after snapshot
// and a field-level change diff, so the trail is complete and queryable.
//
// Entries are written best-effort — an audit failure must never break the
// primary operation — so Record returns an error for the caller to log or
// ignore, and never panics.
package auditlog

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

// Entry describes one audit event. Before/After may be nil (create has no
// before, delete has no after); Diff is computed from them when both are set.
type Entry struct {
	Action     string // e.g. "create", "update", "delete", "login", "check", "resolve"
	Status     string // "success" (default) or "failure" — used by login logging
	EntityType string // e.g. "compliance", "user"
	EntityID   uuid.UUID
	ActorID    string // authenticated user id ("" for anonymous attempts)
	IP         string
	UserAgent  string
	Before     any // pre-change snapshot (struct or nil)
	After      any // post-change snapshot (struct or nil)
	Metadata   any // extra context (e.g. username, role); marshaled to JSON
}

// Change is a single field-level before/after pair in the diff.
type Change struct {
	Before any `json:"before"`
	After  any `json:"after"`
}

// systemColumns are never part of the change diff (identity/timestamps).
var systemColumns = map[string]bool{
	"id": true, "created_at": true, "updated_at": true, "deleted_at": true,
}

// sensitiveKeys never reach the audit trail: any value under one of these
// (top-level or nested) is stored as "[REDACTED]" in snapshots, diffs and
// metadata. This is defense in depth — PasswordHash already carries
// json:"-" — so a stray map update can never leak a secret into the logs.
//
// Matching is substring-based on the normalized (lowercased, hyphen->underscore)
// key name, which also catches camelCase variants (accessToken, passwordHash,
// apiKey) without an explicit list. The bare word "key" is deliberately NOT
// matched — too many legitimate fields are named "key" (identifiers, join
// keys) — only qualified forms (api_key, secret_key, private_key, ...) are.
var sensitiveKeys = []string{
	"password", "passwd", // deliberately NOT the bare "pass": it would match bypass/compass
	"token",
	"secret",
	"apikey", "api_key",
	"authorization", "cookie",
	"credential", "private_key", "client_secret",
	"credit_card", "cc_number", "ssn", "pan",
}

func isSensitive(key string) bool {
	norm := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, s := range sensitiveKeys {
		if strings.Contains(norm, s) {
			return true
		}
	}
	return false
}

// redact walks JSON-shaped values and replaces anything under a sensitive key
// with "[REDACTED]". Maps are mutated in place; slices and scalars pass
// through.
func redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if isSensitive(k) {
				t[k] = "[REDACTED]"
			} else {
				t[k] = redact(val)
			}
		}
	case []any:
		for i, item := range t {
			t[i] = redact(item)
		}
	}
	return v
}

// Record writes the audit entry to the database. The before/after snapshots
// are stored as JSON, and Changes holds the field-level diff.
// maxColumnSizes mirror the model's gorm size tags; values are truncated so a
// hostile or unusual input (a 300-char user agent, a 200-char attempted
// username) can never fail the audit insert.
const (
	maxActorID   = 64
	maxUserAgent = 255
)

func Record(ctx context.Context, db *gorm.DB, e Entry) error {
	if e.Action == "" {
		e.Action = "unknown"
	}
	if e.Status == "" {
		e.Status = "success"
	}
	if len(e.ActorID) > maxActorID {
		e.ActorID = e.ActorID[:maxActorID]
	}
	if len(e.UserAgent) > maxUserAgent {
		e.UserAgent = e.UserAgent[:maxUserAgent]
	}
	row := models.AuditLog{
		Action:     e.Action,
		Status:     e.Status,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		ActorID:    e.ActorID,
		IP:         e.IP,
		UserAgent:  e.UserAgent,
	}
	row.BeforeData = marshalJSON(e.Before)
	row.AfterData = marshalJSON(e.After)
	row.Changes = marshalJSON(Diff(e.Before, e.After))
	row.Metadata = marshalJSON(e.Metadata)

	if err := db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	return nil
}

// FromRequest extracts the actor, IP and user agent from a Hertz request
// context (values set by the auth middleware).
func FromRequest(c *app.RequestContext) (actorID, ip, userAgent string) {
	if v := c.GetString("auth_user_id"); v != "" {
		actorID = v
	}
	return actorID, c.ClientIP(), string(c.GetHeader("User-Agent"))
}

// Diff compares two snapshots (structs, maps, pointers, or nil) and returns
// the changed fields with their before/after values. Identity and timestamp
// columns are excluded. Both snapshots are normalized through JSON so jsonb
// values (maps, slices) compare by value, not by pointer.
func Diff(before, after any) map[string]Change {
	beforeMap := toMap(before)
	afterMap := toMap(after)

	changes := make(map[string]Change)
	for key := range union(beforeMap, afterMap) {
		if systemColumns[key] {
			continue
		}
		old, hasOld := beforeMap[key]
		new, hasNew := afterMap[key]
		if hasOld && hasNew && reflect.DeepEqual(old, new) {
			continue
		}
		if !hasOld {
			old = nil
		}
		if !hasNew {
			new = nil
		}
		// Skip fields that were null on both sides (e.g. an unset due_date in
		// a create) — they add noise without information.
		if old == nil && new == nil {
			continue
		}
		changes[key] = Change{Before: old, After: new}
	}
	return changes
}

// toMap normalizes any value to a JSON-shaped map (nil -> empty map) with
// sensitive keys redacted, so the field-level diff can never contain secrets.
func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{}
	}
	return redact(m).(map[string]any)
}

func union(a, b map[string]any) map[string]struct{} {
	out := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		out[k] = struct{}{}
	}
	for k := range b {
		out[k] = struct{}{}
	}
	return out
}

// marshalJSON serializes a snapshot/metadata value, routing it through the
// same redaction as the diff so before/after JSON and metadata are scrubbed
// before persistence.
func marshalJSON(v any) datatypes.JSON {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var anyVal any
	if err := json.Unmarshal(b, &anyVal); err != nil {
		return datatypes.JSON(b)
	}
	out, err := json.Marshal(redact(anyVal))
	if err != nil {
		return datatypes.JSON(b)
	}
	return datatypes.JSON(out)
}
