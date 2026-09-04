package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// searchProjectionCursorOnTitle opens the flat ranked list the jump has to leave
// and lands its cursor on the named result. The ranked list is the presentation
// the jump exits; a query that only narrows the mode's own lanes leaves the
// cursor where it already is, which is why the toggle is part of the setup.
func searchProjectionCursorOnTitle(t *testing.T, out *OutlinerModel, title string) taskref.TaskIdentity {
	t.Helper()
	searchRankedList(out, title)
	if !out.showsRankedList() {
		t.Fatalf("search %q opened no ranked list", title)
	}
	for index, row := range out.rows {
		if row.isTaskRow() && row.task != nil && row.task.Title == title {
			out.Cursor = index
			return out.CurrentTaskIdentity()
		}
	}
	t.Fatalf("search %q listed no row for it", title)
	return taskref.TaskIdentity{}
}

func TestEnterZoomsSectionHeadersAndScopesProjectHeaders(t *testing.T) {
	project := scopeTestProject("project", "Project", "task")
	other := scopeTestProject("other", "Other", "other-task")
	out := scopeTestOutliner(project, other)
	sectionIndex := -1
	for index, row := range out.rows {
		if row.IsSectionHeader {
			sectionIndex = index
			break
		}
	}
	if sectionIndex < 0 {
		t.Fatal("the fixture emits no section header")
	}
	fullRows := len(out.rows)

	out.Cursor = sectionIndex
	sectionName := out.rows[sectionIndex].SectionName
	if !out.zoomSectionAtCursor() {
		t.Fatal("Enter did not zoom into the selected section header")
	}
	if names := sectionHeaderNames(out); len(names) != 1 || names[0] != sectionName {
		t.Fatalf("zoomed sections = %v, want %q alone", names, sectionName)
	}

	if !listsurface.RunEscCascade(out.escSteps()) {
		t.Fatal("Esc did not leave the zoom")
	}
	if out.zoom.active() || len(out.rows) != fullRows {
		t.Fatalf("unzoomed state = zoom %+v rows %d, want no zoom and the pre-zoom %d", out.zoom, len(out.rows), fullRows)
	}

	out.Cursor = sectionHeaderIndex(t, out, sectionName)
	if out.activateGroupAtCursor() {
		t.Fatal("a section header still reached the project scope act")
	}

	out.Cursor = projectHeaderIndex(t, out, project.ID)
	if !out.activateGroupAtCursor() || out.scopedProjectID != project.ID {
		t.Fatalf("Enter on the project header scoped to %q, want %q", out.scopedProjectID, project.ID)
	}
	if out.zoom.active() {
		t.Fatalf("the project header took a zoom %+v", out.zoom)
	}
	if ids := taskRowProjectIDs(out); len(ids) != 1 || ids[0] != project.ID {
		t.Fatalf("scoped task rows = %v, want %q alone", ids, project.ID)
	}
}

func TestFocusConstrainsModesFiltersSortFoldsAndSearch(t *testing.T) {
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	root := primaryRowsTask("root", models.StateBacklog)
	root.Title = "Focus root"

	insideInbox := primaryRowsTask("inside-inbox", models.StateInbox)
	insideToday := primaryRowsTask("inside-today", models.StateBacklog)
	insideNext := primaryRowsTask("inside-next", models.StateBacklog)
	insideBacklog := primaryRowsTask("inside-backlog", models.StateBacklog)
	insideBacklog.Title = "Zulu inside"
	insideDue := primaryRowsTask("inside-due", models.StateWaiting)
	insideDue.DueDate = &tomorrow
	insideRecent := primaryRowsTask("inside-recent", models.StateSomeday)
	insideRecent.UpdatedAt = now
	insideNested := primaryRowsTask("inside-nested", models.StateBacklog)
	insideNested.Title = "Alpha inside"

	for _, task := range []*models.Task{insideInbox, insideToday, insideNext, insideBacklog, insideDue, insideRecent} {
		task.ParentID = &root.ID
	}
	insideNested.ParentID = &insideBacklog.ID

	outsideInbox := primaryRowsTask("outside-inbox", models.StateInbox)
	outsideToday := primaryRowsTask("outside-today", models.StateBacklog)
	outsideNext := primaryRowsTask("outside-next", models.StateBacklog)
	outsideBacklog := primaryRowsTask("outside-backlog", models.StateBacklog)
	outsideBacklog.Title = "Aardvark outside"
	outsideDue := primaryRowsTask("outside-due", models.StateWaiting)
	outsideDue.DueDate = &tomorrow
	outsideRecent := primaryRowsTask("outside-recent", models.StateSomeday)
	outsideRecent.UpdatedAt = now

	project := primaryRowsProject("project", root,
		insideInbox, insideToday, insideNext, insideBacklog, insideDue, insideRecent, insideNested,
		outsideInbox, outsideToday, outsideNext, outsideBacklog, outsideDue, outsideRecent)
	out := scopeTestOutliner(project)
	out.runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: insideToday.ID},
		{ProjectID: project.ID, TaskID: outsideToday.ID},
	})
	out.runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: insideNext.ID},
		{ProjectID: project.ID, TaskID: outsideNext.ID},
	})
	out.crossForceRebuild = true
	out.crossProjectRebuildRows()
	if !out.moveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) || !out.focusSubtreeAtCursor() {
		t.Fatal("could not enter fixture Focus")
	}

	inside := map[string]bool{
		insideInbox.ID: true, insideToday.ID: true, insideNext.ID: true,
		insideBacklog.ID: true, insideDue.ID: true, insideRecent.ID: true, insideNested.ID: true,
	}
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeToday, chrome.ModeInbox, chrome.ModeNext, chrome.ModeBacklog, chrome.ModeDue, chrome.ModeRecent} {
		out.selectOutlinerMode(mode)
		visible := visibleOutlinerTaskIDs(out)
		if len(visible) == 0 {
			t.Fatalf("%s Focus projection is unexpectedly empty", mode)
		}
		for taskID := range visible {
			if !inside[taskID] {
				t.Fatalf("%s Focus escaped to task %q; visible=%v", mode, taskID, visible)
			}
		}
	}

	out.selectOutlinerMode(chrome.ModeAll)
	insideNested.Tags = []string{"focus-match"}
	outsideBacklog.Tags = []string{"focus-match"}
	out.filter.Tags = map[string]bool{"focus-match": true}
	out.adoptFilterCarriers()
	out.rebuildRows()
	if got := visibleOutlinerTaskIDs(out); len(got) != 1 || !got[insideNested.ID] {
		t.Fatalf("focused tag facet rows = %v, want only %s", got, insideNested.ID)
	}

	out.filter = query.NewFilterState()
	out.adoptFilterCarriers()
	out.sort = query.SortState{Field: query.SortByTitle, Ascending: true}
	out.treeSort = out.sort
	out.rebuildRows()
	ordered := searchVisibleTaskIDs(out)
	if len(ordered) == 0 || ordered[0] == outsideBacklog.ID || !inside[ordered[0]] {
		t.Fatalf("focused title sort escaped branch: %v", ordered)
	}

	out.expandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: insideBacklog.ID}] = false
	out.rebuildRows()
	if visible := visibleOutlinerTaskIDs(out); visible[insideNested.ID] || visible[outsideBacklog.ID] {
		t.Fatalf("focused fold rows = %v, want nested child hidden and outside task absent", visible)
	}

	insideDue.Title = "Needle inside"
	insideDue.ShortID = 41
	outsideDue.Title = "Needle outside"
	outsideDue.ShortID = 42
	out.SetSearchQuery("needle")
	if got := searchVisibleTaskIDs(out); !slices.Equal(got, []string{insideDue.ID}) {
		t.Fatalf("search within Focus = %v, want only %s", got, insideDue.ID)
	}
	out.SetSearchQuery(project.Slug + "#42")
	if got := searchVisibleTaskIDs(out); len(got) != 0 {
		t.Fatalf("exact search escaped Focus to %v", got)
	}
	out.ClearSearch()
	if len(out.crossFocusStack) != 1 || out.crossFocusStack[0].TaskID != root.ID {
		t.Fatalf("search restoration lost Focus: %+v", out.crossFocusStack)
	}

	insideInbox.WorkflowState = models.StateBacklog
	out.selectOutlinerMode(chrome.ModeInbox)
	if len(out.rows) != 0 || out.outlinerEmptyMessage() != "No tasks are visible in this Focus. Press Esc to return." {
		t.Fatalf("empty Focus = rows %d message %q", len(out.rows), out.outlinerEmptyMessage())
	}
}
