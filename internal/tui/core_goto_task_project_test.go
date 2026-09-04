package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
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

func gotoProjectOutliner(t *testing.T, driver *UITestDriver) *OutlinerModel {
	t.Helper()
	outliner := driver.App().crossOutliner
	if outliner == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	return outliner
}

const gotoProjectTaskTitle = "Beta rollout checklist"

// gotoProjectCursorOnTask parks the cursor on the row holding Client Beta's task.
func gotoProjectCursorOnTask(t *testing.T, outliner *OutlinerModel) {
	t.Helper()
	for index, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && row.task.Title == gotoProjectTaskTitle {
			outliner.Cursor = index
			return
		}
	}
	t.Fatalf("no row holds the task %q", gotoProjectTaskTitle)
}

func gotoProjectCursorTitle(outliner *OutlinerModel) string {
	if outliner.Cursor < 0 || outliner.Cursor >= len(outliner.rows) {
		return ""
	}
	row := outliner.rows[outliner.Cursor]
	if !row.isTaskRow() || row.task == nil {
		return ""
	}
	return row.task.Title
}

const gotoProjectName = "Client Beta"

func gotoProjectID(t *testing.T, app *AppModel) string {
	t.Helper()
	for _, project := range app.projects {
		if project != nil && project.Name == gotoProjectName {
			return project.ID
		}
	}
	t.Fatalf("no project named %q", gotoProjectName)
	return ""
}

func TestGotoTaskProjectScopesToTheCursorTaskAndForcesAll(t *testing.T) {
	driver := gotoProjectDriver(t)
	outliner := gotoProjectOutliner(t, driver)
	beta := gotoProjectID(t, driver.App())
	gotoProjectCursorOnTask(t, outliner)

	sortPressKeys(t, driver, "g", "p")

	if outliner.scope.projectID != beta {
		t.Fatalf("scoped project = %q, want Client Beta %q", outliner.scope.projectID, beta)
	}
	if outliner.primaryMode != chrome.ModeAll {
		t.Fatalf("mode = %q, want %q", outliner.primaryMode, chrome.ModeAll)
	}
	if got := gotoProjectCursorTitle(outliner); got != gotoProjectTaskTitle {
		t.Fatalf("cursor task = %q, want the task the jump was pressed from", got)
	}
	if status := driver.App().statusMessage; !strings.Contains(status, "Project: Client Beta") {
		t.Fatalf("status = %q, want it to name the destination project", status)
	}
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && row.task.Title == "Alpha migration kickoff" {
			t.Fatal("the scoped projection still lists another project's task")
		}
	}
}

// TestGotoTaskProjectEscRestoresTheOriginMode locks the ordering: the scope push
// captures the frame Esc restores, so it must run before the All switch.
func TestGotoTaskProjectEscRestoresTheOriginMode(t *testing.T) {
	driver := gotoProjectDriver(t)
	outliner := gotoProjectOutliner(t, driver)
	sortPressKeys(t, driver, "g", "b")
	if outliner.primaryMode != chrome.ModeBacklog {
		t.Fatalf("origin mode = %q, want %q", outliner.primaryMode, chrome.ModeBacklog)
	}
	beta := gotoProjectID(t, driver.App())
	gotoProjectCursorOnTask(t, outliner)

	sortPressKeys(t, driver, "g", "p")
	if outliner.scope.projectID != beta || outliner.primaryMode != chrome.ModeAll {
		t.Fatalf(
			"landing = scope %q mode %q, want %q and %q",
			outliner.scope.projectID, outliner.primaryMode, beta, chrome.ModeAll,
		)
	}

	sortPressKeys(t, driver, "esc")
	if outliner.scope.projectID != "" {
		t.Fatalf("Esc left the scope at %q, want the base scope", outliner.scope.projectID)
	}
	if outliner.primaryMode != chrome.ModeBacklog {
		t.Fatalf("Esc restored mode %q, want the origin %q", outliner.primaryMode, chrome.ModeBacklog)
	}
}

// TestGotoTaskProjectKeepsTheOriginModeWhenAllOmitsTheTask covers the Done task
// reached from Recent: All would not list it, so the origin mode stays and the
// status says why.
func TestGotoTaskProjectKeepsTheOriginModeWhenAllOmitsTheTask(t *testing.T) {
	driver := gotoProjectDriver(t)
	outliner := gotoProjectOutliner(t, driver)
	gotoProjectCursorOnTask(t, outliner)
	sortPressKeys(t, driver, "$", "d")
	driver.Wait(defaultWaitTimeout)

	sortPressKeys(t, driver, "g", "r")
	beta := gotoProjectID(t, driver.App())
	gotoProjectCursorOnTask(t, outliner)

	sortPressKeys(t, driver, "g", "p")

	if outliner.scope.projectID != beta {
		t.Fatalf("scoped project = %q, want Client Beta %q", outliner.scope.projectID, beta)
	}
	if outliner.primaryMode != chrome.ModeRecent {
		t.Fatalf("mode = %q, want the origin %q kept", outliner.primaryMode, chrome.ModeRecent)
	}
	if got := gotoProjectCursorTitle(outliner); got != gotoProjectTaskTitle {
		t.Fatalf("cursor task = %q, want the task the jump was pressed from", got)
	}
	status := driver.App().statusMessage
	if !strings.Contains(status, "kept Recent") || !strings.Contains(status, "not listed in All") {
		t.Fatalf("status = %q, want it to explain the kept mode", status)
	}
}

func TestGotoTaskProjectRefusesWithoutASingleTaskCursor(t *testing.T) {
	t.Run("project header", func(t *testing.T) {
		driver := gotoProjectDriver(t)
		outliner := gotoProjectOutliner(t, driver)
		outliner.Cursor = headerTargetProjectHeaderRow(t, outliner)

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
		if outliner.scope.projectID != "" {
			t.Fatalf("a refused press scoped to %q", outliner.scope.projectID)
		}
	})

	t.Run("visual selection", func(t *testing.T) {
		driver := gotoProjectDriver(t)
		outliner := gotoProjectOutliner(t, driver)
		gotoProjectCursorOnTask(t, outliner)

		sortPressKeys(t, driver, "v", "g", "p")

		if outliner.scope.projectID != "" {
			t.Fatalf("a visual selection still scoped to %q", outliner.scope.projectID)
		}
	})

}

func TestGotoTaskProjectJumpsFromASearchResult(t *testing.T) {
	driver := gotoProjectDriver(t)
	outliner := gotoProjectOutliner(t, driver)
	outliner.SetSearchQuery("Beta rollout")
	gotoProjectCursorOnTask(t, outliner)
	identity := outliner.CurrentTaskIdentity()
	_, project := outliner.taskAndProjectForIdentity(identity)
	if project == nil {
		t.Fatalf("search result %+v resolved no project", identity)
	}

	app := driver.App()
	ctx := app.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	executeDirectPath(t, app, ctx, []string{"g", "p"})

	if outliner.hasSearchProjection() {
		t.Fatal("the jump kept the temporary search projection")
	}
	if outliner.scope.projectID != project.ID {
		t.Fatalf("scoped to %q, want the result's project %q", outliner.scope.projectID, project.ID)
	}
	if want := "Project: " + project.Name; app.statusMessage != want {
		t.Fatalf("status = %q, want %q", app.statusMessage, want)
	}
}
