package operation

const (
	OperationTaskSetState    OperationID = "tui.task.set-state"
	OperationTaskSetPriority OperationID = "tui.task.set-priority"
	OperationTaskSetDueDate  OperationID = "tui.task.set-due-date"
	OperationTaskSetTags     OperationID = "tui.task.set-tags"
	OperationTaskSetParent   OperationID = "tui.task.set-parent"
	OperationTaskIndent      OperationID = "tui.task.indent"
	OperationTaskOutdent     OperationID = "tui.task.outdent"

	// OperationTaskAdoptChildren is the `<child` sigil's key. It is a separate
	// operation from set-parent because the sigil is repeatable and additive,
	// which set-parent's single-valued replace semantics cannot express.
	OperationTaskAdoptChildren OperationID = "tui.task.adopt-children"

	// OperationTaskMoveToProject is the `%project` sigil's key. It travels the
	// same cross-project edit path the sigil does rather than adding a second
	// relocation route.
	OperationTaskMoveToProject OperationID = "tui.task.move-to-project"

	// OperationTaskReferences is the `~task` sigil's key. It is a separate
	// operation from every other relationship above because the sigil it answers
	// is unlike theirs on three counts: it is repeatable, so no single-valued
	// replace can express it; it crosses a project boundary, so the candidate
	// population is not one project's tree; and it is soft, so nothing about
	// readiness, ordering, ranking, or completion may read what it records.
	OperationTaskReferences OperationID = "tui.task.references"
)

// OperationValueSelectedMsg resumes a pending operation with the chosen value.
type OperationValueSelectedMsg struct {
	Request OperationExecutionRequest
	Value   any
}

// OperationValueCanceledMsg closes the value picker and returns to its action
// sheet without executing the operation.
type OperationValueCanceledMsg struct{}
