package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// adoptChildrenCmd routes the picker's checked set through the same task-line
// edit the `<child` sigil uses, so adoption is validated as one hierarchy patch
// and lands in one transaction with one undo entry. The title is carried
// verbatim and never reparsed, and the request names no parent patch, so the
// adopting task keeps its own place in the tree.
func (m *AppModel) adoptChildrenCmd(
	identity taskref.TaskIdentity,
	childIDs []string,
	relocation *outlinerRelocationCapture,
) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	req := app.TaskLineEditRequest{
		Source:               identity.Locator(),
		Title:                task.Title,
		DestinationProjectID: identity.ProjectID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentPreserve},
		Action:               "Adopt Children",
		PlanningArtifacts:    shellPlanningReconcileArtifactBuilder(m.runtime, m.configManager),
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
func (m *AppModel) setTaskReferencesCmd(identity taskref.TaskIdentity, value OperationTaskReferenceValue) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	patch := taskReferencePatch(task, value)
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
		PlanningArtifacts:    shellPlanningReconcileArtifactBuilder(m.runtime, m.configManager),
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
	relocation *outlinerRelocationCapture,
) tea.Cmd {
	if len(identities) == 0 {
		return func() tea.Msg {
			return TransactionErrorMsg{Err: fmt.Errorf("select at least one task")}
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
	relocation *outlinerRelocationCapture,
) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	req := app.TaskLineEditRequest{
		Source:               identity.Locator(),
		Title:                task.Title,
		DestinationProjectID: projectID,
		Action:               "Move to Project",
		PlanningArtifacts:    shellStructuralPlanningArtifactBuilder(m.runtime, m.configManager),
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
	relocation *outlinerRelocationCapture,
) tea.Cmd {
	locators := make([]app.TaskLocator, 0, len(identities))
	for _, identity := range identities {
		if !identity.Valid() {
			return func() tea.Msg {
				return TransactionErrorMsg{Err: fmt.Errorf("selected task is no longer available")}
			}
		}
		locators = append(locators, identity.Locator())
	}
	destination := m.moveDestinationName(projectID)
	m.appService.WithPlanningArtifacts(shellStructuralPlanningArtifactBuilder(m.runtime, m.configManager))
	cmd := m.secondaryFeatures.Bulk.MoveToProjectLocators(locators, projectID, func(moved int) string {
		return fmt.Sprintf("Moved %d task(s) to %s", moved, destination)
	})
	return finalizeStaleSelectionBatchResult(withOutlinerRelocationCapture(cmd, relocation), locators)
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
	invocation OperationInvocation,
) OperationHandlerResult {
	identities := make([]taskref.TaskIdentity, 0, len(invocation.Targets.Targets))
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
		}
		identities = append(identities, target.Task)
		locators = append(locators, target.Task.Locator())
	}
	cmdResult := func(cmd tea.Cmd) OperationHandlerResult {
		return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
	}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.TransitionLocators(
			locators,
			state,
			func(changed int) string {
				return fmt.Sprintf("Changed %d task(s) to %s", changed, state)
			},
		))
	case operationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetPriorityLocators(locators, priority))
	case operationTaskSetDueDate:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid date value")}
		}
		if value.Deferred {
			return cmdResult(m.secondaryFeatures.Bulk.DeferLocators(locators, cloneOperationDate(value.Date)))
		}
		if value.Date == nil {
			return cmdResult(m.secondaryFeatures.Bulk.ClearDueDateLocators(locators))
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetDueDateLocators(
			locators,
			*cloneOperationDate(value.Date),
		))
	case operationTaskMoveToProject:
		projectID, ok := invocation.Value.(string)
		if !ok || strings.TrimSpace(projectID) == "" {
			return OperationHandlerResult{Err: fmt.Errorf("invalid project value")}
		}
		return cmdResult(m.moveTaskToProjectCmd(
			identities,
			projectID,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	default:
		return OperationHandlerResult{
			Err: fmt.Errorf("operation %s is not available for %d selected tasks", invocation.Descriptor.ID, count),
		}
	}
}

func cloneOperationDate(date *time.Time) *time.Time {
	if date == nil {
		return nil
	}
	cloned := *date
	return &cloned
}
