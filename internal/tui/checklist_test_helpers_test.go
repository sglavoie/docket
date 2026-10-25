package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/outliner"
)

// checklistFixtureApp launches the checklist fixture and returns the driver, the
// workspace behind it, and the Outliner the assertions read. The checklist
// fixture is the one that carries every owner shape `c` has to answer from: a
// task with a populated checklist, a task that also has a child, that child, and
// a task with no checklist at all.
func checklistFixtureApp(t *testing.T) (*UITestDriver, *uitestWorkspace, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureChecklist, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatal("fixture app has no Outliner")
	}
	return driver, workspace, view
}
