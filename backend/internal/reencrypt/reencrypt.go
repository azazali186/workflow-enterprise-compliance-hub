// Package reencrypt migrates encrypted-at-rest values to the current
// encryption key after an ENCRYPTION_KEY rotation.
//
// Rows whose ciphertext already carries the current key's fingerprint are
// skipped, so the migration is idempotent and safe to run on every boot.
// While it runs — and even if it fails part-way — reads keep working because
// internal/crypto dual-reads the current and previous keys. The previous key
// can be retired from ENCRYPTION_KEY_PREVIOUS only after this job reports no
// rows left to migrate.
//
// All persistence is GORM-driven; no raw SQL.
package reencrypt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/crypto"
	"github.com/aeroxe/compliance-hub/backend/internal/lock"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

// defaultBatchSize bounds how many rows are rewritten per batch statement.
const defaultBatchSize = 500

// Migrator rewrites old-key ciphertext to the current key.
type Migrator struct {
	db        *gorm.DB
	batchSize int
}

// New creates a migrator for the given database.
func New(db *gorm.DB) *Migrator { return &Migrator{db: db, batchSize: defaultBatchSize} }

// ColumnSpec identifies one encrypted column to migrate.
type ColumnSpec struct {
	Model  any    // pointer to the entity struct (provides the table name)
	Column string // snake_case DB column holding encrypted text
}

// Run migrates the outbox payloads and every given Secret column under a
// distributed lock, so concurrent replicas never race the same rows. Returns
// the total number of rows re-encrypted. When another replica holds the lock
// the call is a no-op.
//
// The lock TTL (10 minutes) must comfortably exceed the whole migration —
// batches are small and keyed-paginated, so even large tables finish quickly;
// if it ever expired mid-run a second replica would simply rewrite already
// valid rows (idempotent, different nonce).
func (m *Migrator) Run(ctx context.Context, l lock.Lock, columns ...ColumnSpec) (int64, error) {
	var total int64
	err := lock.WithLock(ctx, l, "reencrypt:worker", 10*time.Minute, func() error {
		n, err := m.MigrateOutbox(ctx)
		if err != nil {
			return err
		}
		total += n
		for _, c := range columns {
			n, err := m.MigrateColumn(ctx, c.Model, c.Column)
			if err != nil {
				return err
			}
			total += n
		}
		return nil
	})
	if errors.Is(err, lock.ErrLocked) {
		return 0, nil // another replica is already migrating
	}
	return total, err
}

// MigrateOutbox re-encrypts every outbox payload still carrying a previous
// key. Payloads are stored as JSON-quoted ciphertext strings in a jsonb
// column; CAST(... AS TEXT) keeps the LIKE filter working on Postgres and
// sqlite alike.
//
// Iteration is keyset-paginated by id (not OFFSET) so rows rewritten in place
// are neither skipped nor revisited within a run. A single poisoned row (key
// no longer in the ring) is logged and skipped, never allowed to block the
// rest — mirroring the outbox dispatcher's dead-lettering.
func (m *Migrator) MigrateOutbox(ctx context.Context) (int64, error) {
	var migrated int64
	var lastID string
	for {
		var rows []models.OutboxEvent
		q := m.db.WithContext(ctx).
			Where("CAST(payload AS TEXT) LIKE ?", "\"enc:%") // JSON string starts with "enc:
		if lastID != "" {
			q = q.Where("id > ?", lastID)
		}
		if err := q.Order("id").Limit(m.batchSize).Find(&rows).Error; err != nil {
			return migrated, err
		}
		if len(rows) == 0 {
			return migrated, nil
		}
		for i := range rows {
			row := &rows[i]
			newPayload, changed, err := m.reencryptJSONPayload(row.Payload)
			if err != nil {
				slog.Warn("reencrypt: skipping outbox row", "event_id", row.ID, "error", err)
				continue
			}
			if !changed {
				continue
			}
			if err := m.db.WithContext(ctx).Model(row).Update("payload", newPayload).Error; err != nil {
				return migrated, fmt.Errorf("outbox row %s: %w", row.ID, err)
			}
			migrated++
		}
		lastID = rows[len(rows)-1].ID.String()
		if len(rows) < m.batchSize {
			return migrated, nil
		}
	}
}

// MigrateColumn re-encrypts a single encrypted column of an entity (a
// models.Secret column, which stores the raw ciphertext string) to the
// current key.
//
// The column is read with Pluck (never scanned into model instances) so the
// raw stored bytes are retrieved without routing through the column type's
// Scanner, which would decrypt/re-encrypt on read and defeat the migration.
func (m *Migrator) MigrateColumn(ctx context.Context, model any, column string) (int64, error) {
	if model == nil || column == "" {
		return 0, errors.New("reencrypt: model and column are required")
	}

	var migrated int64
	var lastID string
	for {
		q := m.db.WithContext(ctx).
			Model(model).
			Where(column+" LIKE ?", "enc:%")
		if lastID != "" {
			q = q.Where("id > ?", lastID)
		}
		var ids []string
		if err := q.Order("id").Limit(m.batchSize).Pluck("id", &ids).Error; err != nil {
			return migrated, err
		}
		if len(ids) == 0 {
			return migrated, nil
		}

		// Values are fetched in the same id order so ids[i] pairs with
		// values[i].
		var values []string
		if err := m.db.WithContext(ctx).
			Model(model).
			Where("id IN ?", ids).
			Order("id").
			Pluck(column, &values).Error; err != nil {
			return migrated, err
		}
		for i, val := range values {
			newVal, changed, err := reencryptString(val)
			if err != nil {
				slog.Warn("reencrypt: skipping column row", "column", column, "id", ids[i], "error", err)
				continue
			}
			if !changed {
				continue
			}
			if err := m.db.WithContext(ctx).
				Model(model).
				Where("id = ?", ids[i]).
				Update(column, newVal).Error; err != nil {
				return migrated, fmt.Errorf("update %s: %w", column, err)
			}
			migrated++
		}

		lastID = ids[len(ids)-1]
		if len(ids) < m.batchSize {
			return migrated, nil
		}
	}
}

// reencryptJSONPayload unwraps the JSON-quoted ciphertext string stored in the
// outbox jsonb column and re-encrypts it under the current key when it used an
// older key. Plaintext (unencrypted legacy) payloads are left untouched.
func (m *Migrator) reencryptJSONPayload(raw datatypes.JSON) (datatypes.JSON, bool, error) {
	var enc string
	if err := json.Unmarshal(raw, &enc); err != nil {
		return raw, false, nil // not a JSON string (legacy plaintext object)
	}
	newEnc, changed, err := reencryptString(enc)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		return raw, false, nil
	}
	b, err := json.Marshal(newEnc)
	if err != nil {
		return nil, false, err
	}
	return datatypes.JSON(b), true, nil
}

// reencryptString rewrites a stored ciphertext string to the current key when
// it was encrypted with a previous key. Plaintext and already-current values
// are returned unchanged with changed=false.
func reencryptString(enc string) (string, bool, error) {
	if !strings.HasPrefix(enc, "enc:") {
		return enc, false, nil
	}
	if crypto.UsesCurrentKey(enc) {
		return enc, false, nil
	}
	plain, err := crypto.DecryptString(enc)
	if err != nil {
		return "", false, err
	}
	newEnc, err := crypto.EncryptString(plain)
	if err != nil {
		return "", false, err
	}
	return newEnc, true, nil
}
