package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
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
		return fmt.Sprintf("Moved %d task(s) to %s", moved, destination)
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
				return fmt.Sprintf("Changed %d task(s) to %s", changed, state)
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
