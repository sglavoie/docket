package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

func gotoProjectDriver(t *testing.T) *UITestDriver {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("fixture %s: %v", FixtureMultiProject, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	return driver
}

func gotoProjectOutliner(t *testing.T, driver *UITestDriver) *outliner.OutlinerModel {
	t.Helper()
	outlinerView := driver.App().crossOutliner
	if outlinerView == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	return outlinerView
}

const gotoProjectTaskTitle = "Beta rollout checklist"

// gotoProjectCursorOnTask parks the cursor on the row holding Client Beta's task.
func gotoProjectCursorOnTask(t *testing.T, outlinerView *outliner.OutlinerModel) {
	t.Helper()
	for index, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.Title == gotoProjectTaskTitle {
			outlinerView.Cursor = index
			return
		}
	}
	t.Fatalf("no row holds the task %q", gotoProjectTaskTitle)
}

func gotoProjectCursorTitle(outlinerView *outliner.OutlinerModel) string {
	if outlinerView.Cursor < 0 || outlinerView.Cursor >= len(outlinerView.Rows) {
		return ""
	}
	row := outlinerView.Rows[outlinerView.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return ""
	}
	return row.Task.Title
}

const gotoProjectName = "Client Beta"

func gotoProjectID(t *testing.T, app *AppModel) string {
	t.Helper()
	for _, project := range app.domain.projects {
		if project != nil && project.Name == gotoProjectName {
			return project.ID
		}
	}
	t.Fatalf("no project named %q", gotoProjectName)
	return ""
}

func TestGotoTaskProjectScopesToTheCursorTaskAndForcesAll(t *testing.T) {
	driver := gotoProjectDriver(t)
	outlinerView := gotoProjectOutliner(t, driver)
	beta := gotoProjectID(t, driver.App())
	gotoProjectCursorOnTask(t, outlinerView)

	sortPressKeys(t, driver, "g", "p")

	if outlinerView.Scope.ProjectID != beta {
		t.Fatalf("scoped project = %q, want Client Beta %q", outlinerView.Scope.ProjectID, beta)
	}
	if outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("mode = %q, want %q", outlinerView.PrimaryMode, chrome.ModeAll)
	}
	if got := gotoProjectCursorTitle(outlinerView); got != gotoProjectTaskTitle {
		t.Fatalf("cursor task = %q, want the task the jump was pressed from", got)
	}
	if status := driver.App().feedback.message; !strings.Contains(status, "Project: Client Beta") {
		t.Fatalf("status = %q, want it to name the destination project", status)
	}
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.Title == "Alpha migration kickoff" {
			t.Fatal("the scoped projection still lists another project's task")
		}
	}
}

// TestGotoTaskProjectEscRestoresTheOriginMode locks the ordering: the scope push
// captures the frame Esc restores, so it must run before the All switch.
func TestGotoTaskProjectEscRestoresTheOriginMode(t *testing.T) {
	driver := gotoProjectDriver(t)
	outlinerView := gotoProjectOutliner(t, driver)
	sortPressKeys(t, driver, "g", "b")
	if outlinerView.PrimaryMode != chrome.ModeBacklog {
		t.Fatalf("origin mode = %q, want %q", outlinerView.PrimaryMode, chrome.ModeBacklog)
	}
	beta := gotoProjectID(t, driver.App())
	gotoProjectCursorOnTask(t, outlinerView)

	sortPressKeys(t, driver, "g", "p")
	if outlinerView.Scope.ProjectID != beta || outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf(
			"landing = scope %q mode %q, want %q and %q",
			outlinerView.Scope.ProjectID, outlinerView.PrimaryMode, beta, chrome.ModeAll,
		)
	}

	sortPressKeys(t, driver, "esc")
	if outlinerView.Scope.ProjectID != "" {
		t.Fatalf("Esc left the scope at %q, want the base scope", outlinerView.Scope.ProjectID)
	}
	if outlinerView.PrimaryMode != chrome.ModeBacklog {
		t.Fatalf("Esc restored mode %q, want the origin %q", outlinerView.PrimaryMode, chrome.ModeBacklog)
	}
}

// TestGotoTaskProjectKeepsTheOriginModeWhenAllOmitsTheTask covers the Done task
// reached from Recent: All would not list it, so the origin mode stays and the
// status says why.
func TestGotoTaskProjectKeepsTheOriginModeWhenAllOmitsTheTask(t *testing.T) {
	driver := gotoProjectDriver(t)
	outlinerView := gotoProjectOutliner(t, driver)
	gotoProjectCursorOnTask(t, outlinerView)
	sortPressKeys(t, driver, "$", "d")
	driver.Wait(defaultWaitTimeout)

	sortPressKeys(t, driver, "g", "r")
	beta := gotoProjectID(t, driver.App())
	gotoProjectCursorOnTask(t, outlinerView)

	sortPressKeys(t, driver, "g", "p")

	if outlinerView.Scope.ProjectID != beta {
		t.Fatalf("scoped project = %q, want Client Beta %q", outlinerView.Scope.ProjectID, beta)
	}
	if outlinerView.PrimaryMode != chrome.ModeRecent {
		t.Fatalf("mode = %q, want the origin %q kept", outlinerView.PrimaryMode, chrome.ModeRecent)
	}
	if got := gotoProjectCursorTitle(outlinerView); got != gotoProjectTaskTitle {
		t.Fatalf("cursor task = %q, want the task the jump was pressed from", got)
	}
	status := driver.App().feedback.message
	if !strings.Contains(status, "kept Recent") || !strings.Contains(status, "not listed in All") {
		t.Fatalf("status = %q, want it to explain the kept mode", status)
	}
}

func TestGotoTaskProjectRefusesWithoutASingleTaskCursor(t *testing.T) {
	t.Run("project header", func(t *testing.T) {
		driver := gotoProjectDriver(t)
		outlinerView := gotoProjectOutliner(t, driver)
		outlinerView.Cursor = headerTargetProjectHeaderRow(t, outlinerView)

		prefix, err := parseUITestKey("g")
		if err != nil {
			t.Fatalf("parse g: %v", err)
		}
		handled, cmd := driver.App().handleSimplifiedInputKey(prefix)
		if !handled || cmd != nil {
			t.Fatalf("g prefix = handled %v cmd %v, want a pending chord", handled, cmd)
		}
		key, err := parseUITestKey("p")
		if err != nil {
			t.Fatalf("parse p: %v", err)
		}
		handled, cmd = driver.App().handleSimplifiedInputKey(key)
		if !handled || cmd == nil {
			t.Fatalf("header press = handled %v cmd %v, want the shared refusal", handled, cmd)
		}
		failure, ok := cmd().(status.ErrorMsg)
		if !ok {
			t.Fatalf("header press produced %T, not a refusal", cmd())
		}
		if failure.Err.Error() != operation.OperationTargetRejectionMessage(operation.OperationCategoryTask) {
			t.Fatalf("refusal reads %q, not the shared sentence", failure.Err.Error())
		}
		if outlinerView.Scope.ProjectID != "" {
			t.Fatalf("a refused press scoped to %q", outlinerView.Scope.ProjectID)
		}
	})

	t.Run("visual selection", func(t *testing.T) {
		driver := gotoProjectDriver(t)
		outlinerView := gotoProjectOutliner(t, driver)
		gotoProjectCursorOnTask(t, outlinerView)

		sortPressKeys(t, driver, "v", "g", "p")

		if outlinerView.Scope.ProjectID != "" {
			t.Fatalf("a visual selection still scoped to %q", outlinerView.Scope.ProjectID)
		}
	})

}

func TestGotoTaskProjectJumpsFromASearchResult(t *testing.T) {
	driver := gotoProjectDriver(t)
	outlinerView := gotoProjectOutliner(t, driver)
	outlinerView.SetSearchQuery("Beta rollout")
	gotoProjectCursorOnTask(t, outlinerView)
	identity := outlinerView.CurrentTaskIdentity()
	_, project := outlinerView.TaskAndProjectForIdentity(identity)
	if project == nil {
		t.Fatalf("search result %+v resolved no project", identity)
	}

	app := driver.App()
	ctx := app.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	executeDirectPath(t, app, ctx, []string{"g", "p"})

	if outlinerView.HasSearchProjection() {
		t.Fatal("the jump kept the temporary search projection")
	}
	if outlinerView.Scope.ProjectID != project.ID {
		t.Fatalf("scoped to %q, want the result's project %q", outlinerView.Scope.ProjectID, project.ID)
	}
	if want := "Project: " + project.Name; app.feedback.message != want {
		t.Fatalf("status = %q, want %q", app.feedback.message, want)
	}
}
