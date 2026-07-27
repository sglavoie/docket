package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// TaskStatePickerModel is the advanced ten-state ListSurface picker used by the
// set-state operation. It is an alias over the generic operation value picker
// so every field uses the same viewport, search, and selection behavior.
type TaskStatePickerModel = OperationValuePickerModel

// NewTaskStatePickerModel builds the exact canonical ten-state picker. Invalid
// transitions stay visible with a concise reason, which is more useful than
// silently hiding terminal-state rules.
func NewTaskStatePickerModel(
	request OperationExecutionRequest,
	current models.WorkflowState,
	taskTitle string,
) (*TaskStatePickerModel, error) {
	return NewTaskStatePickerForTargets(
		request,
		[]models.WorkflowState{current},
		taskTitle,
	)
}

// NewTaskStatePickerForTargets builds one state picker whose enabled values are
// valid for every frozen target. Mixed-state visual selections therefore never
// offer a transition that would silently apply to only a subset.
func NewTaskStatePickerForTargets(
	request OperationExecutionRequest,
	current []models.WorkflowState,
	taskTitle string,
) (*TaskStatePickerModel, error) {
	source := ListSourceFromWorkflowStates(models.AllWorkflowStates())
	for index := range source.Items {
		state, ok := source.Items[index].Data.(models.WorkflowState)
		if !ok {
			continue
		}
		for _, from := range current {
			if reason := models.CanTransitionState(from, state); reason != "" {
				if state == from {
					source.Items[index].DisabledReason = "current state for part of selection"
				} else {
					source.Items[index].DisabledReason = reason
				}
				break
			}
		}
	}
	picker, err := newOperationValuePickerModel(
		request,
		ValueProviderState,
		"Set State",
		taskTitle,
		source,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build state picker: %w", err)
	}
	if len(current) > 0 {
		uniform := true
		for _, state := range current[1:] {
			if state != current[0] {
				uniform = false
				break
			}
		}
		if uniform {
			picker.surface.SetCursorToID(ListItemID("state:" + current[0].String()))
		}
	}
	return picker, nil
}
