package models

import (
	"cmp"
	"strings"
)

// WorkflowStateSortOrder is the status sort's one ladder: active lifecycle
// states lead, followed by deferred and terminal work.
func WorkflowStateSortOrder(state WorkflowState) int {
	switch state {
	case StateInbox:
		return 0
	case StateBacklog:
		return 1
	case StateDoing:
		return 2
	case StatePaused:
		return 3
	case StateSomeday:
		return 4
	case StateWaiting:
		return 5
	case StateDone:
		return 6
	case StateCanceled:
		return 7
	default:
		return 8
	}
}

// CompareTasksBySortField compares two tasks on one named sort field, ascending,
// and reports zero for a tie or the default order: the caller owns direction
// and every tie-break. An undated task sorts after a dated one.
func CompareTasksBySortField(a, b *Task, field QuerySortField) int {
	switch field {
	case QuerySortPriority:
		return cmp.Compare(EffectivePriority(a.Priority), EffectivePriority(b.Priority))
	case QuerySortDue:
		switch {
		case a.DueDate == nil && b.DueDate == nil:
			return 0
		case a.DueDate == nil:
			return 1
		case b.DueDate == nil:
			return -1
		}
		return a.DueDate.Compare(*b.DueDate)
	case QuerySortCreated:
		return a.CreatedAt.Compare(b.CreatedAt)
	case QuerySortStatus:
		return cmp.Compare(WorkflowStateSortOrder(a.State()), WorkflowStateSortOrder(b.State()))
	case QuerySortUpdated:
		return a.UpdatedAt.Compare(b.UpdatedAt)
	case QuerySortTitle:
		return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	default:
		return 0
	}
}
