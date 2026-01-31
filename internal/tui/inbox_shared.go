package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// inboxSnapshot stores task state for undo in inbox processors.
type inboxSnapshot struct {
	section   string
	priority  int
	tags      []string
	dueDate   *time.Time
	dependsOn []string
}

// captureInboxSnapshot creates a snapshot of task state for undo.
func captureInboxSnapshot(task *models.Task) *inboxSnapshot {
	var dueDate *time.Time
	if task.DueDate != nil {
		d := *task.DueDate
		dueDate = &d
	}

	return &inboxSnapshot{
		section:   task.Section,
		priority:  task.Priority,
		tags:      append([]string{}, task.Tags...),
		dueDate:   dueDate,
		dependsOn: append([]string{}, task.DependsOn...),
	}
}

// restoreInboxSnapshot restores a task from a snapshot.
func restoreInboxSnapshot(task *models.Task, snap *inboxSnapshot) {
	task.Section = snap.section
	task.Priority = snap.priority
	task.Tags = append([]string{}, snap.tags...)
	task.DependsOn = append([]string{}, snap.dependsOn...)
	if snap.dueDate != nil {
		d := *snap.dueDate
		task.DueDate = &d
	} else {
		task.DueDate = nil
	}
	task.UpdatedAt = time.Now()
}

// willLeaveInbox returns true if the task will no longer appear in inbox after save.
func willLeaveInbox(task *models.Task) bool {
	return task.Section != models.SectionInbox || task.IsComplete()
}
