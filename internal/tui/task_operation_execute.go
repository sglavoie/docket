package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func (m *AppModel) executeTaskOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	if len(invocation.Targets.Targets) == 0 {
		return operation.OperationHandlerResult{Err: fmt.Errorf("operation %s requires at least one task", invocation.Descriptor.ID)}
	}
	if invocation.Descriptor.ValueProviderID != "" && !invocation.HasValue {
		return operation.OperationHandlerResult{Err: fmt.Errorf("operation %s requires a value", invocation.Descriptor.ID)}
	}
	// Deletion takes every resolved target at once because one atomic
	// transaction removes single and multi-root selections alike.
	if invocation.Descriptor.ID == operationTaskDelete {
		return m.executeTaskDeleteOperation(invocation)
	}
	if invocation.Descriptor.ID == operationTaskTogglePin {
		return m.executeTaskTogglePinOperation(invocation)
	}
	if request, action, summary, handled, err := applicationTaskOperationRequest(invocation); handled {
		if err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
			Cmd: withWaitingCheckBackStep(invocation, withOutlinerRelocationCapture(
				m.secondaryFeatures.TaskItem.ExecuteOperation(request, action, summary),
				capture,
			)),
		}}
	}
	if len(invocation.Targets.Targets) > 1 {
		return m.executeBulkTaskValueOperation(invocation)
	}
	target := invocation.Targets.Targets[0]
	identity := target.Task
	if !identity.Valid() {
		return operation.OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
	}
	return m.executeSingleTaskOperation(invocation, identity)
}

// withWaitingCheckBackStep follows a committed move into Waiting with the
// due-only picker rather than the combined one `@` opens, because the step is
// asking for a check-back deadline and a defer section would answer a different
// question. The chain is keyed off the operation's own value rather than runtime
// context, and it is gated on the transition's own outcome: a rejected move
// returns its error untouched, so no picker opens and no due date can be
// committed on a task that never reached Waiting. On the success path the state
// transition has already committed when the picker opens, so Esc there cancels
// the date alone and the Waiting move stands.
func withWaitingCheckBackStep(invocation operation.OperationInvocation, cmd tea.Cmd) tea.Cmd {
	if invocation.Descriptor.ID != operation.OperationTaskSetState {
		return cmd
	}
	if state, ok := invocation.Value.(models.WorkflowState); !ok || state != models.StateWaiting {
		return cmd
	}
	checkBack := operation.OperationValueRequestMsg{
		Request: operation.OperationExecutionRequest{
			ID:      operation.OperationTaskSetDueDate,
			Context: cloneCommandContext(invocation.Context),
		},
		Provider: operation.ValueProviderDueDate,
	}
	return func() tea.Msg {
		msg := cmd()
		if !transactionCommitted(msg) {
			return msg
		}
		return tea.Sequence(
			func() tea.Msg { return msg },
			func() tea.Msg { return checkBack },
		)()
	}
}

// transactionCommitted reports whether a coordinator command resolved to a
// durable mutation. The two result types mirror withOutlinerRelocationCapture:
// a task operation lands on the batch path even for one target, while the
// single-task commit path still emits TransactionResultMsg.
func transactionCommitted(msg tea.Msg) bool {
	switch msg.(type) {
	case coordinator.BatchTransactionResultMsg, coordinator.TransactionResultMsg:
		return true
	default:
		return false
	}
}

// executeTaskTogglePinOperation applies one deterministic toggle to the whole
// frozen target set: if every target is pinned it unpins all of them, otherwise
// it pins all of them. The application operation validates and commits the
// project-qualified set atomically.
func (m *AppModel) executeTaskTogglePinOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	allPinned := true
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)}
		}
		task, _ := m.taskAndProjectForIdentity(target.Task)
		if task == nil {
			return operation.OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		allPinned = allPinned && task.Pinned
		locators = append(locators, target.Task.Locator())
	}
	pinned := !allPinned
	action, verb := "Pin Task", "Pinned"
	if !pinned {
		action, verb = "Unpin Task", "Unpinned"
	}
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(
			app.TaskOperationRequest{
				Operation: app.TaskOperationSetPinned,
				Targets:   locators,
				Pinned:    pinned,
			},
			action,
			fmt.Sprintf("%s %d task(s)", verb, len(locators)),
		), capture),
	}}
}

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
		return request, "Change State",
			fmt.Sprintf("Change State · Changed %d task(s) to %s", count, state), true, nil

	case operation.OperationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return request, "", "", true, fmt.Errorf("invalid priority value")
		}
		request.Operation = app.TaskOperationSetPriority
		request.Priority = priority
		return request, "Set Priority",
			fmt.Sprintf("Set Priority · Set priority on %d task(s)", count), true, nil

	case operation.OperationTaskSetDueDate:
		value, ok := invocation.Value.(operation.OperationDateValue)
		if !ok {
			return request, "", "", true, fmt.Errorf("invalid date value")
		}
		request.Date = cloneOperationDate(value.Date)
		if value.Deferred {
			request.Operation = app.TaskOperationSetDefer
			verb := "Deferred"
			if value.Date == nil {
				verb = "Cleared defer on"
			}
			return request, "Set Defer",
				fmt.Sprintf("Set Defer · %s %d task(s)", verb, count), true, nil
		}
		request.Operation = app.TaskOperationSetDueDate
		request.PlanForToday = coordinator.DueDateIsToday(value.Date)
		verb := "Set due date on"
		if value.Date == nil {
			verb = "Cleared due date on"
		}
		return request, "Set Due Date",
			fmt.Sprintf("Set Due Date · %s %d task(s)", verb, count), true, nil

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
				fmt.Sprintf("Set Tags & Context · %s %d task(s)", verb, count), true, nil
		}
		return request, "Set Tags",
			fmt.Sprintf("Set Tags · Tagged %d task(s)", count), true, nil

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
		request.Dependencies = taskReferenceBulkPatch(value)
		return request, "Set Dependencies",
			fmt.Sprintf("Set Dependencies · Updated %d task(s)", count), true, nil

	case operationTaskCopyTitle:
		request.Operation = app.TaskOperationCopyTitle
		return request, "Copy Title", "", true, nil

	case operationTaskCopyDetails:
		request.Operation = app.TaskOperationCopyDescription
		return request, "Copy Description", "", true, nil

	case operationTaskCopyTitleDescription:
		request.Operation = app.TaskOperationCopyTitleDescription
		return request, "Copy Title and Description", "", true, nil

	default:
		return app.TaskOperationRequest{}, "", "", false, nil
	}
}

func (m *AppModel) executeSingleTaskOperation(
	invocation operation.OperationInvocation,
	identity taskref.TaskIdentity,
) operation.OperationHandlerResult {
	cmdResult := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	relocationResult := func(cmd tea.Cmd) operation.OperationHandlerResult {
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		return cmdResult(withOutlinerRelocationCapture(cmd, capture))
	}

	switch invocation.Descriptor.ID {
	case operation.OperationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.Transition(identity, state))
	case operation.OperationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetPriority(identity, priority))
	case operation.OperationTaskSetDueDate:
		value, ok := invocation.Value.(operation.OperationDateValue)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid date value")}
		}
		if value.Deferred {
			return cmdResult(m.secondaryFeatures.TaskItem.Defer(identity, cloneOperationDate(value.Date)))
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetDueDate(identity, cloneOperationDate(value.Date)))
	case operation.OperationTaskSetParent:
		value, ok := invocation.Value.(operation.OperationParentValue)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid parent value")}
		}
		return relocationResult(m.secondaryFeatures.TaskItem.Reparent(identity, value.ParentID))
	case operation.OperationTaskAdoptChildren:
		children, ok := invocation.Value.([]string)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid child value")}
		}
		return cmdResult(m.adoptChildrenCmd(
			identity,
			children,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	case operation.OperationTaskReferences:
		// No relocation capture: a reference edit records a soft link and moves no
		// row, so there is no destination for the outliner to follow.
		value, ok := invocation.Value.(operation.OperationTaskReferenceValue)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid dependency value")}
		}
		return cmdResult(m.setTaskReferencesCmd(identity, value))
	case operation.OperationTaskMoveToProject:
		projectID, ok := invocation.Value.(string)
		if !ok || strings.TrimSpace(projectID) == "" {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid project value")}
		}
		return cmdResult(m.moveTaskToProjectCmd(
			[]taskref.TaskIdentity{identity},
			projectID,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	case operation.OperationTaskIndent:
		return relocationResult(m.secondaryFeatures.TaskItem.Indent(identity))
	case operation.OperationTaskOutdent:
		return relocationResult(m.secondaryFeatures.TaskItem.Outdent(identity))
	default:
		return m.executeAuxiliaryTaskOperation(invocation, identity)
	}
}
