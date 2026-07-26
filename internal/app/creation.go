package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// normalizeCreationState resolves the canonical service default for every task
// creation path. Presentation adapters that still expose legacy section/status
// concepts must translate them to an explicit state before calling the service.
func normalizeCreationState(state models.WorkflowState) (models.WorkflowState, error) {
	if state == "" {
		return models.StateInbox, nil
	}
	if !state.IsValid() {
		return "", invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", state))
	}
	return state, nil
}

// initializeCreatedTask applies the one canonical lifecycle initialization path
// and keeps temporary project-section metadata available to legacy views.
func (s *Service) initializeCreatedTask(project *models.Project, task *models.Task, state models.WorkflowState) error {
	state, err := normalizeCreationState(state)
	if err != nil {
		return err
	}
	if section := models.SectionForState(state); section != "" {
		project.EnsureSection(section)
	}
	if err := applyState(task, state, s.clock.Now()); err != nil {
		return invalidInput("state", err.Error())
	}
	return nil
}
