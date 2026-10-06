package query

import (
	"cmp"

	"github.com/sglavoie/tt/internal/models"
)

// SortState bundles a sort field and direction. Embedded by views that support
// the High/Medium/Low/Unset sort system.
type SortState struct {
	Field     SortField
	Ascending bool
}

// NewSortState returns a SortState with default values (no sort, ascending).
func NewSortState() SortState {
	return SortState{Field: SortByDefault, Ascending: true}
}

// Reset restores the sort state to defaults.
func (s *SortState) Reset() {
	s.Field = SortByDefault
	s.Ascending = true
}

// Toggle switches to the given field. If already on that field, the direction
// is flipped; otherwise it switches to the field in ascending order.
func (s *SortState) Toggle(f SortField) {
	if s.Field == f {
		s.Ascending = !s.Ascending
	} else {
		s.Field = f
		s.Ascending = true
	}
}

// CompareTasks compares two tasks based on the given sort field.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// Ties on the primary key are broken by task.ID (lexicographic) to keep
// the ordering deterministic and to satisfy strict weak ordering when
// the result is fed through sort.Slice.
func CompareTasks(a, b *models.Task, field SortField) int {
	primary := compareTasksPrimary(a, b, field)
	if primary != 0 {
		return primary
	}
	return cmp.Compare(a.ID, b.ID)
}

func compareTasksPrimary(a, b *models.Task, field SortField) int {
	return models.CompareTasksBySortField(a, b, querySortFieldFor(field))
}

// querySortFieldFor names the canonical sort field a view sort key stands for.
func querySortFieldFor(field SortField) models.QuerySortField {
	switch field {
	case SortByPriority:
		return models.QuerySortPriority
	case SortByDueDate:
		return models.QuerySortDue
	case SortByCreatedAt:
		return models.QuerySortCreated
	case SortByStatus:
		return models.QuerySortStatus
	case SortByUpdatedAt:
		return models.QuerySortUpdated
	case SortByTitle:
		return models.QuerySortTitle
	default:
		return models.QuerySortDefault
	}
}

// ApplySortDirection converts a three-way compare result into a Less
// boolean honoring the sort direction. Equal items (c == 0) return false
// in both directions so that the resulting predicate satisfies strict
// weak ordering — required by sort.Slice/sort.Stable.
func ApplySortDirection(c int, ascending bool) bool {
	if ascending {
		return c < 0
	}
	return c > 0
}
