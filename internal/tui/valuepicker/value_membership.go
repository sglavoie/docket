package valuepicker

import (
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// CommonTaskProjectID names the one project every selected task already lives
// in, and returns "" when the selection spans more than one. It is what the Move
// to Project picker withholds: for a single-source selection the current project
// is the one destination that could change nothing, while for a selection
// spanning M sources every project is a real destination for the tasks that are
// not already there — withholding the first ref's project would hide a
// legitimate one on the accident of enumeration order.
func CommonTaskProjectID(refs []taskref.TaskProjectRef) string {
	if len(refs) == 0 || refs[0].Project == nil {
		return ""
	}
	projectID := refs[0].Project.ID
	for _, ref := range refs[1:] {
		if ref.Project == nil || ref.Project.ID != projectID {
			return ""
		}
	}
	return projectID
}

func CommonTaskPriority(refs []taskref.TaskProjectRef) (int, bool) {
	if len(refs) == 0 || refs[0].Task == nil {
		return 0, false
	}
	priority := refs[0].Task.Priority
	for _, ref := range refs[1:] {
		if ref.Task == nil || ref.Task.Priority != priority {
			return 0, false
		}
	}
	return priority, true
}

func CommonTaskDate(refs []taskref.TaskProjectRef, deferred bool) (*time.Time, bool) {
	if len(refs) == 0 || refs[0].Task == nil {
		return nil, false
	}
	dateFor := func(task *models.Task) *time.Time {
		if deferred {
			return task.DeferredUntil
		}
		return task.DueDate
	}
	first := dateFor(refs[0].Task)
	for _, ref := range refs[1:] {
		if ref.Task == nil || !sameOperationDate(first, dateFor(ref.Task)) {
			return nil, false
		}
	}
	return taskref.CloneOperationDate(first), true
}

func sameOperationDate(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func TaskTagMembership(refs []taskref.TaskProjectRef) map[string]OperationValueMembership {
	counts := make(map[string]int)
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		for _, tag := range models.DedupeTags(ref.Task.Tags) {
			counts[tag]++
		}
	}
	membership := make(map[string]OperationValueMembership, len(counts))
	for tag, count := range counts {
		if count == len(refs) {
			membership[tag] = OperationValueAll
		} else if count > 0 {
			membership[tag] = OperationValueSome
		}
	}
	return membership
}

// TaskContextMembership reports the context row every target already agrees on
// and the per-row state the checkboxes render. Disagreement leaves the agreed
// row empty, so any chosen row reads as a change.
func TaskContextMembership(refs []taskref.TaskProjectRef) (listsurface.ListItemID, map[listsurface.ListItemID]OperationValueMembership) {
	counts := make(map[string]int)
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		counts[ref.Task.Context]++
	}
	states := make(map[listsurface.ListItemID]OperationValueMembership, len(counts))
	agreed := listsurface.ListItemID("")
	for name, count := range counts {
		id := TaskContextRowID(name)
		if count == len(refs) {
			states[id] = OperationValueAll
			agreed = id
			continue
		}
		states[id] = OperationValueSome
	}
	return agreed, states
}
