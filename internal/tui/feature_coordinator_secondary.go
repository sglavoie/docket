package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// This file extends the feature-coordinator seam (feature_coordinator.go) to the
// secondary TUI views and overlays: bulk operations, attachments, tags, and blockers.
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

// BulkMutator is the narrow port the bulk-operation overlays depend on. Both
// methods commit their whole batch as ONE atomic cross-project transaction.
type BulkMutator interface {
	// BulkTransition moves every task to the target workflow state, aborting the
	// whole batch on the first invalid transition.
	BulkTransition(taskIDs []string, target models.WorkflowState) (app.BulkResult, error)
	BulkTransitionByLocators(locators []app.TaskLocator, target models.WorkflowState) (app.BulkResult, error)
	// BulkDelete permanently removes every requested subtree, folding any durable
	// artifact and cross-project reference cleanup into the same transaction.
	BulkDelete(req app.BulkDeleteRequest, artifacts ...storage.ArtifactChange) (app.BulkDeleteResult, error)
	// BulkSetPriority sets one priority on every selected task.
	BulkSetPriority(taskIDs []string, priority int) (app.BulkResult, error)
	BulkSetPriorityByLocators(locators []app.TaskLocator, priority int) (app.BulkResult, error)
	// BulkSetDueDate sets one due date on every selected task.
	BulkSetDueDate(taskIDs []string, due time.Time) (app.BulkResult, error)
	BulkSetDueDateByLocators(locators []app.TaskLocator, due time.Time) (app.BulkResult, error)
	// BulkClearDueDate removes the due date from every selected task.
	BulkClearDueDate(taskIDs []string) (app.BulkResult, error)
	BulkClearDueDateByLocators(locators []app.TaskLocator) (app.BulkResult, error)
	// BulkSnooze hides every selected task until until, or clears the snooze when
	// until is nil.
	BulkSnooze(taskIDs []string, until *time.Time) (app.BulkResult, error)
	BulkSnoozeByLocators(locators []app.TaskLocator, until *time.Time) (app.BulkResult, error)
	// BulkSetTags replaces the tag set on every selected task with tags.
	BulkSetTags(taskIDs []string, tags []string) (app.BulkResult, error)
	BulkSetTagsByLocators(locators []app.TaskLocator, tags []string) (app.BulkResult, error)
	// ApplyPlanningBulk applies Today, Next, or Backlog semantics to the whole
	// qualified selection in one task-and-planning transaction.
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
	MoveTaskToTopByLocator(locator app.TaskLocator) (app.TaskResult, error)
	// MoveTaskToBottom moves one task to the bottom of its model-order siblings.
	MoveTaskToBottomByLocator(locator app.TaskLocator) (app.TaskResult, error)
	// MoveTaskUp swaps one task with its previous model-order sibling.
	MoveTaskUpByLocator(locator app.TaskLocator) (app.TaskResult, error)
	// MoveTaskDown swaps one task with its next model-order sibling.
	MoveTaskDownByLocator(locator app.TaskLocator) (app.TaskResult, error)
	// ReorderSiblingTasks swaps two explicit same-project same-parent tasks.
	ReorderSiblingTasksByLocator(task, target app.TaskLocator) (app.TaskResult, error)
	// IndentTask nests one task under the task visually/model above it.
	IndentTaskByLocator(locator app.TaskLocator) (app.TaskResult, error)
	// OutdentTask moves one task to become a sibling of its current parent.
	OutdentTaskByLocator(locator app.TaskLocator) (app.TaskResult, error)
	// Transition moves one task to a target workflow state.
	Transition(req app.StateRequest) (app.TaskResult, error)
	// Edit applies content changes to one task.
	Edit(req app.EditRequest) (app.TaskResult, error)
	// Capture creates a new task in a project and commits it as one ChangeSet.
	Capture(req app.CaptureRequest) (app.TaskResult, error)
	// SetPriority sets one task's 1..PriorityMax priority.
	SetPriorityByLocator(locator app.TaskLocator, priority int) (app.TaskResult, error)
	// SetDueDate sets or clears (nil due) one task's due date.
	SetDueDateByLocator(locator app.TaskLocator, due *time.Time) (app.TaskResult, error)
	// Snooze hides one task until the given date, or clears the snooze when nil.
	SnoozeByLocator(locator app.TaskLocator, until *time.Time) (app.TaskResult, error)
	// SetPinned pins or unpins one task.
	SetPinnedByLocator(locator app.TaskLocator, pinned bool) (app.TaskResult, error)
	// ReorderPinnedTasks swaps two pinned tasks' PinnedAt ordering as one
	// (possibly cross-project) transaction.
	ReorderPinnedTasksByLocator(taskA, taskB app.TaskLocator) (app.PinnedReorderResult, error)
	// Reparent changes one task's parent within its owning project.
	ReparentByLocator(locator app.TaskLocator, newParentID *string) (app.TaskResult, error)
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
	AttachURLByLocator(locator app.TaskLocator, url string) (app.TaskResult, error)
	AttachCodeReferenceByLocator(locator app.TaskLocator, path string, line int) (app.TaskResult, error)
	AttachFileByLocator(locator app.TaskLocator, sourcePath string) (app.AttachFileResult, error)
	RemoveAttachmentByLocator(locator app.TaskLocator, index int, deleteFile bool) (app.RemoveAttachmentResult, error)
	ReorderAttachmentByLocator(locator app.TaskLocator, from, to int) (app.TaskResult, error)
}

// TagMutator is the narrow port task-tag overlays depend on.
type TagMutator interface {
	AddTagByLocator(locator app.TaskLocator, tag string) (app.TaskResult, error)
	RemoveTagByLocator(locator app.TaskLocator, tag string) (app.TaskResult, error)
}

// SubviewMutator is the narrow port the task subview panels (comments,
// decisions, checklist) depend on. Every method commits one ChangeSet and
// re-validates the item index against a fresh snapshot, so a stale panel index
// surfaces as an error instead of silently targeting the wrong item.
type SubviewMutator interface {
	AddCommentByLocator(locator app.TaskLocator, text string) (app.CommentResult, error)
	EditCommentByLocator(locator app.TaskLocator, index int, text string) (app.CommentResult, error)
	RemoveCommentByLocator(locator app.TaskLocator, index int) (app.CommentResult, error)
	AddDecisionByLocator(locator app.TaskLocator, text, rationale string) (app.DecisionResult, error)
	EditDecisionByLocator(locator app.TaskLocator, index int, text, rationale string) (app.DecisionResult, error)
	RemoveDecisionByLocator(locator app.TaskLocator, index int) (app.DecisionResult, error)
	AddChecklistItemByLocator(locator app.TaskLocator, text string) (app.ChecklistResult, error)
	EditChecklistItemByLocator(locator app.TaskLocator, index int, text string) (app.ChecklistResult, error)
	RemoveChecklistItemByLocator(locator app.TaskLocator, index int) (app.ChecklistResult, error)
	ToggleChecklistItemByLocator(locator app.TaskLocator, index int) (app.ChecklistResult, error)
	ReorderChecklistItemByLocator(locator app.TaskLocator, from, to int) (app.ChecklistResult, error)
}

// Compile-time assurance that the production services satisfy every secondary
// port, so the coordinators can be built from one *app.Service.
var (
	_ BulkMutator       = (*app.Service)(nil)
	_ TaskItemMutator   = (*app.Service)(nil)
	_ AttachmentMutator = (*app.Service)(nil)
	_ TagMutator        = (*app.Service)(nil)
	_ SubviewMutator    = (*app.Service)(nil)
)

// BatchTransactionResultMsg is emitted when a project-scoped or multi-task
// mutation commits (bulk transition/delete, blocker updates). It carries the
// affected project IDs so the shell reloads exactly those projects, and a
// presentation-neutral summary for the status line.
type BatchTransactionResultMsg struct {
	Feature           string
	Action            string
	TouchedProjectIDs []string
	Summary           string
	// ClearSelectionTaskIDs are the task IDs whose multiselect state the shell
	// clears once the batch commits. Clearing happens only on this success path, so
	// a TransactionErrorMsg (which never reaches handleBatchTransactionResult)
	// leaves the selection intact for the user to retry. Nil when the caller owns
	// its own selection lifecycle.
	ClearSelectionTaskIDs []string
	// ClearSelectionTaskLocators is the project-qualified equivalent used by
	// cross-project selections where task IDs may collide.
	ClearSelectionTaskLocators []app.TaskLocator
	// RemovedTaskIDs lists every task UUID a bulk delete removed (roots plus
	// descendants). Nil for non-delete mutations.
	RemovedTaskIDs []string
}

// finalizeBatchResult decorates a coordinator batch command with the shell-side
// bookkeeping the async result path needs: the selected task IDs to clear from
// multiselect after a successful commit and an optional per-view summary
// override. A committed BatchTransactionResultMsg is enriched in place; a
// TransactionErrorMsg (or any other message) passes through untouched, so the
// selection survives a rejected batch. summaryOverride == "" keeps the
// coordinator's own summary.
func finalizeBatchResult(cmd tea.Cmd, clearIDs []string, summaryOverride string) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		batch, ok := msg.(BatchTransactionResultMsg)
		if !ok {
			return msg
		}
		batch.ClearSelectionTaskIDs = clearIDs
		if summaryOverride != "" {
			batch.Summary = summaryOverride
		}
		return batch
	}
}

func finalizeLocatedBatchResult(cmd tea.Cmd, locators []app.TaskLocator, summaryOverride string) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		batch, ok := msg.(BatchTransactionResultMsg)
		if !ok {
			return msg
		}
		batch.ClearSelectionTaskLocators = locators
		batch.ClearSelectionTaskIDs = make([]string, 0, len(locators))
		for _, locator := range locators {
			batch.ClearSelectionTaskIDs = append(batch.ClearSelectionTaskIDs, locator.TaskID)
		}
		if summaryOverride != "" {
			batch.Summary = summaryOverride
		}
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
func runTask(feature, action string, mutate func() (app.TaskResult, error)) tea.Cmd {
	return func() tea.Msg {
		res, err := mutate()
		if err != nil {
			return TransactionErrorMsg{Feature: feature, Action: action, Err: err}
		}
		return TransactionResultMsg{Feature: feature, Action: action, Result: res}
	}
}

// runBatch wraps a project-scoped or multi-task mutation as an async command
// resolving to a BatchTransactionResultMsg (or TransactionErrorMsg on rejection).
func runBatch(feature, action string, mutate func() ([]string, string, error)) tea.Cmd {
	return func() tea.Msg {
		touched, summary, err := mutate()
		if err != nil {
			return TransactionErrorMsg{Feature: feature, Action: action, Err: err}
		}
		return BatchTransactionResultMsg{Feature: feature, Action: action, TouchedProjectIDs: touched, Summary: summary}
	}
}

// --- Secondary feature coordinators ----------------------------------------

// BulkCoordinator backs the bulk-edit / bulk-capture selection overlays.
type BulkCoordinator struct {
	name string
	bulk BulkMutator
}

// Transition moves every selected task to the target state as one transaction.
func (c BulkCoordinator) Transition(taskIDs []string, target models.WorkflowState) tea.Cmd {
	return runBatch(c.name, "Bulk Transition", func() ([]string, string, error) {
		res, err := c.bulk.BulkTransition(taskIDs, target)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Moved %d task(s) to %s", len(res.Tasks), target), nil
	})
}

// TransitionLocators moves a project-qualified selection as one transaction.
func (c BulkCoordinator) TransitionLocators(locators []app.TaskLocator, target models.WorkflowState, summary func(changed int) string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.bulk.BulkTransitionByLocators(locators, target)
		if err != nil {
			return TransactionErrorMsg{Feature: c.name, Action: "Bulk Transition", Err: err}
		}
		msg := ""
		if summary != nil {
			msg = summary(len(res.Tasks))
		}
		return BatchTransactionResultMsg{
			Feature:           c.name,
			Action:            "Bulk Transition",
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           msg,
		}
	}
}

// Delete permanently removes every selected subtree as one transaction. The caller
// must have already confirmed the destructive intent; Confirm is set here so the
// application layer accepts the batch. Any artifacts (e.g. a config store with the
// removed tasks' durable planning refs cleaned) ride the same transaction. The
// result carries every removed task UUID for the shared transaction result.
func (c BulkCoordinator) Delete(taskIDs []string, artifacts ...storage.ArtifactChange) tea.Cmd {
	return c.delete(app.BulkDeleteRequest{TaskIDs: taskIDs, Confirm: true}, artifacts...)
}

func (c BulkCoordinator) DeleteLocators(locators []app.TaskLocator, artifacts ...storage.ArtifactChange) tea.Cmd {
	return c.delete(app.BulkDeleteRequest{TaskLocators: locators, Confirm: true}, artifacts...)
}

func (c BulkCoordinator) delete(req app.BulkDeleteRequest, artifacts ...storage.ArtifactChange) tea.Cmd {
	return func() tea.Msg {
		res, err := c.bulk.BulkDelete(req, artifacts...)
		if err != nil {
			return TransactionErrorMsg{Feature: c.name, Action: "Bulk Delete", Err: err}
		}
		removedCount := len(res.RemovedTaskIDs)
		if len(res.RemovedTaskLocators) > 0 {
			removedCount = len(res.RemovedTaskLocators)
		}
		return BatchTransactionResultMsg{
			Feature:           c.name,
			Action:            "Bulk Delete",
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           fmt.Sprintf("Deleted %d task(s)", removedCount),
			RemovedTaskIDs:    res.RemovedTaskIDs,
		}
	}
}

func (c BulkCoordinator) SetPriorityLocators(locators []app.TaskLocator, priority int) tea.Cmd {
	return runBatch(c.name, "Bulk Set Priority", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetPriorityByLocators(locators, priority)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Set priority on %d task(s)", len(res.Tasks)), nil
	})
}

// SetPriority sets one priority on every selected task as one transaction.
func (c BulkCoordinator) SetPriority(taskIDs []string, priority int) tea.Cmd {
	return runBatch(c.name, "Bulk Set Priority", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetPriority(taskIDs, priority)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Set priority on %d task(s)", len(res.Tasks)), nil
	})
}

// SetDueDate sets one due date on every selected task as one transaction.
func (c BulkCoordinator) SetDueDate(taskIDs []string, due time.Time) tea.Cmd {
	return runBatch(c.name, "Bulk Set Due Date", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetDueDate(taskIDs, due)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Set due date on %d task(s)", len(res.Tasks)), nil
	})
}

func (c BulkCoordinator) SetDueDateLocators(locators []app.TaskLocator, due time.Time) tea.Cmd {
	return runBatch(c.name, "Bulk Set Due Date", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetDueDateByLocators(locators, due)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Set due date on %d task(s)", len(res.Tasks)), nil
	})
}

// ClearDueDate removes the due date from every selected task as one transaction.
func (c BulkCoordinator) ClearDueDate(taskIDs []string) tea.Cmd {
	return runBatch(c.name, "Bulk Clear Due Date", func() ([]string, string, error) {
		res, err := c.bulk.BulkClearDueDate(taskIDs)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Cleared due date on %d task(s)", len(res.Tasks)), nil
	})
}

func (c BulkCoordinator) ClearDueDateLocators(locators []app.TaskLocator) tea.Cmd {
	return runBatch(c.name, "Bulk Clear Due Date", func() ([]string, string, error) {
		res, err := c.bulk.BulkClearDueDateByLocators(locators)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Cleared due date on %d task(s)", len(res.Tasks)), nil
	})
}

// Snooze hides every selected task until until, or clears the snooze when until
// is nil, as one transaction.
func (c BulkCoordinator) Snooze(taskIDs []string, until *time.Time) tea.Cmd {
	return runBatch(c.name, "Bulk Snooze", func() ([]string, string, error) {
		res, err := c.bulk.BulkSnooze(taskIDs, until)
		if err != nil {
			return nil, "", err
		}
		verb := "Snoozed"
		if until == nil {
			verb = "Cleared snooze on"
		}
		return res.TouchedProjectIDs, fmt.Sprintf("%s %d task(s)", verb, len(res.Tasks)), nil
	})
}

func (c BulkCoordinator) SnoozeLocators(locators []app.TaskLocator, until *time.Time) tea.Cmd {
	return runBatch(c.name, "Bulk Snooze", func() ([]string, string, error) {
		res, err := c.bulk.BulkSnoozeByLocators(locators, until)
		if err != nil {
			return nil, "", err
		}
		verb := "Snoozed"
		if until == nil {
			verb = "Cleared snooze on"
		}
		return res.TouchedProjectIDs, fmt.Sprintf("%s %d task(s)", verb, len(res.Tasks)), nil
	})
}

// SetTags replaces the tag set on every selected task as one transaction.
func (c BulkCoordinator) SetTags(taskIDs []string, tags []string) tea.Cmd {
	return runBatch(c.name, "Bulk Set Tags", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetTags(taskIDs, tags)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Tagged %d task(s)", len(res.Tasks)), nil
	})
}

func (c BulkCoordinator) SetTagsLocators(locators []app.TaskLocator, tags []string) tea.Cmd {
	return runBatch(c.name, "Bulk Set Tags", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetTagsByLocators(locators, tags)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Tagged %d task(s)", len(res.Tasks)), nil
	})
}

// ApplyPlanningLocators applies one planning intent to every selected task as a
// single transaction.
func (c BulkCoordinator) ApplyPlanningLocators(
	locators []app.TaskLocator,
	action app.PlanningAction,
	summary string,
) tea.Cmd {
	return runBatch(c.name, "Bulk Planning", func() ([]string, string, error) {
		res, err := c.bulk.ApplyPlanningBulk(app.PlanningBulkRequest{
			Targets: locators,
			Action:  action,
		})
		if err != nil {
			return nil, "", err
		}
		if summary == "" {
			summary = fmt.Sprintf("Updated %d task(s)", len(res.Tasks))
		}
		return res.TouchedProjectIDs, summary, nil
	})
}

// TaskItemCoordinator backs the single-task outliner mutation handlers, parallel
// to BulkCoordinator but for one task at a time. The attribute seams
// attribute operations resolve to a TransactionResultMsg (a
// single-task commit, reloaded by handleTransactionResult); the reorder seams
// (MoveToTop/MoveToBottom) resolve to a BatchTransactionResultMsg because they
// reorder a whole project's task slice, reloaded by handleBatchTransactionResult.
// Both messages flow through the same save/reload path. The shell starts a
// cross-project reload only while the Outliner is routable and otherwise defers
// it until the view reappears. Every task argument is project-qualified so a
// coordinator cannot accidentally fall back to workspace iteration order.
type TaskItemCoordinator struct {
	name string
	task TaskItemMutator
}

// ExecuteOperation runs title/description edits, clipboard projections, child
// creation, and direct state/metadata actions through one application request.
// Mutations use the batch result path even for one target, so single and bulk
// operations share identical reload and transaction semantics.
func (c TaskItemCoordinator) ExecuteOperation(req app.TaskOperationRequest, action, summary string) tea.Cmd {
	return func() tea.Msg {
		result, err := c.task.ExecuteTaskOperation(req)
		if err != nil {
			return TransactionErrorMsg{Feature: c.name, Action: action, Err: err}
		}
		if result.Clipboard != nil {
			return TaskClipboardProjectedMsg{Projection: *result.Clipboard}
		}
		if summary == "" {
			summary = action
			if len(result.Tasks) > 1 {
				summary = fmt.Sprintf("%s on %d task(s)", action, len(result.Tasks))
			}
		}
		return BatchTransactionResultMsg{
			Feature:           c.name,
			Action:            action,
			TouchedProjectIDs: result.TouchedProjectIDs,
			Summary:           summary,
		}
	}
}

// MoveToTop moves one task to the top of its siblings as one project-scoped
// transaction, emitting a BatchTransactionResultMsg so the touched project is
// reloaded through the shared batch path.
func (c TaskItemCoordinator) MoveToTop(task TaskIdentity) tea.Cmd {
	return runBatch(c.name, "Move to Top", func() ([]string, string, error) {
		res, err := c.task.MoveTaskToTopByLocator(task.Locator())
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d to top", res.ShortID), nil
	})
}

// MoveToBottom moves one task to the bottom of its siblings as one project-scoped
// transaction (see MoveToTop).
func (c TaskItemCoordinator) MoveToBottom(task TaskIdentity) tea.Cmd {
	return runBatch(c.name, "Move to Bottom", func() ([]string, string, error) {
		res, err := c.task.MoveTaskToBottomByLocator(task.Locator())
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d to bottom", res.ShortID), nil
	})
}

// MoveUp swaps one task with its previous model-order sibling as one
// project-scoped transaction (see MoveToTop for the reload path).
func (c TaskItemCoordinator) MoveUp(task TaskIdentity) tea.Cmd {
	return runBatch(c.name, "Move Up", func() ([]string, string, error) {
		res, err := c.task.MoveTaskUpByLocator(task.Locator())
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d up", res.ShortID), nil
	})
}

// MoveDown swaps one task with its next model-order sibling as one project-scoped
// transaction.
func (c TaskItemCoordinator) MoveDown(task TaskIdentity) tea.Cmd {
	return runBatch(c.name, "Move Down", func() ([]string, string, error) {
		res, err := c.task.MoveTaskDownByLocator(task.Locator())
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d down", res.ShortID), nil
	})
}

// ReorderSiblings swaps the two explicit sibling identities as one
// project-scoped transaction. Passing the adjacent visible target lets filtered
// and collapsed Outliner projections preserve hidden siblings' relative order.
func (c TaskItemCoordinator) ReorderSiblings(task, target TaskIdentity) tea.Cmd {
	return runBatch(c.name, "Reorder Siblings", func() ([]string, string, error) {
		res, err := c.task.ReorderSiblingTasksByLocator(task.Locator(), target.Locator())
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Reordered #%d", res.ShortID), nil
	})
}

// Indent nests one task under the task model-above it as one project-scoped
// transaction.
func (c TaskItemCoordinator) Indent(task TaskIdentity) tea.Cmd {
	return runBatch(c.name, "Indent", func() ([]string, string, error) {
		res, err := c.task.IndentTaskByLocator(task.Locator())
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Indented #%d", res.ShortID), nil
	})
}

// Outdent moves one task to become a sibling of its parent as one project-scoped
// transaction.
func (c TaskItemCoordinator) Outdent(task TaskIdentity) tea.Cmd {
	return runBatch(c.name, "Outdent", func() ([]string, string, error) {
		res, err := c.task.OutdentTaskByLocator(task.Locator())
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Outdented #%d", res.ShortID), nil
	})
}

// Transition moves one task to a target workflow state as one transaction.
func (c TaskItemCoordinator) Transition(task TaskIdentity, target models.WorkflowState) tea.Cmd {
	name := c.name
	return func() tea.Msg {
		res, err := c.task.Transition(app.StateRequest{
			Locator: task.Locator(),
			Target:  target,
		})
		if err != nil {
			return TransactionErrorMsg{Feature: name, Action: "Change State", Err: err}
		}
		return TransactionResultMsg{
			Feature: name, Action: "Change State", Result: res,
			PlanningChanged: true,
		}
	}
}

// Edit applies content/form changes to one task as one transaction.
func (c TaskItemCoordinator) Edit(task TaskIdentity, req app.EditRequest) tea.Cmd {
	req.TaskID = ""
	req.Locator = task.Locator()
	return runTask(c.name, "Edit Task", func() (app.TaskResult, error) {
		return c.task.Edit(req)
	})
}

// Capture creates a new task in a project as one transaction.
func (c TaskItemCoordinator) Capture(req app.CaptureRequest) tea.Cmd {
	return runTask(c.name, "Add Task", func() (app.TaskResult, error) {
		return c.task.Capture(req)
	})
}

// SetPriority sets one task's priority as one transaction.
func (c TaskItemCoordinator) SetPriority(task TaskIdentity, priority int) tea.Cmd {
	return runTask(c.name, "Set Priority", func() (app.TaskResult, error) {
		return c.task.SetPriorityByLocator(task.Locator(), priority)
	})
}

// SetDueDate sets or clears (nil due) one task's due date as one transaction.
func (c TaskItemCoordinator) SetDueDate(task TaskIdentity, due *time.Time) tea.Cmd {
	return runTask(c.name, "Set Due Date", func() (app.TaskResult, error) {
		return c.task.SetDueDateByLocator(task.Locator(), due)
	})
}

// Snooze hides one task until until, or clears the snooze when until is nil, as
// one transaction.
func (c TaskItemCoordinator) Snooze(task TaskIdentity, until *time.Time) tea.Cmd {
	return runTask(c.name, "Snooze", func() (app.TaskResult, error) {
		return c.task.SnoozeByLocator(task.Locator(), until)
	})
}

// SetPinned pins or unpins one task as one transaction.
func (c TaskItemCoordinator) SetPinned(task TaskIdentity, pinned bool) tea.Cmd {
	return runTask(c.name, "Toggle Pin", func() (app.TaskResult, error) {
		return c.task.SetPinnedByLocator(task.Locator(), pinned)
	})
}

// Reparent changes one task's parent within its owning project as one transaction.
func (c TaskItemCoordinator) Reparent(task TaskIdentity, newParentID *string) tea.Cmd {
	return runTask(c.name, "Reparent Task", func() (app.TaskResult, error) {
		return c.task.ReparentByLocator(task.Locator(), newParentID)
	})
}

// SetDependencies replaces one task's dependency set as one transaction.
func (c TaskItemCoordinator) SetDependencies(task TaskIdentity, dependencies []string) tea.Cmd {
	return func() tea.Msg {
		return TransactionErrorMsg{Feature: c.name, Action: "Set Dependencies", Err: app.RejectRetiredCapability(app.RetiredCapabilityDependency)}
	}
}

// DeleteTask permanently removes one task's subtree as one transaction, folding
// the caller-supplied durable planning-ref artifacts into it. The result carries
// every removed task UUID (mirroring BulkCoordinator.Delete).
func (c TaskItemCoordinator) DeleteTask(task TaskIdentity, artifacts []storage.ArtifactChange, summary func(app.DeleteResult) string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.Delete(app.DeleteRequest{Locator: task.Locator(), Confirm: true}, artifacts...)
		if err != nil {
			return TransactionErrorMsg{Feature: c.name, Action: "Delete Task", Err: err}
		}
		msg := ""
		if summary != nil {
			msg = summary(res)
		}
		return BatchTransactionResultMsg{
			Feature:           c.name,
			Action:            "Delete Task",
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           msg,
			RemovedTaskIDs:    res.RemovedTaskIDs,
		}
	}
}

// CommitProject commits an already-mutated project through the application
// layer, resolving to a
// BatchTransactionResultMsg (or TransactionErrorMsg on rejection). It is the
// project-scoped analog of the *Workflow single-task methods, backing the
// ProjectTree and Outliner structural / multi-field mutations (reorder,
// indent/outdent, reparent, bulk edits, section moves, paste) whose in-memory
// change spans the whole project rather than one field of one task. The view
// mutated the project synchronously (so its cursor/tree logic and unit tests see
// the change); this seam commits that state durably and the shell reloads every
// touched project. summary sets the status-line text; an empty summary leaves the
// view's synchronously-set status intact (the shell skips an empty success line).
func (c TaskItemCoordinator) CommitProject(project *models.Project, opts app.ProjectCommitOptions, summary string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.CommitProject(project, opts)
		if err != nil {
			return TransactionErrorMsg{Feature: c.name, Action: "Save", Err: err}
		}
		return BatchTransactionResultMsg{
			Feature:           c.name,
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           summary,
		}
	}
}

// AttachmentCoordinator backs the task attachment overlay.
type AttachmentCoordinator struct {
	name   string
	attach AttachmentMutator
}

// AttachURL records a URL attachment on a task.
func (c AttachmentCoordinator) AttachURL(task TaskIdentity, url string) tea.Cmd {
	return runTask(c.name, "Attach URL", func() (app.TaskResult, error) {
		return c.attach.AttachURLByLocator(task.Locator(), url)
	})
}

// AttachCodeReference records a file:line code reference on a task.
func (c AttachmentCoordinator) AttachCodeReference(task TaskIdentity, path string, line int) tea.Cmd {
	return runTask(c.name, "Attach Code Ref", func() (app.TaskResult, error) {
		return c.attach.AttachCodeReferenceByLocator(task.Locator(), path, line)
	})
}

// AttachFile ingests an external file into a task's attachments as one transaction.
func (c AttachmentCoordinator) AttachFile(task TaskIdentity, sourcePath string) tea.Cmd {
	return runTask(c.name, "Attach File", func() (app.TaskResult, error) {
		res, err := c.attach.AttachFileByLocator(task.Locator(), sourcePath)
		return res.Task, err
	})
}

// RemoveAttachment drops an attachment (and optionally its bytes) as one transaction.
func (c AttachmentCoordinator) RemoveAttachment(task TaskIdentity, index int, deleteFile bool) tea.Cmd {
	return runTask(c.name, "Remove Attachment", func() (app.TaskResult, error) {
		res, err := c.attach.RemoveAttachmentByLocator(task.Locator(), index, deleteFile)
		return res.Task, err
	})
}

// Reorder swaps two attachments' positions as one transaction.
func (c AttachmentCoordinator) Reorder(task TaskIdentity, from, to int) tea.Cmd {
	return runTask(c.name, "Reorder Attachments", func() (app.TaskResult, error) {
		return c.attach.ReorderAttachmentByLocator(task.Locator(), from, to)
	})
}

// TagCoordinator backs the tag-manager overlay.
type TagCoordinator struct {
	name string
	tag  TagMutator
}

// Add adds a tag to a task.
func (c TagCoordinator) Add(task TaskIdentity, tag string) tea.Cmd {
	return runTask(c.name, "Add Tag", func() (app.TaskResult, error) {
		return c.tag.AddTagByLocator(task.Locator(), tag)
	})
}

// Remove removes a tag from a task.
func (c TagCoordinator) Remove(task TaskIdentity, tag string) tea.Cmd {
	return runTask(c.name, "Remove Tag", func() (app.TaskResult, error) {
		return c.tag.RemoveTagByLocator(task.Locator(), tag)
	})
}

// SubviewCoordinator backs the task subview panels (comments, decisions,
// checklist). Every method resolves to the shared transaction messages; the
// panels adapt them back to their own message vocabulary with
// adaptSubviewCommit so their existing toasts and refresh behavior survive the
// move onto the application layer.
type SubviewCoordinator struct {
	name string
	sub  SubviewMutator
}

// AddComment appends a comment to a task.
func (c SubviewCoordinator) AddComment(task TaskIdentity, text string) tea.Cmd {
	return runTask(c.name, "Add Comment", func() (app.TaskResult, error) {
		res, err := c.sub.AddCommentByLocator(task.Locator(), text)
		return res.Task, err
	})
}

// EditComment replaces a comment's text, preserving its timestamp.
func (c SubviewCoordinator) EditComment(task TaskIdentity, index int, text string) tea.Cmd {
	return runTask(c.name, "Edit Comment", func() (app.TaskResult, error) {
		res, err := c.sub.EditCommentByLocator(task.Locator(), index, text)
		return res.Task, err
	})
}

// RemoveComment removes the comment at index.
func (c SubviewCoordinator) RemoveComment(task TaskIdentity, index int) tea.Cmd {
	return runTask(c.name, "Remove Comment", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveCommentByLocator(task.Locator(), index)
		return res.Task, err
	})
}

// AddDecision appends a lean decision with optional rationale to a task.
func (c SubviewCoordinator) AddDecision(task TaskIdentity, text, rationale string) tea.Cmd {
	return runTask(c.name, "Add Decision", func() (app.TaskResult, error) {
		result, err := c.sub.AddDecisionByLocator(task.Locator(), text, rationale)
		return result.Task, err
	})
}

// EditDecision replaces a decision's text and rationale while preserving its timestamp.
func (c SubviewCoordinator) EditDecision(task TaskIdentity, index int, text, rationale string) tea.Cmd {
	return runTask(c.name, "Edit Decision", func() (app.TaskResult, error) {
		result, err := c.sub.EditDecisionByLocator(task.Locator(), index, text, rationale)
		return result.Task, err
	})
}

// RemoveDecision removes the decision at index.
func (c SubviewCoordinator) RemoveDecision(task TaskIdentity, index int) tea.Cmd {
	return runTask(c.name, "Remove Decision", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveDecisionByLocator(task.Locator(), index)
		return res.Task, err
	})
}

// AddChecklistItem appends a checklist item to a task.
func (c SubviewCoordinator) AddChecklistItem(task TaskIdentity, text string) tea.Cmd {
	return runTask(c.name, "Add Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.AddChecklistItemByLocator(task.Locator(), text)
		return res.Task, err
	})
}

// EditChecklistItem replaces a checklist item's text, preserving done state.
func (c SubviewCoordinator) EditChecklistItem(task TaskIdentity, index int, text string) tea.Cmd {
	return runTask(c.name, "Edit Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.EditChecklistItemByLocator(task.Locator(), index, text)
		return res.Task, err
	})
}

// RemoveChecklistItem removes the checklist item at index.
func (c SubviewCoordinator) RemoveChecklistItem(task TaskIdentity, index int) tea.Cmd {
	return runTask(c.name, "Remove Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveChecklistItemByLocator(task.Locator(), index)
		return res.Task, err
	})
}

// ToggleChecklistItem flips a checklist item's done state.
func (c SubviewCoordinator) ToggleChecklistItem(task TaskIdentity, index int) tea.Cmd {
	return runTask(c.name, "Toggle Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.ToggleChecklistItemByLocator(task.Locator(), index)
		return res.Task, err
	})
}

// ReorderChecklistItem swaps the checklist items at from and to.
func (c SubviewCoordinator) ReorderChecklistItem(task TaskIdentity, from, to int) tea.Cmd {
	return runTask(c.name, "Reorder Checklist", func() (app.TaskResult, error) {
		res, err := c.sub.ReorderChecklistItemByLocator(task.Locator(), from, to)
		return res.Task, err
	})
}

// LinkCoordinator backs dependency selection.
type LinkCoordinator struct {
	name string
}

// AddDependency adds a dependency edge onto blockerRef (validated for
// existence and cycles across the workspace).
func (c LinkCoordinator) AddDependency(task TaskIdentity, blockerRef string) tea.Cmd {
	return func() tea.Msg {
		return TransactionErrorMsg{Feature: c.name, Action: "Add Dependency", Err: app.RejectRetiredCapability(app.RetiredCapabilityDependency)}
	}
}

// RemoveDependency drops a dependency edge.
func (c LinkCoordinator) RemoveDependency(task TaskIdentity, blockerRef string) tea.Cmd {
	return func() tea.Msg {
		return TransactionErrorMsg{Feature: c.name, Action: "Remove Dependency", Err: app.RejectRetiredCapability(app.RetiredCapabilityDependency)}
	}
}

// adaptSubviewCommit runs a coordinator command inline and adapts its outcome
// back to a view-specific message contract: a TransactionErrorMsg surfaces as
// the shared ErrorMsg, and a committed transaction resolves to success(), which
// runs after the commit so it can re-resolve fresh task pointers. This is the
// single-task analog of commitBlockersThroughCoordinator — the migrated
// panels keep their existing message vocabulary (and therefore their toasts and
// refresh behavior) while the durable write flows through the application layer.
func adaptSubviewCommit(commit tea.Cmd, success func() tea.Msg) tea.Cmd {
	return func() tea.Msg {
		if msg, ok := commit().(TransactionErrorMsg); ok {
			return ErrorMsg{Err: msg.Err}
		}
		return success()
	}
}

// SecondaryCoordinators aggregates the secondary-view coordinators the AppModel
// hands to retained overlays as they migrate onto the application layer.
type SecondaryCoordinators struct {
	Bulk       BulkCoordinator
	TaskItem   TaskItemCoordinator
	Attachment AttachmentCoordinator
	Tag        TagCoordinator
	Subview    SubviewCoordinator
	Link       LinkCoordinator
}

// newSecondaryCoordinators builds the secondary coordinators over the shared
// application service. Each coordinator stores only its narrow
// host port even though every task/artifact port is satisfied by the one service.
// In production svc is the TUI app service; scenario tests inject an in-memory
// service.
func newSecondaryCoordinators(svc *app.Service) SecondaryCoordinators {
	return SecondaryCoordinators{
		Bulk:       BulkCoordinator{name: "bulk", bulk: svc},
		TaskItem:   TaskItemCoordinator{name: "task-item", task: svc},
		Attachment: AttachmentCoordinator{name: "attachment", attach: svc},
		Tag:        TagCoordinator{name: "tag", tag: svc},
		Subview:    SubviewCoordinator{name: "subview", sub: svc},
		Link:       LinkCoordinator{name: "link"},
	}
}
