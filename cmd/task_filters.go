package cmd

import (
	"github.com/sglavoie/tt/internal/models"
)

// TaskFilter is a predicate function for filtering tasks.
type TaskFilter func(*models.Task) bool

// CanMoveUpFilter returns a filter for tasks that can be moved up among siblings.
func CanMoveUpFilter(project *models.Project) TaskFilter {
	return func(t *models.Task) bool {
		siblings := project.GetSiblings(t)
		for i, s := range siblings {
			if s.ID == t.ID && i > 0 {
				return true
			}
		}
		return false
	}
}

// CanMoveDownFilter returns a filter for tasks that can be moved down among siblings.
func CanMoveDownFilter(project *models.Project) TaskFilter {
	return func(t *models.Task) bool {
		siblings := project.GetSiblings(t)
		for i, s := range siblings {
			if s.ID == t.ID && i < len(siblings)-1 {
				return true
			}
		}
		return false
	}
}

// CanIndentFilter returns a filter for tasks that can be indented.
func CanIndentFilter(project *models.Project) TaskFilter {
	return func(t *models.Task) bool {
		targetParent := project.GetTaskAbove(t)
		if targetParent == nil {
			return false
		}
		ok, _ := t.CanIndent(targetParent)
		return ok
	}
}

// CanOutdentFilter returns a filter for tasks that can be outdented.
func CanOutdentFilter() TaskFilter {
	return func(t *models.Task) bool {
		ok, _ := t.CanOutdent()
		return ok
	}
}
