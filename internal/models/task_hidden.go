package models

// taskHiddenAncestorLimit bounds the ancestor walk. Stored parentage is capped
// far below it, so the bound only ever stops a malformed cycle.
const taskHiddenAncestorLimit = 64

// TaskEffectivelyHidden reports whether a task is outside every view: it
// carries the stored Hidden flag itself, or an ancestor does. Only the toggled
// task stores the flag, so this is the one predicate every reader asks, and
// none of them may read Task.Hidden alone.
//
// The walk follows the linked Parent when it agrees with the stored ParentID
// and otherwise resolves ParentID inside project, so a detached clone, which
// carries no Parent pointer, answers the same as the task it was cloned from.
// A nil project leaves only the links the task itself holds to follow.
func TaskEffectivelyHidden(project *Project, task *Task) bool {
	for hops := 0; task != nil && hops < taskHiddenAncestorLimit; hops++ {
		if task.Hidden {
			return true
		}
		task = taskHiddenParent(project, task)
	}
	return false
}

func taskHiddenParent(project *Project, task *Task) *Task {
	if task.ParentID == nil || *task.ParentID == "" {
		return nil
	}
	if task.Parent != nil && task.Parent.ID == *task.ParentID {
		return task.Parent
	}
	if project == nil {
		return nil
	}
	return project.GetTaskByID(*task.ParentID)
}
