package tui

import (
	"time"

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

// StatusOrder returns the sort order for a status (workflow order).
// This is the single source of truth used by all views that sort by status.
func StatusOrder(status models.TaskStatus) int {
	switch status {
	case models.StatusTodo:
		return 0
	case models.StatusInProgress:
		return 1
	case models.StatusReview:
		return 2
	case models.StatusStandby:
		return 3
	case models.StatusBlocked:
		return 4
	case models.StatusDone:
		return 5
	case models.StatusCanceled:
		return 6
	default:
		return 7
	}
}

// CompareTasks compares two tasks based on the given sort field.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// The projectID and cfg are needed for score-based comparison.
func CompareTasks(a, b *models.Task, field SortField, projectID string, cfg *config.Config) int {
	switch field {
	case SortByPriority:
		return compareInts(a.Priority, b.Priority)

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
		return compareTimes(*a.DueDate, *b.DueDate)

	case SortByCreatedAt:
		return compareTimes(a.CreatedAt, b.CreatedAt)

	case SortByStatus:
		return compareInts(StatusOrder(a.Status), StatusOrder(b.Status))

	case SortByScore:
		profile := cfg.GetProjectScoringProfile(projectID)
		if profile != nil {
			scoreA := scoring.ComputeScore(a, profile)
			scoreB := scoring.ComputeScore(b, profile)
			// Higher score first (descending by default)
			if scoreA > scoreB {
				return -1
			}
			if scoreA < scoreB {
				return 1
			}
		}
		return 0
	default:
	}

	return 0
}

// CompareTasksCrossProject compares two tasks that may belong to different projects.
// Each task has its own projectID for score resolution.
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
		if sa > sb {
			return -1
		}
		if sa < sb {
			return 1
		}
		return 0
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

// ApplySortDirection flips a comparison result if descending.
func ApplySortDirection(cmp int, ascending bool) bool {
	less := cmp < 0
	if !ascending {
		less = !less
	}
	return less
}

func compareInts(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareTimes(a, b time.Time) int {
	if a.Before(b) {
		return -1
	}
	if a.After(b) {
		return 1
	}
	return 0
}
