package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

func (c *inboxCore) recordAction(actionType string, snapshot *inboxSnapshot) {
	task, project := c.currentTaskRef()
	if task == nil {
		return
	}
	projectID := ""
	if project != nil {
		projectID = project.ID
	}
	// A newly recorded action invalidates any pending redo, matching the linear
	// undo/redo contract the workspace ledger enforces for stateful actions.
	c.redoHistory = nil
	c.processingHistory = append(c.processingHistory, processingAction{
		taskID:       task.ID,
		projectID:    projectID,
		actionType:   actionType,
		timestamp:    time.Now(),
		prevSnapshot: snapshot,
	})
}

// undoLastAction reverses the most recent inbox action. Skip actions are pure
// in-session queue bookkeeping (no disk write), so they undo locally. Stateful
// triage (state change, section move, priority, due date, snooze, …) committed a
// durable workspace-ledger transaction, so it undoes through the ledger — which
// restores the task's exact workflow state AND any Today/Next planning artifact
// captured in the same transaction, and survives a restart because it reads the
// durable ledger rather than the in-memory processingHistory.
func (c *inboxCore) undoLastAction() tea.Cmd {
	n := len(c.processingHistory)

	// A skip on top of the in-session history undoes locally: re-enqueue the task
	// and refocus. Skips never reach disk, so there is nothing to restore and no
	// ledger entry to consume.
	if n > 0 && c.processingHistory[n-1].prevSnapshot == nil {
		action := c.processingHistory[n-1]
		c.processingHistory = c.processingHistory[:n-1]
		c.redoHistory = append(c.redoHistory, action)
		delete(c.processedIDs, action.taskID)
		delete(c.skippedIDs, action.taskID)
		c.pendingFocusID = action.taskID
		c.realignCurrentIndex()
		c.statusMessage = fmt.Sprintf("Undone: %s", action.actionType)
		return nil
	}

	// Stateful triage (or a post-restart empty history): prefer the durable
	// workspace ledger so the undo restores exact task state AND the Today/Next
	// planning artifact captured in the same transaction, and survives a restart.
	// The matching in-session action, when present, moves to the redo stack for a
	// subsequent ctrl+r; after a restart the ledger alone drives redo.
	if desc, ok := c.ledgerRestore(false); ok {
		if n > 0 {
			action := c.processingHistory[n-1]
			c.processingHistory = c.processingHistory[:n-1]
			c.redoHistory = append(c.redoHistory, action)
			delete(c.processedIDs, action.taskID)
			delete(c.skippedIDs, action.taskID)
			c.pendingFocusID = action.taskID
		}
		c.realignCurrentIndex()
		c.statusMessage = fmt.Sprintf("Undone: %s", desc)
		return nil
	}

	// No durable ledger transaction is available (e.g. an isolated harness with no
	// workspace). Fall back to the in-memory task snapshot so the last stateful
	// action is still reversible in-session.
	if n == 0 {
		c.statusMessage = "Nothing to undo"
		return nil
	}
	action := c.processingHistory[n-1]
	c.processingHistory = c.processingHistory[:n-1]
	delete(c.processedIDs, action.taskID)
	delete(c.skippedIDs, action.taskID)
	c.pendingFocusID = action.taskID

	task, project := c.backend.findTaskByID(action.taskID, action.projectID)
	if task == nil || project == nil {
		c.err = fmt.Errorf("cannot undo: task not found")
		c.pendingFocusID = ""
		return nil
	}
	restoreInboxSnapshot(task, action.prevSnapshot)
	c.statusMessage = fmt.Sprintf("Undone: %s", action.actionType)
	return c.backend.saveProjectCmd(project)
}

// redoLastAction replays the most recently undone action. Skip actions re-skip
// locally; stateful actions replay through the ledger so the redo restores exact
// task state and Today/Next planning artifacts.
func (c *inboxCore) redoLastAction() tea.Cmd {
	if n := len(c.redoHistory); n > 0 && c.redoHistory[n-1].prevSnapshot == nil {
		action := c.redoHistory[n-1]
		c.redoHistory = c.redoHistory[:n-1]
		c.processingHistory = append(c.processingHistory, action)
		c.skippedIDs[action.taskID] = true
		delete(c.processedIDs, action.taskID)
		c.pendingFocusID = action.taskID
		c.realignCurrentIndex()
		c.statusMessage = fmt.Sprintf("Redone: %s", action.actionType)
		return nil
	}

	desc, ok := c.ledgerRestore(true)
	if !ok {
		c.statusMessage = "Nothing to redo"
		return nil
	}
	if n := len(c.redoHistory); n > 0 {
		action := c.redoHistory[n-1]
		c.redoHistory = c.redoHistory[:n-1]
		c.processingHistory = append(c.processingHistory, action)
		c.pendingFocusID = action.taskID
		// If the replayed action moved the task out of the inbox (done, section
		// change), re-mark it processed so it does not reappear in the queue.
		if task, project := c.backend.findTaskByID(action.taskID, action.projectID); task != nil && project != nil && willLeaveInbox(task) {
			c.processedIDs[action.taskID] = true
			c.pendingFocusID = ""
		}
	}
	c.realignCurrentIndex()
	c.statusMessage = fmt.Sprintf("Redone: %s", desc)
	return nil
}

// ledgerRestore reverses (undo) or replays (redo) the newest workspace-ledger
// transaction, reloads every project the transaction touched, and refreshes the
// runtime so restored planning artifacts are observed. It returns the restore
// description and whether an entry was applied; an exhausted ledger returns
// ("", false) quietly so callers surface a plain "Nothing to …" status.
func (c *inboxCore) ledgerRestore(redo bool) (string, bool) {
	// Without a rooted workspace there is no durable ledger to consult; report no
	// entry so the caller falls back to the in-memory snapshot path.
	if c.runtime == nil || c.runtime.DataDir() == "" {
		return "", false
	}
	ledger := workspaceLedgerFromConfig(c.runtime)
	restore := ledger.UndoWithEntry
	if redo {
		restore = ledger.RedoWithEntry
	}
	entry, desc, err := restore(storage.GlobalScope())
	if err != nil {
		if !strings.Contains(err.Error(), "history available") {
			c.err = fmt.Errorf("inbox restore: %w", err)
		}
		return "", false
	}
	for _, id := range entry.ProjectIDs {
		if p := c.backend.projectByID(id); p != nil {
			_ = storage.LoadProject(p)
		}
	}
	if c.runtime != nil {
		_ = c.runtime.Reload()
	}
	c.backend.rebuildTasks()
	return desc, true
}

func (c *inboxCore) setPriorityQuick(key string) tea.Cmd {
	priority, _ := strconv.Atoi(key)

	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}

	snapshot := captureInboxSnapshot(task)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})

	c.recordAction(fmt.Sprintf("priority set to %d", priority), snapshot)
	c.statusMessage = fmt.Sprintf("Priority set to %d", priority)

	return c.backend.saveCmd()
}

func (c *inboxCore) changeTaskState(newState models.WorkflowState, verb string) tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}

	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	if msg := models.CanTransitionState(current.State(), newState); msg != "" {
		c.statusMessage = msg
		return nil
	}

	if newState == models.StateDone {
		if project.HasIncompleteDependencies(current) {
			incompleteDeps := project.GetIncompleteDependencies(current)
			depIDs := make([]string, 0, len(incompleteDeps))
			for _, dep := range incompleteDeps {
				depIDs = append(depIDs, fmt.Sprintf("#%d", dep.ShortID))
			}
			// Count cross-project deps not included in GetIncompleteDependencies.
			crossCount := 0
			for _, depID := range current.DependsOn {
				if models.ParseTaskRef(depID).IsCrossProject() {
					crossCount++
				}
			}
			if crossCount > 0 {
				depIDs = append(depIDs, fmt.Sprintf("%d cross-project", crossCount))
			}
			c.err = fmt.Errorf("cannot complete: incomplete dependencies (%s)",
				strings.Join(depIDs, ", "))
			return nil
		}
	}

	snapshot := captureInboxSnapshot(current)
	current.ApplyStateTransition(newState)

	c.recordAction(verb+" task", snapshot)
	c.statusMessage = fmt.Sprintf("Task #%d %s", current.ShortID, verb)

	if c.backend.shouldProcessOnStateChange(newState) {
		c.processedIDs[current.ID] = true
		return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
	}

	return c.backend.saveCmd()
}

func (c *inboxCore) sendToWaiting() tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}

	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	snapshot := captureInboxSnapshot(current)
	if !project.MutateTask(task.ID, func(t *models.Task) {
		t.SetSection(models.SectionWaiting)
	}) {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}

	c.recordAction("sent to Waiting", snapshot)
	c.statusMessage = fmt.Sprintf("Task #%d sent to Waiting", current.ShortID)
	c.processedIDs[task.ID] = true

	return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
}

func (c *inboxCore) handleSectionPickerDone(msg SectionPickerDoneMsg) tea.Cmd {
	if !inboxHandleSectionPickerCore(c, msg) {
		return nil
	}
	task, project := c.currentTaskRef()
	if task != nil && c.backend.shouldProcessOnSectionChange(task, project) {
		c.processedIDs[task.ID] = true
		return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
	}
	return c.backend.saveCmd()
}

// requestDelete emits the canonical, shell-owned deletion request for the
// current inbox task. Both lowercase x and uppercase X route here. The shell
// confirms and runs one atomic transaction; the processor defers all queue
// bookkeeping to applyCommittedDelete, invoked only after the commit succeeds,
// so a rejected transaction leaves the same task selected and unprocessed.
//
// The post-delete focus target is computed here, against the intact queue, so
// it matches the legacy in-memory delete (which advanced before the queue was
// rebuilt and thus landed on the natural next task rather than skipping it).
func (c *inboxCore) requestDelete() tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil || project == nil {
		return nil
	}
	focusID := c.nextInboxFocusAfterDelete()
	if c.pendingDeletes == nil {
		c.pendingDeletes = make(map[string]inboxPendingDelete)
	}
	c.pendingDeletes[task.ID] = inboxPendingDelete{focusID: focusID}
	origin := c.backend.viewType()
	ref := TaskProjectRef{Task: task, Project: project}
	return func() tea.Msg {
		return TaskDeleteRequestedMsg{Refs: []TaskProjectRef{ref}, Origin: origin}
	}
}

// nextInboxFocusAfterDelete returns the task the cursor should land on once the
// current task leaves the queue. It mirrors advanceToNext's forward-then-backward
// scan but performs no mutation, so it can run at request time — before the shell
// reloads the committed queue. A blank result means no other unprocessed/unskipped
// task remained at request time; whether the processor actually closes is decided
// at apply time by shouldCloseAfterDelete, not by this request-time scan.
func (c *inboxCore) nextInboxFocusAfterDelete() (focusID string) {
	for i := c.currentIndex + 1; i < c.backend.taskCount(); i++ {
		task, _ := c.backend.taskRef(i)
		if !c.processedIDs[task.ID] && !c.skippedIDs[task.ID] {
			return task.ID
		}
	}
	for i := 0; i < c.currentIndex; i++ {
		task, _ := c.backend.taskRef(i)
		if !c.processedIDs[task.ID] && !c.skippedIDs[task.ID] {
			return task.ID
		}
	}
	return ""
}

// shouldCloseAfterDelete reports whether the processor should auto-close now that
// a committed delete has rebuilt the queue. It recomputes closure — rather than
// trusting a request-time willClose snapshot — from two live signals:
//
//  1. no other delete this processor requested is still awaiting its commit
//     (c.pendingDeletes is empty after the current entry was removed), and
//  2. no unprocessed/unskipped task remains in the rebuilt committed queue.
//
// This is the fix for back-to-back deletion of the final two tasks: while both
// were in flight each still saw the other queued, so a request-time snapshot
// captured willClose=false for both and neither closed. Gating on the remaining
// in-flight bookkeeping makes only the last delete to commit close the processor,
// so it closes exactly once with complete counts in either result order. Skipped
// tasks left in the queue never block closure, mirroring advanceToNext.
func (c *inboxCore) shouldCloseAfterDelete() bool {
	if len(c.pendingDeletes) > 0 {
		return false
	}
	for i := 0; i < c.backend.taskCount(); i++ {
		task, _ := c.backend.taskRef(i)
		if !c.processedIDs[task.ID] && !c.skippedIDs[task.ID] {
			return false
		}
	}
	return true
}

// applyCommittedDelete finalizes processor state after the shell-owned
// transaction commits the canonical deletion of taskID. It mirrors the advance,
// processed-count, and cursor-anchoring bookkeeping the legacy in-memory
// executeDelete performed inline, but runs post-commit: the committed project
// data (reloaded by the shell) no longer contains the task, so the queue is
// rebuilt and the cursor re-anchors on the focus target captured at request
// time. Auto-close is recomputed here against the rebuilt queue plus the
// remaining in-flight deletes (shouldCloseAfterDelete) rather than a request-time
// snapshot, so back-to-back deletion of the final two tasks closes exactly once,
// reporting the same processed/skipped counts the header showed.
func (c *inboxCore) applyCommittedDelete(taskID string) tea.Cmd {
	if taskID == "" {
		return nil
	}
	pending, ok := c.pendingDeletes[taskID]
	if !ok {
		return nil
	}
	delete(c.pendingDeletes, taskID)
	focusID := pending.focusID

	c.backend.rebuildTasks()
	c.statusMessage = "Task deleted"

	if !c.backend.markDeletedProcessed() {
		c.clampCurrentIndex()
		return nil
	}
	c.processedIDs[taskID] = true

	if c.shouldCloseAfterDelete() {
		c.pendingFocusID = ""
		processed, skipped := len(c.processedIDs), len(c.skippedIDs)
		return func() tea.Msg { return c.backend.closeMsg(processed, skipped) }
	}

	c.pendingFocusID = focusID
	c.realignCurrentIndex()
	return nil
}

// --- inboxOpsView implementation (used by inbox_operations_shared.go) ---

func (c *inboxCore) getInboxTaskRef() (*models.Task, *models.Project) {
	return c.currentTaskRef()
}

func (c *inboxCore) inboxSave() tea.Cmd        { return c.backend.saveCmd() }
func (c *inboxCore) inboxReload() tea.Cmd      { return c.backend.reloadCmd() }
func (c *inboxCore) setInboxStatus(msg string) { c.statusMessage = msg }
func (c *inboxCore) inboxCaptureSnapshot(task *models.Task) *inboxSnapshot {
	return captureInboxSnapshot(task)
}
func (c *inboxCore) inboxRecordAction(desc string, snapshot *inboxSnapshot) {
	c.recordAction(desc, snapshot)
}

func (c *inboxCore) changePriority(priority int) tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	snapshot := captureInboxSnapshot(task)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.Priority = priority
	})
	c.recordAction(fmt.Sprintf("priority set to %d", priority), snapshot)
	c.statusMessage = fmt.Sprintf("Priority set to %d", priority)
	return c.backend.saveCmd()
}

func (c *inboxCore) setQuickDueDate(days int) tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}
	dueDate := util.StartOfDay(time.Now()).AddDate(0, 0, days)
	snapshot := captureInboxSnapshot(current)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = &dueDate
		t.HasTime = false
	})
	c.recordAction("due date set", snapshot)
	c.statusMessage = fmt.Sprintf("Due date set to %s", dueDate.Format("Jan 2"))
	return c.backend.saveCmd()
}

func (c *inboxCore) clearDueDate() tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}
	snapshot := captureInboxSnapshot(current)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.DueDate = nil
		t.HasTime = false
	})
	c.recordAction("due date cleared", snapshot)
	c.statusMessage = "Due date cleared"
	return c.backend.saveCmd()
}

func (c *inboxCore) setSnooze(snoozeDate *time.Time) tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}
	snapshot := captureInboxSnapshot(current)
	project.MutateTask(task.ID, func(t *models.Task) {
		t.SnoozedUntil = snoozeDate
	})
	c.recordAction("snooze updated", snapshot)
	if snoozeDate == nil {
		c.statusMessage = "Snooze cleared"
	} else {
		c.statusMessage = fmt.Sprintf("Snoozed until %s", snoozeDate.Format("Mon Jan 2"))
	}
	return c.backend.saveCmd()
}

func (c *inboxCore) moveToSection(section string) tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	current := project.GetTaskByID(task.ID)
	if current == nil {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}
	snapshot := captureInboxSnapshot(current)
	if !project.MutateTask(task.ID, func(t *models.Task) {
		t.SetSection(section)
	}) {
		c.statusMessage = "Error: task no longer exists in project"
		return nil
	}
	sectionName := section
	if sectionName == "" {
		sectionName = "Active"
	}
	c.recordAction(fmt.Sprintf("moved to %s", sectionName), snapshot)
	c.statusMessage = fmt.Sprintf("Moved to %s", sectionName)
	if c.backend.shouldProcessOnSectionChange(task, project) {
		c.processedIDs[task.ID] = true
		return tea.Batch(c.backend.saveCmd(), c.advanceToNext(true))
	}
	return c.backend.saveCmd()
}

func (c *inboxCore) openSnoozePicker() tea.Cmd {
	task, project := c.currentTaskRef()
	if task == nil {
		return nil
	}
	return func() tea.Msg {
		return SetSnoozeDateMsg{Task: task, Project: project}
	}
}
