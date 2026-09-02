package tui

import (
	"time"
)

const (
	operationTaskSetState    OperationID = "tui.task.set-state"
	operationTaskSetPriority OperationID = "tui.task.set-priority"
	operationTaskSetDueDate  OperationID = "tui.task.set-due-date"
	operationTaskSetTags     OperationID = "tui.task.set-tags"
	operationTaskSetParent   OperationID = "tui.task.set-parent"
	operationTaskIndent      OperationID = "tui.task.indent"
	operationTaskOutdent     OperationID = "tui.task.outdent"

	// operationTaskAdoptChildren is the `<child` sigil's key. It is a separate
	// operation from set-parent because the sigil is repeatable and additive,
	// which set-parent's single-valued replace semantics cannot express.
	operationTaskAdoptChildren OperationID = "tui.task.adopt-children"

	// operationTaskMoveToProject is the `%project` sigil's key. It travels the
	// same cross-project edit path the sigil does rather than adding a second
	// relocation route.
	operationTaskMoveToProject OperationID = "tui.task.move-to-project"

	// operationTaskReferences is the `~task` sigil's key. It is a separate
	// operation from every other relationship above because the sigil it answers
	// is unlike theirs on three counts: it is repeatable, so no single-valued
	// replace can express it; it crosses a project boundary, so the candidate
	// population is not one project's tree; and it is soft, so nothing about
	// readiness, ordering, ranking, or completion may read what it records.
	operationTaskReferences OperationID = "tui.task.references"
)

// OperationParentValue is the picker-neutral reparent payload. A nil ParentID
// is the project root, which is what the `>0` edit sigil expresses.
type OperationParentValue struct {
	ParentID *string
}

// OperationDateValue is the picker-neutral due/defer payload. Nil clears the
// field. Deferred names which of the two fields the value writes, because one
// picker answers both.
type OperationDateValue struct {
	Date     *time.Time
	Deferred bool
}

// OperationTagsValue is the tri-state bulk tag edit emitted by the value
// picker. Add and Remove are uniform patches; tags in neither set preserve
// their existing per-task membership. ContextSet carries the singleton task
// context in the same payload so both land in one transaction; an empty Context
// clears the field.
type OperationTagsValue struct {
	Add        []string
	Remove     []string
	ContextSet bool
	Context    string
}

// OperationTaskReferenceValue is the soft-reference edit emitted by the `~`
// picker. It is a difference rather than the final checked set: Add names the
// rows the user checked that the picker had not preselected, Remove the
// preselected rows the user unchecked, and a reference in neither half is one
// the user never touched and the commit must leave alone.
type OperationTaskReferenceValue struct {
	Add    []string
	Remove []string
}

// OperationValueSelectedMsg resumes a pending operation with the chosen value.
type OperationValueSelectedMsg struct {
	Request OperationExecutionRequest
	Value   any
}

// OperationValueCanceledMsg closes the value picker and returns to its action
// sheet without executing the operation.
type OperationValueCanceledMsg struct{}
