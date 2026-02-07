package tui

import "testing"

func TestNavHistory_RecordAndGoBack(t *testing.T) {
	h := NewNavHistory()

	h.Record(NavEntry{ViewType: ViewDashboard})
	h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})
	h.Record(NavEntry{ViewType: ViewKanban, ProjectID: "p1"})

	if !h.CanGoBack() {
		t.Fatal("expected CanGoBack to be true")
	}

	entry, ok := h.GoBack()
	if !ok {
		t.Fatal("expected GoBack to succeed")
	}
	if entry.ViewType != ViewProjectTree || entry.ProjectID != "p1" {
		t.Fatalf("unexpected entry: %+v", entry)
	}

	entry, ok = h.GoBack()
	if !ok {
		t.Fatal("expected GoBack to succeed")
	}
	if entry.ViewType != ViewDashboard {
		t.Fatalf("expected Dashboard, got %v", entry.ViewType)
	}

	if h.CanGoBack() {
		t.Fatal("expected CanGoBack to be false at beginning")
	}
}

func TestNavHistory_GoForward(t *testing.T) {
	h := NewNavHistory()

	h.Record(NavEntry{ViewType: ViewDashboard})
	h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})
	h.Record(NavEntry{ViewType: ViewKanban, ProjectID: "p1"})

	h.GoBack()
	h.GoBack()

	if !h.CanGoForward() {
		t.Fatal("expected CanGoForward to be true")
	}

	entry, ok := h.GoForward()
	if !ok {
		t.Fatal("expected GoForward to succeed")
	}
	if entry.ViewType != ViewProjectTree {
		t.Fatalf("expected ProjectTree, got %v", entry.ViewType)
	}

	entry, ok = h.GoForward()
	if !ok {
		t.Fatal("expected GoForward to succeed")
	}
	if entry.ViewType != ViewKanban {
		t.Fatalf("expected Kanban, got %v", entry.ViewType)
	}

	if h.CanGoForward() {
		t.Fatal("expected CanGoForward to be false at end")
	}
}

func TestNavHistory_RecordTruncatesForwardHistory(t *testing.T) {
	h := NewNavHistory()

	h.Record(NavEntry{ViewType: ViewDashboard})
	h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})
	h.Record(NavEntry{ViewType: ViewKanban, ProjectID: "p1"})

	// Go back to Dashboard
	h.GoBack()
	h.GoBack()

	// Record a new entry — should truncate forward history
	h.Record(NavEntry{ViewType: ViewDailyReview})

	if h.CanGoForward() {
		t.Fatal("expected forward history to be truncated")
	}

	// Only Dashboard and DailyReview should remain
	entry, ok := h.GoBack()
	if !ok || entry.ViewType != ViewDashboard {
		t.Fatalf("expected Dashboard, got %+v (ok=%v)", entry, ok)
	}
	if h.CanGoBack() {
		t.Fatal("expected no more back history")
	}
}

func TestNavHistory_DeduplicateConsecutive(t *testing.T) {
	h := NewNavHistory()

	h.Record(NavEntry{ViewType: ViewDashboard})
	h.Record(NavEntry{ViewType: ViewDashboard})
	h.Record(NavEntry{ViewType: ViewDashboard})

	// Should only have one entry, so can't go back
	if h.CanGoBack() {
		t.Fatal("expected duplicates to be collapsed")
	}
}

func TestNavHistory_DeduplicateIncludesProjectAndTask(t *testing.T) {
	h := NewNavHistory()

	h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})
	h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p2"})

	// Different ProjectID means they are NOT duplicates
	if !h.CanGoBack() {
		t.Fatal("different ProjectID entries should not be deduplicated")
	}
}

func TestNavHistory_CapAt200(t *testing.T) {
	h := NewNavHistory()

	for i := 0; i < 250; i++ {
		// Alternate between two view types to avoid dedup
		if i%2 == 0 {
			h.Record(NavEntry{ViewType: ViewDashboard})
		} else {
			h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})
		}
	}

	if len(h.entries) != maxNavHistoryEntries {
		t.Fatalf("expected %d entries, got %d", maxNavHistoryEntries, len(h.entries))
	}

	// Cursor should be at the end
	if h.cursor != len(h.entries)-1 {
		t.Fatalf("expected cursor at %d, got %d", len(h.entries)-1, h.cursor)
	}

	// Should still be able to go back
	if !h.CanGoBack() {
		t.Fatal("expected CanGoBack after cap")
	}
}

func TestNavHistory_LockPreventsRecording(t *testing.T) {
	h := NewNavHistory()

	h.Record(NavEntry{ViewType: ViewDashboard})
	h.Lock()
	h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})
	h.Unlock()

	if h.CanGoBack() {
		t.Fatal("expected locked recording to be skipped")
	}
}

func TestNavHistory_EmptyHistory(t *testing.T) {
	h := NewNavHistory()

	if h.CanGoBack() {
		t.Fatal("expected CanGoBack false on empty")
	}
	if h.CanGoForward() {
		t.Fatal("expected CanGoForward false on empty")
	}

	_, ok := h.GoBack()
	if ok {
		t.Fatal("expected GoBack to fail on empty")
	}
	_, ok = h.GoForward()
	if ok {
		t.Fatal("expected GoForward to fail on empty")
	}
}

func TestNavHistory_TaskScopedEntry(t *testing.T) {
	h := NewNavHistory()

	h.Record(NavEntry{ViewType: ViewDashboard})
	h.Record(NavEntry{ViewType: ViewTaskComments, ProjectID: "p1", TaskID: "t1"})

	entry, ok := h.GoBack()
	if !ok || entry.ViewType != ViewDashboard {
		t.Fatalf("unexpected: %+v", entry)
	}

	entry, ok = h.GoForward()
	if !ok || entry.ViewType != ViewTaskComments || entry.TaskID != "t1" {
		t.Fatalf("unexpected: %+v", entry)
	}
}

func TestNavHistory_Current(t *testing.T) {
	t.Run("empty history returns false", func(t *testing.T) {
		h := NewNavHistory()
		_, ok := h.Current()
		if ok {
			t.Fatal("expected Current to return false on empty history")
		}
	})

	t.Run("returns last recorded entry", func(t *testing.T) {
		h := NewNavHistory()
		h.Record(NavEntry{ViewType: ViewDashboard})
		h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})

		entry, ok := h.Current()
		if !ok {
			t.Fatal("expected Current to succeed")
		}
		if entry.ViewType != ViewProjectTree || entry.ProjectID != "p1" {
			t.Fatalf("unexpected entry: %+v", entry)
		}
	})

	t.Run("idempotent — does not move cursor", func(t *testing.T) {
		h := NewNavHistory()
		h.Record(NavEntry{ViewType: ViewDashboard})
		h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})

		for i := 0; i < 3; i++ {
			entry, ok := h.Current()
			if !ok || entry.ViewType != ViewProjectTree {
				t.Fatalf("call %d: unexpected result %+v ok=%v", i, entry, ok)
			}
		}

		// Cursor should still allow GoBack since Current didn't move it
		if !h.CanGoBack() {
			t.Fatal("expected CanGoBack to be true after repeated Current calls")
		}
	})

	t.Run("reflects position after GoBack", func(t *testing.T) {
		h := NewNavHistory()
		h.Record(NavEntry{ViewType: ViewDashboard})
		h.Record(NavEntry{ViewType: ViewProjectTree, ProjectID: "p1"})
		h.Record(NavEntry{ViewType: ViewKanban, ProjectID: "p1"})

		h.GoBack() // cursor now at ProjectTree

		entry, ok := h.Current()
		if !ok || entry.ViewType != ViewProjectTree {
			t.Fatalf("expected ProjectTree after GoBack, got %+v", entry)
		}
	})
}

func TestIsTrackedView(t *testing.T) {
	tracked := []ViewType{
		ViewDashboard, ViewProjectTree, ViewKanban, ViewDailyReview,
		ViewTaskComments, ViewTaskChecklist, ViewTaskAttachments, ViewTaskActivity,
		ViewGlobalNotes, ViewSettings, ViewScoreProfileManager,
	}
	for _, vt := range tracked {
		if !isTrackedView(vt) {
			t.Errorf("expected ViewType %d to be tracked", vt)
		}
	}

	untracked := []ViewType{
		ViewProjectCreator, ViewProjectEditor, ViewTaskCreator, ViewTaskEditor,
		ViewHelp, ViewDateInput, ViewCommandPalette, ViewFilter,
	}
	for _, vt := range untracked {
		if isTrackedView(vt) {
			t.Errorf("expected ViewType %d to NOT be tracked", vt)
		}
	}
}

func TestIsTaskScopedView(t *testing.T) {
	if !isTaskScopedView(ViewTaskComments) {
		t.Error("ViewTaskComments should be task-scoped")
	}
	if !isTaskScopedView(ViewTaskChecklist) {
		t.Error("ViewTaskChecklist should be task-scoped")
	}
	if isTaskScopedView(ViewDashboard) {
		t.Error("ViewDashboard should not be task-scoped")
	}
}
