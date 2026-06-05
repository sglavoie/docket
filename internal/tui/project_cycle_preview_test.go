package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func newProjectCyclePreviewTestApp(t *testing.T, width, height, taskCount int) (*AppModel, []string) {
	t.Helper()

	app := newTestAppModel()
	app.width = width
	app.height = height

	tasks := make([]*models.Task, 0, taskCount)
	titles := make([]string, 0, taskCount)
	for i := 1; i <= taskCount; i++ {
		title := fmt.Sprintf("Preview task %02d", i)
		task := newTestProjectTreeTask(t, title)
		tasks = append(tasks, task)
		titles = append(titles, title)
	}

	project := newTestProjectTreeProject(t, "Preview Project", tasks)
	project.BuildTaskTree()
	app.config.Projects = []*models.Project{project}
	app.config.CurrentProject = project.ID
	app.showProjectCycle = true
	app.projectCycleAllItems = []ProjectInfo{{Project: project, TaskCount: len(tasks)}}
	app.projectCycleItems = app.projectCycleAllItems

	return app, titles
}

// TestProjectCyclePreviewBackground_OutOfBoundsCursor verifies the preview does
// not panic when the cursor index is stale relative to projectCycleItems. The
// caller normally clamps the cursor, but a race between the picker rebuilding
// items and the render can leave the cursor briefly out of range.
func TestProjectCyclePreviewBackground_OutOfBoundsCursor(t *testing.T) {
	app := newTestAppModel()
	app.showProjectCycle = true
	app.projectCycleItems = []ProjectInfo{
		{Project: &models.Project{ID: "p1", Name: "P1"}},
	}

	app.projectCycleCursor = 5 // out of range
	got := app.renderCyclePreviewBackground(10)
	if got != "" {
		t.Errorf("expected empty render with out-of-range cursor, got %q", got)
	}

	app.projectCycleCursor = -1 // also out of range
	got = app.renderCyclePreviewBackground(10)
	if got != "" {
		t.Errorf("expected empty render with negative cursor, got %q", got)
	}
}

// TestProjectCyclePreviewBackground_NilProject verifies the preview does not
// panic when an item carries a nil Project (defensive against future bugs in
// project list assembly).
func TestProjectCyclePreviewBackground_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.showProjectCycle = true
	app.projectCycleItems = []ProjectInfo{{Project: nil}}
	app.projectCycleCursor = 0

	got := app.renderCyclePreviewBackground(10)
	if got != "" {
		t.Errorf("expected empty render with nil project, got %q", got)
	}
}

func TestProjectCyclePreviewBackgroundClampsTinyVisibleRows(t *testing.T) {
	for _, visibleRows := range []int{-3, 0} {
		t.Run(fmt.Sprintf("visibleRows=%d", visibleRows), func(t *testing.T) {
			app, _ := newProjectCyclePreviewTestApp(t, 40, 1, 3)

			got := renderCyclePreviewBackgroundNoPanic(t, app, visibleRows)
			if got == "" {
				t.Fatal("expected preview render for tiny visible-row budget")
			}
			if app.projectCyclePreview == nil {
				t.Fatal("expected preview tree to be cached")
			}
			if got := app.projectCyclePreview.Height(); got != 1 {
				t.Fatalf("preview height = %d, want 1", got)
			}
		})
	}
}

func renderCyclePreviewBackgroundNoPanic(t *testing.T, app *AppModel, visibleRows int) (rendered string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("renderCyclePreviewBackground(%d) panicked: %v", visibleRows, r)
		}
	}()
	return app.renderCyclePreviewBackground(visibleRows)
}
