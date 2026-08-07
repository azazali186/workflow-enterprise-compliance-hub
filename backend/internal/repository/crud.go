// Package repository provides a generic, cache-aware CRUD layer built on GORM.
// Every persistence operation in the application flows through this package
// or the exposed *gorm.DB scopes — no raw SQL is written anywhere.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/aeroxe/compliance-hub/backend/internal/cache"
	"github.com/aeroxe/compliance-hub/backend/internal/models"
)

// ErrNotFound is returned when a record does not exist (or is soft deleted).
var ErrNotFound = errors.New("record not found")

// Repository[T] is a typed CRUD repository for any GORM model.
type Repository[T models.Entity] struct {
	db    *gorm.DB
	cache cache.Cache
	slug  string // cache key segment, e.g. "compliance"
}

// New creates a repository. The slug becomes part of cache keys
// (`cache:<slug>:<id>`).
func New[T models.Entity](db *gorm.DB, c cache.Cache, slug string) *Repository[T] {
	return &Repository[T]{db: db, cache: c, slug: slug}
}

// DB returns the underlying GORM handle for structured queries
// (Where/Group/Count chains) when callers need more than the standard CRUD.
func (r *Repository[T]) DB() *gorm.DB { return r.db }

// Scope returns a Model-scoped *gorm.DB for query chaining.
func (r *Repository[T]) Scope() *gorm.DB {
	var zero T
	return r.db.Model(&zero)
}

// Create persists a new entity and invalidates the cache.
func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("create %s: %w", r.slug, err)
	}
	r.invalidate(ctx, (*entity).GetID())
	return nil
}

// GetByID reads an entity, serving from cache when available
// (README: `cache:<slug>:<id>` with 15m TTL).
func (r *Repository[T]) GetByID(ctx context.Context, id uuid.UUID) (*T, error) {
	var zero T
	key := cache.EntityKey(r.slug, id.String())
	if raw, ok := r.cache.Get(ctx, key); ok {
		var cached T
		if err := json.Unmarshal([]byte(raw), &cached); err == nil {
			return &cached, nil
		}
	}

	entity := zero
	err := r.db.WithContext(ctx).First(&entity, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", r.slug, err)
	}

	if b, err := json.Marshal(entity); err == nil {
		_ = r.cache.Set(ctx, key, string(b), cache.TTL)
	}
	return &entity, nil
}

// List returns a page of entities plus the total count.
// page and pageSize are 1-based; sort accepts "created_at" / "-created_at"
// style column names (a leading "-" means descending). Only an explicit
// allowlist of sort keys is accepted to keep ordering predictable.
func (r *Repository[T]) List(ctx context.Context, page, pageSize int, sort string) ([]T, int64, error) {
	var items []T
	var total int64

	q := r.Scope()
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count %s: %w", r.slug, err)
	}
	if err := q.Scopes(Paginate(page, pageSize), Sort(sort)).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list %s: %w", r.slug, err)
	}
	return items, total, nil
}

// Update persists full-entity changes and invalidates the cache.
func (r *Repository[T]) Update(ctx context.Context, entity *T) error {
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("update %s: %w", r.slug, err)
	}
	r.invalidate(ctx, (*entity).GetID())
	return nil
}

// UpdatePartial applies a partial update (map of columns) to a single record.
// This is the GORM idiomatic equivalent of an UPDATE ... SET.
func (r *Repository[T]) UpdatePartial(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	var zero T
	res := r.db.WithContext(ctx).Model(&zero).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("update %s: %w", r.slug, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	r.invalidate(ctx, id)
	return nil
}

// Delete soft-deletes an entity (GORM sets deleted_at; rows stay in the table).
func (r *Repository[T]) Delete(ctx context.Context, id uuid.UUID) error {
	var zero T
	res := r.db.WithContext(ctx).Delete(&zero, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("delete %s: %w", r.slug, res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	r.invalidate(ctx, id)
	return nil
}

func (r *Repository[T]) invalidate(ctx context.Context, id uuid.UUID) {
	_ = r.cache.Del(ctx, cache.EntityKey(r.slug, id.String()))
}

// Paginate is a GORM scope applying page/pageSize with sane bounds.
func Paginate(page, pageSize int) func(*gorm.DB) *gorm.DB {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return func(db *gorm.DB) *gorm.DB {
		return db.Limit(pageSize).Offset(offset)
	}
}

// Sort is a GORM scope applying an allowlisted ORDER BY.
func Sort(sort string) func(*gorm.DB) *gorm.DB {
	column, desc := parseSort(sort)
	if column == "" {
		return func(db *gorm.DB) *gorm.DB { return db.Order("created_at DESC") }
	}
	expr := column
	if desc {
		expr += " DESC"
	}
	return func(db *gorm.DB) *gorm.DB { return db.Order(expr) }
}

// parseSort only accepts known column names to avoid injecting arbitrary
// expressions into ORDER BY.
func parseSort(sort string) (column string, desc bool) {
	switch sort {
	case "", "created_at", "-created_at":
		return "created_at", sort == "-created_at"
	case "updated_at", "-updated_at":
		return "updated_at", sort == "-updated_at"
	case "status", "-status":
		return "status", sort == "-status"
	case "due_date", "-due_date":
		return "due_date", sort == "-due_date"
	case "deadline_at", "-deadline_at":
		return "deadline_at", sort == "-deadline_at"
	}
	return "", false
}

// Now is exported so background jobs and handlers share one clock source.
var Now = func() time.Time { return time.Now().UTC() }
