package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/app"
)

// This file exercises the primary-view feature coordinators end to end through
// the real transactional storage stack: each scenario runs a coordinator
// mutation, reloads the affected project from disk, renders it through the
// query surface, and undoes the mutation through the workspace ledger — the
// mutation→reload→render→undo cycle every migrated primary workflow must honor.

// TestCaptureScenario covers the Quick Capture / task-form primary workflow:
// capture commits a new task, the reloaded project shows it, and undo removes it.
func TestCaptureScenario(t *testing.T) {
	env := newScenarioEnv(t, "capture")

	res := runResult(t, env.features.Capture.Capture(app.CaptureRequest{
		ProjectID: env.project.ID,
		Title:     "Write the migration notes",
	}))
	if res.Title != "Write the migration notes" {
		t.Fatalf("captured title = %q", res.Title)
	}
	if res.Revision == "" {
		t.Fatal("capture result carried no committed revision")
	}

	// reload: the committed task is on disk.
	if got := env.reload(t); got.GetTaskByID(res.TaskID) == nil || len(got.Tasks) != 1 {
		t.Fatal("captured task absent from reloaded project")
	}

	// undo: the capture is reversed on disk.
	env.undo(t)
	if got := env.reload(t); got.GetTaskByID(res.TaskID) != nil {
		t.Fatal("captured task survived undo")
	}
}
