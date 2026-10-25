package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
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
	if invocation.Descriptor.ID == operation.OperationTaskDelete {
		return m.executeTaskDeleteOperation(invocation)
	}
	if invocation.Descriptor.ID == operation.OperationTaskTogglePin {
		return m.executeTaskTogglePinOperation(invocation)
	}
	if invocation.Descriptor.ID == operation.OperationTaskToggleReview {
		return m.executeTaskToggleReviewOperation(invocation)
	}
	if invocation.Descriptor.ID == operation.OperationTaskToggleHidden {
		return m.executeTaskToggleHiddenOperation(invocation)
	}
	if invocation.Descriptor.ID == operation.OperationTaskMarkReviewed {
		return m.executeTaskMarkReviewedOperation(invocation)
	}
	if direction, ok := dueStepDirectionForOperation(invocation.Descriptor.ID); ok {
		return m.executeTaskStepDueDateOperation(invocation, direction)
	}
	if operationProjectsClipboard(invocation.Descriptor.ID) {
		return m.executeTaskCopyOperation(invocation)
	}
	if request, action, summary, handled, err := applicationTaskOperationRequest(invocation); handled {
		if err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		cmd := m.secondaryFeatures.TaskItem.ExecuteOperation(request, action, summary)
		if invocation.Descriptor.ID == operation.OperationTaskSetCheckIn {
			value := invocation.Value.(operation.OperationCheckInValue)
			cmd = operationhost.RetainCheckInDraftOnFailure(operation.OperationExecutionRequest{
				ID:      invocation.Descriptor.ID,
				Context: operation.CloneCommandContext(invocation.Context),
			}, value, cmd)
		}
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
			Cmd: withOutlinerRelocationCapture(cmd, capture),
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
			verb+" "+util.Count(len(locators), "task"),
		), capture),
	}}
}

// executeTaskToggleReviewOperation hands the whole frozen target set to the one
// application operation and lets it decide the direction. Unlike the pin toggle
// above, nothing here reads the flag: `bulkToggleReview` computes flag-all
// versus clear-all from the same snapshot it commits, so a second read in the
// presentation layer could only disagree with the mutation it is describing.
// The success line is left empty for the same reason — the coordinator builds it
// from the committed result, whose task count is the deduplicated one the
// transaction actually wrote.
func (m *AppModel) executeTaskToggleReviewOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)}
		}
		locators = append(locators, target.Task.Locator())
	}
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(
			app.TaskOperationRequest{
				Operation: app.TaskOperationToggleReview,
				Targets:   locators,
			},
			"Toggle Review",
			"",
		), capture),
	}}
}

// executeTaskToggleHiddenOperation applies the pin toggle's rule to the hidden
// flag: if every target stores it, unhide all of them, otherwise hide all of
// them. The direction is read here, as the pin toggle reads its own, because the
// toast has to say which way the rows went: a hidden row departs every view but
// fH, so the line names the key that brings it back. The stored flag is what is
// read, never inheritance, which is the same rule the application layer applies.
func (m *AppModel) executeTaskToggleHiddenOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	allHidden := true
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
		allHidden = allHidden && task.Hidden
		locators = append(locators, target.Task.Locator())
	}
	hidden := !allHidden
	action, headline := "Hide Tasks", hiddenToggleHeadline(true, len(locators))
	if !hidden {
		action, headline = "Unhide Tasks", hiddenToggleHeadline(false, len(locators))
	}
	cmd := m.secondaryFeatures.TaskItem.ExecuteOperation(
		app.TaskOperationRequest{
			Operation: app.TaskOperationSetHidden,
			Targets:   locators,
			Hidden:    hidden,
		},
		action,
		headline,
	)
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withOutlinerRelocationCapture(withRecoveryHeadline(cmd, headline), capture),
	}}
}

// hiddenToggleHeadline is the sentence a hide or unhide answers with. Hiding
// names fH because the rows have just left every view that does not ask for
// them; unhiding returns them to views that already show them, so it says no
// more than what happened.
func hiddenToggleHeadline(hidden bool, count int) string {
	if hidden {
		return "Hidden " + util.Count(count, "task") + " · fH to view"
	}
	return "Unhidden " + util.Count(count, "task")
}

// withRecoveryHeadline stamps a committed batch's toast sentence. A refusal
// passes through untouched, so it still reads as the error it is.
func withRecoveryHeadline(cmd tea.Cmd, headline string) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(coordinator.BatchTransactionResultMsg); ok {
			batch.RecoveryHeadline = headline
			return batch
		}
		return msg
	}
}

// dueStepDirectionForOperation reads the direction off the descriptor rather
// than off the key, which is what lets the two deadline steps stay two
// descriptors with two labels while sharing one handler and one transaction
// shape.
func dueStepDirectionForOperation(id operation.OperationID) (app.DueStepDirection, bool) {
	switch id {
	case operation.OperationTaskStepDueEarlier:
		return app.DueStepBackward, true
	case operation.OperationTaskStepDueLater:
		return app.DueStepForward, true
	default:
		return 0, false
	}
}

// executeTaskStepDueDateOperation freezes one deadline gesture and hands it to
// the queue. Freezing happens here, at the keypress, so the targets are the ones
// the cursor or the visual selection named while the user was looking at them:
// a rebuild, a filtered relocation, or a cursor move between this keypress and
// the commit cannot retarget the work.
//
// Nothing is computed here. The direction is the whole argument, and the
// deadline each step starts from is read at dispatch out of the state the
// preceding gesture committed — which is also why a gesture that arrives while
// one is in flight returns an empty result: it has been recorded, it will run,
// and there is nothing to say about it yet.
func (m *AppModel) executeTaskStepDueDateOperation(
	invocation operation.OperationInvocation,
	direction app.DueStepDirection,
) operation.OperationHandlerResult {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)}
		}
		locators = append(locators, target.Task.Locator())
	}
	gesture := coordinator.DueStepGesture{
		Direction: direction,
		Targets:   locators,
		Hidden:    operationSelectionCounts(invocation.Context, len(locators)).Hidden,
	}
	sequence, dispatch := m.dueSteps.Enqueue(gesture)
	if !dispatch {
		return operation.OperationHandlerResult{}
	}
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: m.dispatchDueStepGesture(gesture, sequence),
	}}
}

// executeTaskMarkReviewedOperation acknowledges the one task under the cursor
// and arms the advance to the next visible candidate. It is single-target by
// construction: the descriptor is not Bulk, so a visual selection refuses at
// resolution and never reaches here.
//
// Eligibility is models.TaskAllowsExplicitReview — the wider question, because
// naming a task is what makes an early review of scheduled work legitimate,
// while the narrower attention predicate is what the advance navigates by. It is
// read from the live task rather than from the row's drawing, and refused by
// name. The application layer reads it again against its own clock before it
// commits, so this check is what keeps the keystroke honest rather than what
// makes the mutation safe: a row admitted here and changed underneath is still
// refused there.
func (m *AppModel) executeTaskMarkReviewedOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	target := invocation.Targets.Targets[0]
	identity := target.Task
	if !identity.Valid() {
		return operation.OperationHandlerResult{Err: fmt.Errorf(
			"operation %s resolved an invalid task",
			invocation.Descriptor.ID,
		)}
	}
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
	}
	if models.TaskEffectivelyHidden(project, task) {
		return operation.OperationHandlerResult{Err: fmt.Errorf("task is hidden: unhide it with wh to review it")}
	}
	if !models.TaskAllowsExplicitReview(task, time.Now()) {
		return operation.OperationHandlerResult{Err: fmt.Errorf(
			"task has no review to acknowledge: flag it with = or schedule a check-in",
		)}
	}
	capture := m.captureOutlinerReviewAdvance(invocation.Targets)
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: withOutlinerRelocationCapture(
			m.secondaryFeatures.TaskItem.MarkReviewed(identity),
			capture,
		),
	}}
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
			return cmdResult(m.secondaryFeatures.TaskItem.Defer(identity, taskref.CloneOperationDate(value.Date)))
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetDueDate(identity, taskref.CloneOperationDate(value.Date)))
	case operation.OperationTaskSetResumeNote:
		// The empty string is a real value here, not a missing one: it is how the
		// note is cleared. So the type assertion is the whole check.
		note, ok := invocation.Value.(string)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid resume note value")}
		}
		// The retry request is rebuilt from the invocation rather than carried
		// through the mutation: ID and context are all the prompt factory reads,
		// and the value it would carry is exactly the one being retried.
		return cmdResult(m.setResumeNoteCmd(operation.OperationExecutionRequest{
			ID:      invocation.Descriptor.ID,
			Context: invocation.Context,
		}, identity, note))
	case operation.OperationTaskAddComment:
		// The prompt already refused an empty buffer and the application layer
		// refuses one again, so the assertion is all this layer owes.
		text, ok := invocation.Value.(string)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid comment value")}
		}
		// No relocation capture: a comment changes no axis any lane sorts or
		// filters on, so the row the cursor is on is the row it stays on.
		return cmdResult(m.addCommentCmd(operation.OperationExecutionRequest{
			ID:      invocation.Descriptor.ID,
			Context: invocation.Context,
		}, identity, text))
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
