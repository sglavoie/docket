package models

import "time"

// CloneTaskDeep returns a copy of a task's stored fields that shares no memory
// with the source: every pointer, slice, and comment is copied, and time values
// keep their locations. The computed tree links (Children, Parent, Project,
// Level) are left zero, because a clone belongs to whichever project rebuilds
// its tree. An empty collection is cloned as nil, the value a task that never
// held one carries.
//
// It is the rollback copy an application transaction takes before mutating, so
// it runs over the whole workspace on every commit; a field-by-field copy is
// what keeps that cheap. TestCloneTaskDeepDetachesEveryField fails when a new
// reference-typed field is added without a copy here.
func CloneTaskDeep(task *Task) *Task {
	if task == nil {
		return nil
	}
	clone := *task
	clone.ParentID = cloneStringPointer(task.ParentID)
	if task.ChecklistOrigin != nil {
		origin := *task.ChecklistOrigin
		clone.ChecklistOrigin = &origin
	}
	clone.Tags = cloneSliceOrNil(task.Tags)
	clone.CompletedAt = cloneTimePointer(task.CompletedAt)
	clone.DueDate = cloneTimePointer(task.DueDate)
	clone.PinnedAt = cloneTimePointer(task.PinnedAt)
	clone.Comments = nil
	if len(task.Comments) > 0 {
		clone.Comments = make([]*Comment, len(task.Comments))
		for i, comment := range task.Comments {
			if comment != nil {
				copied := *comment
				clone.Comments[i] = &copied
			}
		}
	}
	clone.Attachments = cloneSliceOrNil(task.Attachments)
	clone.DeferredUntil = cloneTimePointer(task.DeferredUntil)
	clone.Checklist = cloneSliceOrNil(task.Checklist)
	clone.Dependencies = CloneTaskReferences(task.Dependencies)
	clone.NextCheckIn = cloneTimePointer(task.NextCheckIn)
	clone.LastReviewedAt = cloneTimePointer(task.LastReviewedAt)
	clone.Children = nil
	clone.Parent = nil
	clone.Project = nil
	clone.Level = 0
	return &clone
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneSliceOrNil[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return append([]T(nil), values...)
}
