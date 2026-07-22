package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// Session 8.1 repair (canonical-standard-delete): every standard uppercase-X
// task action must emit the shell-owned TaskDeleteRequestedMsg (which prompts
// before any mutation) instead of committing a delete directly, and the
// selection-aware surfaces must target the full active selection rather than
// only the cursor task. These tests drive the built keymap's "X" action and
// assert the emitted canonical request. deleteRequestFromCmd / requestTaskIDs
// live in session3_uppercase_delete_test.go.

// --- FR-005: standard surfaces emit the canonical request (no bypass) --------

// TestRecentActivityUppercaseXDeleteEmitsCanonicalRequest proves the Recent
// Activity surface — which previously inherited a direct coordinator delete
// fallback and deleted without a prompt — now emits the canonical request for
// the cursor task and performs no mutation before shell confirmation.
func TestRecentActivityUppercaseXDeleteEmitsCanonicalRequest(t *testing.T) {
	cfg := newTestConfig()
	project := testutil.NewTestProject("recent")
	task := mustNewTestTask(project, "activity task")
	task.ApplyWorkflowState(models.StateReady)
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	m := NewProjectRecentActivityModel(testRuntimeAt(cfg.DataDir(), cfg), nil, project)
	// Anchor the cursor on a known filtered row without relying on the tab
	// filter plumbing: getTaskRef reads m.filtered directly.
	m.filtered = []CrossProjectTask{{Task: task, Project: project}}
	m.cursor = 0

	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); !slices.Equal(got, []string{task.ID}) {
		t.Fatalf("recent activity delete targets = %v, want [%s]", got, task.ID)
	}
	if msg.Refs[0].Project.ID != project.ID {
		t.Errorf("ref project = %q, want %q", msg.Refs[0].Project.ID, project.ID)
	}
	if msg.Origin != ViewProjectRecentActivity {
		t.Errorf("origin = %v, want ViewProjectRecentActivity", msg.Origin)
	}
}

// --- FR-006: remaining surfaces honor their advertised X target --------------

// TestProjectFocusUppercaseXDeleteEmitsCanonicalRequest proves Project Focus,
// which previously overrode X with a no-op, now emits the canonical request for
// the cursor task.
func TestProjectFocusUppercaseXDeleteEmitsCanonicalRequest(t *testing.T) {
	m, project := newTestProjectFocusView(t)
	if len(m.tasks) == 0 {
		t.Fatal("project focus fixture has no queued task")
	}
	m.cursor = 0
	cursor := m.getTaskAtCursor()
	if cursor == nil {
		t.Fatal("no task under cursor")
	}

	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); !slices.Equal(got, []string{cursor.ID}) {
		t.Fatalf("project focus delete targets = %v, want [%s]", got, cursor.ID)
	}
	if msg.Refs[0].Project.ID != project.ID {
		t.Errorf("ref project = %q, want %q", msg.Refs[0].Project.ID, project.ID)
	}
	if msg.Origin != ViewProjectFocusView {
		t.Errorf("origin = %v, want ViewProjectFocusView", msg.Origin)
	}
}

// TestWeeklyReviewUppercaseXDeleteTargetsSelection proves Weekly Review targets
// the full active multi-selection (not just the cursor) through the canonical
// request.
func TestWeeklyReviewUppercaseXDeleteTargetsSelection(t *testing.T) {
	project, tasks := newSharedBulkProject("Weekly", "one", "two", "three")
	m := &WeeklyReviewModel{
		MultiSelectState: MultiSelectState{
			multiSelectMode: true,
			selectedTasks:   map[string]bool{tasks[1].ID: true, tasks[2].ID: true},
		},
		staleTasks: []WeeklyReviewTask{
			{Task: tasks[0], Project: project},
			{Task: tasks[1], Project: project},
			{Task: tasks[2], Project: project},
		},
	}

	wantIDs := []string{tasks[1].ID, tasks[2].ID}
	slices.Sort(wantIDs)
	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(m), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); !slices.Equal(got, wantIDs) {
		t.Fatalf("weekly review delete targets = %v, want the selection %v", got, wantIDs)
	}
	if msg.Origin != ViewWeeklyReview {
		t.Errorf("origin = %v, want ViewWeeklyReview", msg.Origin)
	}
}

// TestDailyReviewUppercaseXDeleteTargetsSelection proves Daily Review targets the
// full active multi-selection through the canonical request.
func TestDailyReviewUppercaseXDeleteTargetsSelection(t *testing.T) {
	project, tasks := newSharedBulkProject("Daily", "one", "two", "three")
	m := &DailyReviewModel{
		MultiSelectState: MultiSelectState{
			multiSelectMode: true,
			selectedTasks:   map[string]bool{tasks[0].ID: true, tasks[2].ID: true},
		},
		overdue: []CrossProjectTask{
			{Task: tasks[0], Project: project},
			{Task: tasks[1], Project: project},
			{Task: tasks[2], Project: project},
		},
	}

	wantIDs := []string{tasks[0].ID, tasks[2].ID}
	slices.Sort(wantIDs)
	action := findSharedTaskKeymapAction(t, m.buildKeyMap(), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); !slices.Equal(got, wantIDs) {
		t.Fatalf("daily review delete targets = %v, want the selection %v", got, wantIDs)
	}
	if msg.Origin != ViewDailyReview {
		t.Errorf("origin = %v, want ViewDailyReview", msg.Origin)
	}
}

// TestPriorityUppercaseXDeleteTargetsSelection proves Priority targets the full
// active multi-selection through the canonical request.
func TestPriorityUppercaseXDeleteTargetsSelection(t *testing.T) {
	km, want := setupPrioritySharedBulkKeymap(t)

	action := findSharedTaskKeymapAction(t, km, "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)

	wantIDs := make([]string, 0, len(want))
	for _, ref := range want {
		wantIDs = append(wantIDs, ref.Task.ID)
	}
	slices.Sort(wantIDs)
	if got := requestTaskIDs(msg); !slices.Equal(got, wantIDs) {
		t.Fatalf("priority delete targets = %v, want the selection %v", got, wantIDs)
	}
	if msg.Origin != ViewPriorityView {
		t.Errorf("origin = %v, want ViewPriorityView", msg.Origin)
	}
}
