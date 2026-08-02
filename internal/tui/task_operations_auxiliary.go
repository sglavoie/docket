package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

const (
	operationTaskChecklist            OperationID = "tui.task.checklist"
	operationTaskComments             OperationID = "tui.task.comments"
	operationTaskDecisions            OperationID = "tui.task.decisions"
	operationTaskAttachments          OperationID = "tui.task.attachments"
	operationTaskCopyTitle            OperationID = "tui.task.copy-title"
	operationTaskCopyDetails          OperationID = "tui.task.copy-details"
	operationTaskCopyTitleDescription OperationID = "tui.task.copy-title-description"
	operationTaskPin                  OperationID = "tui.task.pin"
	operationTaskUnpin                OperationID = "tui.task.unpin"
	operationTaskTogglePin            OperationID = "tui.task.toggle-pin"
	operationTaskNewChild             OperationID = "tui.task.new-child"
)

// taskAuxiliaryAllowsOperation gates the paired auxiliary actions and the child
// creators on live task state that the frozen EntityContext does not carry, so
// the two halves of pin are never offered together and a child is never offered
// under a task that cannot hold one. A target that no longer
// resolves stays applicable so the handler reports the single authoritative
// "no longer available" error instead of the row vanishing.
func (m *AppModel) taskAuxiliaryAllowsOperation(id OperationID, entity EntityContext) bool {
	switch id {
	case operationTaskPin, operationTaskUnpin,
		operationTaskNewChild:
	default:
		return true
	}
	task, _ := m.taskAndProjectForIdentity(TaskIdentity{
		ProjectID: entity.ProjectID,
		TaskID:    entity.TaskID,
	})
	if task == nil {
		return true
	}
	switch id {
	case operationTaskPin:
		return !task.Pinned
	case operationTaskUnpin:
		return task.Pinned
	case operationTaskNewChild:
		return taskAcceptsDeeperChild(task)
	default:
		return true
	}
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
	invocation OperationInvocation,
	identity TaskIdentity,
) OperationHandlerResult {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
	}
	message := func(msg any) OperationHandlerResult {
		return OperationHandlerResult{Message: msg}
	}
	coordinator := m.secondaryFeatures.TaskItem

	//nolint:exhaustive // Only auxiliary task operations reach this switch.
	switch invocation.Descriptor.ID {
	case operationTaskChecklist:
		return message(ShowTaskChecklistMsg{Task: task, Project: project})
	case operationTaskComments:
		return message(ShowTaskCommentsMsg{Task: task, Project: project})
	case operationTaskDecisions:
		return message(ShowTaskDecisionsMsg{Task: task, Project: project})
	case operationTaskAttachments:
		return message(ShowTaskAttachmentsMsg{Task: task, Project: project})
	case operationTaskCopyTitle:
		return message(CopyTaskTitleMsg{Task: task})
	case operationTaskCopyDetails:
		return message(CopyTaskDetailsMsg{Task: task})
	case operationTaskCopyTitleDescription:
		return message(CopyTaskTitleAndDescMsg{Task: task})
	case operationTaskPin:
		return message(OperationCommandResultMsg{Cmd: coordinator.SetPinned(identity, true)})
	case operationTaskUnpin:
		return message(OperationCommandResultMsg{Cmd: coordinator.SetPinned(identity, false)})
	case operationTaskNewChild:
		return message(CreateTaskMsg{ParentTask: task, Project: project})
	default:
		return OperationHandlerResult{Err: fmt.Errorf("%w: %s", ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

// taskAndProjectForTargets reloads the live task named by the first resolved
// target, so core task operations act on the frozen identity rather than
// whatever row the cursor has since moved to.
func (m *AppModel) taskAndProjectForTargets(
	targets ResolvedOperationTargets,
) (*models.Task, *models.Project) {
	if len(targets.Targets) == 0 {
		return nil, nil
	}
	return m.taskAndProjectForIdentity(targets.Targets[0].Task)
}
