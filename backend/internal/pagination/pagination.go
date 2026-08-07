// Package pagination implements server-side cursor (keyset) pagination for
// every list and report endpoint: opaque cursors, dynamic allowlisted column
// sorting, equality + IN filters, date-range filters and per-resource summary
// aggregates. All queries are built with structured GORM calls — no raw SQL.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Cursor is the keyset marker: the sort position (value of the sort column +
// id) of the last row of the previous page. It is opaque to clients. SortValue
// is the JSON-encoded value of the sort column so pagination works regardless
// of which column the client sorts by.
type Cursor struct {
	SortColumn string    `json:"sort_column"`
	SortValue  any       `json:"sort_value"`
	ID         uuid.UUID `json:"id"`
}

// Encode serializes the cursor for the client.
func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Decode parses a client-provided cursor.
func Decode(raw string) (*Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}
	return &c, nil
}

// Sort specifies the ordering of a page.
type Sort struct {
	Column    string `json:"column"`    // logical column name (allowlisted)
	Direction string `json:"direction"` // "asc" | "desc"
}

// DateRange filters rows by a timestamp column.
type DateRange struct {
	Field string     `json:"field"` // logical column, defaults to created_at
	From  *time.Time `json:"from"`
	To    *time.Time `json:"to"`
}

// Query is the body of every list/report request.
type Query struct {
	Cursor         string         `json:"cursor"`
	Limit          int            `json:"limit"`
	Sort           *Sort          `json:"sort"`
	Filters        map[string]any `json:"filters"`
	DateRange      *DateRange     `json:"date_range"`
	IncludeSummary bool           `json:"include_summary"`
}

// Summary is the pagination block returned with every page.
type Summary struct {
	Count      int    `json:"count"`                 // rows in this page
	Limit      int    `json:"limit"`                 // max rows requested
	HasMore    bool   `json:"has_more"`              // another page exists
	Cursor     string `json:"cursor,omitempty"`      // cursor used for this page
	NextCursor string `json:"next_cursor,omitempty"` // pass this to fetch the next page
	DBSummary  any    `json:"summary,omitempty"`     // aggregate data (when include_summary)
}

// Result is the standard paginated envelope.
type Result[T any] struct {
	Items      []T     `json:"items"`
	Pagination Summary `json:"pagination"`
}

// Apply executes a cursor-paginated search on a model-scoped *gorm.DB.
// columns maps logical (client-facing) column names to safe DB columns for
// sorting, equality filters and the date range. summaryBy is a logical column
// whose distinct values get a grouped count breakdown in the summary.
func Apply[T any](db *gorm.DB, q Query, columns map[string]string, summaryBy string) (*Result[T], error) {
	cols := defaultColumns()
	for k, v := range columns {
		cols[k] = v
	}

	limit := q.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// --- ordering (dynamic but allowlisted) ---
	sortCol, sortDesc := cols["created_at"], true
	if q.Sort != nil && q.Sort.Column != "" {
		col, ok := cols[q.Sort.Column]
		if !ok {
			return nil, fmt.Errorf("invalid sort column %q", q.Sort.Column)
		}
		sortCol = col
	}
	if q.Sort != nil {
		switch q.Sort.Direction {
		case "asc":
			sortDesc = false
		case "desc":
			sortDesc = true
		case "":
		default:
			return nil, fmt.Errorf("invalid sort direction %q", q.Sort.Direction)
		}
	}

	// --- equality / IN filters ---
	filtered := db.Session(&gorm.Session{})
	for key, val := range q.Filters {
		col, ok := cols[key]
		if !ok {
			return nil, fmt.Errorf("invalid filter column %q", key)
		}
		switch v := val.(type) {
		case []any:
			filtered = filtered.Where(col+" IN ?", v)
		default:
			filtered = filtered.Where(col+" = ?", val)
		}
	}

	// --- date range filter ---
	if q.DateRange != nil {
		rangeCol := cols["created_at"]
		if q.DateRange.Field != "" {
			col, ok := cols[q.DateRange.Field]
			if !ok {
				return nil, fmt.Errorf("invalid date range column %q", q.DateRange.Field)
			}
			rangeCol = col
		}
		if q.DateRange.From != nil {
			filtered = filtered.Where(rangeCol+" >= ?", *q.DateRange.From)
		}
		if q.DateRange.To != nil {
			filtered = filtered.Where(rangeCol+" <= ?", *q.DateRange.To)
		}
	}

	// The summary reflects ALL rows matching the filters (never the cursor
	// window) so totals are consistent across pages.
	var summary any
	if q.IncludeSummary {
		s, err := buildSummary(filtered.Session(&gorm.Session{}), cols, summaryBy)
		if err != nil {
			return nil, err
		}
		summary = s
	}

	// --- keyset cursor (typed to the current sort column) ---
	sortField := sortSchemaField[T](db, sortCol)
	if q.Cursor != "" {
		cur, err := Decode(q.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		if cur.SortColumn != "" && cur.SortColumn != sortCol {
			return nil, fmt.Errorf("cursor was created with sort column %q, current sort is %q", cur.SortColumn, sortCol)
		}
		sortVal, err := coerceSortValue(sortField, cur.SortValue)
		if err != nil {
			return nil, err
		}
		op := "<"
		if !sortDesc {
			op = ">"
		}
		filtered = filtered.Where(fmt.Sprintf("(%s, id) %s (?, ?)", sortCol, op), sortVal, cur.ID)
	}

	// --- fetch limit+1 rows so has_more can be derived ---
	orderDir := "DESC"
	if !sortDesc {
		orderDir = "ASC"
	}
	fetch := filtered.Session(&gorm.Session{}).
		Order(sortCol + " " + orderDir).
		Order("id " + orderDir).
		Limit(limit + 1)

	var items []T
	if err := fetch.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	res := Result[T]{
		Items: items,
		Pagination: Summary{
			Count:   len(items),
			Limit:   limit,
			HasMore: hasMore,
			Cursor:  q.Cursor,
		},
	}
	if hasMore && len(items) > 0 {
		if cur, ok := cursorFor(items[len(items)-1], sortField); ok {
			res.Pagination.NextCursor = cur.Encode()
		}
	}
	if q.IncludeSummary {
		res.Pagination.DBSummary = summary
	}
	return &res, nil
}

// defaultColumns are always safe to sort/filter on.
func defaultColumns() map[string]string {
	return map[string]string{
		"id":         "id",
		"created_at": "created_at",
		"updated_at": "updated_at",
		"status":     "status",
	}
}

// sortSchemaField resolves the sort column to its GORM schema field so cursor
// values can be (de)serialized with the right Go type. db.Statement.Schema is
// only populated after the first query on a session, so the schema is parsed
// explicitly from a throwaway clone (side-effect free).
func sortSchemaField[T any](db *gorm.DB, column string) *schema.Field {
	var zero T
	stmt := db.Session(&gorm.Session{})
	if err := stmt.Statement.Parse(&zero); err != nil || stmt.Statement.Schema == nil {
		return nil
	}
	for _, f := range stmt.Statement.Schema.Fields {
		if f.DBName == column {
			return f
		}
	}
	return nil
}

// cursorFor reads the keyset marker from a model row: the value of the sort
// column plus the row id (via reflection on the GORM field name).
func cursorFor(item any, sortField *schema.Field) (Cursor, bool) {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	idField := v.FieldByName("ID")
	if !idField.IsValid() {
		return Cursor{}, false
	}
	id, ok := idField.Interface().(uuid.UUID)
	if !ok {
		return Cursor{}, false
	}

	column := "created_at"
	sortValue := any(v.FieldByName("CreatedAt").Interface())
	if sortField != nil {
		f := v.FieldByName(sortField.Name)
		if f.IsValid() && f.CanInterface() {
			column = sortField.DBName
			sortValue = f.Interface()
		}
	}
	return Cursor{SortColumn: column, SortValue: sortValue, ID: id}, true
}

// coerceSortValue converts a decoded cursor value (JSON types: string, float64,
// bool) back into the sort column's Go type so the keyset predicate is well
// typed for the database driver (e.g. time.Time for timestamptz columns).
func coerceSortValue(f *schema.Field, raw any) (any, error) {
	if raw == nil {
		return nil, fmt.Errorf("cursor sort value is null; sort by a non-null column instead")
	}
	if f == nil {
		return raw, nil
	}
	switch f.DataType {
	case schema.Time:
		switch v := raw.(type) {
		case string:
			return time.Parse(time.RFC3339Nano, v)
		case time.Time:
			return v, nil
		}
		return nil, fmt.Errorf("invalid cursor time value %v", raw)
	case schema.Int:
		if v, ok := raw.(float64); ok {
			return int64(v), nil
		}
		return raw, nil
	case schema.Uint:
		if v, ok := raw.(float64); ok {
			return uint64(v), nil
		}
		return raw, nil
	case schema.Bool:
		if v, ok := raw.(bool); ok {
			return v, nil
		}
		return raw, nil
	default:
		return raw, nil
	}
}

// buildSummary returns the total filtered count plus a grouped breakdown of
// distinct values on the summary column.
func buildSummary(db *gorm.DB, cols map[string]string, summaryBy string) (any, error) {
	var total int64
	if err := db.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("summary total: %w", err)
	}
	out := map[string]any{"total": total}
	if summaryBy == "" {
		return out, nil
	}

	col, ok := cols[summaryBy]
	if !ok {
		return nil, fmt.Errorf("invalid summary column %q", summaryBy)
	}

	var values []string
	if err := db.Session(&gorm.Session{}).Distinct(col).Where(col+" IS NOT NULL").Pluck(col, &values).Error; err != nil {
		return nil, fmt.Errorf("summary distinct: %w", err)
	}
	grouped := make(map[string]int64, len(values))
	for _, v := range values {
		var n int64
		if err := db.Session(&gorm.Session{}).Where(col+" = ?", v).Count(&n).Error; err != nil {
			return nil, fmt.Errorf("summary count %q: %w", v, err)
		}
		grouped[v] = n
	}
	out[summaryBy] = grouped
	return out, nil
}
