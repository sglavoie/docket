package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestCaptureInboxSnapshotIncludesStatusFields(t *testing.T) {
	completedAt := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	task := &models.Task{
		ID:          "t1",
		Status:      models.StatusDone,
		CompletedAt: &completedAt,
		Active:      false,
		Section:     models.SectionCompleted,
		Priority:    2,
		Tags:        []string{"backend"},
		DueDate:     &dueDate,
		HasTime:     true,
		DependsOn:   []string{"dep-1"},
	}

	snap := captureInboxSnapshot(task)

	if snap.status != models.StatusDone {
		t.Errorf("status: got %q, want %q", snap.status, models.StatusDone)
	}
	if snap.completedAt == nil {
		t.Fatal("completedAt should not be nil")
	}
	if !snap.completedAt.Equal(completedAt) {
		t.Errorf("completedAt: got %v, want %v", *snap.completedAt, completedAt)
	}
	if snap.active != false {
		t.Error("active: got true, want false")
	}
	if snap.section != models.SectionCompleted {
		t.Errorf("section: got %q, want %q", snap.section, models.SectionCompleted)
	}
}

func TestCaptureInboxSnapshotDeepCopiesTimePointers(t *testing.T) {
	completedAt := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	dueDate := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	task := &models.Task{
		ID:          "t1",
		Status:      models.StatusDone,
		CompletedAt: &completedAt,
		DueDate:     &dueDate,
		Tags:        []string{},
		DependsOn:   []string{},
	}

	snap := captureInboxSnapshot(task)

	// Mutating original should not affect snapshot
	newTime := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	task.CompletedAt = &newTime
	task.DueDate = &newTime

	if snap.completedAt.Equal(newTime) {
		t.Error("completedAt should be a deep copy, not a shared pointer")
	}
	if snap.dueDate.Equal(newTime) {
		t.Error("dueDate should be a deep copy, not a shared pointer")
	}
}

func TestRestoreInboxSnapshotRestoresStatusFields(t *testing.T) {
	// Start with a task in "todo" state
	task := &models.Task{
		ID:        "t1",
		Status:    models.StatusTodo,
		Active:    false,
		Section:   models.SectionInbox,
		Priority:  3,
		Tags:      []string{"frontend"},
		DependsOn: []string{},
	}

	// Capture snapshot in todo state
	snap := captureInboxSnapshot(task)

	// Simulate completing the task (as changeTaskStatus does)
	task.ApplyTransition(models.StatusDone)

	// Verify task is now completed
	if task.Status != models.StatusDone {
		t.Fatalf("precondition: task should be done, got %q", task.Status)
	}
	if task.CompletedAt == nil {
		t.Fatal("precondition: CompletedAt should be set after completion")
	}
	if task.Section != models.SectionCompleted {
		t.Fatalf("precondition: section should be Completed, got %q", task.Section)
	}

	// Undo by restoring snapshot
	restoreInboxSnapshot(task, snap)

	// Status should be restored to todo
	if task.Status != models.StatusTodo {
		t.Errorf("status after restore: got %q, want %q", task.Status, models.StatusTodo)
	}
	if task.CompletedAt != nil {
		t.Error("completedAt should be nil after restoring todo snapshot")
	}
	if task.Active != false {
		t.Error("active should be false after restoring todo snapshot")
	}
	if task.Section != models.SectionInbox {
		t.Errorf("section after restore: got %q, want %q", task.Section, models.SectionInbox)
	}
	if task.Priority != 3 {
		t.Errorf("priority after restore: got %d, want 3", task.Priority)
	}
}

func TestRestoreInboxSnapshotRestoresCompletedAt(t *testing.T) {
	// Edge case: restoring a snapshot that HAD a completedAt
	// (e.g., undoing a reopen)
	completedAt := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:          "t1",
		Status:      models.StatusDone,
		CompletedAt: &completedAt,
		Active:      false,
		Section:     models.SectionCompleted,
		Tags:        []string{},
		DependsOn:   []string{},
	}

	snap := captureInboxSnapshot(task)

	// Simulate reopening
	task.Reopen()

	if task.Status != models.StatusTodo {
		t.Fatalf("precondition: reopened task should be todo, got %q", task.Status)
	}

	// Undo the reopen
	restoreInboxSnapshot(task, snap)

	if task.Status != models.StatusDone {
		t.Errorf("status after restore: got %q, want %q", task.Status, models.StatusDone)
	}
	if task.CompletedAt == nil {
		t.Fatal("completedAt should be restored")
	}
	if !task.CompletedAt.Equal(completedAt) {
		t.Errorf("completedAt: got %v, want %v", *task.CompletedAt, completedAt)
	}
}

func TestRestoreInboxSnapshotRestoresSnoozedUntil(t *testing.T) {
	snoozedUntil := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	task := &models.Task{
		ID:           "t1",
		Status:       models.StatusTodo,
		Section:      models.SectionInbox,
		SnoozedUntil: &snoozedUntil,
		Tags:         []string{},
		DependsOn:    []string{},
	}

	snap := captureInboxSnapshot(task)

	// Verify snapshot captured the snooze date
	if snap.snoozedUntil == nil {
		t.Fatal("snapshot should capture snoozedUntil")
	}
	if !snap.snoozedUntil.Equal(snoozedUntil) {
		t.Errorf("snapshot snoozedUntil: got %v, want %v", *snap.snoozedUntil, snoozedUntil)
	}

	// Simulate a status change that clears SnoozedUntil
	task.ApplyTransition(models.StatusInProgress)
	if task.SnoozedUntil != nil {
		t.Fatal("precondition: SnoozedUntil should be cleared by Start()")
	}

	// Undo
	restoreInboxSnapshot(task, snap)

	if task.SnoozedUntil == nil {
		t.Fatal("SnoozedUntil should be restored after undo")
	}
	if !task.SnoozedUntil.Equal(snoozedUntil) {
		t.Errorf("SnoozedUntil after restore: got %v, want %v", *task.SnoozedUntil, snoozedUntil)
	}

	// Verify deep copy: mutating original should not affect restored value
	newTime := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	*snap.snoozedUntil = newTime
	if task.SnoozedUntil.Equal(newTime) {
		t.Error("SnoozedUntil should be a deep copy, not a shared pointer")
	}
}

func TestCaptureInboxSnapshotNilCompletedAt(t *testing.T) {
	task := &models.Task{
		ID:        "t1",
		Status:    models.StatusTodo,
		Tags:      []string{},
		DependsOn: []string{},
	}

	snap := captureInboxSnapshot(task)

	if snap.completedAt != nil {
		t.Error("completedAt should be nil for a todo task")
	}
	if snap.status != models.StatusTodo {
		t.Errorf("status: got %q, want %q", snap.status, models.StatusTodo)
	}
}

func TestInboxSnapshotCapturesAndRestoresStage(t *testing.T) {
	task := &models.Task{
		ID:        "t1",
		Status:    models.StatusTodo,
		Section:   models.SectionInbox,
		Priority:  3,
		Stage:     "discovery",
		Tags:      []string{},
		DependsOn: []string{},
	}

	snap := captureInboxSnapshot(task)
	if snap.stage != "discovery" {
		t.Fatalf("snapshot stage: got %q, want %q", snap.stage, "discovery")
	}

	// Simulate setStage mutation.
	task.Stage = "validation"

	restoreInboxSnapshot(task, snap)
	if task.Stage != "discovery" {
		t.Errorf("stage after restore: got %q, want %q", task.Stage, "discovery")
	}

	// Restoring an empty-stage snapshot must also clear a later-set stage.
	emptyTask := &models.Task{
		ID:        "t2",
		Status:    models.StatusTodo,
		Tags:      []string{},
		DependsOn: []string{},
	}
	emptySnap := captureInboxSnapshot(emptyTask)
	emptyTask.Stage = "set-after-snapshot"
	restoreInboxSnapshot(emptyTask, emptySnap)
	if emptyTask.Stage != "" {
		t.Errorf("expected stage to be cleared on restore, got %q", emptyTask.Stage)
	}
}

func TestRenderInboxCurrentSettingsIgnoresNonGTDSection(t *testing.T) {
	task := &models.Task{
		ID:        "t1",
		Priority:  3,
		Tags:      []string{},
		DependsOn: []string{},
	}
	task.SetSection("Sprint-1")

	rendered := renderInboxCurrentSettings(task, &config.Config{SectionColors: map[string]string{}})

	if strings.Contains(rendered, "Sprint-1") {
		t.Fatalf("did not expect non-GTD section name in current settings, got %q", rendered)
	}
	if !strings.Contains(rendered, "(no section)") {
		t.Fatalf("expected no section placeholder, got %q", rendered)
	}
}

func TestRenderInboxOperationsPanelShowsSnoozeZPrefix(t *testing.T) {
	rendered := ansi.Strip(renderInboxOperationsPanel())

	if !strings.Contains(rendered, "z*: Snooze") {
		t.Fatalf("expected z* snooze hint in operations panel, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "s*: Snooze") {
		t.Fatalf("expected stale s* snooze hint to be absent, got:\n%s", rendered)
	}
}

// TestRenderInboxOperationsPanelFramesRoutingQuestion verifies that the panel
// reframes the inbox loop around "Where should this task go next?", chunks the
// vocabulary into the four labeled groups (Route/Schedule/Resolve/Refine), and
// surfaces the w: Today action in the Schedule row so power users can plan
// today's work without leaving the triage loop.
func TestRenderInboxOperationsPanelFramesRoutingQuestion(t *testing.T) {
	rendered := ansi.Strip(renderInboxOperationsPanel())

	if !strings.Contains(rendered, "Where should this task go next?") {
		t.Fatalf("expected routing question header, got:\n%s", rendered)
	}

	wantPrefixHints := []string{
		"m*: Route",
		"w: Today",
		"d*: Schedule",
		"z*: Snooze",
		"D: Done",
		",*: Status",
		"!/1-5: Priority",
		"e: Edit",
		"T: Tags",
		"c*: Linked",
	}
	for _, hint := range wantPrefixHints {
		if !strings.Contains(rendered, hint) {
			t.Errorf("expected %q in operations panel, got:\n%s", hint, rendered)
		}
	}

	wantGroupLabels := []string{"Route", "Schedule", "Resolve", "Refine"}
	for _, label := range wantGroupLabels {
		if !strings.Contains(rendered, label) {
			t.Errorf("expected group label %q in operations panel, got:\n%s", label, rendered)
		}
	}

	if !strings.Contains(rendered, "t* task ops") {
		t.Errorf("expected t* task ops pointer in More hint, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "? help") {
		t.Errorf("expected ? help pointer in More hint, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, ": palette") {
		t.Errorf("expected palette pointer in More hint, got:\n%s", rendered)
	}

	staleHints := []string{
		"Operations",
		"ctrl+s: Section",
		"ctrl+w: Waiting",
		"m*: Move",
	}
	for _, hint := range staleHints {
		if strings.Contains(rendered, hint) {
			t.Errorf("expected stale hint %q to be absent, got:\n%s", hint, rendered)
		}
	}
}

// TestRenderInboxFooterShowsPrimaryTriageVocabulary asserts that the footer
// promotes only the six primary triage actions shared by both processors plus
// Esc — secondary keys must live in help / palette.
func TestRenderInboxFooterShowsPrimaryTriageVocabulary(t *testing.T) {
	rendered := ansi.Strip(renderInboxFooter(120))

	wantPrimary := []string{
		"n: Skip",
		"e: Edit",
		"D: Done",
		"T: Tags",
		"u: Undo",
		"?: Help",
		"esc: Back",
	}
	for _, hint := range wantPrimary {
		if !strings.Contains(rendered, hint) {
			t.Errorf("expected %q in inbox footer, got:\n%s", hint, rendered)
		}
	}

	demotedHints := []string{
		"j/k: Navigate",
		"gg/G: First/last",
		",s/d/b/x/o",
		"ctrl+s",
		"ctrl+w",
	}
	for _, hint := range demotedHints {
		if strings.Contains(rendered, hint) {
			t.Errorf("expected demoted hint %q to be absent from footer, got:\n%s", hint, rendered)
		}
	}
}
