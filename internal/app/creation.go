package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// normalizeCreationState resolves the canonical service default for every task
// creation path.
func normalizeCreationState(state models.WorkflowState) (models.WorkflowState, error) {
	if state == "" {
		return models.StateInbox, nil
	}
	if !state.IsValid() {
		return "", invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", state))
	}
	return state, nil
}

// initializeCreatedTask applies the one canonical lifecycle initialization path.
func (s *Service) initializeCreatedTask(task *models.Task, state models.WorkflowState) error {
	state, err := normalizeCreationState(state)
	if err != nil {
		return err
	}
	if err := applyState(task, state, s.clock.Now()); err != nil {
		return invalidInput("state", err.Error())
	}
	return nil
}
