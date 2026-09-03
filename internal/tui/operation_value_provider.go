package tui

import (
	"github.com/sglavoie/tt/internal/tui/operation"
)

const (
	operationTaskSetState    operation.OperationID = "tui.task.set-state"
	operationTaskSetPriority operation.OperationID = "tui.task.set-priority"
	operationTaskSetDueDate  operation.OperationID = "tui.task.set-due-date"
	operationTaskSetTags     operation.OperationID = "tui.task.set-tags"
	operationTaskSetParent   operation.OperationID = "tui.task.set-parent"
	operationTaskIndent      operation.OperationID = "tui.task.indent"
	operationTaskOutdent     operation.OperationID = "tui.task.outdent"

	// operationTaskAdoptChildren is the `<child` sigil's key. It is a separate
	// operation from set-parent because the sigil is repeatable and additive,
	// which set-parent's single-valued replace semantics cannot express.
	operationTaskAdoptChildren operation.OperationID = "tui.task.adopt-children"

	// operationTaskMoveToProject is the `%project` sigil's key. It travels the
	// same cross-project edit path the sigil does rather than adding a second
	// relocation route.
	operationTaskMoveToProject operation.OperationID = "tui.task.move-to-project"

	// operationTaskReferences is the `~task` sigil's key. It is a separate
	// operation from every other relationship above because the sigil it answers
	// is unlike theirs on three counts: it is repeatable, so no single-valued
	// replace can express it; it crosses a project boundary, so the candidate
	// population is not one project's tree; and it is soft, so nothing about
	// readiness, ordering, ranking, or completion may read what it records.
	operationTaskReferences operation.OperationID = "tui.task.references"
)

// OperationValueSelectedMsg resumes a pending operation with the chosen value.
type OperationValueSelectedMsg struct {
	Request OperationExecutionRequest
	Value   any
}

// OperationValueCanceledMsg closes the value picker and returns to its action
// sheet without executing the operation.
type OperationValueCanceledMsg struct{}
