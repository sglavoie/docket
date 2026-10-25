package app

import "github.com/sglavoie/docket/internal/models"

// TaskStateEffect is immutable committed evidence. An empty Before denotes
// creation; Unscheduled describes the result of this transaction, not a later
// read of a cursor or selection.
type TaskStateEffect struct {
	Before      models.WorkflowState
	After       models.WorkflowState
	Unscheduled bool
}

func taskStateEffect(ref TaskRef) *TaskStateEffect {
	if ref.Base == nil {
		return nil
	}
	base := &models.Project{Tasks: ref.Base.Tasks}
	previous := base.GetTaskByID(ref.Task.ID)
	before := models.WorkflowState("")
	if previous != nil {
		before = previous.State()
	}
	if before == ref.Task.State() {
		return nil
	}
	return &TaskStateEffect{Before: before, After: ref.Task.State(), Unscheduled: ref.Task.CheckIn().IsEmpty()}
}

// OffersCheckIn reports an actual entry into active work without a schedule.
// Adapters decide whether the initiating interaction permits an offer.
func (e *TaskStateEffect) OffersCheckIn() bool {
	return e != nil && e.Before != e.After && e.Unscheduled &&
		(e.After == models.StateDoing || e.After == models.StateWaiting)
}
