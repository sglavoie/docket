package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestRenderCyclePreviewBackground_OutOfBoundsCursor verifies the preview does
// not panic when the cursor index is stale relative to projectCycleItems. The
// caller normally clamps the cursor, but a race between the picker rebuilding
// items and the render can leave the cursor briefly out of range.
func TestRenderCyclePreviewBackground_OutOfBoundsCursor(t *testing.T) {
	app := newTestAppModel()
	app.showProjectCycle = true
	app.projectCycleItems = []ProjectInfo{
		{Project: &models.Project{ID: "p1", Name: "P1"}},
	}

	app.projectCycleCursor = 5 // out of range
	got := app.renderCyclePreviewBackground()
	if got != "" {
		t.Errorf("expected empty render with out-of-range cursor, got %q", got)
	}

	app.projectCycleCursor = -1 // also out of range
	got = app.renderCyclePreviewBackground()
	if got != "" {
		t.Errorf("expected empty render with negative cursor, got %q", got)
	}
}

// TestRenderCyclePreviewBackground_NilProject verifies the preview does not
// panic when an item carries a nil Project (defensive against future bugs in
// project list assembly).
func TestRenderCyclePreviewBackground_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.showProjectCycle = true
	app.projectCycleItems = []ProjectInfo{{Project: nil}}
	app.projectCycleCursor = 0

	got := app.renderCyclePreviewBackground()
	if got != "" {
		t.Errorf("expected empty render with nil project, got %q", got)
	}
}
