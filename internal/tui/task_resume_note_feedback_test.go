package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// TestResumeNoteSentences pins both sentences: a departure says whether the
// task carries a note and names the keyless entry, and a start reads the note
// back whole. Each names the task only when the line it rides does not.
func TestResumeNoteSentences(t *testing.T) {
	spoken := "Change Status: Fix reset link → Paused"
	if got := leftOffNote("Fix reset link", "", spoken, "F10"); got != "No resume note · F10: Note Where I Left Off" {
		t.Fatalf("departure without a note = %q", got)
	}
	if got := leftOffNote("Fix reset link", "expired token", spoken, "Ctrl+M"); got != "Resume note kept · Ctrl+M: Note Where I Left Off" {
		t.Fatalf("departure with a note = %q", got)
	}
	if got := leftOffNote("Fix reset link", "", "Moved 1 task", "F10"); got != "No resume note: Fix reset link · F10: Note Where I Left Off" {
		t.Fatalf("departure on a line naming no task = %q", got)
	}
	if got := resumeStartNote("Fix reset link", "", spoken); got != "" {
		t.Fatalf("a start without a note adds nothing, got %q", got)
	}
	if got := resumeStartNote("Fix reset link", "expired token", spoken); got != "Left off: expired token" {
		t.Fatalf("start = %q", got)
	}
	if got := resumeStartNote("Fix reset link", "expired token", ""); got != "Fix reset link · Left off: expired token" {
		t.Fatalf("start on no line = %q", got)
	}
}

// TestResumeNoteFeedbackSpeaksForOneDepartureAndRemembersIt covers the session
// memory the menu entry writes for: one task leaving Doing is named and
// remembered, a redelivery says nothing, and a bulk departure is silent and
// forgets, since a selection has no one place it stopped.
func TestResumeNoteFeedbackSpeaksForOneDepartureAndRemembersIt(t *testing.T) {
	first, second := newBulkPickerTestTask(t, 1, "first"), newBulkPickerTestTask(t, 2, "second")
	first.ApplyWorkflowState(models.StateDoing)
	second.ApplyWorkflowState(models.StateDoing)
	env := newAppBulkPickerEnv(t, []*models.Task{first, second})
	model := env.appModel()
	registerCheckInTestOperations(t, model)

	msg := commitCheckInState(t, model, bulkTaskActionTestContext(env.projects[0], first), models.StatePaused)
	model.Update(msg)
	if !strings.Contains(model.feedback.message, "No resume note") || !strings.Contains(model.feedback.message, leftOffEntryLabel) {
		t.Fatalf("footer = %q", model.feedback.message)
	}
	if want := (taskref.TaskIdentity{ProjectID: env.projects[0].ID, TaskID: first.ID}); model.resumeNoteFeedback.leftOff != want {
		t.Fatalf("remembered %+v, want %+v", model.resumeNoteFeedback.leftOff, want)
	}
	model.clearStatus()
	model.Update(msg)
	if strings.Contains(model.feedback.message, "resume note") {
		t.Fatalf("a redelivery spoke again: %q", model.feedback.message)
	}

	// The entry writes for the remembered task, not the cursor's.
	model.openLeftOffNote()
	if model.operations.TextPrompt == nil {
		t.Fatalf("the entry opened no prompt: %v", model.feedback.err)
	}
	model.operations.TextPrompt = nil

}

// TestResumeNoteFeedbackBulkDepartureIsSilentAndForgets pins that a selection
// is never spoken for as one task, even when only one of its members was in
// Doing: the gesture was about the selection.
func TestResumeNoteFeedbackBulkDepartureIsSilentAndForgets(t *testing.T) {
	first, second := newBulkPickerTestTask(t, 1, "first"), newBulkPickerTestTask(t, 2, "second")
	first.ApplyWorkflowState(models.StateDoing)
	env := newAppBulkPickerEnv(t, []*models.Task{first, second})
	model := env.appModel()
	registerCheckInTestOperations(t, model)
	model.resumeNoteFeedback.leftOff = taskref.TaskIdentity{ProjectID: env.projects[0].ID, TaskID: first.ID}
	model.Update(commitCheckInState(t, model, bulkTaskActionTestContext(env.projects[0], first, second), models.StatePaused))
	if !strings.Contains(model.feedback.message, "Paused") || strings.Contains(model.feedback.message, "resume note") {
		t.Fatalf("a bulk departure spoke for one task: %q", model.feedback.message)
	}
	if model.resumeNoteFeedback.leftOff.Valid() {
		t.Fatal("a bulk departure kept a single task in memory")
	}
}
