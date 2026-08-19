package tui

import (
	"strings"
	"testing"
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
			outliner.cursor = index
			return
		}
	}
	t.Fatalf("no row holds the task %q", gotoProjectTaskTitle)
}

func gotoProjectCursorTitle(outliner *OutlinerModel) string {
	if outliner.cursor < 0 || outliner.cursor >= len(outliner.rows) {
		return ""
	}
	row := outliner.rows[outliner.cursor]
	if !row.isTaskRow() || row.task == nil {
		return ""
	}
	return row.task.Title
}

func gotoProjectIDNamed(t *testing.T, app *AppModel, name string) string {
	t.Helper()
	for _, project := range app.projects {
		if project != nil && project.Name == name {
			return project.ID
		}
	}
	t.Fatalf("no project named %q", name)
	return ""
}

func TestGotoTaskProjectScopesToTheCursorTaskAndForcesAll(t *testing.T) {
	driver := gotoProjectDriver(t)
	outliner := gotoProjectOutliner(t, driver)
	beta := gotoProjectIDNamed(t, driver.App(), "Client Beta")
	gotoProjectCursorOnTask(t, outliner)

	sortPressKeys(t, driver, "ctrl+p")

	if outliner.scopedProjectID != beta {
		t.Fatalf("scoped project = %q, want Client Beta %q", outliner.scopedProjectID, beta)
	}
	if outliner.primaryMode != ModeAll {
		t.Fatalf("mode = %q, want %q", outliner.primaryMode, ModeAll)
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
	if outliner.primaryMode != ModeBacklog {
		t.Fatalf("origin mode = %q, want %q", outliner.primaryMode, ModeBacklog)
	}
	beta := gotoProjectIDNamed(t, driver.App(), "Client Beta")
	gotoProjectCursorOnTask(t, outliner)

	sortPressKeys(t, driver, "ctrl+p")
	if outliner.scopedProjectID != beta || outliner.primaryMode != ModeAll {
		t.Fatalf(
			"landing = scope %q mode %q, want %q and %q",
			outliner.scopedProjectID, outliner.primaryMode, beta, ModeAll,
		)
	}

	sortPressKeys(t, driver, "esc")
	if outliner.scopedProjectID != "" {
		t.Fatalf("Esc left the scope at %q, want the base scope", outliner.scopedProjectID)
	}
	if outliner.primaryMode != ModeBacklog {
		t.Fatalf("Esc restored mode %q, want the origin %q", outliner.primaryMode, ModeBacklog)
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
	beta := gotoProjectIDNamed(t, driver.App(), "Client Beta")
	gotoProjectCursorOnTask(t, outliner)

	sortPressKeys(t, driver, "ctrl+p")

	if outliner.scopedProjectID != beta {
		t.Fatalf("scoped project = %q, want Client Beta %q", outliner.scopedProjectID, beta)
	}
	if outliner.primaryMode != ModeRecent {
		t.Fatalf("mode = %q, want the origin %q kept", outliner.primaryMode, ModeRecent)
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
		outliner.cursor = headerTargetProjectHeaderRow(t, outliner)

		key, err := parseUITestKey("ctrl+p")
		if err != nil {
			t.Fatalf("parse ctrl+p: %v", err)
		}
		handled, cmd := driver.App().handleSimplifiedInputKey(key)
		if !handled || cmd == nil {
			t.Fatalf("header press = handled %v cmd %v, want the shared refusal", handled, cmd)
		}
		failure, ok := cmd().(ErrorMsg)
		if !ok {
			t.Fatalf("header press produced %T, not a refusal", cmd())
		}
		if failure.Err.Error() != operationTargetRejectionMessage(OperationCategoryTask) {
			t.Fatalf("refusal reads %q, not the shared sentence", failure.Err.Error())
		}
		if outliner.scopedProjectID != "" {
			t.Fatalf("a refused press scoped to %q", outliner.scopedProjectID)
		}
	})

	t.Run("visual selection", func(t *testing.T) {
		driver := gotoProjectDriver(t)
		outliner := gotoProjectOutliner(t, driver)
		gotoProjectCursorOnTask(t, outliner)

		sortPressKeys(t, driver, "v", "ctrl+p")

		if outliner.scopedProjectID != "" {
			t.Fatalf("a visual selection still scoped to %q", outliner.scopedProjectID)
		}
	})

}

func TestGotoTaskProjectJumpsFromASearchResult(t *testing.T) {
	driver := gotoProjectDriver(t)
	outliner := gotoProjectOutliner(t, driver)
	outliner.SetSearchQuery("Beta rollout")
	gotoProjectCursorOnTask(t, outliner)
	identity := outliner.currentTaskIdentity()
	_, project := outliner.taskAndProjectForIdentity(identity)
	if project == nil {
		t.Fatalf("search result %+v resolved no project", identity)
	}

	app := driver.App()
	ctx := app.operationCommandContext()
	ctx.View = ViewCrossProjectOutliner
	executeDirectPath(t, app, ctx, []string{"ctrl+p"})

	if outliner.hasSearchProjection() {
		t.Fatal("the jump kept the temporary search projection")
	}
	if outliner.scopedProjectID != project.ID {
		t.Fatalf("scoped to %q, want the result's project %q", outliner.scopedProjectID, project.ID)
	}
	if want := "Project: " + project.Name; app.statusMessage != want {
		t.Fatalf("status = %q, want %q", app.statusMessage, want)
	}
}
