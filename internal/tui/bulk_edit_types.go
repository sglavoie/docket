package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// PendingChanges represents the changes to apply to selected tasks.
type PendingChanges struct {
	SetStatus    *models.TaskStatus // nil = no change
	SetType      *models.TaskType   // nil = no change
	SetPriority  *int               // nil = no change
	AddTags      []string           // tags to add
	RemoveTags   []string           // tags to remove
	SetSection   *string            // nil = no change, empty string = clear section
	SetDueDate      *time.Time                // nil = no change
	ClearDueDate    bool                      // true = remove due date
	SetRecurrence   *models.RecurrencePattern // nil = no change
	ClearRecurrence bool                      // true = remove recurrence
	SetParentID     *string                   // nil = no change (pointer to parent task ID)
	ClearParent     bool                      // true = move to root level
}

// HasChanges returns true if any changes are pending.
func (p *PendingChanges) HasChanges() bool {
	return p.SetStatus != nil ||
		p.SetType != nil ||
		p.SetPriority != nil ||
		len(p.AddTags) > 0 ||
		len(p.RemoveTags) > 0 ||
		p.SetSection != nil ||
		p.SetDueDate != nil ||
		p.ClearDueDate ||
		p.SetRecurrence != nil ||
		p.ClearRecurrence ||
		p.SetParentID != nil ||
		p.ClearParent
}

// ShowBulkEditMsg is sent to open the bulk edit panel.
type ShowBulkEditMsg struct {
	Tasks   []*models.Task
	Project *models.Project
}

// BulkEditDoneMsg is sent when the bulk edit panel is closed.
type BulkEditDoneMsg struct {
	Changes   PendingChanges
	Tasks     []*models.Task
	Project   *models.Project
	TaskCount int
	Applied   bool // false = canceled
}
