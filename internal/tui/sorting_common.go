package tui

import (
	"cmp"
	"strings"

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

// StateOrder returns the sort order for a workflow state (workflow order).
// This is the single source of truth used by all views that sort by
// status/state. Active lifecycle states lead, followed by deferred and terminal work.
func StateOrder(state models.WorkflowState) int {
	switch state {
	case models.StateInbox:
		return 0
	case models.StateBacklog:
		return 1
	case models.StateDoing:
		return 2
	case models.StatePaused:
		return 3
	case models.StateSomeday:
		return 4
	case models.StateWaiting:
		return 5
	case models.StateDone:
		return 6
	case models.StateCanceled:
		return 7
	default:
		return 8
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
	switch field {
	case SortByPriority:
		return cmp.Compare(models.EffectivePriority(a.Priority), models.EffectivePriority(b.Priority))

	case SortByDueDate:
		if a.DueDate == nil && b.DueDate == nil {
			return 0
		}
		if a.DueDate == nil {
			return 1 // nil goes after
		}
		if b.DueDate == nil {
			return -1 // nil goes after
		}
		return a.DueDate.Compare(*b.DueDate)

	case SortByCreatedAt:
		return a.CreatedAt.Compare(b.CreatedAt)

	case SortByStatus:
		return cmp.Compare(StateOrder(a.State()), StateOrder(b.State()))

	case SortByUpdatedAt:
		return a.UpdatedAt.Compare(b.UpdatedAt)

	case SortByTitle:
		return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))

	default:
	}

	return 0
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
