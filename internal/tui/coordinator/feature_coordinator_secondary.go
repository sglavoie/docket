package coordinator

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// This file extends the feature-coordinator seam (feature_coordinator.go) to the
// secondary TUI views and overlays: bulk operations, attachments, and tags.
// Historically
// each of these mutated its own model or config copy and called a direct
// saveProjectSync / persister.Save / configManager.Save helper. That left every
// overlay re-implementing the load/mutate/save dance with no single durable seam.
//
// Where the primary coordinators depend on the fat FeatureMutator, the secondary
// coordinators each depend on a NARROW host port describing exactly the
// application use cases that one feature performs. *app.Service satisfies every
// task/artifact port, so a production coordinator is built from the same
// service the primary views use,
// while a scenario test can drive each coordinator against an in-memory service
// with the same use-case semantics. Splitting the ports per feature keeps a
// bulk-only view from depending on attachment methods it never calls.

// BulkMutator is the narrow port the bulk-operation overlays depend on. Every
// method is project-qualified: the workspace-unique task-ID forms are gone, so a
// bulk overlay cannot fall back to workspace iteration order.
type BulkMutator interface {
	// BulkTransition moves every task to the target workflow state,
	// aborting the whole batch on the first invalid transition.
	BulkTransition(locators []app.TaskLocator, target models.WorkflowState) (app.BulkResult, error)
	// BulkDelete permanently removes every requested subtree, folding any durable
	// artifact and cross-project reference cleanup into the same transaction.
	BulkDelete(req app.BulkDeleteRequest, artifacts ...storage.ArtifactChange) (app.BulkDeleteResult, error)
	// BulkSetPriority sets one priority on every selected task.
	BulkSetPriority(locators []app.TaskLocator, priority int) (app.BulkResult, error)
	// BulkSetDueDate sets one due date on every selected task, and the
	// PlanToday twin folds Today planning into the same transaction.
	BulkSetDueDate(locators []app.TaskLocator, due time.Time) (app.BulkResult, error)
	BulkSetDueDateAndPlanToday(locators []app.TaskLocator, due time.Time) (app.BulkResult, error)
	// BulkClearDueDate removes the due date from every selected task.
	BulkClearDueDate(locators []app.TaskLocator) (app.BulkResult, error)
	// BulkDefer hides every selected task until until, or clears the
	// deferral when until is nil.
	BulkDefer(locators []app.TaskLocator, until *time.Time) (app.BulkResult, error)
	// BulkMoveToProject relocates every selected subtree into one destination
	// project, folding the source projects, the destination, the planning
	// requalification, the attachment bytes, and every soft reference that had to
	// follow into one transaction.
	BulkMoveToProject(locators []app.TaskLocator, destProjectID string) (app.BulkResult, error)
	// ApplyPlanningBulk applies Today, Next, or Backlog semantics to the whole
	// qualified selection in one task-and-planning transaction.
	//
	// Setting tags has no port method at all: `#` is a value operation, and
	// applicationTaskOperationRequest claims operationTaskSetTags before either
	// the single-task or the bulk value dispatch is reached, so the whole tags
	// path — patch payload and singleton context alike — commits through
	// TaskItem.ExecuteOperation. Do not reintroduce a BulkCoordinator wrapper
	// here; it would be unreachable.
	ApplyPlanningBulk(req app.PlanningBulkRequest) (app.BulkResult, error)
}

// TaskItemMutator is the narrow port the single-task outliner mutation handlers
// depend on. Each method commits ONE single-task or project-scoped transaction
// through the ledger, mirroring BulkMutator but for one task at a time. Every
// cross-project single-task handler now routes through it; the legacy raw-save
// family (crossPersistFn/crossSaveProject) it replaced has been removed.
type TaskItemMutator interface {
	// ExecuteTaskOperation runs the presentation-neutral daily operation path for
	// one or many project-qualified targets.
	ExecuteTaskOperation(req app.TaskOperationRequest) (app.TaskOperationResult, error)
	// MoveTaskToTop moves one task to the top of its model-order siblings.
	MoveTaskToTop(locator app.TaskLocator) (app.TaskResult, error)
	// MoveTaskToBottom moves one task to the bottom of its model-order siblings.
	MoveTaskToBottom(locator app.TaskLocator) (app.TaskResult, error)
	// MoveTaskUp swaps one task with its previous model-order sibling.
	MoveTaskUp(locator app.TaskLocator) (app.TaskResult, error)
	// MoveTaskDown swaps one task with its next model-order sibling.
	MoveTaskDown(locator app.TaskLocator) (app.TaskResult, error)
	// ReorderSiblingTasks swaps two explicit same-project same-parent tasks.
	ReorderSiblingTasks(task, target app.TaskLocator) (app.TaskResult, error)
	// ReorderSiblingOrder permutes an explicit same-project same-parent set into
	// the given top-to-bottom order.
	ReorderSiblingOrder(moved app.TaskLocator, order []app.TaskLocator) (app.TaskResult, error)
	// IndentTask nests one task under the task visually/model above it.
	IndentTask(locator app.TaskLocator) (app.TaskResult, error)
	// OutdentTask moves one task to become a sibling of its current parent.
	OutdentTask(locator app.TaskLocator) (app.TaskResult, error)
	// Transition moves one task to a target workflow state.
	Transition(req app.StateRequest) (app.TaskResult, error)
	// Edit applies content changes to one task.
	Edit(req app.EditRequest) (app.TaskResult, error)
	// EditTaskLine applies a parsed inline patch and optional subtree move as one
	// atomic workspace transaction.
	EditTaskLine(req app.TaskLineEditRequest) (app.TaskLineEditResult, error)
	// Capture creates a new task in a project and commits it as one ChangeSet.
	Capture(req app.CaptureRequest) (app.TaskResult, error)
	// SetPriority sets one task's Unset/High/Medium/Low priority.
	SetPriority(locator app.TaskLocator, priority int) (app.TaskResult, error)
	// SetDueDate sets or clears (nil due) one task's due date.
	SetDueDate(locator app.TaskLocator, due *time.Time) (app.TaskResult, error)
	SetDueDateAndPlanToday(locator app.TaskLocator, due time.Time) (app.TaskResult, error)
	// Defer hides one task until the given date, or clears the deferral when nil.
	Defer(locator app.TaskLocator, until *time.Time) (app.TaskResult, error)
	// SetPinned pins or unpins one task.
	SetPinned(locator app.TaskLocator, pinned bool) (app.TaskResult, error)
	// ReorderPinnedTasks swaps two pinned tasks' PinnedAt ordering as one
	// (possibly cross-project) transaction.
	ReorderPinnedTasks(taskA, taskB app.TaskLocator) (app.PinnedReorderResult, error)
	// ReorderPinnedTasksOrder rewrites a whole pinned chain's PinnedAt ordering
	// into the given top-to-bottom order as one transaction.
	ReorderPinnedTasksOrder(moved app.TaskLocator, order []app.TaskLocator) (app.PinnedReorderResult, error)
	// SetResumeNote writes, replaces, or clears (empty note) one task's resume
	// note, reporting whether anything actually changed.
	SetResumeNote(locator app.TaskLocator, note string) (app.ResumeNoteResult, error)
	// MarkReviewed acknowledges one task a person raised, rechecking eligibility
	// against the service clock before it clears the flag, records the review,
	// and moves any repeating check-in on.
	MarkReviewed(locator app.TaskLocator) (app.TaskResult, error)
	// Reparent changes one task's parent within its owning project.
	Reparent(locator app.TaskLocator, newParentID *string) (app.TaskResult, error)
	// Delete permanently removes one task's subtree, folding any caller-supplied
	// durable planning-ref artifacts into the same transaction.
	Delete(req app.DeleteRequest, artifacts ...storage.ArtifactChange) (app.DeleteResult, error)
	// CommitProject commits an already-mutated project through the transactional
	// repository and records one grouped ledger entry.
	CommitProject(project *models.Project, opts app.ProjectCommitOptions) (app.BulkResult, error)
}

// AttachmentMutator is the narrow port the attachment overlay depends on. File
// attachment and removal ride their byte-copy/delete side effects on the same
// journaled ChangeSet as the metadata write.
type AttachmentMutator interface {
	AttachURL(locator app.TaskLocator, url string) (app.TaskResult, error)
	AttachCodeReference(locator app.TaskLocator, path string, line int) (app.TaskResult, error)
	AttachFile(locator app.TaskLocator, sourcePath string) (app.AttachFileResult, error)
	// RemoveAttachmentExpecting removes the attachment at index only while it is
	// still expected, so a confirmed removal never lands on a neighbor a reload
	// shifted into that position.
	RemoveAttachmentExpecting(locator app.TaskLocator, index int, expected models.Attachment, deleteFile bool) (app.RemoveAttachmentResult, error)
	ReorderAttachment(locator app.TaskLocator, from, to int) (app.TaskResult, error)
}

// SubviewMutator is the narrow port the task subview panels (comments and
// checklist) depend on. Every method commits one ChangeSet and checks bounds
// against a fresh snapshot. Comment edits and removals also check the stored
// value the panel read, so a shifted index cannot redirect a mutation.
type SubviewMutator interface {
	AddComment(locator app.TaskLocator, text string) (app.CommentResult, error)
	EditComment(locator app.TaskLocator, index int, expected models.Comment, text string) (app.CommentResult, error)
	RemoveComment(locator app.TaskLocator, index int, expected models.Comment) (app.CommentResult, error)
	AddChecklistItem(locator app.TaskLocator, text string) (app.ChecklistResult, error)
	EditChecklistItem(locator app.TaskLocator, index int, text string) (app.ChecklistResult, error)
	RemoveChecklistItem(locator app.TaskLocator, index int) (app.ChecklistResult, error)
	ToggleChecklistItem(locator app.TaskLocator, index int) (app.ChecklistResult, error)
	ReorderChecklistItem(locator app.TaskLocator, from, to int) (app.ChecklistResult, error)
	MoveChecklistItem(locator app.TaskLocator, from, to int) (app.ChecklistResult, error)
	// PromoteChecklistItem converts one checklist item into a child task of its
	// owner in one transaction. It is the one checklist method that does not take
	// a bare index: a checklist item stores no identity, so the request carries
	// the complete ordered checklist the panel read and the promotion is refused
	// whole unless the owner still holds exactly that list.
	PromoteChecklistItem(req app.ChecklistPromotionRequest) (app.ChecklistPromotionResult, error)
	MakeChecklistItem(req app.ChecklistDemotionRequest) (app.ChecklistDemotionResult, error)
}

// Compile-time assurance that the production services satisfy every secondary
// port, so the coordinators can be built from one *app.Service.
var (
	_ BulkMutator           = (*app.Service)(nil)
	_ TaskItemMutator       = (*app.Service)(nil)
	_ AttachmentMutator     = (*app.Service)(nil)
	_ SubviewMutator        = (*app.Service)(nil)
	_ ProjectCommentMutator = (*app.Service)(nil)
)

// BatchTransactionResultMsg is emitted when a project-scoped or multi-task
// mutation commits (bulk transition/delete and metadata updates). It carries the
// affected project IDs so the shell reloads exactly those projects, and a
// presentation-neutral summary for the status line.
type BatchTransactionResultMsg struct {
	Tasks             []app.TaskResult
	Action            string
	TouchedProjectIDs []string
	Summary           string
	TransactionID     string
	// Relocation is the opaque navigation capture attached only to a successful
	// state/planning/metadata mutation. It is nil whenever the operation cannot
	// relocate anything.
	Relocation TransactionRelocation
	// ClearSelectionTaskIDs are compatibility identifiers whose multiselect state
	// the shell clears once the batch commits.
	ClearSelectionTaskIDs []string
	// ClearSelectionTaskLocators is the authoritative selection clear set for
	// cross-project identities where task IDs may collide. Only a batch that
	// invalidates the locators it was given sets it: a delete removes them, and a
	// cross-project move leaves every one of them naming a project the task no
	// longer lives in.
	ClearSelectionTaskLocators []app.TaskLocator
	// CommentID is the project comment the batch committed, and is set only by
	// the project-comment mutations, which are the only ones that address a
	// minted identity. It is what lets a surface point at the comment it just
	// wrote instead of inferring it from a reloaded list, where another writer's
	// addition is indistinguishable from this one.
	CommentID string
	// DueEffect is the typed evidence this transaction carries about deadlines
	// and task addresses, for the session clock memory (see TransactionResultMsg).
	DueEffect CommittedDueEffect
	// CommittedTarget is the pre-commit address of the one task this batch was
	// aimed at, for the session undo-focus memory. Only the seams that dispatch a
	// single-task gesture through the batch path set it: a bulk gesture names
	// several tasks and a project-scoped commit names none, and both leave the
	// zero value rather than electing one of the addresses in DueEffect, whose
	// Moves and Removed lists are full of descendants nobody pointed at.
	CommittedTarget app.TaskLocator
	// RecoveryContext replaces the ledger entry's own context in the undoable
	// toast. The deadline stepper is its one writer: what its ledger entry
	// records is the gesture (`+1 day`), which is what the undo browser should
	// say, while the press the user just made must be answered with the deadline
	// it produced. An empty value leaves the ledger's context alone.
	RecoveryContext string
	// RecoveryHeadline replaces the ledger entry's action and context together
	// in the undoable toast, for an outcome the `Action: context` sentence cannot
	// say. Hiding is its one writer: the rows leave the view, so the toast names
	// the key that brings them back. An empty value leaves both alone.
	RecoveryHeadline string
}

// FinalizeStaleSelectionBatchResult clears exactly the project-qualified
// locators a committed batch invalidated — the roots a delete removed, or the
// source addresses a cross-project move emptied. Canceled and failed batches
// never produce a successful result and therefore leave selection intact.
func FinalizeStaleSelectionBatchResult(cmd tea.Cmd, locators []app.TaskLocator) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		batch, ok := msg.(BatchTransactionResultMsg)
		if !ok {
			return msg
		}
		batch.ClearSelectionTaskLocators = append([]app.TaskLocator(nil), locators...)
		batch.ClearSelectionTaskIDs = make([]string, 0, len(locators))
		for _, locator := range locators {
			batch.ClearSelectionTaskIDs = append(batch.ClearSelectionTaskIDs, locator.TaskID)
		}
		return batch
	}
}

// StampCommittedFocusTarget names, on the batch a gesture produced, the one
// task the gesture was aimed at. It decorates rather than threading the address
// through runBatch because the address is the caller's own argument and the
// mutation closure never sees it. A canceled or failed gesture resolves to
// TransactionErrorMsg and is left untouched, so nothing can correlate a
// transaction that never committed.
//
// It is exported for the one dispatch that lives in the shell rather than in a
// coordinator: the canonical task deletion commits through the batch delete for
// every cardinality, so only the shell that armed the prompt knows whether the
// gesture named one task. An invalid target leaves the command undecorated,
// which is what keeps a multi-target gesture uncorrelated.
func StampCommittedFocusTarget(cmd tea.Cmd, target app.TaskLocator) tea.Cmd {
	if cmd == nil || !target.Valid() {
		return cmd
	}
	return func() tea.Msg {
		msg := cmd()
		batch, ok := msg.(BatchTransactionResultMsg)
		if !ok {
			return msg
		}
		batch.CommittedTarget = target
		return batch
	}
}

// SuccessMessage renders the status line shown when the batch commits.
func (m BatchTransactionResultMsg) SuccessMessage() string {
	if m.Summary != "" {
		return m.Summary
	}
	return m.Action
}

// runTask wraps a single-task application mutation as an async command resolving
// to a TransactionResultMsg (or TransactionErrorMsg on rejection), reusing the
// primary seam's message contract so migrated overlays render identically.
func runTask(action string, mutate func() (app.TaskResult, error)) tea.Cmd {
	return func() tea.Msg {
		res, err := mutate()
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return TransactionResultMsg{Action: action, Result: res}
	}
}

// runBatch wraps a project-scoped or multi-task mutation as an async command
// resolving to a BatchTransactionResultMsg (or TransactionErrorMsg on rejection).
func runBatch(action string, mutate func() ([]string, string, string, error)) tea.Cmd {
	return func() tea.Msg {
		touched, summary, transactionID, err := mutate()
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return BatchTransactionResultMsg{
			Action: action, TouchedProjectIDs: touched, Summary: summary, TransactionID: transactionID,
		}
	}
}

// runBatchWithDueEffect is runBatch for the mutations that also report typed
// deadline evidence. The effect rides back from the same call that produced the
// transaction, so the memory it feeds and the commit it describes can never come
// from two different reads.
func runBatchWithDueEffect(action string, mutate func() ([]string, string, string, CommittedDueEffect, error)) tea.Cmd {
	return func() tea.Msg {
		touched, summary, transactionID, effect, err := mutate()
		if err != nil {
			return TransactionErrorMsg{Err: err}
		}
		return BatchTransactionResultMsg{
			Action: action, TouchedProjectIDs: touched, Summary: summary, TransactionID: transactionID,
			DueEffect: effect,
		}
	}
}

// --- Secondary feature coordinators ----------------------------------------

// SecondaryCoordinators aggregates the secondary-view coordinators the AppModel
// hands to retained overlays as they migrate onto the application layer.
type SecondaryCoordinators struct {
	Bulk           BulkCoordinator
	TaskItem       TaskItemCoordinator
	Attachment     AttachmentCoordinator
	Subview        SubviewCoordinator
	ProjectComment ProjectCommentCoordinator
}

// NewSecondaryCoordinators builds the secondary coordinators over the shared
// application service. Each coordinator stores only its narrow
// host port even though every task/artifact port is satisfied by the one service.
// In production svc is the TUI app service; scenario tests inject an in-memory
// service.
func NewSecondaryCoordinators(svc *app.Service) SecondaryCoordinators {
	return SecondaryCoordinators{
		Bulk:           BulkCoordinator{bulk: svc},
		TaskItem:       TaskItemCoordinator{task: svc},
		Attachment:     NewAttachmentCoordinator(svc),
		Subview:        SubviewCoordinator{sub: svc},
		ProjectComment: NewProjectCommentCoordinator(svc),
	}
}
