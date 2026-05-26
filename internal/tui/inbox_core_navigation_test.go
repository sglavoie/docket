package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// stubInboxBackend is a minimal backend for testing advanceToNext.
type stubInboxBackend struct {
	tasks []*models.Task
}

func (s *stubInboxBackend) taskCount() int { return len(s.tasks) }
func (s *stubInboxBackend) taskRef(i int) (*models.Task, *models.Project) {
	return s.tasks[i], &models.Project{ID: "p1"}
}
func (s *stubInboxBackend) closeMsg(processed, skipped int) tea.Msg {
	return struct{ closed bool }{closed: true}
}
func (s *stubInboxBackend) handleSavedMsg(tea.Msg) (bool, error)         { return false, nil }
func (s *stubInboxBackend) saveCmd() tea.Cmd                             { return nil }
func (s *stubInboxBackend) reloadCmd() tea.Cmd                           { return nil }
func (s *stubInboxBackend) rebuildTasks()                                {}
func (s *stubInboxBackend) deleteFromData(*models.Task, *models.Project) {}
func (s *stubInboxBackend) markDeletedProcessed() bool                   { return false }
func (s *stubInboxBackend) shouldProcessOnSectionChange(*models.Task, *models.Project) bool {
	return false
}
func (s *stubInboxBackend) shouldProcessOnStatusChange(models.TaskStatus) bool { return false }
func (s *stubInboxBackend) showProjectInHeader() bool                          { return false }
func (s *stubInboxBackend) headerTitle() string                                { return "" }
func (s *stubInboxBackend) headerSubtitleExtra() string                        { return "" }
func (s *stubInboxBackend) emptyStateText() string                             { return "" }
func (s *stubInboxBackend) emptyInboxText() string                             { return "" }
func (s *stubInboxBackend) returnHint() string                                 { return "" }
func (s *stubInboxBackend) viewType() ViewType                                 { return ViewInboxProcessor }
func (s *stubInboxBackend) helpViewName() string                               { return "" }

// newTestInboxCore builds an inboxCore wired to the given stub, ready for
// navigation tests without requiring a full config or model.
func newTestInboxCore(tasks []*models.Task) *inboxCore {
	stub := &stubInboxBackend{tasks: tasks}
	c := &inboxCore{
		processedIDs: make(map[string]bool),
		skippedIDs:   make(map[string]bool),
		backend:      stub,
	}
	return c
}

// TestAdvanceToNext_BackwardSearchPreventsEarlyClose is the regression test for
// the bug where advanceToNext closed the inbox when the cursor was on the last
// item, even if earlier unprocessed items remained.
//
// Scenario: 3 tasks [t0, t1, t2]. Cursor at index 2. t2 is processed/skipped.
// t0 and t1 are unprocessed. advanceToNext must NOT emit a close message and
// must move the cursor back to t0 (or t1).
func TestAdvanceToNext_BackwardSearchPreventsEarlyClose(t *testing.T) {
	tasks := []*models.Task{
		{ID: "t0", Title: "Task 0"},
		{ID: "t1", Title: "Task 1"},
		{ID: "t2", Title: "Task 2"},
	}
	c := newTestInboxCore(tasks)
	c.currentIndex = 2        // cursor on last item
	c.skippedIDs["t2"] = true // mark last item as skipped

	cmd := c.advanceToNext(true)

	// Must NOT emit a close message.
	if cmd != nil {
		msg := cmd()
		if _, closed := msg.(struct{ closed bool }); closed {
			t.Fatal("advanceToNext closed the inbox even though t0 and t1 are unprocessed")
		}
	}

	// Cursor must have moved back to t0 (the first unprocessed item).
	if c.currentIndex != 0 {
		t.Errorf("expected cursor to move back to index 0, got %d", c.currentIndex)
	}
}

// TestAdvanceToNext_ClosesWhenAllProcessed ensures that the inbox does close
// once all tasks have been handled (no false negatives from the backward search).
func TestAdvanceToNext_ClosesWhenAllProcessed(t *testing.T) {
	tasks := []*models.Task{
		{ID: "t0", Title: "Task 0"},
		{ID: "t1", Title: "Task 1"},
		{ID: "t2", Title: "Task 2"},
	}
	c := newTestInboxCore(tasks)
	c.currentIndex = 2
	c.processedIDs["t0"] = true
	c.processedIDs["t1"] = true
	c.skippedIDs["t2"] = true

	cmd := c.advanceToNext(true)

	if cmd == nil {
		t.Fatal("expected a close command when all tasks are processed/skipped, got nil")
	}
	msg := cmd()
	if _, closed := msg.(struct{ closed bool }); !closed {
		t.Errorf("expected close message, got %T", msg)
	}
}

// TestAdvanceToNext_ForwardSearchStillWorks ensures that the forward search
// path still works correctly (regression guard).
func TestAdvanceToNext_ForwardSearchStillWorks(t *testing.T) {
	tasks := []*models.Task{
		{ID: "t0", Title: "Task 0"},
		{ID: "t1", Title: "Task 1"},
		{ID: "t2", Title: "Task 2"},
	}
	c := newTestInboxCore(tasks)
	c.currentIndex = 0
	c.processedIDs["t0"] = true

	cmd := c.advanceToNext(true)

	// Should advance to t1 without closing.
	if cmd != nil {
		t.Errorf("expected nil cmd (moved forward to t1), got non-nil")
	}
	if c.currentIndex != 1 {
		t.Errorf("expected cursor at index 1, got %d", c.currentIndex)
	}
}
