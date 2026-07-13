package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// session4Backend is a richer inboxBackend stub for session-4 contract tests.
// It owns a project (for findTaskByID/saveProjectCmd parity with the real
// backends), exposes scope-style hooks, and tracks which project was passed
// to saveProjectCmd so undo can be verified against the action's project.
type session4Backend struct {
	project           *models.Project
	scope             string // "local" or "global"
	tasksFn           func() []*models.Task
	lastSavedProject  *models.Project
	closeMsgProcessed int
	closeMsgSkipped   int
	closeMsgFired     bool
}

func newSession4Backend(scope string, project *models.Project, tasksFn func() []*models.Task) *session4Backend {
	return &session4Backend{
		project: project,
		scope:   scope,
		tasksFn: tasksFn,
	}
}

func (s *session4Backend) taskCount() int { return len(s.tasksFn()) }
func (s *session4Backend) taskRef(i int) (*models.Task, *models.Project) {
	return s.tasksFn()[i], s.project
}
func (s *session4Backend) closeMsg(processed, skipped int) tea.Msg {
	s.closeMsgFired = true
	s.closeMsgProcessed = processed
	s.closeMsgSkipped = skipped
	return CloseInboxProcessorMsg{ProcessedCount: processed, SkippedCount: skipped}
}
func (s *session4Backend) handleSavedMsg(tea.Msg) (bool, error) { return false, nil }
func (s *session4Backend) saveCmd() tea.Cmd {
	return s.saveProjectCmd(s.project)
}
func (s *session4Backend) saveProjectCmd(project *models.Project) tea.Cmd {
	s.lastSavedProject = project
	return func() tea.Msg { return InboxProjectSavedMsg{} }
}
func (s *session4Backend) reloadCmd() tea.Cmd { return nil }
func (s *session4Backend) rebuildTasks()      {}
func (s *session4Backend) deleteFromData(task *models.Task, project *models.Project) {
	if task == nil {
		return
	}
	if project == nil {
		project = s.project
	}
	if project == nil {
		return
	}
	for i, t := range project.Tasks {
		if t.ID == task.ID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			return
		}
	}
}

func (s *session4Backend) markDeletedProcessed() bool { return true }
func (s *session4Backend) shouldProcessOnSectionChange(task *models.Task, project *models.Project) bool {
	current := project.GetTaskByID(task.ID)
	return current != nil && willLeaveInbox(current)
}
func (s *session4Backend) shouldProcessOnStateChange(newState models.WorkflowState) bool {
	return newState.IsTerminal()
}
func (s *session4Backend) findTaskByID(taskID, _ string) (*models.Task, *models.Project) {
	if t := s.project.GetTaskByID(taskID); t != nil {
		return t, s.project
	}
	return nil, nil
}
func (s *session4Backend) showProjectInHeader() bool   { return s.scope == "global" }
func (s *session4Backend) headerTitle() string         { return "" }
func (s *session4Backend) headerSubtitleExtra() string { return "" }
func (s *session4Backend) emptyStateText() string      { return "" }
func (s *session4Backend) emptyInboxText() string      { return "" }
func (s *session4Backend) returnHint() string          { return "" }
func (s *session4Backend) viewType() ViewType          { return ViewInboxProcessor }
func (s *session4Backend) helpViewName() string        { return "" }

// newSession4Core builds a project with three processable inbox tasks plus a
// wired-up inboxCore + session4Backend. The returned `tasks` closure mirrors
// what the real backends do — it filters out tasks that left the inbox so
// rebuildTasks-style updates land naturally.
func newSession4Core(t *testing.T) (*inboxCore, *session4Backend, *models.Project) {
	t.Helper()
	project := &models.Project{ID: "p1", Name: "Project One"}
	for i, title := range []string{"Triage A", "Triage B", "Triage C"} {
		project.Tasks = append(project.Tasks, &models.Task{
			ID:            title + "-id",
			ShortID:       i + 1,
			Title:         title,
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateInbox,
			Priority:      3,
		})
	}
	var inboxTasks []*models.Task
	rebuild := func() {
		inboxTasks = inboxTasks[:0]
		for _, task := range project.Tasks {
			if isProcessableInboxTask(task) {
				inboxTasks = append(inboxTasks, task)
			}
		}
	}
	rebuild()

	backend := newSession4Backend("local", project, func() []*models.Task { return inboxTasks })
	wrapper := &session4BackendRebuilder{session4Backend: backend, rebuild: rebuild}

	c := &inboxCore{
		processedIDs: make(map[string]bool),
		skippedIDs:   make(map[string]bool),
		backend:      wrapper,
	}
	return c, backend, project
}

// session4BackendRebuilder lets tests inject a rebuild callback so the
// in-memory inboxTasks slice mirrors what the real processors do after a
// task leaves the inbox.
type session4BackendRebuilder struct {
	*session4Backend
	rebuild func()
}

func (s *session4BackendRebuilder) rebuildTasks() { s.rebuild() }

func collectCommandMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, subCmd := range batch {
			msgs = append(msgs, collectCommandMessages(subCmd)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

// --- Tests ---

// TestProgressStatsRemainStableAcrossDeletion verifies the
// "processed + skipped + remaining = total" invariant survives a rebuild that
// removes a processed task from the inbox queue. The pre-session-4 formula
// used taskCount() as `total`, which made the figure shrink each time a task
// left the queue and confused users who had just completed something.
func TestProgressStatsRemainStableAcrossDeletion(t *testing.T) {
	c, backend, project := newSession4Core(t)
	_ = backend

	// Initial state: 3 tasks in inbox, none handled.
	if p, s, r, total := c.getProgressStats(); p != 0 || s != 0 || r != 3 || total != 3 {
		t.Fatalf("initial stats: got (p=%d s=%d r=%d total=%d), want (0 0 3 3)", p, s, r, total)
	}

	// Simulate completing the first task: mark processed, move it to Completed
	// section, rebuild. The user's perception is "I've handled one of three";
	// total must stay at 3.
	c.processedIDs[project.Tasks[0].ID] = true
	project.Tasks[0].ApplyTransition(models.StatusDone)
	c.backend.rebuildTasks()

	p, s, r, total := c.getProgressStats()
	if p != 1 || s != 0 || r != 2 || total != 3 {
		t.Errorf("after completion: got (p=%d s=%d r=%d total=%d), want (1 0 2 3)", p, s, r, total)
	}

	// Sectioning another task to Next must also keep total stable.
	c.processedIDs[project.Tasks[1].ID] = true
	project.Tasks[1].SetSection(models.SectionNext)
	c.backend.rebuildTasks()

	p, s, r, total = c.getProgressStats()
	if p != 2 || s != 0 || r != 1 || total != 3 {
		t.Errorf("after section change: got (p=%d s=%d r=%d total=%d), want (2 0 1 3)", p, s, r, total)
	}

	// Skipping the last visible task increments skipped, decrements remaining,
	// and keeps total = 3.
	c.skippedIDs[project.Tasks[2].ID] = true
	p, s, r, total = c.getProgressStats()
	if p != 2 || s != 1 || r != 0 || total != 3 {
		t.Errorf("after skip: got (p=%d s=%d r=%d total=%d), want (2 1 0 3)", p, s, r, total)
	}
}

// TestUndoAfterAutoAdvanceRestoresCompletedTask covers the headline session-4
// fix: after marking a task done, the inbox auto-advances and reloads the
// queue (removing the completed task). Pressing u must still undo the
// completion — restoring status, section, and CompletedAt — and refocus the
// cursor on the restored task.
func TestUndoAfterAutoAdvanceRestoresCompletedTask(t *testing.T) {
	c, backend, project := newSession4Core(t)

	first := project.Tasks[0]
	originalUpdatedAt := first.UpdatedAt

	// Simulate "complete the first task, auto-advance, rebuild".
	snapshot := captureInboxSnapshot(first)
	first.ApplyTransition(models.StatusDone)
	c.processedIDs[first.ID] = true
	c.processingHistory = append(c.processingHistory, processingAction{
		taskID:       first.ID,
		projectID:    project.ID,
		actionType:   "completed task",
		timestamp:    time.Now(),
		prevSnapshot: snapshot,
	})
	c.currentIndex = 1 // auto-advance moved past index 0
	c.pendingFocusID = project.Tasks[1].ID
	c.backend.rebuildTasks() // first leaves the queue
	c.realignCurrentIndex()  // refocus on auto-advanced target

	// Sanity: queue is now [B, C], cursor on B.
	if backend.taskCount() != 2 {
		t.Fatalf("post-rebuild taskCount: got %d, want 2", backend.taskCount())
	}
	if got, _ := c.currentTaskRef(); got == nil || got.ID != project.Tasks[1].ID {
		t.Fatalf("post-rebuild cursor: got %v, want %s", got, project.Tasks[1].ID)
	}

	// Now undo. The completion must reverse even though the action's task is
	// no longer the current task.
	cmd := c.undoLastAction()
	if cmd == nil {
		t.Fatal("undoLastAction returned nil cmd; expected a saveProjectCmd")
	}
	_ = cmd() // exercise the closure to make sure it doesn't panic

	if first.State() != models.StateInbox {
		t.Errorf("state after undo: got %q, want %q", first.State(), models.StateInbox)
	}
	if first.CompletedAt != nil {
		t.Error("CompletedAt should be nil after undoing completion")
	}
	if c.processedIDs[first.ID] {
		t.Error("processedIDs should not contain the undone task")
	}
	if c.pendingFocusID != first.ID {
		t.Errorf("pendingFocusID: got %q, want %q (the restored task)", c.pendingFocusID, first.ID)
	}
	if backend.lastSavedProject == nil || backend.lastSavedProject.ID != project.ID {
		t.Errorf("undo must save action's project; lastSavedProject=%v", backend.lastSavedProject)
	}
	if originalUpdatedAt.IsZero() && first.UpdatedAt.IsZero() {
		t.Error("restoreInboxSnapshot should refresh UpdatedAt")
	}

	// After save+rebuild, the restored task is back in the queue.
	c.backend.rebuildTasks()
	c.realignCurrentIndex()
	if backend.taskCount() != 3 {
		t.Fatalf("after rebuild, taskCount: got %d, want 3", backend.taskCount())
	}
	if got, _ := c.currentTaskRef(); got == nil || got.ID != first.ID {
		t.Fatalf("cursor should land on restored task, got %v", got)
	}
}

// TestUndoAfterAutoAdvanceRestoresSectionChange exercises the
// "move to Next, then undo" flow that the goal explicitly calls out. Same
// shape as the completion case but the snapshot/restore path covers the
// Section field instead of Status.
func TestUndoAfterAutoAdvanceRestoresSectionChange(t *testing.T) {
	c, _, project := newSession4Core(t)

	first := project.Tasks[0]
	snapshot := captureInboxSnapshot(first)
	first.SetSection(models.SectionNext)
	c.processedIDs[first.ID] = true
	c.processingHistory = append(c.processingHistory, processingAction{
		taskID:       first.ID,
		projectID:    project.ID,
		actionType:   "moved to Next",
		prevSnapshot: snapshot,
	})
	c.currentIndex = 1
	c.pendingFocusID = project.Tasks[1].ID
	c.backend.rebuildTasks()
	c.realignCurrentIndex()

	c.undoLastAction()

	if first.State() != models.StateInbox {
		t.Errorf("state after undo: got %q, want Inbox", first.State())
	}
	if c.processedIDs[first.ID] {
		t.Error("processedIDs should not contain the undone task")
	}
}

// TestUndoSkippedTaskRemovesFromSkippedSet verifies the "undo skip" flow. Skip
// records a no-snapshot action so undo can pop the skip without trying to
// touch task state. The task must come back as remaining and the cursor must
// refocus on it.
func TestUndoSkippedTaskRemovesFromSkippedSet(t *testing.T) {
	c, _, project := newSession4Core(t)

	// Simulate: skip first task, auto-advance to second.
	first := project.Tasks[0]
	c.skippedIDs[first.ID] = true
	c.processingHistory = append(c.processingHistory, processingAction{
		taskID:     first.ID,
		projectID:  project.ID,
		actionType: "skip",
	})
	c.currentIndex = 1

	// Confirm skipped count is correct.
	_, s, _, _ := c.getProgressStats()
	if s != 1 {
		t.Errorf("skipped count after skip: got %d, want 1", s)
	}

	c.undoLastAction()

	if c.skippedIDs[first.ID] {
		t.Error("skippedIDs should not contain the unskipped task")
	}
	_, s, r, total := c.getProgressStats()
	if s != 0 || r != 3 || total != 3 {
		t.Errorf("stats after undo skip: got (s=%d r=%d total=%d), want (0 3 3)", s, r, total)
	}
	if c.pendingFocusID != "" {
		// Pure-skip undo realigns synchronously, so pendingFocusID is consumed.
		// Verify the cursor actually landed on the unskipped task.
		t.Errorf("pendingFocusID should be consumed by realignCurrentIndex, got %q", c.pendingFocusID)
	}
	if got, _ := c.currentTaskRef(); got == nil || got.ID != first.ID {
		t.Errorf("cursor should land on the unskipped task, got %v", got)
	}
}

// TestUndoNothingWhenHistoryEmpty exercises the empty-history branch — the
// status line must say "Nothing to undo" without mutating state.
func TestUndoNothingWhenHistoryEmpty(t *testing.T) {
	c, _, _ := newSession4Core(t)

	cmd := c.undoLastAction()
	if cmd != nil {
		t.Errorf("undo with empty history should return nil cmd, got %v", cmd)
	}
	if c.statusMessage != "Nothing to undo" {
		t.Errorf("status: got %q, want %q", c.statusMessage, "Nothing to undo")
	}
}

// TestCloseMessageReportsAccurateCounts verifies that the close command fired
// by advanceToNext carries the final processed/skipped counts the user just
// saw in the header. Session 5 builds the inbox-clear status copy off these
// numbers, so they need to match getProgressStats.
func TestCloseMessageReportsAccurateCounts(t *testing.T) {
	c, backend, project := newSession4Core(t)

	// Mark every task: one processed, one skipped, one processed.
	c.processedIDs[project.Tasks[0].ID] = true
	c.skippedIDs[project.Tasks[1].ID] = true
	c.processedIDs[project.Tasks[2].ID] = true
	c.currentIndex = 2

	cmd := c.advanceToNext(true)
	if cmd == nil {
		t.Fatal("advanceToNext should return a close command when all tasks are handled")
	}
	cmd() // fire the close command
	if !backend.closeMsgFired {
		t.Fatal("close message should have fired")
	}
	if backend.closeMsgProcessed != 2 {
		t.Errorf("processed count in close msg: got %d, want 2", backend.closeMsgProcessed)
	}
	if backend.closeMsgSkipped != 1 {
		t.Errorf("skipped count in close msg: got %d, want 1", backend.closeMsgSkipped)
	}
}

func TestExecuteDeleteFinalItemClosesWithProcessedCount(t *testing.T) {
	c, backend, project := newSession4Core(t)

	deletedID := project.Tasks[2].ID
	c.processedIDs[project.Tasks[0].ID] = true
	c.skippedIDs[project.Tasks[1].ID] = true
	c.currentIndex = 2

	cmd := c.executeDelete()
	if cmd == nil {
		t.Fatal("executeDelete should return save and close commands")
	}

	if project.GetTaskByID(deletedID) != nil {
		t.Fatal("deleted task should be removed from project data synchronously")
	}
	if !c.processedIDs[deletedID] {
		t.Fatal("deleted task should be marked processed")
	}
	if backend.lastSavedProject != project {
		t.Fatal("delete should capture a save command for the deleted task's project")
	}

	msgs := collectCommandMessages(cmd)
	var sawSave bool
	for _, msg := range msgs {
		if _, ok := msg.(InboxProjectSavedMsg); ok {
			sawSave = true
			break
		}
	}
	if !sawSave {
		t.Fatalf("delete command should include a save message, got %T (%v)", msgs, msgs)
	}
	if !backend.closeMsgFired {
		t.Fatal("delete should close the processor when no unprocessed tasks remain")
	}
	if backend.closeMsgProcessed != 2 {
		t.Errorf("processed count in close msg: got %d, want 2", backend.closeMsgProcessed)
	}
	if backend.closeMsgSkipped != 1 {
		t.Errorf("skipped count in close msg: got %d, want 1", backend.closeMsgSkipped)
	}
}

func TestExecuteDeleteAdvancesWhenUnprocessedTaskRemains(t *testing.T) {
	c, backend, project := newSession4Core(t)

	deletedID := project.Tasks[0].ID
	nextID := project.Tasks[1].ID
	c.currentIndex = 0

	cmd := c.executeDelete()
	if cmd == nil {
		t.Fatal("executeDelete should return a save command")
	}
	if project.GetTaskByID(deletedID) != nil {
		t.Fatal("deleted task should be removed from project data synchronously")
	}
	if !c.processedIDs[deletedID] {
		t.Fatal("deleted task should be marked processed")
	}
	if backend.closeMsgFired {
		t.Fatal("delete should not close while an unprocessed task remains")
	}
	if c.pendingFocusID != nextID {
		t.Fatalf("pending focus ID: got %q, want %q", c.pendingFocusID, nextID)
	}

	msgs := collectCommandMessages(cmd)
	for _, msg := range msgs {
		if _, ok := msg.(CloseInboxProcessorMsg); ok {
			t.Fatalf("delete command should not include close msg while tasks remain: %T", msg)
		}
	}

	c.backend.rebuildTasks()
	c.realignCurrentIndex()
	task, _ := c.currentTaskRef()
	if task == nil || task.ID != nextID {
		t.Fatalf("after rebuild, current task ID = %v, want %q", task, nextID)
	}
}

// TestRealignCurrentIndexFindsFocusedTaskAfterRebuild guards the auto-advance
// + rebuild interaction. Before session 4, the cursor would skip past the
// next task whenever the queue compacted (because clampCurrentIndex only
// bounded the integer, never matched on task identity).
func TestRealignCurrentIndexFindsFocusedTaskAfterRebuild(t *testing.T) {
	c, backend, project := newSession4Core(t)
	_ = backend

	// Move first task out of the inbox so a rebuild compacts the queue.
	project.Tasks[0].SetSection(models.SectionNext)
	c.processedIDs[project.Tasks[0].ID] = true
	c.pendingFocusID = project.Tasks[1].ID // expected auto-advance target
	c.currentIndex = 1                     // pre-rebuild index of project.Tasks[1]

	c.backend.rebuildTasks()
	c.realignCurrentIndex()

	if got, _ := c.currentTaskRef(); got == nil || got.ID != project.Tasks[1].ID {
		t.Errorf("realign should land on focus target; got %v", got)
	}
	if c.pendingFocusID != "" {
		t.Error("pendingFocusID should be cleared after realign succeeds")
	}
}
