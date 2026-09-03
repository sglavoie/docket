package tui

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func (m *AppModel) operationTaskRefs(context operation.CommandContext) ([]TaskProjectRef, error) {
	identities := operation.NormalizedTaskIdentities(context.Selection.Tasks)
	if len(identities) == 0 {
		if context.Selection.Visual {
			return nil, fmt.Errorf("select at least one task")
		}
		if current := operation.CurrentTaskIdentity(context); current.Valid() {
			identities = []taskref.TaskIdentity{current}
		}
	}
	refs := make([]TaskProjectRef, 0, len(identities))
	for _, identity := range identities {
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			return nil, fmt.Errorf("selected task is no longer available")
		}
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("select a task")
	}
	return refs, nil
}

// commonTaskProjectID names the one project every selected task already lives
// in, and returns "" when the selection spans more than one. It is what the Move
// to Project picker withholds: for a single-source selection the current project
// is the one destination that could change nothing, while for a selection
// spanning M sources every project is a real destination for the tasks that are
// not already there — withholding the first ref's project would hide a
// legitimate one on the accident of enumeration order.
func commonTaskProjectID(refs []TaskProjectRef) string {
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

func commonTaskPriority(refs []TaskProjectRef) (int, bool) {
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

func commonTaskDate(refs []TaskProjectRef, deferred bool) (*time.Time, bool) {
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
	return cloneOperationDate(first), true
}

func sameOperationDate(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func taskTagMembership(refs []TaskProjectRef) map[string]operationValueMembership {
	counts := make(map[string]int)
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		for _, tag := range models.DedupeTags(ref.Task.Tags) {
			counts[tag]++
		}
	}
	membership := make(map[string]operationValueMembership, len(counts))
	for tag, count := range counts {
		if count == len(refs) {
			membership[tag] = operationValueAll
		} else if count > 0 {
			membership[tag] = operationValueSome
		}
	}
	return membership
}

// taskContextMembership reports the context row every target already agrees on
// and the per-row state the checkboxes render. Disagreement leaves the agreed
// row empty, so any chosen row reads as a change.
func taskContextMembership(refs []TaskProjectRef) (ListItemID, map[ListItemID]operationValueMembership) {
	counts := make(map[string]int)
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		counts[ref.Task.Context]++
	}
	states := make(map[ListItemID]operationValueMembership, len(counts))
	agreed := ListItemID("")
	for name, count := range counts {
		id := taskContextRowID(name)
		if count == len(refs) {
			states[id] = operationValueAll
			agreed = id
			continue
		}
		states[id] = operationValueSome
	}
	return agreed, states
}
