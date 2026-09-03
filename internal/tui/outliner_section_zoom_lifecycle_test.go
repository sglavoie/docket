package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func projectHeaderIndex(t *testing.T, outliner *OutlinerModel, projectID string) int {
	t.Helper()
	for index, row := range outliner.rows {
		if row.IsProjectHeader && row.ProjectID == projectID {
			return index
		}
	}
	t.Fatalf("projection has no project sub-header for %q", projectID)
	return -1
}

// sectionHeaderAboveCursor names the section the cursor row belongs to, so a
// fixture test can zoom into it without knowing the fixture's section labels.
func sectionHeaderAboveCursor(t *testing.T, outliner *OutlinerModel) int {
	t.Helper()
	for index := outliner.cursor; index >= 0; index-- {
		row := outliner.rows[index]
		if row.IsSectionHeader && !row.IsProjectHeader {
			return index
		}
	}
	t.Fatal("no section header stands above the cursor")
	return -1
}

// TestSectionZoomScopeRoundTripDropsAndRestoresTheZoom locks the decided
// behavior: zoom and project scope do not compose, so Enter on a project
// sub-header drops the zoom and the scope frame owes it back on the way out.
func TestSectionZoomScopeRoundTripDropsAndRestoresTheZoom(t *testing.T) {
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-doing", models.StateDoing))
	beta := primaryRowsProject("beta", primaryRowsTask("beta-doing", models.StateDoing))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(ModeAll)

	doingLabel := models.StateGroupDoing.Label()
	zoomIntoSection(t, outliner, doingLabel)
	zoomed := outliner.zoom

	headerIndex := projectHeaderIndex(t, outliner, beta.ID)
	outliner.cursor = headerIndex
	if !outliner.activateGroupAtCursor() {
		t.Fatal("Enter on the project sub-header was refused")
	}
	if outliner.scopedProjectID != beta.ID {
		t.Fatalf("scoped project = %q, want %q", outliner.scopedProjectID, beta.ID)
	}
	if outliner.zoom.active() {
		t.Fatalf("the scope push kept the zoom %+v", outliner.zoom)
	}
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.ProjectID == alpha.ID {
			t.Fatal("the scoped projection still lists the other project's tasks")
		}
	}

	if !outliner.popProjectScope() {
		t.Fatal("Esc did not pop the project scope")
	}
	if outliner.zoom != zoomed {
		t.Fatalf("the popped scope restored zoom %+v, want %+v", outliner.zoom, zoomed)
	}
	if outliner.cursor != headerIndex {
		t.Fatalf("restored cursor = %d, want the pre-push %d", outliner.cursor, headerIndex)
	}
	if names := sectionHeaderNames(outliner); len(names) != 1 || names[0] != doingLabel {
		t.Fatalf("restored sections = %v, want the zoomed %q alone", names, doingLabel)
	}

	if !outliner.exitSectionZoom() {
		t.Fatal("the second Esc did not leave the restored zoom")
	}
	if outliner.zoom.active() || outliner.isProjectScoped() {
		t.Fatalf("base state = zoom %+v scoped %t, want neither", outliner.zoom, outliner.isProjectScoped())
	}
}

func TestSectionZoomIsClearedByAModeChange(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	project := primaryRowsProject("alpha", doing, backlog)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeAll)

	zoomIntoSection(t, outliner, models.StateGroupDoing.Label())
	outliner.selectOutlinerMode(ModeBacklog)

	if outliner.zoom.active() {
		t.Fatalf("the mode change kept the zoom %+v, which names a section Backlog never emits", outliner.zoom)
	}
	listed := false
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task.ID == backlog.ID {
			listed = true
		}
	}
	if !listed {
		t.Fatal("a carried-over zoom silenced Backlog's own rows")
	}
}

func TestSectionZoomIsSuspendedForSearchAndRestoredOnClear(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	project := primaryRowsProject("alpha", doing, backlog)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeAll)

	doingLabel := models.StateGroupDoing.Label()
	zoomIntoSection(t, outliner, doingLabel)
	zoomed := outliner.zoom

	searchRankedList(outliner, "backlog")
	if outliner.zoom.active() {
		t.Fatalf("search left the zoom %+v live over rows no section gate builds", outliner.zoom)
	}
	if names := sectionHeaderNames(outliner); len(names) != 0 {
		t.Fatalf("search rows carried section headers %v", names)
	}
	reached := false
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task.ID == backlog.ID {
			reached = true
		}
	}
	if !reached {
		t.Fatal("search stayed inside the suspended zoom instead of ranging over the scope")
	}

	outliner.ClearSearch()
	if outliner.zoom != zoomed {
		t.Fatalf("ClearSearch restored zoom %+v, want %+v", outliner.zoom, zoomed)
	}
	if names := sectionHeaderNames(outliner); len(names) != 1 || names[0] != doingLabel {
		t.Fatalf("restored sections = %v, want the zoomed %q alone", names, doingLabel)
	}
}

func TestSectionZoomIsClearedByResetToGlobalAll(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	project := primaryRowsProject("alpha", doing, backlog)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeAll)

	zoomIntoSection(t, outliner, models.StateGroupDoing.Label())
	outliner.resetToGlobalAll()

	if outliner.zoom.active() {
		t.Fatalf("the global reset left the zoom %+v narrowing the root", outliner.zoom)
	}
	if outliner.primaryMode != ModeAll {
		t.Fatalf("reset mode = %q, want %q", outliner.primaryMode, ModeAll)
	}
	if len(sectionHeaderNames(outliner)) < 2 {
		t.Fatalf("reset sections = %v, want the whole mode back", sectionHeaderNames(outliner))
	}
}

// TestSectionZoomThawsTheRecencyFreeze records the documented consequence of
// putting the zoom in jumpProjectionFingerprint: Recent re-reads the world on a
// fold-class change rather than serving the frozen order.
func TestSectionZoomThawsTheRecencyFreeze(t *testing.T) {
	project := primaryRowsProject(
		"alpha",
		recentTask("touched-today", models.StateBacklog, 0),
		recentTask("touched-yesterday", models.StateBacklog, 1),
	)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeRecent)

	frozen := outliner.recencyProjection
	if frozen == "" {
		t.Fatal("Recent froze no projection fingerprint to compare against")
	}
	zoomIntoSection(t, outliner, "Yesterday")
	if outliner.recencyProjection == frozen {
		t.Fatal("the zoom left the recency freeze intact")
	}
}

// TestSectionZoomIsClearedByGotoTaskProject covers gp, which pushes a scope
// and forces All — two clears the jump must not have to think about.
func TestSectionZoomIsClearedByGotoTaskProject(t *testing.T) {
	driver := gotoProjectDriver(t)
	outliner := gotoProjectOutliner(t, driver)
	beta := gotoProjectID(t, driver.App())

	gotoProjectCursorOnTask(t, outliner)
	headerIndex := sectionHeaderAboveCursor(t, outliner)
	outliner.cursor = headerIndex
	if !outliner.enterSectionZoom(outliner.rows[headerIndex]) {
		t.Fatal("the section header refused the zoom")
	}
	gotoProjectCursorOnTask(t, outliner)

	sortPressKeys(t, driver, "g", "p")

	if outliner.zoom.active() {
		t.Fatalf("gp landed still zoomed to %+v", outliner.zoom)
	}
	if outliner.scopedProjectID != beta || outliner.primaryMode != ModeAll {
		t.Fatalf(
			"landing = scope %q mode %q, want %q and %q",
			outliner.scopedProjectID, outliner.primaryMode, beta, ModeAll,
		)
	}
}

// TestEscCascadeUnwindsZoomBetweenFocusAndScope locks the ordering the whole
// change depends on: narrowest first, so the zoom outlives task Focus and dies
// before the project scope that owns its mode.
func TestEscCascadeUnwindsZoomBetweenFocusAndScope(t *testing.T) {
	root := primaryRowsTask("root", models.StateDoing)
	child := primaryRowsTask("child", models.StateDoing)
	child.ParentID = &root.ID
	project := primaryRowsProject("alpha", root, child)
	outliner := scopeTestOutliner(project)
	outliner.selectOutlinerMode(ModeAll)
	outliner.pushProjectScope(project.ID)

	zoomIntoSection(t, outliner, models.StateGroupDoing.Label())
	if !outliner.moveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) ||
		!outliner.focusSubtreeAtCursor() {
		t.Fatal("could not seed task Focus inside the zoom")
	}
	outliner.peekMode = true

	assertState := func(step string, peek bool, focusDepth int, zoomed, scoped bool) {
		t.Helper()
		if outliner.peekMode != peek || len(outliner.crossFocusStack) != focusDepth ||
			outliner.zoom.active() != zoomed || outliner.isProjectScoped() != scoped {
			t.Fatalf("%s state = peek %t focus %d zoomed %t scoped %t",
				step, outliner.peekMode, len(outliner.crossFocusStack),
				outliner.zoom.active(), outliner.isProjectScoped())
		}
	}
	assertState("initial", true, 1, true, true)

	for index, want := range []struct {
		peek           bool
		focusDepth     int
		zoomed, scoped bool
	}{
		{false, 1, true, true},
		{false, 0, true, true},
		{false, 0, false, true},
		{false, 0, false, false},
	} {
		if !RunEscCascade(outliner.escSteps()) {
			t.Fatalf("Esc step %d was not handled", index+1)
		}
		assertState("after Esc", want.peek, want.focusDepth, want.zoomed, want.scoped)
	}
}
