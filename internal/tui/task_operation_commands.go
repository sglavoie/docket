package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/entityops"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
	"github.com/sglavoie/docket/internal/tui/taskform"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/valuepicker"
	"github.com/sglavoie/docket/internal/util"
)

// adoptChildrenCmd routes the picker's checked set through the same task-line
// edit the `<child` sigil uses, so adoption is validated as one hierarchy patch
// and lands in one transaction with one undo entry. The title is carried
// verbatim and never reparsed, and the request names no parent patch, so the
// adopting task keeps its own place in the tree.
func (m *AppModel) adoptChildrenCmd(
	identity taskref.TaskIdentity,
	childIDs []string,
	relocation *outliner.OutlinerRelocationCapture,
) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return coordinator.TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	req := app.TaskLineEditRequest{
		Source:               identity.Locator(),
		Title:                task.Title,
		DestinationProjectID: identity.ProjectID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentPreserve},
		Action:               "Adopt Children",
		PlanningArtifacts:    coordinator.PlanningArtifacts(m.domain.runtime, m.domain.manager),
	}
	for _, childID := range childIDs {
		req.Children = append(req.Children, app.TaskLocator{
			ProjectID: identity.ProjectID,
			TaskID:    childID,
		})
	}
	return m.secondaryFeatures.TaskItem.EditTaskLine(req, relocation)
}

// setTaskReferencesCmd routes the picker's selection through the same task-line
// edit the `~task` sigil uses, so both halves of the diff are resolved against
// one snapshot and land in one transaction with one undo entry. The paired add
// and remove services would be two of each, and neither would see the other's
// half of the request.
//
// The task is resolved again here rather than carried from the picker, and the
// selection is merged into that fresh state rather than replacing it, so a
// reference republished by a cross-project load while the overlay was open is
// left standing.
//
// A patch with nothing on either side issues no command at all: an empty
// transaction would still write a ledger entry `u` could take back, and a
// picker closed unchanged — or one whose every edit the fresh state already
// reflects — is not something to undo.
func (m *AppModel) setTaskReferencesCmd(identity taskref.TaskIdentity, value operation.OperationTaskReferenceValue) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return coordinator.TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	patch := valuepicker.TaskReferencePatch(task, value)
	if patch.IsZero() {
		return func() tea.Msg { return status.InfoStatusMsg("Dependencies unchanged") }
	}
	req := app.TaskLineEditRequest{
		Source:               identity.Locator(),
		Title:                task.Title,
		DestinationProjectID: identity.ProjectID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentPreserve},
		Dependencies:         patch,
		Action:               "Set Dependencies",
		PlanningArtifacts:    coordinator.PlanningArtifacts(m.domain.runtime, m.domain.manager),
	}
	return m.secondaryFeatures.TaskItem.EditTaskLine(req, nil)
}

// ResumeNoteRejectedMsg carries a refused resume-note save back to the shell so
// the prompt can reopen on the text the user already typed. It exists because
// the failure can only be known after the overlay closed — the commit resolves
// the task again and may find a newer revision — and asking someone to retype a
// sentence they already wrote is the wrong answer to that.
type ResumeNoteRejectedMsg struct {
	Request operation.OperationExecutionRequest
	Note    string
	Err     error
}

// setResumeNoteCmd runs the resume-note write and turns only its rejection into
// the retry message. Success, the unchanged notice, and every other message the
// coordinator produces pass through untouched, so the ordinary paths keep the
// one transaction-result contract every other task mutation has.
func (m *AppModel) setResumeNoteCmd(
	request operation.OperationExecutionRequest,
	identity taskref.TaskIdentity,
	note string,
) tea.Cmd {
	cmd := m.secondaryFeatures.TaskItem.SetResumeNote(identity, note)
	return func() tea.Msg {
		msg := cmd()
		if failure, ok := msg.(coordinator.TransactionErrorMsg); ok {
			return ResumeNoteRejectedMsg{Request: request, Note: note, Err: failure.Err}
		}
		return msg
	}
}

// reopenResumeNotePrompt puts the refused note back in front of the user, on
// the same buffer they typed it in, with the reason the save was refused.
//
// A prompt that cannot be rebuilt means the target itself is gone rather than
// the note being wrong, and there is nothing to retry into, so that degrades to
// the ordinary error notice every other failed mutation gets.
func (m *AppModel) reopenResumeNotePrompt(msg ResumeNoteRejectedMsg) tea.Cmd {
	prompt, err := m.newOperationTextPrompt(msg.Request, operation.ValueProviderResumeNote)
	if err != nil {
		m.feedback.err = msg.Err
		return status.StatusTimeout(5 * time.Second)
	}
	prompt.Restore(msg.Note, operationRefusalReason(msg.Err))
	m.operations.TextPrompt = prompt
	return prompt.InitCmd()
}

// CommentRejectedMsg is the resume-note message's twin, for the same reason: a
// comment save can only fail after the popup closed, and a paragraph someone
// just typed is exactly the thing that must not be thrown away by a refusal.
type CommentRejectedMsg struct {
	Request operation.OperationExecutionRequest
	Text    string
	Err     error
}

// addCommentCmd appends one comment through the same SubviewCoordinator the
// Comments panel commits through, so the two routes share one mutation, one
// transaction, and one undo entry shape. Only a rejection is rewritten; every
// other message the coordinator produces reaches the shell untouched and gets
// the ordinary reconciliation.
func (m *AppModel) addCommentCmd(
	request operation.OperationExecutionRequest,
	identity taskref.TaskIdentity,
	text string,
) tea.Cmd {
	cmd := m.secondaryFeatures.Subview.AddComment(identity, text)
	return func() tea.Msg {
		msg := cmd()
		if failure, ok := msg.(coordinator.TransactionErrorMsg); ok {
			return CommentRejectedMsg{Request: request, Text: text, Err: failure.Err}
		}
		return msg
	}
}

// reopenCommentPrompt puts the refused comment back in the buffer it was typed
// in. It degrades the same way the resume note does: a prompt that cannot be
// rebuilt means the task itself is gone, and there is nothing left to retry
// into, so the refusal becomes the ordinary error notice.
func (m *AppModel) reopenCommentPrompt(msg CommentRejectedMsg) tea.Cmd {
	prompt, err := m.newOperationTextPrompt(msg.Request, operation.ValueProviderComment)
	if err != nil {
		m.feedback.err = msg.Err
		return status.StatusTimeout(5 * time.Second)
	}
	prompt.Restore(msg.Text, operationRefusalReason(msg.Err))
	m.operations.TextPrompt = prompt
	return prompt.InitCmd()
}

// reopenChecklistItemPrompt puts the refused item back in the buffer it was
// typed in, on the owner the append was frozen onto. It is the third of the
// same shape as the resume note and the comment, and it degrades the same way:
// an owner that no longer resolves means the task itself is gone rather than
// the item being wrong, so there is nothing to retry into and the refusal
// becomes the ordinary error notice.
//
// The owner is rechecked here rather than trusted, because unlike the other two
// the checklist prompt resolves no target when it is built: Add Item appends to
// a checklist the task may not have yet, so the factory reads nothing back.
func (m *AppModel) reopenChecklistItemPrompt(msg entityops.ChecklistItemRejectedMsg) tea.Cmd {
	refuse := func() tea.Cmd {
		m.feedback.err = msg.Err
		return status.StatusTimeout(5 * time.Second)
	}
	if task, _ := m.taskAndProjectForIdentity(msg.Owner); task == nil {
		return refuse()
	}
	prompt, err := m.newOperationTextPrompt(msg.Request, operation.ValueProviderChecklistItem)
	if err != nil {
		return refuse()
	}
	prompt.Restore(msg.Text, operationRefusalReason(msg.Err))
	m.operations.TextPrompt = prompt
	return prompt.InitCmd()
}

// moveTaskToProjectCmd relocates one or many selected subtrees into the picked
// project. Cardinality chooses the transaction, not the meaning: one target
// routes through the same task-line edit the `%project` sigil uses, so a
// selection-free `%` is byte-for-byte the move it always was, while a visual
// selection routes through the batch service that commits every source project,
// the destination, and the references that had to follow as one transaction with
// one undo entry. Either way the title is carried verbatim so the edit never
// reads as a rename, and the destination ShortIDs are reissued by the same
// model primitive.
func (m *AppModel) moveTaskToProjectCmd(
	identities []taskref.TaskIdentity,
	projectID string,
	relocation *outliner.OutlinerRelocationCapture,
) tea.Cmd {
	if len(identities) == 0 {
		return func() tea.Msg {
			return coordinator.TransactionErrorMsg{Err: fmt.Errorf("select at least one task")}
		}
	}
	if len(identities) == 1 {
		return m.moveOneTaskToProjectCmd(identities[0], projectID, relocation)
	}
	return m.moveTaskSelectionToProjectCmd(identities, projectID, relocation)
}

func (m *AppModel) moveOneTaskToProjectCmd(
	identity taskref.TaskIdentity,
	projectID string,
	relocation *outliner.OutlinerRelocationCapture,
) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return coordinator.TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	req := app.TaskLineEditRequest{
		Source:               identity.Locator(),
		Title:                task.Title,
		DestinationProjectID: projectID,
		Action:               "Move to Project",
		PlanningArtifacts:    coordinator.StructuralPlanningArtifacts(m.domain.runtime, m.domain.manager),
	}
	return m.secondaryFeatures.TaskItem.EditTaskLine(req, relocation)
}

// moveTaskSelectionToProjectCmd commits a whole visual selection as one batch.
// The committed result clears the selection, because every locator in it names
// the project the task just left: unlike the other bulk operations, which leave
// their targets addressable and therefore chainable, a move invalidates the very
// identities the selection is made of.
func (m *AppModel) moveTaskSelectionToProjectCmd(
	identities []taskref.TaskIdentity,
	projectID string,
	relocation *outliner.OutlinerRelocationCapture,
) tea.Cmd {
	locators := make([]app.TaskLocator, 0, len(identities))
	for _, identity := range identities {
		if !identity.Valid() {
			return func() tea.Msg {
				return coordinator.TransactionErrorMsg{Err: fmt.Errorf("selected task is no longer available")}
			}
		}
		locators = append(locators, identity.Locator())
	}
	destination := m.moveDestinationName(projectID)
	m.domain.service.WithPlanningArtifacts(coordinator.StructuralPlanningArtifacts(m.domain.runtime, m.domain.manager))
	cmd := m.secondaryFeatures.Bulk.MoveToProjectLocators(locators, projectID, func(moved int) string {
		return fmt.Sprintf("Moved %s to %s", util.Count(moved, "task"), destination)
	})
	return coordinator.FinalizeStaleSelectionBatchResult(withOutlinerRelocationCapture(cmd, relocation), locators)
}

// moveDestinationName names the destination the way the picker row did, and
// degrades to the raw ID rather than dropping the destination from the status
// sentence entirely.
func (m *AppModel) moveDestinationName(projectID string) string {
	if project := m.projectByID(projectID); project != nil && project.Name != "" {
		return project.Name
	}
	return projectID
}

func (m *AppModel) executeBulkTaskValueOperation(
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	identities := make([]taskref.TaskIdentity, 0, len(invocation.Targets.Targets))
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
		}
		identities = append(identities, target.Task)
		locators = append(locators, target.Task.Locator())
	}
	cmdResult := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operation.OperationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.TransitionLocators(
			locators,
			state,
			func(changed int) string {
				return fmt.Sprintf("Changed %s to %s", util.Count(changed, "task"), state)
			},
		))
	case operation.OperationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetPriorityLocators(locators, priority))
	case operation.OperationTaskSetDueDate:
		value, ok := invocation.Value.(operation.OperationDateValue)
		if !ok {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid date value")}
		}
		if value.Deferred {
			return cmdResult(m.secondaryFeatures.Bulk.DeferLocators(locators, taskref.CloneOperationDate(value.Date)))
		}
		if value.Date == nil {
			return cmdResult(m.secondaryFeatures.Bulk.ClearDueDateLocators(locators))
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetDueDateLocators(
			locators,
			*taskref.CloneOperationDate(value.Date),
		))
	case operation.OperationTaskMoveToProject:
		projectID, ok := invocation.Value.(string)
		if !ok || strings.TrimSpace(projectID) == "" {
			return operation.OperationHandlerResult{Err: fmt.Errorf("invalid project value")}
		}
		return cmdResult(m.moveTaskToProjectCmd(
			identities,
			projectID,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	default:
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("operation %s is not available for %d selected tasks", invocation.Descriptor.ID, count),
		}
	}
}

// taskAuxiliaryAllowsOperation gates the child creator on live task state that
// the frozen EntityContext does not carry, so a child is never offered under a
// task that cannot hold one. A target that no longer resolves stays applicable
// so the handler reports the single authoritative "no longer available" error
// instead of the row vanishing.
func (m *AppModel) taskAuxiliaryAllowsOperation(id operation.OperationID, entity operation.EntityContext) bool {
	if id != operation.OperationTaskNewChild {
		return true
	}
	task, _ := m.taskAndProjectForIdentity(taskref.TaskIdentity{
		ProjectID: entity.ProjectID,
		TaskID:    entity.TaskID,
	})
	if task == nil {
		return true
	}
	return taskAcceptsDeeperChild(task)
}

// taskAcceptsDeeperChild reports whether a child nested under the task would
// stay within the nesting cap. Beyond it, BuildTaskTree detaches the child back
// to the root instead of honoring the relationship the user asked for.
func taskAcceptsDeeperChild(task *models.Task) bool {
	return task.Level+1 <= models.MaxNestingDepth
}

// executeAuxiliaryTaskOperation reloads live domain state from the frozen
// identity and hands off to the coordinator or message that already owns the
// behavior. No auxiliary handler mutates a task itself.
//
//nolint:gocyclo // One auditable switch keeps the auxiliary catalog in a single place.
func (m *AppModel) executeAuxiliaryTaskOperation(
	invocation operation.OperationInvocation,
	identity taskref.TaskIdentity,
) operation.OperationHandlerResult {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
	}
	message := func(msg any) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: msg}
	}

	//nolint:exhaustive // Only auxiliary task operations reach this switch.
	switch invocation.Descriptor.ID {
	case operation.OperationTaskComments:
		return message(taskdetail.ShowTaskCommentsMsg{Task: task, Project: project})
	case operation.OperationTaskAttachments:
		return message(taskdetail.ShowTaskAttachmentsMsg{Task: task, Project: project})
	case operation.OperationTaskNewChild:
		return message(taskform.CreateTaskMsg{ParentTask: task, Project: project})
	case operation.OperationTaskRepeat:
		preset, err := m.repeatComposerPreset(project, task)
		if err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
		return message(taskform.ShowTaskComposerMsg{Preset: preset})
	default:
		return operation.OperationHandlerResult{Err: fmt.Errorf("%w: %s", operation.ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

// taskAndProjectForTargets reloads the live task named by the first resolved
// target, so core task operations act on the frozen identity rather than
// whatever row the cursor has since moved to.
func (m *AppModel) taskAndProjectForTargets(
	targets operation.ResolvedOperationTargets,
) (*models.Task, *models.Project) {
	if len(targets.Targets) == 0 {
		return nil, nil
	}
	return m.taskAndProjectForIdentity(targets.Targets[0].Task)
}
