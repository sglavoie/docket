package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// BulkOperationComponent encapsulates bulk operation state and functionality.
type BulkOperationComponent struct {
	state        models.WorkflowState    // Target state to apply after confirmation
	tasks        []*models.Task          // Tasks to change
	blockedCount int                     // Count of tasks with incomplete deps
	prevStates   map[string]*models.Task // Previous states for event triggers
	locators     []app.TaskLocator       // Qualified targets for cross-project operations
}

// NewBulkOperationComponent creates a new bulk operation component.
func NewBulkOperationComponent() *BulkOperationComponent {
	return &BulkOperationComponent{
		state:        "",
		tasks:        nil,
		blockedCount: 0,
		prevStates:   make(map[string]*models.Task),
	}
}

// IsActive returns whether a bulk operation is pending.
func (b *BulkOperationComponent) IsActive() bool {
	return len(b.tasks) > 0
}

// GetState returns the target workflow state to apply.
func (b *BulkOperationComponent) GetState() models.WorkflowState {
	return b.state
}

// GetTasks returns the tasks to change.
func (b *BulkOperationComponent) GetTasks() []*models.Task {
	return b.tasks
}

func (b *BulkOperationComponent) GetLocators() []app.TaskLocator { return b.locators }

// GetPrevStates returns the previous states for event triggers.
func (b *BulkOperationComponent) GetPrevStates() map[string]*models.Task {
	return b.prevStates
}

// SetOperation sets up a bulk operation targeting a workflow state.
func (b *BulkOperationComponent) SetOperation(state models.WorkflowState, tasks []*models.Task, blockedCount int, prevStates map[string]*models.Task) {
	b.state = state
	b.tasks = tasks
	b.blockedCount = blockedCount
	b.prevStates = prevStates
	b.locators = nil
}

func (b *BulkOperationComponent) SetLocatedOperation(state models.WorkflowState, tasks []*models.Task, locators []app.TaskLocator) {
	b.SetOperation(state, tasks, 0, nil)
	b.locators = append([]app.TaskLocator(nil), locators...)
}

// Clear clears the bulk operation state.
func (b *BulkOperationComponent) Clear() {
	b.state = ""
	b.tasks = nil
	b.blockedCount = 0
	b.prevStates = make(map[string]*models.Task)
	b.locators = nil
}
