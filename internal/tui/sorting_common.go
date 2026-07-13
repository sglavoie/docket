package tui

import (
	"cmp"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// SortState bundles a sort field and direction. Embedded by views that support
// the 1-5/0 sort key system.
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

// SortFieldForKey maps a keyboard key to its corresponding SortField.
// Returns the field, whether this is a reset (key "0"), and whether the key matched.
func SortFieldForKey(key string) (SortField, bool, bool) {
	switch key {
	case "1":
		return SortByPriority, false, true
	case "2":
		return SortByDueDate, false, true
	case "3":
		return SortByCreatedAt, false, true
	case "4":
		return SortByStatus, false, true
	case "5":
		return SortByScore, false, true
	case "0":
		return SortByDefault, true, true
	}
	return SortByDefault, false, false
}

// StateOrder returns the sort order for a workflow state (workflow order).
// This is the single source of truth used by all views that sort by
// status/state. It mirrors the legacy StatusOrder with the three todo-ish
// planning states slotted where todo sat (next, then inbox, then ready),
// waiting where blocked sat, paused where standby sat, and someday where
// maybe sat.
func StateOrder(state models.WorkflowState) int {
	switch state {
	case models.StateNext:
		return 0
	case models.StateInbox:
		return 1
	case models.StateReady:
		return 2
	case models.StateInProgress:
		return 3
	case models.StateReview:
		return 4
	case models.StatePaused:
		return 5
	case models.StateSomeday:
		return 6
	case models.StateWaiting:
		return 7
	case models.StateDone:
		return 8
	case models.StateCanceled:
		return 9
	default:
		return 10
	}
}

// TodayStateOrder ranks workflow states by activeness for the Today tab.
// In-progress first, review pushed below other active states, terminal last.
// The three todo-ish planning states share one rank so the daily-plan position
// keeps ordering them.
func TodayStateOrder(state models.WorkflowState) int {
	switch state {
	case models.StateInProgress:
		return 0
	case models.StateInbox, models.StateReady, models.StateNext:
		return 1
	case models.StateWaiting:
		return 2
	case models.StatePaused:
		return 3
	case models.StateSomeday:
		return 4
	case models.StateReview:
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
// The projectID and cfg are needed for score-based comparison.
// Ties on the primary key are broken by task.ID (lexicographic) to keep
// the ordering deterministic and to satisfy strict weak ordering when
// the result is fed through sort.Slice.
func CompareTasks(a, b *models.Task, field SortField, projectID string, cfg *config.Config) int {
	primary := compareTasksPrimary(a, b, field, projectID, cfg)
	if primary != 0 {
		return primary
	}
	return cmp.Compare(a.ID, b.ID)
}

func compareTasksPrimary(a, b *models.Task, field SortField, projectID string, cfg *config.Config) int {
	switch field {
	case SortByPriority:
		return cmp.Compare(a.Priority, b.Priority)

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

	case SortByScore:
		profile := cfg.GetProjectScoringProfile(projectID)
		if profile != nil {
			scoreA := scoring.ComputeScore(a, profile)
			scoreB := scoring.ComputeScore(b, profile)
			// Higher score first (descending by default)
			return -cmp.Compare(scoreA, scoreB)
		}
		return 0
	default:
	}

	return 0
}

// CompareTasksCrossProject compares two tasks that may belong to different projects.
// Each task has its own projectID for score resolution. Ties are broken by
// task.ID for deterministic ordering under sort.Slice.
func CompareTasksCrossProject(a, b *models.Task, field SortField, projectIDA, projectIDB string, cfg *config.Config) int {
	if field == SortByScore {
		pa := cfg.GetProjectScoringProfile(projectIDA)
		pb := cfg.GetProjectScoringProfile(projectIDB)
		var sa, sb float64
		if pa != nil {
			sa = scoring.ComputeScore(a, pa)
		}
		if pb != nil {
			sb = scoring.ComputeScore(b, pb)
		}
		// Higher score first (descending by default)
		if c := -cmp.Compare(sa, sb); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	}
	return CompareTasks(a, b, field, "", cfg)
}

// sortHierarchical sorts a flat slice of nodes while preserving parent-child
// relationships. Nodes are grouped by parent (using getParentID), each group
// is sorted with sortSlice, and the result is emitted in depth-first order.
// getParentID should return "" for root nodes (no parent or parent not in set).
func sortHierarchical[T any](
	nodes []T,
	getID func(T) string,
	getParentID func(T) string,
	sortSlice func([]T),
) []T {
	nodesByParent := make(map[string][]T)
	var roots []T

	for _, n := range nodes {
		if pid := getParentID(n); pid != "" {
			nodesByParent[pid] = append(nodesByParent[pid], n)
		} else {
			roots = append(roots, n)
		}
	}

	sortSlice(roots)

	result := make([]T, 0, len(nodes))
	var walk func(n T)
	walk = func(n T) {
		result = append(result, n)
		children := nodesByParent[getID(n)]
		sortSlice(children)
		for _, child := range children {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	return result
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
