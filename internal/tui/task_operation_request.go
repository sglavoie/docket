package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
	"github.com/sglavoie/tt/internal/util"
)

func applicationTaskOperationRequest(
	invocation operation.OperationInvocation,
) (app.TaskOperationRequest, string, string, bool, error) {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return app.TaskOperationRequest{}, "", "", true, fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)
		}
		locators = append(locators, target.Task.Locator())
	}
	request := app.TaskOperationRequest{Targets: locators}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operation.OperationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return request, "", "", true, fmt.Errorf("invalid state value")
		}
		request.Operation = app.TaskOperationSetState
		request.State = state
		return request, "Change Status",
			fmt.Sprintf("Change Status · Changed %s to %s", util.Count(count, "task"), state), true, nil

	case operation.OperationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return request, "", "", true, fmt.Errorf("invalid priority value")
		}
		request.Operation = app.TaskOperationSetPriority
		request.Priority = priority
		return request, "Set Priority",
			"Set Priority · Set priority on " + util.Count(count, "task"), true, nil

	case operation.OperationTaskSetDueDate:
		return applicationDateOperationRequest(request, invocation.Value)

	case operation.OperationTaskSetCheckIn:
		value, ok := invocation.Value.(operation.OperationCheckInValue)
		if !ok || (value.Clear && !value.Schedule.IsEmpty()) || (!value.Clear && value.Schedule.Next == nil) {
			return request, "", "", true, fmt.Errorf("invalid check-in value")
		}
		request.Operation = app.TaskOperationSetCheckIn
		request.CheckIn = value.Schedule.Clone()
		verb := "Scheduled check-in for"
		if value.Clear {
			verb = "Cleared check-in on"
		}
		return request, "Set Check-In",
			fmt.Sprintf("Set Check-In · %s %s", verb, util.Count(count, "task")), true, nil

	case operation.OperationTaskSetTags:
		switch value := invocation.Value.(type) {
		case []string:
			request.Tags = append([]string(nil), value...)
		case operation.OperationTagsValue:
			request.TagPatch = true
			request.TagsToAdd = append([]string(nil), value.Add...)
			request.TagsToRemove = append([]string(nil), value.Remove...)
			request.ContextSet = value.ContextSet
			request.Context = value.Context
		default:
			return request, "", "", true, fmt.Errorf("invalid tag value")
		}
		request.Operation = app.TaskOperationSetTags
		if request.ContextSet {
			verb := "Set context on"
			if request.Context == "" {
				verb = "Cleared context on"
			}
			return request, "Set Tags & Context",
				fmt.Sprintf("Set Tags & Context · %s %s", verb, util.Count(count, "task")), true, nil
		}
		return request, "Set Tags",
			"Set Tags · Tagged " + util.Count(count, "task"), true, nil

	case operation.OperationTaskReferences:
		// Cardinality chooses the transaction, not the meaning, exactly as it does
		// for a move: one target is left to the task-line edit the `~task` sigil
		// writes, so a selection-free `~` stays byte-for-byte the edit it always
		// was, and only two or more take the batch.
		if count < 2 {
			return app.TaskOperationRequest{}, "", "", false, nil
		}
		value, ok := invocation.Value.(operation.OperationTaskReferenceValue)
		if !ok {
			return request, "", "", true, fmt.Errorf("invalid dependency value")
		}
		request.Operation = app.TaskOperationSetDependencies
		request.Dependencies = valuepicker.TaskReferenceBulkPatch(value)
		return request, "Set Dependencies",
			"Set Dependencies · Updated " + util.Count(count, "task"), true, nil

	case operation.OperationTaskCopyTitle:
		request.Operation = app.TaskOperationCopyTitle
		return request, "Copy Title", "", true, nil

	case operation.OperationTaskCopyDetails:
		request.Operation = app.TaskOperationCopyDescription
		return request, "Copy Description", "", true, nil

	case operation.OperationTaskCopyTitleDescription:
		request.Operation = app.TaskOperationCopyTitleDescription
		return request, "Copy Title and Description", "", true, nil

	default:
		return app.TaskOperationRequest{}, "", "", false, nil
	}
}

// applicationDateOperationRequest keeps deadline planning and deferral effects
// confined to date payloads; a check-in never enters this translation.
func applicationDateOperationRequest(request app.TaskOperationRequest, raw any) (app.TaskOperationRequest, string, string, bool, error) {
	count := len(request.Targets)
	value, ok := raw.(operation.OperationDateValue)
	if !ok {
		return request, "", "", true, fmt.Errorf("invalid date value")
	}
	request.Date = taskref.CloneOperationDate(value.Date)
	if value.Deferred {
		request.Operation = app.TaskOperationSetDefer
		verb := "Deferred"
		if value.Date == nil {
			verb = "Cleared defer on"
		}
		return request, "Set Defer",
			fmt.Sprintf("Set Defer · %s %s", verb, util.Count(count, "task")), true, nil
	}
	request.Operation = app.TaskOperationSetDueDate
	request.PlanForToday = coordinator.DueDateIsToday(value.Date)
	verb := "Set due date on"
	if value.Date == nil {
		verb = "Cleared due date on"
	}
	return request, "Set Due Date",
		fmt.Sprintf("Set Due Date · %s %s", verb, util.Count(count, "task")), true, nil
}
