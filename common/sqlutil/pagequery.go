package sqlutil

import (
	"fmt"
	"strings"
)

type PageQueryBuilder struct {
	parts []string
	args  []any
}

func NewPageQueryBuilder() *PageQueryBuilder {
	return &PageQueryBuilder{
		parts: []string{"1=1"},
		args:  make([]any, 0, 8),
	}
}

func (b *PageQueryBuilder) EqInt64(column string, value int64) {
	if value == 0 {
		return
	}
	b.parts = append(b.parts, fmt.Sprintf("%s = ?", column))
	b.args = append(b.args, value)
}

func (b *PageQueryBuilder) EqString(column string, value string) {
	if value == "" {
		return
	}
	b.parts = append(b.parts, fmt.Sprintf("%s = ?", column))
	b.args = append(b.args, value)
}

func (b *PageQueryBuilder) LikeString(column string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.parts = append(b.parts, fmt.Sprintf("%s LIKE ?", column))
	b.args = append(b.args, "%"+value+"%")
}

func (b *PageQueryBuilder) GteInt64(column string, value int64) {
	if value == 0 {
		return
	}
	b.parts = append(b.parts, fmt.Sprintf("%s >= ?", column))
	b.args = append(b.args, value)
}

func (b *PageQueryBuilder) LteInt64(column string, value int64) {
	if value == 0 {
		return
	}
	b.parts = append(b.parts, fmt.Sprintf("%s <= ?", column))
	b.args = append(b.args, value)
}

func (b *PageQueryBuilder) InInt64(column string, values []int64) {
	if len(values) == 0 {
		return
	}
	holders := make([]string, 0, len(values))
	for _, value := range values {
		holders = append(holders, "?")
		b.args = append(b.args, value)
	}
	b.parts = append(b.parts, fmt.Sprintf("%s IN (%s)", column, strings.Join(holders, ",")))
}

func (b *PageQueryBuilder) And(clause string, args ...any) {
	if clause == "" {
		return
	}
	b.parts = append(b.parts, clause)
	b.args = append(b.args, args...)
}

func (b *PageQueryBuilder) Or(clauses []string, args ...any) {
	filtered := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		if clause == "" {
			continue
		}
		filtered = append(filtered, clause)
	}
	if len(filtered) == 0 {
		return
	}
	if len(filtered) == 1 {
		b.parts = append(b.parts, filtered[0])
	} else {
		b.parts = append(b.parts, fmt.Sprintf("(%s)", strings.Join(filtered, " OR ")))
	}
	b.args = append(b.args, args...)
}

func (b *PageQueryBuilder) Where() string {
	return strings.Join(b.parts, " AND ")
}

func (b *PageQueryBuilder) Args() []any {
	return append([]any{}, b.args...)
}

func NormalizeLimit(limit int64) int64 {
	if limit <= 0 {
		return 10
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// KnownCount returns the first positive total supplied by a pagination client.
// A non-positive result means the caller must calculate an exact count.
func KnownCount(counts ...int64) int64 {
	if len(counts) == 0 || counts[0] <= 0 {
		return 0
	}
	return counts[0]
}
