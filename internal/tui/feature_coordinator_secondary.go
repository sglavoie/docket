package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// This file extends the feature-coordinator seam (feature_coordinator.go) to the
// secondary TUI views and overlays: bulk operations, attachments, tags, blockers,
// and the config-domain editors (settings, workflow rules, scoring). Historically
// each of these mutated its own model or config copy and called a direct
// saveProjectSync / persister.Save / configManager.Save helper. That left every
// overlay re-implementing the load/mutate/save dance with no single durable seam.
//
// Where the primary coordinators depend on the fat FeatureMutator, the secondary
// coordinators each depend on a NARROW host port describing exactly the
// application use cases that one feature performs. *app.Service satisfies every
// task/artifact port and *config.Manager satisfies the config gateway, so a
// production coordinator is built from the same service the primary views use,
// while a scenario test can drive each coordinator against an in-memory service
// with the same use-case semantics. Splitting the ports per feature keeps a
// bulk-only view from depending on attachment methods it never calls.

// BulkMutator is the narrow port the bulk-operation overlays depend on. Both
// methods commit their whole batch as ONE atomic cross-project transaction.
type BulkMutator interface {
	// BulkTransition moves every task to the target workflow state, aborting the
	// whole batch on the first invalid transition.
	BulkTransition(taskIDs []string, target models.WorkflowState) (app.BulkResult, error)
	// BulkTransitionWithWorkflow moves every task to the target state and runs the
	// workflow-rule cascade, supporting best-effort (SkipInvalid) selection so the
	// multiselect pipeline can operate on the valid subset.
	BulkTransitionWithWorkflow(req app.BulkTransitionRequest) (app.BulkResult, error)
	// BulkDelete permanently removes every requested subtree, folding any durable
	// artifact and cross-project reference cleanup into the same transaction.
	BulkDelete(req app.BulkDeleteRequest, artifacts ...storage.ArtifactChange) (app.BulkDeleteResult, error)
	// BulkSetPriority sets one priority on every selected task.
	BulkSetPriority(taskIDs []string, priority int) (app.BulkResult, error)
	BulkSetPriorityByLocators(locators []app.TaskLocator, priority int) (app.BulkResult, error)
	// BulkSetDueDate sets one due date on every selected task.
	BulkSetDueDate(taskIDs []string, due time.Time, hasTime bool) (app.BulkResult, error)
	BulkSetDueDateByLocators(locators []app.TaskLocator, due time.Time, hasTime bool) (app.BulkResult, error)
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
}

// TaskItemMutator is the narrow port the single-task outliner mutation handlers
// depend on. Each method commits ONE single-task or project-scoped transaction
// through the ledger, mirroring BulkMutator but for one task at a time. Every
// cross-project single-task handler now routes through it; the legacy raw-save
// family (crossPersistFn/crossSaveProject) it replaced has been removed.
type TaskItemMutator interface {
	// SetSection commits one task's legacy GTD section field (resyncing state).
	SetSection(taskID, section string) (app.TaskResult, error)
	// MoveTaskToTop moves one task to the top of its model-order siblings.
	MoveTaskToTop(taskID string) (app.TaskResult, error)
	// MoveTaskToBottom moves one task to the bottom of its model-order siblings.
	MoveTaskToBottom(taskID string) (app.TaskResult, error)
	// MoveTaskUp swaps one task with its previous model-order sibling.
	MoveTaskUp(taskID string) (app.TaskResult, error)
	// MoveTaskDown swaps one task with its next model-order sibling.
	MoveTaskDown(taskID string) (app.TaskResult, error)
	// IndentTask nests one task under the task visually/model above it.
	IndentTask(taskID string) (app.TaskResult, error)
	// OutdentTask moves one task to become a sibling of its current parent.
	OutdentTask(taskID string) (app.TaskResult, error)
	// Transition moves one task to a target workflow state.
	Transition(req app.StateRequest) (app.TaskResult, error)
	// ClearRecurrence removes one task's recurrence pattern.
	ClearRecurrence(taskID string) (app.RecurrenceResult, error)
	// Edit applies content changes (title/description/type or a full form) to one task.
	Edit(req app.EditRequest) (app.TaskResult, error)
	// Capture creates a new task in a project and commits it as one ChangeSet.
	Capture(req app.CaptureRequest) (app.TaskResult, error)
	// SetPriority sets one task's 1..PriorityMax priority.
	SetPriority(taskID string, priority int) (app.TaskResult, error)
	// SetDueDate sets or clears (nil due) one task's due date.
	SetDueDate(taskID string, due *time.Time, hasTime bool) (app.TaskResult, error)
	// Snooze hides one task until the given date, or clears the snooze when nil.
	Snooze(taskID string, until *time.Time) (app.TaskResult, error)
	// SetPinned pins or unpins one task.
	SetPinned(taskID string, pinned bool) (app.TaskResult, error)
	// ReorderPinnedTasks swaps two pinned tasks' PinnedAt ordering as one
	// (possibly cross-project) transaction.
	ReorderPinnedTasks(taskAID, taskBID string) (app.PinnedReorderResult, error)
	// ReplaceScoreDimensions sets one task's manual scoring dimensions to exactly
	// the given set (replace, not merge) as one transaction.
	ReplaceScoreDimensions(taskID string, dims map[string]int) (app.TaskResult, error)
	// ToggleDimmed flips one task's dimmed flag (clearing Active when dimming).
	ToggleDimmed(taskID string) (app.TaskResult, error)
	// SkipRecurrence advances one recurring task to its next occurrence.
	SkipRecurrence(taskID string) (app.RecurrenceResult, error)
	// DeferRecurrence pushes one recurring task's due date forward by days.
	DeferRecurrence(taskID string, days int) (app.RecurrenceResult, error)
	// ChangeTaskType applies a type-hierarchy mutation (promote/demote/cycle).
	ChangeTaskType(taskID string, change app.TaskTypeChange) (app.TaskResult, error)
	// Reparent changes one task's parent within its owning project.
	Reparent(taskID string, newParentID *string) (app.TaskResult, error)

	// The *WithWorkflow methods commit one single-task mutation AND run the
	// legacy workflow-rule cascade (follow-ups, deferred cross-project moves,
	// project status-icon rules, rule-trigger counters) atomically, returning a
	// BulkResult so the shell reloads every touched project and persists fired
	// trigger counts. They back secondary views (review, activity,
	// due-date, focus, focus-board, project-focus) that historically ran the full
	// workflow pipeline after every single-task save.
	StateWithWorkflow(taskID string, target models.WorkflowState, opts app.SingleTaskWorkflowOptions) (app.BulkResult, error)
	SetPriorityWithWorkflow(taskID string, priority int, opts app.SingleTaskWorkflowOptions) (app.BulkResult, error)
	SetDueDateWithWorkflow(taskID string, due *time.Time, hasTime bool, opts app.SingleTaskWorkflowOptions) (app.BulkResult, error)
	SnoozeWithWorkflow(taskID string, until *time.Time, opts app.SingleTaskWorkflowOptions) (app.BulkResult, error)
	SetSectionWithWorkflow(taskID, section string, opts app.SingleTaskWorkflowOptions) (app.BulkResult, error)
	SetPinnedWithWorkflow(taskID string, pinned *bool, opts app.SingleTaskWorkflowOptions) (app.BulkResult, error)
	ToggleDimmedWithWorkflow(taskID string, opts app.SingleTaskWorkflowOptions) (app.BulkResult, error)
	// Delete permanently removes one task's subtree, folding any caller-supplied
	// durable planning-ref artifacts into the same transaction.
	Delete(req app.DeleteRequest, artifacts ...storage.ArtifactChange) (app.DeleteResult, error)
	// CommitProjectWithWorkflow commits an already-mutated project (the ProjectTree
	// and Outliner structural / multi-field mutations that operate on the whole
	// in-memory project) through the transactional repository, running the workflow
	// cascade and recording one grouped ledger entry across every touched project.
	CommitProjectWithWorkflow(project *models.Project, opts app.ProjectWorkflowOptions) (app.BulkResult, error)
}

// AttachmentMutator is the narrow port the attachment overlay depends on. File
// attachment and removal ride their byte-copy/delete side effects on the same
// journaled ChangeSet as the metadata write.
type AttachmentMutator interface {
	AttachURL(taskID, url string) (app.TaskResult, error)
	AttachCodeReference(taskID, path string, line int) (app.TaskResult, error)
	AttachFile(taskID, sourcePath string) (app.AttachFileResult, error)
	RemoveAttachment(taskID string, index int, deleteFile bool) (app.RemoveAttachmentResult, error)
	ReorderAttachment(taskID string, from, to int) (app.TaskResult, error)
}

// TagMutator is the narrow port the tag-manager overlay depends on. Task-tag
// edits are single-task mutations; the catalog methods mutate the project's tag
// catalog (add/remove/rename ripple into task usages) as one project commit.
type TagMutator interface {
	AddTag(taskID, tag string) (app.TaskResult, error)
	RemoveTag(taskID, tag string) (app.TaskResult, error)
	AddProjectTag(projectID, tag string) (app.TagCatalogResult, error)
	RemoveProjectTag(projectID, tag string) (app.TagCatalogResult, error)
	RenameProjectTag(projectID, oldName, newName string) (app.TagCatalogResult, error)
	TogglePinnedProjectTag(projectID, tag string) (app.TagCatalogResult, error)
	RenameWorkspaceTag(oldName, newName string) (app.WorkspaceTagRenameResult, error)
}

// SubviewMutator is the narrow port the task subview panels (comments,
// decisions, checklist) depend on. Every method commits one ChangeSet and
// re-validates the item index against a fresh snapshot, so a stale panel index
// surfaces as an error instead of silently targeting the wrong item.
type SubviewMutator interface {
	AddComment(taskID, text string) (app.CommentResult, error)
	EditComment(taskID string, index int, text string) (app.CommentResult, error)
	RemoveComment(taskID string, index int) (app.CommentResult, error)
	ToggleCommentResolved(taskID string, index int) (app.CommentResult, error)
	AddDecision(taskID, text, rationale string, category models.DecisionCategory) (app.DecisionResult, error)
	EditDecision(taskID string, index int, text, rationale string, category *models.DecisionCategory) (app.DecisionResult, error)
	RemoveDecision(taskID string, index int) (app.DecisionResult, error)
	ToggleDecisionResolved(taskID string, index int) (app.DecisionResult, error)
	AddChecklistItem(taskID, text string) (app.ChecklistResult, error)
	EditChecklistItem(taskID string, index int, text string) (app.ChecklistResult, error)
	RemoveChecklistItem(taskID string, index int) (app.ChecklistResult, error)
	ToggleChecklistItem(taskID string, index int) (app.ChecklistResult, error)
	ReorderChecklistItem(taskID string, from, to int) (app.ChecklistResult, error)
}

// LinkMutator is the narrow port the task-link surfaces (linked-task selector,
// cross-project linker) depend on. Dependency edges are stored on the depending
// task (one project commit); a bidirectional related link commits every touched
// project as one transaction.
type LinkMutator interface {
	AddDependency(taskID, blockerRef string) (app.TaskResult, error)
	RemoveDependency(taskID, blockerRef string) (app.TaskResult, error)
	RemoveRelated(taskID, relatedRef string) (app.TaskResult, error)
	AddRelatedBidirectional(taskID, otherRef string) (app.RelatedLinkResult, error)
	RemoveRelatedBidirectional(taskID, otherRef string) (app.RelatedLinkResult, error)
}

// BlockerMutator is the narrow port the project blocker editor depends on.
type BlockerMutator interface {
	SetBlockers(projectID string, blockers []*models.Blocker) (app.BlockersResult, error)
}

// SettingsCommitter and AutomationCommitter are the narrow, domain-scoped ports
// the config-domain coordinators depend on, replacing the whole-config gateway.
// A settings-editor change commits only the settings + machine-local domains and
// a workflow/scoring change commits only the automation domain, each under an
// optimistic-concurrency check rather than rewriting every synced file.
// *config.Manager satisfies both.
type SettingsCommitter interface {
	CommitSettings(settings *config.Settings, machine *config.MachineState) error
}

type AutomationCommitter interface {
	CommitAutomation(automation *config.AutomationConfig) error
}

// ConfigCommitter is the combined capability newSecondaryCoordinators is built
// from; the one config manager satisfies both domain-scoped committers.
type ConfigCommitter interface {
	SettingsCommitter
	AutomationCommitter
}

// Compile-time assurance that the production services satisfy every secondary
// port, so the coordinators can be built from one *app.Service plus the config
// manager.
var (
	_ BulkMutator         = (*app.Service)(nil)
	_ TaskItemMutator     = (*app.Service)(nil)
	_ AttachmentMutator   = (*app.Service)(nil)
	_ TagMutator          = (*app.Service)(nil)
	_ SubviewMutator      = (*app.Service)(nil)
	_ LinkMutator         = (*app.Service)(nil)
	_ BlockerMutator      = (*app.Service)(nil)
	_ SettingsCommitter   = (*config.Manager)(nil)
	_ AutomationCommitter = (*config.Manager)(nil)
	// The domain-native runtime snapshot is also a settings committer: the TUI
	// settings editor commits through it so the in-memory snapshot refreshes on
	// success (RuntimeState.CommitSettings), rather than through the manager.
	_ SettingsCommitter = (*config.RuntimeState)(nil)
	// The runtime snapshot is likewise an automation committer: the workflow-rule
	// and task-template editors commit through it (RuntimeState.CommitAutomation)
	// so the in-memory automation snapshot refreshes on success.
	_ AutomationCommitter = (*config.RuntimeState)(nil)
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
	// RuleTriggerCounts carries workflow-rule trigger increments a bulk-with-rules
	// mutation fired. Trigger counters live in workspace config, not project
	// Markdown, so they ride outside the ChangeSet; the shell persists them
	// synchronously in Update() (avoiding a data race on the shared config) after
	// reloading the committed projects. Nil for mutations that evaluated no rules.
	RuleTriggerCounts map[string]int
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
	// descendants). The shell repairs machine-local recent visits for these after
	// the commit — non-authoritative UI state cleaned separately from the durable
	// planning refs, which ride the delete transaction as a config artifact. Nil
	// for non-delete mutations.
	RemovedTaskIDs      []string
	RemovedTaskLocators []app.TaskLocator
	// DeleteRequestToken correlates a committed canonical delete with the Inbox /
	// Weekly Review post-commit bookkeeping it armed. Only the batch result whose
	// token matches the armed request consumes that in-flight state, so an
	// unrelated batch that commits first can never strand or falsely credit it.
	// Zero for every non-delete batch (and for deletes that armed no bookkeeping);
	// the correlation guards ignore zero.
	DeleteRequestToken uint64
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

// stampDeleteRequestToken tags the canonical delete command's outcome with the
// request token that armed the Inbox / Weekly Review post-commit bookkeeping, so
// only the exact result or error this request produced consumes that in-flight
// state. A committed BatchTransactionResultMsg and a rejecting TransactionErrorMsg
// both carry the token; any other message (and a nil command) passes through
// untouched, leaving the correlation guards to ignore the zero token.
func stampDeleteRequestToken(cmd tea.Cmd, token uint64) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		switch msg := cmd().(type) {
		case BatchTransactionResultMsg:
			msg.DeleteRequestToken = token
			return msg
		case TransactionErrorMsg:
			msg.DeleteRequestToken = token
			return msg
		default:
			return msg
		}
	}
}

// SuccessMessage renders the status line shown when the batch commits.
func (m BatchTransactionResultMsg) SuccessMessage() string {
	if m.Summary != "" {
		return m.Summary
	}
	return m.Action
}

// ConfigTransactionResultMsg is emitted when a config-domain coordinator persists
// the workspace config (settings, workflow rules, scoring). It carries no task or
// project reference because it mutates config, not task Markdown.
type ConfigTransactionResultMsg struct {
	Feature string
	Action  string
}

// SuccessMessage renders the status line shown when the config write commits.
func (m ConfigTransactionResultMsg) SuccessMessage() string { return m.Action }

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
	name                   string
	bulk                   BulkMutator
	projectStatusArtifacts app.WorkflowProjectStatusArtifactBuilder
	planningArtifacts      app.PlanningArtifactBuilder
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

// TransitionWithWorkflow moves every task to the target state, runs the workflow
// cascade, and commits every touched project as one transaction. It is the
// workflow-aware, best-effort analog of Transition used by the multiselect
// pipeline: the request should carry SkipInvalid so tasks that cannot transition
// are dropped rather than aborting the batch. summary builds the status line from
// the number of tasks actually changed; fired rule trigger counts are carried on
// the result message for the shell to persist. On rejection it emits a
// TransactionErrorMsg, matching the other coordinator commands.
func (c BulkCoordinator) TransitionWithWorkflow(req app.BulkTransitionRequest, summary func(changed int) string) tea.Cmd {
	req.ProjectStatusArtifacts = c.projectStatusArtifacts
	req.PlanningArtifacts = c.planningArtifacts
	return func() tea.Msg {
		res, err := c.bulk.BulkTransitionWithWorkflow(req)
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
			RuleTriggerCounts: res.RuleTriggerCounts,
		}
	}
}

// Delete permanently removes every selected subtree as one transaction. The caller
// must have already confirmed the destructive intent; Confirm is set here so the
// application layer accepts the batch. Any artifacts (e.g. a config store with the
// removed tasks' durable planning refs cleaned) ride the same transaction. The
// result carries every removed task UUID so the shell can repair machine-local
// recent visits after the commit.
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
			Feature:             c.name,
			Action:              "Bulk Delete",
			TouchedProjectIDs:   res.TouchedProjectIDs,
			Summary:             fmt.Sprintf("Deleted %d task(s)", removedCount),
			RemovedTaskIDs:      res.RemovedTaskIDs,
			RemovedTaskLocators: res.RemovedTaskLocators,
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
func (c BulkCoordinator) SetDueDate(taskIDs []string, due time.Time, hasTime bool) tea.Cmd {
	return runBatch(c.name, "Bulk Set Due Date", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetDueDate(taskIDs, due, hasTime)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Set due date on %d task(s)", len(res.Tasks)), nil
	})
}

func (c BulkCoordinator) SetDueDateLocators(locators []app.TaskLocator, due time.Time, hasTime bool) tea.Cmd {
	return runBatch(c.name, "Bulk Set Due Date", func() ([]string, string, error) {
		res, err := c.bulk.BulkSetDueDateByLocators(locators, due, hasTime)
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

// TaskItemCoordinator backs the single-task outliner mutation handlers, parallel
// to BulkCoordinator but for one task at a time. The attribute seams
// (ToggleActive/SetSection/SetStage) resolve to a TransactionResultMsg (a
// single-task commit, reloaded by handleTransactionResult); the reorder seams
// (MoveToTop/MoveToBottom) resolve to a BatchTransactionResultMsg because they
// reorder a whole project's task slice, reloaded by handleBatchTransactionResult.
// Both messages flow through the same save/reload path that ends in
// crossOutliner.reloadCrossProjects(). Nothing routes through it yet — it is the
// seam a later child migrates the cross-project single-task raw-save family onto.
type TaskItemCoordinator struct {
	name                   string
	task                   TaskItemMutator
	projectStatusArtifacts app.WorkflowProjectStatusArtifactBuilder
	planningArtifacts      app.PlanningArtifactBuilder
}

// SetSection commits one task's legacy GTD section field as one transaction.
func (c TaskItemCoordinator) SetSection(taskID, section string) tea.Cmd {
	return runTask(c.name, "Set Section", func() (app.TaskResult, error) {
		return c.task.SetSection(taskID, section)
	})
}

// MoveToTop moves one task to the top of its siblings as one project-scoped
// transaction, emitting a BatchTransactionResultMsg so the touched project is
// reloaded through the shared batch path.
func (c TaskItemCoordinator) MoveToTop(taskID string) tea.Cmd {
	return runBatch(c.name, "Move to Top", func() ([]string, string, error) {
		res, err := c.task.MoveTaskToTop(taskID)
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d to top", res.ShortID), nil
	})
}

// MoveToBottom moves one task to the bottom of its siblings as one project-scoped
// transaction (see MoveToTop).
func (c TaskItemCoordinator) MoveToBottom(taskID string) tea.Cmd {
	return runBatch(c.name, "Move to Bottom", func() ([]string, string, error) {
		res, err := c.task.MoveTaskToBottom(taskID)
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d to bottom", res.ShortID), nil
	})
}

// MoveUp swaps one task with its previous model-order sibling as one
// project-scoped transaction (see MoveToTop for the reload path).
func (c TaskItemCoordinator) MoveUp(taskID string) tea.Cmd {
	return runBatch(c.name, "Move Up", func() ([]string, string, error) {
		res, err := c.task.MoveTaskUp(taskID)
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d up", res.ShortID), nil
	})
}

// MoveDown swaps one task with its next model-order sibling as one project-scoped
// transaction.
func (c TaskItemCoordinator) MoveDown(taskID string) tea.Cmd {
	return runBatch(c.name, "Move Down", func() ([]string, string, error) {
		res, err := c.task.MoveTaskDown(taskID)
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Moved #%d down", res.ShortID), nil
	})
}

// Indent nests one task under the task model-above it as one project-scoped
// transaction.
func (c TaskItemCoordinator) Indent(taskID string) tea.Cmd {
	return runBatch(c.name, "Indent", func() ([]string, string, error) {
		res, err := c.task.IndentTask(taskID)
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Indented #%d", res.ShortID), nil
	})
}

// Outdent moves one task to become a sibling of its parent as one project-scoped
// transaction.
func (c TaskItemCoordinator) Outdent(taskID string) tea.Cmd {
	return runBatch(c.name, "Outdent", func() ([]string, string, error) {
		res, err := c.task.OutdentTask(taskID)
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Outdented #%d", res.ShortID), nil
	})
}

// Transition moves one task to a target workflow state as one transaction.
func (c TaskItemCoordinator) Transition(taskID string, target models.WorkflowState) tea.Cmd {
	return runTask(c.name, "Change State", func() (app.TaskResult, error) {
		return c.task.Transition(app.StateRequest{TaskID: taskID, Target: target})
	})
}

// ClearRecurrence removes one recurring task's pattern as one transaction.
func (c TaskItemCoordinator) ClearRecurrence(taskID string) tea.Cmd {
	return runTask(c.name, "Clear Recurrence", func() (app.TaskResult, error) {
		res, err := c.task.ClearRecurrence(taskID)
		return res.Task, err
	})
}

// Edit applies content/form changes to one task as one transaction.
func (c TaskItemCoordinator) Edit(req app.EditRequest) tea.Cmd {
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
func (c TaskItemCoordinator) SetPriority(taskID string, priority int) tea.Cmd {
	return runTask(c.name, "Set Priority", func() (app.TaskResult, error) {
		return c.task.SetPriority(taskID, priority)
	})
}

// SetDueDate sets or clears (nil due) one task's due date as one transaction.
func (c TaskItemCoordinator) SetDueDate(taskID string, due *time.Time, hasTime bool) tea.Cmd {
	return runTask(c.name, "Set Due Date", func() (app.TaskResult, error) {
		return c.task.SetDueDate(taskID, due, hasTime)
	})
}

// Snooze hides one task until until, or clears the snooze when until is nil, as
// one transaction.
func (c TaskItemCoordinator) Snooze(taskID string, until *time.Time) tea.Cmd {
	return runTask(c.name, "Snooze", func() (app.TaskResult, error) {
		return c.task.Snooze(taskID, until)
	})
}

// SetPinned pins or unpins one task as one transaction.
func (c TaskItemCoordinator) SetPinned(taskID string, pinned bool) tea.Cmd {
	return runTask(c.name, "Toggle Pin", func() (app.TaskResult, error) {
		return c.task.SetPinned(taskID, pinned)
	})
}

// SetScore replaces one task's manual scoring dimensions as one transaction.
func (c TaskItemCoordinator) SetScore(taskID string, dims map[string]int) tea.Cmd {
	return runTask(c.name, "Set Score", func() (app.TaskResult, error) {
		return c.task.ReplaceScoreDimensions(taskID, dims)
	})
}

// ToggleDimmed flips one task's dimmed flag as one transaction.
func (c TaskItemCoordinator) ToggleDimmed(taskID string) tea.Cmd {
	return runTask(c.name, "Toggle Dim", func() (app.TaskResult, error) {
		return c.task.ToggleDimmed(taskID)
	})
}

// SkipRecurrence advances one recurring task to its next occurrence as one
// transaction.
func (c TaskItemCoordinator) SkipRecurrence(taskID string) tea.Cmd {
	return runTask(c.name, "Skip Recurrence", func() (app.TaskResult, error) {
		res, err := c.task.SkipRecurrence(taskID)
		return res.Task, err
	})
}

// DeferRecurrence pushes one recurring task's due date forward by days as one
// transaction.
func (c TaskItemCoordinator) DeferRecurrence(taskID string, days int) tea.Cmd {
	return runTask(c.name, "Defer Recurrence", func() (app.TaskResult, error) {
		res, err := c.task.DeferRecurrence(taskID, days)
		return res.Task, err
	})
}

// ChangeType applies a type-hierarchy mutation to one task as one transaction.
func (c TaskItemCoordinator) ChangeType(taskID string, change app.TaskTypeChange) tea.Cmd {
	return runTask(c.name, "Change Type", func() (app.TaskResult, error) {
		return c.task.ChangeTaskType(taskID, change)
	})
}

// Reparent changes one task's parent within its owning project as one transaction.
func (c TaskItemCoordinator) Reparent(taskID string, newParentID *string) tea.Cmd {
	return runTask(c.name, "Reparent Task", func() (app.TaskResult, error) {
		return c.task.Reparent(taskID, newParentID)
	})
}

// runTaskWorkflow wraps a workflow-aware single-task mutation as an async command
// resolving to a BatchTransactionResultMsg (or TransactionErrorMsg on rejection).
// A single-task mutation that runs the workflow cascade can touch several projects
// (deferred moves) and fire rule-trigger counters, so it reuses the batch result
// contract — the shell reloads every touched project and persists the trigger
// counts, exactly as the bulk-with-workflow path does. summary builds the status
// line from the single mutated task's result.
func runTaskWorkflow(feature, action string, summary func(app.TaskResult) string, mutate func() (app.BulkResult, error)) tea.Cmd {
	return func() tea.Msg {
		res, err := mutate()
		if err != nil {
			return TransactionErrorMsg{Feature: feature, Action: action, Err: err}
		}
		msg := ""
		if summary != nil && len(res.Tasks) > 0 {
			msg = summary(res.Tasks[0])
		}
		return BatchTransactionResultMsg{
			Feature:           feature,
			Action:            action,
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           msg,
			RuleTriggerCounts: res.RuleTriggerCounts,
		}
	}
}

// TransitionWorkflow moves one task to a target state and runs the workflow
// cascade as one atomic transaction.
func (c TaskItemCoordinator) TransitionWorkflow(taskID string, target models.WorkflowState, opts app.SingleTaskWorkflowOptions, summary func(app.TaskResult) string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	opts.PlanningArtifacts = c.planningArtifacts
	return runTaskWorkflow(c.name, "Change State", summary, func() (app.BulkResult, error) {
		return c.task.StateWithWorkflow(taskID, target, opts)
	})
}

// SetPriorityWorkflow sets one task's priority and runs the workflow cascade.
func (c TaskItemCoordinator) SetPriorityWorkflow(taskID string, priority int, opts app.SingleTaskWorkflowOptions, summary func(app.TaskResult) string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	return runTaskWorkflow(c.name, "Set Priority", summary, func() (app.BulkResult, error) {
		return c.task.SetPriorityWithWorkflow(taskID, priority, opts)
	})
}

// SetDueDateWorkflow sets or clears one task's due date and runs the workflow cascade.
func (c TaskItemCoordinator) SetDueDateWorkflow(taskID string, due *time.Time, hasTime bool, opts app.SingleTaskWorkflowOptions, summary func(app.TaskResult) string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	return runTaskWorkflow(c.name, "Set Due Date", summary, func() (app.BulkResult, error) {
		return c.task.SetDueDateWithWorkflow(taskID, due, hasTime, opts)
	})
}

// SnoozeWorkflow sets or clears one task's snooze date and runs the workflow cascade.
func (c TaskItemCoordinator) SnoozeWorkflow(taskID string, until *time.Time, opts app.SingleTaskWorkflowOptions, summary func(app.TaskResult) string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	return runTaskWorkflow(c.name, "Snooze", summary, func() (app.BulkResult, error) {
		return c.task.SnoozeWithWorkflow(taskID, until, opts)
	})
}

// SetSectionWorkflow sets one task's GTD section and runs the workflow cascade.
func (c TaskItemCoordinator) SetSectionWorkflow(taskID, section string, opts app.SingleTaskWorkflowOptions, summary func(app.TaskResult) string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	opts.PlanningArtifacts = c.planningArtifacts
	return runTaskWorkflow(c.name, "Move to Section", summary, func() (app.BulkResult, error) {
		return c.task.SetSectionWithWorkflow(taskID, section, opts)
	})
}

// TogglePinnedWorkflow toggles (nil) or forces one task's Pinned flag and runs
// the workflow cascade.
func (c TaskItemCoordinator) TogglePinnedWorkflow(taskID string, pinned *bool, opts app.SingleTaskWorkflowOptions, summary func(app.TaskResult) string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	return runTaskWorkflow(c.name, "Toggle Pin", summary, func() (app.BulkResult, error) {
		return c.task.SetPinnedWithWorkflow(taskID, pinned, opts)
	})
}

// ToggleDimmedWorkflow flips one task's Dimmed flag and runs the workflow cascade.
func (c TaskItemCoordinator) ToggleDimmedWorkflow(taskID string, opts app.SingleTaskWorkflowOptions, summary func(app.TaskResult) string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	return runTaskWorkflow(c.name, "Toggle Dim", summary, func() (app.BulkResult, error) {
		return c.task.ToggleDimmedWithWorkflow(taskID, opts)
	})
}

// DeleteTask permanently removes one task's subtree as one transaction, folding
// the caller-supplied durable planning-ref artifacts into it. The result carries
// every removed task UUID so the shell repairs machine-local recent visits after
// the commit (mirroring BulkCoordinator.Delete).
func (c TaskItemCoordinator) DeleteTask(taskID string, artifacts []storage.ArtifactChange, summary func(app.DeleteResult) string) tea.Cmd {
	return func() tea.Msg {
		res, err := c.task.Delete(app.DeleteRequest{TaskID: taskID, Confirm: true}, artifacts...)
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

// CommitProjectWorkflow commits an already-mutated project through the
// application layer with the workflow cascade, resolving to a
// BatchTransactionResultMsg (or TransactionErrorMsg on rejection). It is the
// project-scoped analog of the *Workflow single-task methods, backing the
// ProjectTree and Outliner structural / multi-field mutations (reorder,
// indent/outdent, reparent, bulk edits, section moves, paste) whose in-memory
// change spans the whole project rather than one field of one task. The view
// mutated the project synchronously (so its cursor/tree logic and unit tests see
// the change); this seam commits that state durably and the shell reloads every
// touched project. summary sets the status-line text; an empty summary leaves the
// view's synchronously-set status intact (the shell skips an empty success line).
func (c TaskItemCoordinator) CommitProjectWorkflow(project *models.Project, opts app.ProjectWorkflowOptions, summary string) tea.Cmd {
	opts.ProjectStatusArtifacts = c.projectStatusArtifacts
	opts.PlanningArtifacts = c.planningArtifacts
	return func() tea.Msg {
		res, err := c.task.CommitProjectWithWorkflow(project, opts)
		if err != nil {
			return TransactionErrorMsg{Feature: c.name, Action: "Save", Err: err}
		}
		return BatchTransactionResultMsg{
			Feature:           c.name,
			TouchedProjectIDs: res.TouchedProjectIDs,
			Summary:           summary,
			RuleTriggerCounts: res.RuleTriggerCounts,
		}
	}
}

// AttachmentCoordinator backs the task attachment overlay.
type AttachmentCoordinator struct {
	name   string
	attach AttachmentMutator
}

// AttachURL records a URL attachment on a task.
func (c AttachmentCoordinator) AttachURL(taskID, url string) tea.Cmd {
	return runTask(c.name, "Attach URL", func() (app.TaskResult, error) {
		return c.attach.AttachURL(taskID, url)
	})
}

// AttachCodeReference records a file:line code reference on a task.
func (c AttachmentCoordinator) AttachCodeReference(taskID, path string, line int) tea.Cmd {
	return runTask(c.name, "Attach Code Ref", func() (app.TaskResult, error) {
		return c.attach.AttachCodeReference(taskID, path, line)
	})
}

// AttachFile ingests an external file into a task's attachments as one transaction.
func (c AttachmentCoordinator) AttachFile(taskID, sourcePath string) tea.Cmd {
	return runTask(c.name, "Attach File", func() (app.TaskResult, error) {
		res, err := c.attach.AttachFile(taskID, sourcePath)
		return res.Task, err
	})
}

// RemoveAttachment drops an attachment (and optionally its bytes) as one transaction.
func (c AttachmentCoordinator) RemoveAttachment(taskID string, index int, deleteFile bool) tea.Cmd {
	return runTask(c.name, "Remove Attachment", func() (app.TaskResult, error) {
		res, err := c.attach.RemoveAttachment(taskID, index, deleteFile)
		return res.Task, err
	})
}

// Reorder swaps two attachments' positions as one transaction.
func (c AttachmentCoordinator) Reorder(taskID string, from, to int) tea.Cmd {
	return runTask(c.name, "Reorder Attachments", func() (app.TaskResult, error) {
		return c.attach.ReorderAttachment(taskID, from, to)
	})
}

// TagCoordinator backs the tag-manager overlay.
type TagCoordinator struct {
	name string
	tag  TagMutator
}

// Add adds a tag to a task.
func (c TagCoordinator) Add(taskID, tag string) tea.Cmd {
	return runTask(c.name, "Add Tag", func() (app.TaskResult, error) {
		return c.tag.AddTag(taskID, tag)
	})
}

// Remove removes a tag from a task.
func (c TagCoordinator) Remove(taskID, tag string) tea.Cmd {
	return runTask(c.name, "Remove Tag", func() (app.TaskResult, error) {
		return c.tag.RemoveTag(taskID, tag)
	})
}

// The catalog methods run synchronously and return the application result
// directly (the WorkflowCoordinator.SaveNow shape): the tag manager renders
// mutation outcomes through its own message vocabulary (TagRenamedMsg,
// TagAddedMsg, ...) and needs the result fields to build those messages, so it
// wraps these calls in its own async commands instead of consuming the shared
// transaction messages.

// AddCatalog adds a tag to a project's catalog as one transaction.
func (c TagCoordinator) AddCatalog(projectID, tag string) (app.TagCatalogResult, error) {
	return c.tag.AddProjectTag(projectID, tag)
}

// RemoveCatalog deletes a tag from a project's catalog, task usages, and pins
// as one transaction.
func (c TagCoordinator) RemoveCatalog(projectID, tag string) (app.TagCatalogResult, error) {
	return c.tag.RemoveProjectTag(projectID, tag)
}

// RenameCatalog renames a tag across a project's catalog, task usages, and
// pins as one transaction.
func (c TagCoordinator) RenameCatalog(projectID, oldName, newName string) (app.TagCatalogResult, error) {
	return c.tag.RenameProjectTag(projectID, oldName, newName)
}

// TogglePinned flips a tag's quick-access pin as one transaction.
func (c TagCoordinator) TogglePinned(projectID, tag string) (app.TagCatalogResult, error) {
	return c.tag.TogglePinnedProjectTag(projectID, tag)
}

// RenameWorkspace renames a tag across every project that uses it — catalog,
// pins, and task usages — as one grouped transaction. Like the catalog methods
// it runs synchronously and returns the application result directly; the global
// tag manager wraps it in its own async command to build its GlobalTagRenamedMsg.
func (c TagCoordinator) RenameWorkspace(oldName, newName string) (app.WorkspaceTagRenameResult, error) {
	return c.tag.RenameWorkspaceTag(oldName, newName)
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
func (c SubviewCoordinator) AddComment(taskID, text string) tea.Cmd {
	return runTask(c.name, "Add Comment", func() (app.TaskResult, error) {
		res, err := c.sub.AddComment(taskID, text)
		return res.Task, err
	})
}

// EditComment replaces a comment's text, preserving its timestamp.
func (c SubviewCoordinator) EditComment(taskID string, index int, text string) tea.Cmd {
	return runTask(c.name, "Edit Comment", func() (app.TaskResult, error) {
		res, err := c.sub.EditComment(taskID, index, text)
		return res.Task, err
	})
}

// RemoveComment removes the comment at index.
func (c SubviewCoordinator) RemoveComment(taskID string, index int) tea.Cmd {
	return runTask(c.name, "Remove Comment", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveComment(taskID, index)
		return res.Task, err
	})
}

// ToggleCommentResolved flips a comment's resolved state.
func (c SubviewCoordinator) ToggleCommentResolved(taskID string, index int) tea.Cmd {
	return runTask(c.name, "Resolve Comment", func() (app.TaskResult, error) {
		res, err := c.sub.ToggleCommentResolved(taskID, index)
		return res.Task, err
	})
}

// AddDecision appends a decision to a task.
func (c SubviewCoordinator) AddDecision(taskID, text, rationale string, category models.DecisionCategory) tea.Cmd {
	return runTask(c.name, "Add Decision", func() (app.TaskResult, error) {
		res, err := c.sub.AddDecision(taskID, text, rationale, category)
		return res.Task, err
	})
}

// EditDecision replaces a decision's text/rationale; a nil category keeps the
// existing one.
func (c SubviewCoordinator) EditDecision(taskID string, index int, text, rationale string, category *models.DecisionCategory) tea.Cmd {
	return runTask(c.name, "Edit Decision", func() (app.TaskResult, error) {
		res, err := c.sub.EditDecision(taskID, index, text, rationale, category)
		return res.Task, err
	})
}

// RemoveDecision removes the decision at index.
func (c SubviewCoordinator) RemoveDecision(taskID string, index int) tea.Cmd {
	return runTask(c.name, "Remove Decision", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveDecision(taskID, index)
		return res.Task, err
	})
}

// ToggleDecisionResolved flips a decision's resolved state.
func (c SubviewCoordinator) ToggleDecisionResolved(taskID string, index int) tea.Cmd {
	return runTask(c.name, "Resolve Decision", func() (app.TaskResult, error) {
		res, err := c.sub.ToggleDecisionResolved(taskID, index)
		return res.Task, err
	})
}

// AddChecklistItem appends a checklist item to a task.
func (c SubviewCoordinator) AddChecklistItem(taskID, text string) tea.Cmd {
	return runTask(c.name, "Add Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.AddChecklistItem(taskID, text)
		return res.Task, err
	})
}

// EditChecklistItem replaces a checklist item's text, preserving done state.
func (c SubviewCoordinator) EditChecklistItem(taskID string, index int, text string) tea.Cmd {
	return runTask(c.name, "Edit Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.EditChecklistItem(taskID, index, text)
		return res.Task, err
	})
}

// RemoveChecklistItem removes the checklist item at index.
func (c SubviewCoordinator) RemoveChecklistItem(taskID string, index int) tea.Cmd {
	return runTask(c.name, "Remove Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.RemoveChecklistItem(taskID, index)
		return res.Task, err
	})
}

// ToggleChecklistItem flips a checklist item's done state.
func (c SubviewCoordinator) ToggleChecklistItem(taskID string, index int) tea.Cmd {
	return runTask(c.name, "Toggle Checklist Item", func() (app.TaskResult, error) {
		res, err := c.sub.ToggleChecklistItem(taskID, index)
		return res.Task, err
	})
}

// ReorderChecklistItem swaps the checklist items at from and to.
func (c SubviewCoordinator) ReorderChecklistItem(taskID string, from, to int) tea.Cmd {
	return runTask(c.name, "Reorder Checklist", func() (app.TaskResult, error) {
		res, err := c.sub.ReorderChecklistItem(taskID, from, to)
		return res.Task, err
	})
}

// LinkCoordinator backs the linked-task selector and the cross-project linker.
type LinkCoordinator struct {
	name string
	link LinkMutator
}

// AddDependency adds a dependency edge onto blockerRef (validated for
// existence and cycles across the workspace).
func (c LinkCoordinator) AddDependency(taskID, blockerRef string) tea.Cmd {
	return runTask(c.name, "Add Dependency", func() (app.TaskResult, error) {
		return c.link.AddDependency(taskID, blockerRef)
	})
}

// RemoveDependency drops a dependency edge.
func (c LinkCoordinator) RemoveDependency(taskID, blockerRef string) tea.Cmd {
	return runTask(c.name, "Remove Dependency", func() (app.TaskResult, error) {
		return c.link.RemoveDependency(taskID, blockerRef)
	})
}

// RemoveRelated drops a related link.
func (c LinkCoordinator) RemoveRelated(taskID, relatedRef string) tea.Cmd {
	return runTask(c.name, "Remove Related Link", func() (app.TaskResult, error) {
		return c.link.RemoveRelated(taskID, relatedRef)
	})
}

// RelateBidirectional links two tasks with reciprocal related references as one
// transaction across every touched project.
func (c LinkCoordinator) RelateBidirectional(taskID, otherRef string) tea.Cmd {
	return runBatch(c.name, "Add Related Link", func() ([]string, string, error) {
		res, err := c.link.AddRelatedBidirectional(taskID, otherRef)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, fmt.Sprintf("Linked #%d <-> #%d", res.SourceShortID, res.TargetShortID), nil
	})
}

// RemoveRelatedBidirectional drops a related link from both sides as one
// transaction across every touched project.
func (c LinkCoordinator) RemoveRelatedBidirectional(taskID, otherRef string) tea.Cmd {
	return runBatch(c.name, "Remove Related Link", func() ([]string, string, error) {
		res, err := c.link.RemoveRelatedBidirectional(taskID, otherRef)
		if err != nil {
			return nil, "", err
		}
		return res.TouchedProjectIDs, "Removed related link", nil
	})
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

// BlockerCoordinator backs the project blocker editor overlay.
type BlockerCoordinator struct {
	name    string
	blocker BlockerMutator
}

// Set replaces a project's blocker list as one transaction.
func (c BlockerCoordinator) Set(projectID string, blockers []*models.Blocker) tea.Cmd {
	return runBatch(c.name, "Update Blockers", func() ([]string, string, error) {
		res, err := c.blocker.SetBlockers(projectID, blockers)
		if err != nil {
			return nil, "", err
		}
		return []string{res.ProjectID}, fmt.Sprintf("Saved %d blocker(s) (%d active)", res.Count, res.ActiveCount), nil
	})
}

// SettingsCoordinator backs the settings editor. It commits the settings +
// machine-local domains through the domain-scoped committer, keeping the
// machine-local file outside the synced set and never rewriting the domains the
// editor did not touch.
type SettingsCoordinator struct {
	name      string
	committer SettingsCommitter
}

// Save persists the settings and machine-local domains and resolves to a
// ConfigTransactionResultMsg.
func (c SettingsCoordinator) Save(settings *config.Settings, machine *config.MachineState) tea.Cmd {
	return runConfig(c.name, "Save Settings", func() error { return commitSettings(c.committer, settings, machine) })
}

// SaveThen persists the settings/machine domains and resolves to next() on
// success, so a view keeps its own post-save message (pop, refresh) while the
// durable write still flows through the coordinator. Failures resolve to the
// coordinator's TransactionErrorMsg.
func (c SettingsCoordinator) SaveThen(settings *config.Settings, machine *config.MachineState, next func() tea.Msg) tea.Cmd {
	return configThen(c.Save(settings, machine), next)
}

// WorkflowCoordinator backs the workflow-rule and scoring editors, which commit
// only the automation domain.
type WorkflowCoordinator struct {
	name      string
	committer AutomationCommitter
}

// Save persists the automation domain and resolves to a
// ConfigTransactionResultMsg.
func (c WorkflowCoordinator) Save(automation *config.AutomationConfig) tea.Cmd {
	return runConfig(c.name, "Save Workflow", func() error { return commitAutomation(c.committer, automation) })
}

// SaveThen persists the automation domain and resolves to next() on success
// (see SettingsCoordinator.SaveThen).
func (c WorkflowCoordinator) SaveThen(automation *config.AutomationConfig, next func() tea.Msg) tea.Cmd {
	return configThen(c.Save(automation), next)
}

// SaveNow persists synchronously for views that set their own status line in
// the same Update pass (the score profile manager). It reuses the same committer
// seam as Save.
func (c WorkflowCoordinator) SaveNow(automation *config.AutomationConfig) error {
	return commitAutomation(c.committer, automation)
}

// commitSettings and commitAutomation guard the domain write against a missing
// committer (isolated render tests construct config-domain views without a
// manager).
func commitSettings(committer SettingsCommitter, settings *config.Settings, machine *config.MachineState) error {
	if committer == nil {
		return fmt.Errorf("config manager unavailable")
	}
	return committer.CommitSettings(settings, machine)
}

func commitAutomation(committer AutomationCommitter, automation *config.AutomationConfig) error {
	if committer == nil {
		return fmt.Errorf("config manager unavailable")
	}
	return committer.CommitAutomation(automation)
}

// configThen chains a view-specific follow-up message onto a successful config
// transaction, letting the error message propagate unchanged.
func configThen(inner tea.Cmd, next func() tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg := inner()
		if _, ok := msg.(ConfigTransactionResultMsg); ok && next != nil {
			return next()
		}
		return msg
	}
}

// runConfig wraps a config-domain write as an async command resolving to a
// ConfigTransactionResultMsg (or TransactionErrorMsg on failure).
func runConfig(feature, action string, save func() error) tea.Cmd {
	return func() tea.Msg {
		if err := save(); err != nil {
			return TransactionErrorMsg{Feature: feature, Action: action, Err: err}
		}
		return ConfigTransactionResultMsg{Feature: feature, Action: action}
	}
}

// SecondaryCoordinators aggregates the secondary-view coordinators the AppModel
// hands to overlays and editors as they migrate onto the application layer.
type SecondaryCoordinators struct {
	Bulk       BulkCoordinator
	TaskItem   TaskItemCoordinator
	Attachment AttachmentCoordinator
	Tag        TagCoordinator
	Subview    SubviewCoordinator
	Link       LinkCoordinator
	Blocker    BlockerCoordinator
	Settings   SettingsCoordinator
	Workflow   WorkflowCoordinator
}

// newSecondaryCoordinators builds the secondary coordinators over the shared
// application service and config gateway. Each coordinator stores only its narrow
// host port even though every task/artifact port is satisfied by the one service.
// In production svc is the TUI app service and gateway is the config manager; a
// scenario test injects an in-memory service and a temp-dir config manager.
func newSecondaryCoordinators(
	svc *app.Service,
	committer ConfigCommitter,
) SecondaryCoordinators {
	var projectStatusArtifacts app.WorkflowProjectStatusArtifactBuilder
	var planningArtifacts app.PlanningArtifactBuilder
	if manager, ok := committer.(*config.Manager); ok {
		projectStatusArtifacts = workflowProjectStatusArtifactBuilder(manager)
		planningArtifacts = planningReconcileArtifactBuilder(manager)
	}
	return SecondaryCoordinators{
		Bulk:       BulkCoordinator{name: "bulk", bulk: svc, projectStatusArtifacts: projectStatusArtifacts, planningArtifacts: planningArtifacts},
		TaskItem:   TaskItemCoordinator{name: "task-item", task: svc, projectStatusArtifacts: projectStatusArtifacts, planningArtifacts: planningArtifacts},
		Attachment: AttachmentCoordinator{name: "attachment", attach: svc},
		Tag:        TagCoordinator{name: "tag", tag: svc},
		Subview:    SubviewCoordinator{name: "subview", sub: svc},
		Link:       LinkCoordinator{name: "link", link: svc},
		Blocker:    BlockerCoordinator{name: "blocker", blocker: svc},
		Settings:   newSettingsCoordinator(committer),
		Workflow:   newWorkflowCoordinator(committer),
	}
}

// newSettingsCoordinator and newWorkflowCoordinator let the config-domain
// views build their coordinator over the same manager they already hold,
// keeping one seam without threading SecondaryCoordinators through every
// constructor. Callers must pass nil (not a typed nil manager) when no
// committer exists so the commit guards fire.
func newSettingsCoordinator(committer SettingsCommitter) SettingsCoordinator {
	return SettingsCoordinator{name: "settings", committer: committer}
}

func newWorkflowCoordinator(committer AutomationCommitter) WorkflowCoordinator {
	return WorkflowCoordinator{name: "workflow", committer: committer}
}
