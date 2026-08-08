package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// openScopedProjectDrilldown drills the global Outliner into a single project
// scope (the consolidated replacement for the retired temporary Outliner) and
// returns the driver plus the scoped project's loaded copy. Refresh regressions
// then assert that mutations reach the crossOutliner's own project pointer.
func openScopedProjectDrilldown(t *testing.T) (*UITestDriver, *models.Project) {
	t.Helper()
	driver, ws, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	// The Outliner is the sole root and already open on launch; ensure its
	// cross-project cache is loaded before drilling into a single project scope.
	driver.dispatch(launchOutlinerMsg{})
	driver.drain()

	cross := driver.App().crossOutliner
	if cross == nil {
		t.Fatal("fixture did not open the cross-project Outliner")
	}

	var targetProject *models.Project
	for _, project := range cross.loadedProjects {
		if project != nil && len(project.Tasks) > 0 {
			targetProject = project
			break
		}
	}
	if targetProject == nil {
		t.Fatal("fixture did not load a scope-drilldown target project")
	}

	// Push the project scope and reveal a task, mirroring the project-header
	// scope entry (Enter on a project header / Ctrl+X scope picker).
	cross.pushProjectScope(targetProject.ID)
	cross.NavigateToTaskInProject(targetProject.Tasks[0], targetProject)
	driver.drain()

	if driver.App().currentView() != ViewCrossProjectOutliner {
		t.Fatalf("current view = %s, want CrossProjectOutliner", driver.App().currentView())
	}
	if !cross.isProjectScoped() || cross.scopedProjectID != targetProject.ID {
		t.Fatalf("outliner scope = %q (scoped=%v), want %q", cross.scopedProjectID, cross.isProjectScoped(), targetProject.ID)
	}
	return driver, targetProject
}

func TestScopedDrilldownEscRestoresCrossProjectParent(t *testing.T) {
	driver, _ := openScopedProjectDrilldown(t)
	cross := driver.App().crossOutliner
	if !cross.isProjectScoped() {
		t.Fatal("setup should leave the outliner scoped to one project")
	}
	if _, err := driver.ApplyAction(UITestAction{Type: UITestActionKey, Key: "esc"}, ObserveOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := driver.App().currentView(); got != ViewCrossProjectOutliner {
		t.Fatalf("current view after Esc = %s, want CrossProjectOutliner", got)
	}
	if cross.isProjectScoped() {
		t.Fatal("Esc should pop the project scope back to all projects")
	}
}

func TestScopedPinnedLeafCompletionKeepsProjectOutlinerVisible(t *testing.T) { //nolint:gocyclo // End-to-end regression keeps the complete UI transition in one scenario.
	driver, project := openScopedProjectDrilldown(t)
	cross := driver.App().crossOutliner

	var target, survivor *models.Task
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		if target == nil && len(task.Children) == 0 && !task.IsTerminalState() {
			target = task
			continue
		}
		if survivor == nil && !task.IsTerminalState() {
			survivor = task
		}
	}
	if target == nil || survivor == nil {
		t.Fatal("fixture needs a non-terminal leaf and another visible task")
	}
	target.ApplyWorkflowState(models.StateDoing)
	target.Pin()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("persist pinned completion fixture: %v", err)
	}
	cross.crossForceRebuild = true
	cross.crossProjectRebuildRows()
	cross.NavigateToTaskInProject(target, project)

	row := cross.rows[cross.cursor]
	if !row.isTaskRow() || row.task == nil || row.task.ID != target.ID || !row.IsPinnedSection {
		t.Fatalf("completion target is not selected in the pinned section: %#v", row)
	}
	driver.SetDrainTimeout(750 * time.Millisecond)

	// Send the two-key direct chord without draining the prefix timeout between
	// strokes, matching interactive keyboard timing.
	if err := driver.SendKey("s"); err != nil {
		t.Fatalf("send state prefix: %v", err)
	}
	if len(driver.App().pendingChord) == 0 {
		t.Fatal("state prefix did not arm the direct-key chord")
	}
	if err := driver.SendKey("d"); err != nil {
		t.Fatalf("send done state: %v", err)
	}
	if driver.pendingCmd == nil {
		t.Fatal("done chord did not dispatch a mutation command")
	}
	driver.drain()

	app := driver.App()
	if app.err != nil {
		t.Fatalf("completion raised an internal error: %v", app.err)
	}
	if got := app.currentView(); got != ViewCrossProjectOutliner {
		t.Fatalf("current view = %s, want CrossProjectOutliner", got)
	}
	if cross.scopedProjectID != project.ID {
		t.Fatalf("project scope = %q, want %q", cross.scopedProjectID, project.ID)
	}
	if cross.crossLoading {
		t.Fatal("completion left the Outliner refresh pending")
	}
	if screen := driver.Render(); strings.Contains(screen, "Unknown view") || strings.Contains(screen, "internal error:") {
		t.Fatalf("completion left an invalid root frame:\n%s", screen)
	}
	if !outlinerHasTask(cross, project.ID, survivor.ID) {
		t.Fatalf("surviving task %q disappeared from the scoped Outliner", survivor.ID)
	}
	if len(cross.rows) == 0 || cross.cursor < 0 || cross.cursor >= len(cross.rows) ||
		!cross.rows[cross.cursor].isTaskRow() || cross.rows[cross.cursor].task == nil {
		t.Fatalf("completion left the cursor off a task row: cursor=%d rows=%d", cross.cursor, len(cross.rows))
	}

	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload completed project: %v", err)
	}
	completed := fresh.GetTaskByID(target.ID)
	if completed == nil || completed.State() != models.StateDone {
		visible := project.GetTaskByID(target.ID)
		visibleState := models.WorkflowState("")
		if visible != nil {
			visibleState = visible.State()
		}
		t.Fatalf("durable task state = %#v, visible state=%v status=%q pending=%d, want done",
			completed, visibleState, app.statusMessage, len(driver.pendingFutures))
	}
}
