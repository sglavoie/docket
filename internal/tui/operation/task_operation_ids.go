package operation

// The single-task operation IDs. They are `OperationID` vocabulary rather than
// descriptors or handlers, so they sit beside the registry that owns the type
// while the surfaces that register and execute them stay above it: the
// simplified input strategy, the task manifest, and the task action sheet all
// name them, and only the first of those is inside this package.
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

	OperationTaskComments             OperationID = "tui.task.comments"
	OperationTaskAttachments          OperationID = "tui.task.attachments"
	OperationTaskCopyTitle            OperationID = "tui.task.copy-title"
	OperationTaskCopyDetails          OperationID = "tui.task.copy-details"
	OperationTaskCopyTitleDescription OperationID = "tui.task.copy-title-description"
	OperationTaskTogglePin            OperationID = "tui.task.toggle-pin"
	OperationTaskNewChild             OperationID = "tui.task.new-child"

	// OperationTaskToggleReview is the `=review` sigil's key. Review is a stored
	// boolean axis beside Pinned rather than a lifecycle state, so it toggles the
	// way pinning does instead of joining the `$` destinations, and `f=` keeps
	// narrowing by the same flag this writes.
	OperationTaskToggleReview OperationID = "tui.task.toggle-review"

	// OperationTaskSetResumeNote writes, replaces, or clears the one-line resume
	// note. It is a content edit rather than a workflow act: every lifecycle state
	// may carry a note, and pausing, starting, or completing a task neither
	// prompts for one nor clears the one already stored.
	OperationTaskSetResumeNote OperationID = "tui.task.set-resume-note"

	// OperationTaskAddComment appends one comment to one task without leaving the
	// list. It is a separate operation from OperationTaskComments because that
	// one zooms into the panel that owns reading, editing, reordering, and
	// deleting comments, and this one only writes a new one; both commit through
	// the same Service.AddComment, so the two routes cannot disagree about what a
	// comment is.
	OperationTaskAddComment OperationID = "tui.task.add-comment"

	// OperationTaskRepeat opens a fresh creation draft seeded from one task. It
	// is neither a copy of the task nor a relationship to it: nothing links the
	// two afterwards, and no recurrence is recorded. The operation only prepares
	// a draft, so pressing it writes nothing; the ordinary creation transaction
	// is still what mints the identity, timestamps, and undo entry if the human
	// saves.
	OperationTaskRepeat OperationID = "tui.task.repeat"

	// OperationTaskDelete carries no direct key. Delete is the whole task
	// `Space` menu and confirms first, so the ID is manifest vocabulary the
	// sheet reads rather than a binding the strategy declares.
	OperationTaskDelete OperationID = "tui.task.delete"
)
