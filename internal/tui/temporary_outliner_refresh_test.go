package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
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
	driver.dispatch(ShowGlobalOutlinerMsg{})
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
