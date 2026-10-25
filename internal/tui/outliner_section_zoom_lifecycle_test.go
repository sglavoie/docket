package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// projectHeaderIndex folds one project's groups and returns the first summary
// that leaves behind. The fold is the only way to a project header: an open
// group draws none, and its summary is the row every project operation resolves
// from.
func projectHeaderIndex(t *testing.T, outlinerView *outliner.OutlinerModel, projectID string) int {
	t.Helper()
	folded := false
	for _, group := range outlinerView.VisibleProjectGroups() {
		if group.ProjectID != projectID {
			continue
		}
		outlinerView.SetProjectGroupFolded(group, true)
		folded = true
	}
	if folded {
		outlinerView.RebuildRows()
	}
	for index, row := range outlinerView.Rows {
		if row.IsProjectHeader && row.ProjectID == projectID {
			return index
		}
	}
	t.Fatalf("projection has no project group for %q", projectID)
	return -1
}

// sectionHeaderAboveCursor names the section the cursor row belongs to, so a
// fixture test can zoom into it without knowing the fixture's section labels.
func sectionHeaderAboveCursor(t *testing.T, outlinerView *outliner.OutlinerModel) int {
	t.Helper()
	for index := outlinerView.Cursor; index >= 0; index-- {
		row := outlinerView.Rows[index]
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
	outlinerView, _ := primaryRowsOutliner(alpha, beta)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	doingLabel := models.StateGroupDoing.Label()
	zoomIntoSection(t, outlinerView, doingLabel)
	zoomed := outlinerView.Focus.Zoom

	headerIndex := projectHeaderIndex(t, outlinerView, beta.ID)
	outlinerView.Cursor = headerIndex
	if !outlinerView.ActivateGroupAtCursor() {
		t.Fatal("Enter on the project sub-header was refused")
	}
	if outlinerView.Scope.ProjectID != beta.ID {
		t.Fatalf("scoped project = %q, want %q", outlinerView.Scope.ProjectID, beta.ID)
	}
	if outlinerView.Focus.Zoom.Active() {
		t.Fatalf("the scope push kept the zoom %+v", outlinerView.Focus.Zoom)
	}
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.ProjectID == alpha.ID {
			t.Fatal("the scoped projection still lists the other project's tasks")
		}
	}

	if !outlinerView.PopProjectScope() {
		t.Fatal("Esc did not pop the project scope")
	}
	if outlinerView.Focus.Zoom != zoomed {
		t.Fatalf("the popped scope restored zoom %+v, want %+v", outlinerView.Focus.Zoom, zoomed)
	}
	if outlinerView.Cursor != headerIndex {
		t.Fatalf("restored cursor = %d, want the pre-push %d", outlinerView.Cursor, headerIndex)
	}
	if names := sectionHeaderNames(outlinerView); len(names) != 1 || names[0] != doingLabel {
		t.Fatalf("restored sections = %v, want the zoomed %q alone", names, doingLabel)
	}

	if !outlinerView.ExitSectionZoom() {
		t.Fatal("the second Esc did not leave the restored zoom")
	}
	if outlinerView.Focus.Zoom.Active() || outlinerView.IsProjectScoped() {
		t.Fatalf("base state = zoom %+v scoped %t, want neither", outlinerView.Focus.Zoom, outlinerView.IsProjectScoped())
	}
}

func TestSectionZoomIsClearedByAModeChange(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	project := primaryRowsProject("alpha", doing, backlog)
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	zoomIntoSection(t, outlinerView, models.StateGroupDoing.Label())
	outlinerView.SelectOutlinerMode(chrome.ModeBacklog)

	if outlinerView.Focus.Zoom.Active() {
		t.Fatalf("the mode change kept the zoom %+v, which names a section Backlog never emits", outlinerView.Focus.Zoom)
	}
	listed := false
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task.ID == backlog.ID {
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
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	doingLabel := models.StateGroupDoing.Label()
	zoomIntoSection(t, outlinerView, doingLabel)
	zoomed := outlinerView.Focus.Zoom

	searchRankedList(outlinerView, "backlog")
	if outlinerView.Focus.Zoom.Active() {
		t.Fatalf("search left the zoom %+v live over rows no section gate builds", outlinerView.Focus.Zoom)
	}
	if names := sectionHeaderNames(outlinerView); len(names) != 0 {
		t.Fatalf("search rows carried section headers %v", names)
	}
	reached := false
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task.ID == backlog.ID {
			reached = true
		}
	}
	if !reached {
		t.Fatal("search stayed inside the suspended zoom instead of ranging over the scope")
	}

	outlinerView.ClearSearch()
	if outlinerView.Focus.Zoom != zoomed {
		t.Fatalf("ClearSearch restored zoom %+v, want %+v", outlinerView.Focus.Zoom, zoomed)
	}
	if names := sectionHeaderNames(outlinerView); len(names) != 1 || names[0] != doingLabel {
		t.Fatalf("restored sections = %v, want the zoomed %q alone", names, doingLabel)
	}
}

func TestSectionZoomIsClearedByResetToGlobalAll(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	project := primaryRowsProject("alpha", doing, backlog)
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	zoomIntoSection(t, outlinerView, models.StateGroupDoing.Label())
	outlinerView.ResetToGlobalAll()

	if outlinerView.Focus.Zoom.Active() {
		t.Fatalf("the global reset left the zoom %+v narrowing the root", outlinerView.Focus.Zoom)
	}
	if outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("reset mode = %q, want %q", outlinerView.PrimaryMode, chrome.ModeAll)
	}
	if len(sectionHeaderNames(outlinerView)) < 2 {
		t.Fatalf("reset sections = %v, want the whole mode back", sectionHeaderNames(outlinerView))
	}
}

// TestSectionZoomThawsTheRecencyFreeze records the documented consequence of
// putting the zoom in jumpProjectionFingerprint: Recent re-reads the world on a
// fold-class change rather than serving the frozen order.
func TestSectionZoomThawsTheRecencyFreeze(t *testing.T) {
	project := primaryRowsProject(
		"alpha",
		recentTask("touched-today", 0),
		recentTask("touched-yesterday", 1),
	)
	outlinerView, _ := primaryRowsOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeRecent)

	frozen := outlinerView.Recency.Projection
	if frozen == "" {
		t.Fatal("Recent froze no projection fingerprint to compare against")
	}
	zoomIntoSection(t, outlinerView, "Yesterday")
	if outlinerView.Recency.Projection == frozen {
		t.Fatal("the zoom left the recency freeze intact")
	}
}

// TestSectionZoomIsClearedByGotoTaskProject covers gp, which pushes a scope
// and forces All — two clears the jump must not have to think about.
func TestSectionZoomIsClearedByGotoTaskProject(t *testing.T) {
	driver := gotoProjectDriver(t)
	outlinerView := gotoProjectOutliner(t, driver)
	beta := gotoProjectID(t, driver.App())

	gotoProjectCursorOnTask(t, outlinerView)
	headerIndex := sectionHeaderAboveCursor(t, outlinerView)
	outlinerView.Cursor = headerIndex
	if !outlinerView.EnterSectionZoom(outlinerView.Rows[headerIndex]) {
		t.Fatal("the section header refused the zoom")
	}
	gotoProjectCursorOnTask(t, outlinerView)

	sortPressKeys(t, driver, "g", "p")

	if outlinerView.Focus.Zoom.Active() {
		t.Fatalf("gp landed still zoomed to %+v", outlinerView.Focus.Zoom)
	}
	if outlinerView.Scope.ProjectID != beta || outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf(
			"landing = scope %q mode %q, want %q and %q",
			outlinerView.Scope.ProjectID, outlinerView.PrimaryMode, beta, chrome.ModeAll,
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
	outlinerView := scopeTestOutliner(project)
	outlinerView.SelectOutlinerMode(chrome.ModeAll)
	outlinerView.PushProjectScope(project.ID)

	zoomIntoSection(t, outlinerView, models.StateGroupDoing.Label())
	if !outlinerView.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) ||
		!outlinerView.FocusSubtreeAtCursor() {
		t.Fatal("could not seed task Focus inside the zoom")
	}
	outlinerView.PeekMode = true

	assertState := func(step string, peek bool, focusDepth int, zoomed, scoped bool) {
		t.Helper()
		if outlinerView.PeekMode != peek || len(outlinerView.Focus.CrossStack) != focusDepth ||
			outlinerView.Focus.Zoom.Active() != zoomed || outlinerView.IsProjectScoped() != scoped {
			t.Fatalf("%s state = peek %t focus %d zoomed %t scoped %t",
				step, outlinerView.PeekMode, len(outlinerView.Focus.CrossStack),
				outlinerView.Focus.Zoom.Active(), outlinerView.IsProjectScoped())
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
		if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
			t.Fatalf("Esc step %d was not handled", index+1)
		}
		assertState("after Esc", want.peek, want.focusDepth, want.zoomed, want.scoped)
	}
}
