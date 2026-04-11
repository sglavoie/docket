package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// BulkOperationComponent encapsulates bulk operation state and functionality.
type BulkOperationComponent struct {
	status       models.TaskStatus       // Status to apply after confirmation
	tasks        []*models.Task          // Tasks to change
	blockedCount int                     // Count of tasks with incomplete deps
	prevStates   map[string]*models.Task // Previous states for event triggers
}

// NewBulkOperationComponent creates a new bulk operation component.
func NewBulkOperationComponent() *BulkOperationComponent {
	return &BulkOperationComponent{
		status:       "",
		tasks:        nil,
		blockedCount: 0,
		prevStates:   make(map[string]*models.Task),
	}
}

// IsActive returns whether a bulk operation is pending.
func (b *BulkOperationComponent) IsActive() bool {
	return len(b.tasks) > 0
}

// GetStatus returns the status to apply.
func (b *BulkOperationComponent) GetStatus() models.TaskStatus {
	return b.status
}

// GetTasks returns the tasks to change.
func (b *BulkOperationComponent) GetTasks() []*models.Task {
	return b.tasks
}

// GetPrevStates returns the previous states for event triggers.
func (b *BulkOperationComponent) GetPrevStates() map[string]*models.Task {
	return b.prevStates
}

// SetOperation sets up a bulk operation.
func (b *BulkOperationComponent) SetOperation(status models.TaskStatus, tasks []*models.Task, blockedCount int, prevStates map[string]*models.Task) {
	b.status = status
	b.tasks = tasks
	b.blockedCount = blockedCount
	b.prevStates = prevStates
}

// Clear clears the bulk operation state.
func (b *BulkOperationComponent) Clear() {
	b.status = ""
	b.tasks = nil
	b.blockedCount = 0
	b.prevStates = make(map[string]*models.Task)
}
