package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

const (
	operationTaskComments             OperationID = "tui.task.comments"
	operationTaskAttachments          OperationID = "tui.task.attachments"
	operationTaskCopyTitle            OperationID = "tui.task.copy-title"
	operationTaskCopyDetails          OperationID = "tui.task.copy-details"
	operationTaskCopyTitleDescription OperationID = "tui.task.copy-title-description"
	operationTaskTogglePin            OperationID = "tui.task.toggle-pin"
	operationTaskNewChild             OperationID = "tui.task.new-child"
)

// taskAuxiliaryAllowsOperation gates the child creator on live task state that
// the frozen EntityContext does not carry, so a child is never offered under a
// task that cannot hold one. A target that no longer resolves stays applicable
// so the handler reports the single authoritative "no longer available" error
// instead of the row vanishing.
func (m *AppModel) taskAuxiliaryAllowsOperation(id OperationID, entity EntityContext) bool {
	if id != operationTaskNewChild {
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
	invocation OperationInvocation,
	identity taskref.TaskIdentity,
) OperationHandlerResult {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
	}
	message := func(msg any) OperationHandlerResult {
		return OperationHandlerResult{Message: msg}
	}

	//nolint:exhaustive // Only auxiliary task operations reach this switch.
	switch invocation.Descriptor.ID {
	case operationTaskComments:
		return message(ShowTaskCommentsMsg{Task: task, Project: project})
	case operationTaskAttachments:
		return message(ShowTaskAttachmentsMsg{Task: task, Project: project})
	case operationTaskCopyTitle:
		return message(CopyTaskTitleMsg{Task: task})
	case operationTaskCopyDetails:
		return message(CopyTaskDetailsMsg{Task: task})
	case operationTaskCopyTitleDescription:
		return message(CopyTaskTitleAndDescMsg{Task: task})
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
