package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// searchProjectionCursorOnTitle opens the flat ranked list the jump has to leave
// and lands its cursor on the named result. The ranked list is the presentation
// the jump exits; a query that only narrows the mode's own lanes leaves the
// cursor where it already is, which is why the toggle is part of the setup.
func searchProjectionCursorOnTitle(t *testing.T, out *outliner.OutlinerModel, title string) taskref.TaskIdentity {
	t.Helper()
	searchRankedList(out, title)
	if !out.ShowsRankedList() {
		t.Fatalf("search %q opened no ranked list", title)
	}
	for index, row := range out.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.Title == title {
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
	for index, row := range out.Rows {
		if row.IsSectionHeader {
			sectionIndex = index
			break
		}
	}
	if sectionIndex < 0 {
		t.Fatal("the fixture emits no section header")
	}
	fullRows := len(out.Rows)

	out.Cursor = sectionIndex
	sectionName := out.Rows[sectionIndex].SectionName
	if !out.ZoomSectionAtCursor() {
		t.Fatal("Enter did not zoom into the selected section header")
	}
	if names := sectionHeaderNames(out); len(names) != 1 || names[0] != sectionName {
		t.Fatalf("zoomed sections = %v, want %q alone", names, sectionName)
	}

	if !listsurface.RunEscCascade(out.EscSteps()) {
		t.Fatal("Esc did not leave the zoom")
	}
	if out.Focus.Zoom.Active() || len(out.Rows) != fullRows {
		t.Fatalf("unzoomed state = zoom %+v rows %d, want no zoom and the pre-zoom %d", out.Focus.Zoom, len(out.Rows), fullRows)
	}

	out.Cursor = sectionHeaderIndex(t, out, sectionName)
	if out.ActivateGroupAtCursor() {
		t.Fatal("a section header still reached the project scope act")
	}

	out.Cursor = projectHeaderIndex(t, out, project.ID)
	if !out.ActivateGroupAtCursor() || out.Scope.ProjectID != project.ID {
		t.Fatalf("Enter on the project header scoped to %q, want %q", out.Scope.ProjectID, project.ID)
	}
	if out.Focus.Zoom.Active() {
		t.Fatalf("the project header took a zoom %+v", out.Focus.Zoom)
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
	out.Runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: insideToday.ID},
		{ProjectID: project.ID, TaskID: outsideToday.ID},
	})
	out.Runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: insideNext.ID},
		{ProjectID: project.ID, TaskID: outsideNext.ID},
	})
	out.Population.Invalidate()
	out.CrossProjectRebuildRows()
	if !out.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: project.ID, TaskID: root.ID}) || !out.FocusSubtreeAtCursor() {
		t.Fatal("could not enter fixture Focus")
	}

	inside := map[string]bool{
		insideInbox.ID: true, insideToday.ID: true, insideNext.ID: true,
		insideBacklog.ID: true, insideDue.ID: true, insideRecent.ID: true, insideNested.ID: true,
	}
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeToday, chrome.ModeInbox, chrome.ModeNext, chrome.ModeBacklog, chrome.ModeDue, chrome.ModeRecent} {
		out.SelectOutlinerMode(mode)
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

	out.SelectOutlinerMode(chrome.ModeAll)
	insideNested.Tags = []string{"focus-match"}
	outsideBacklog.Tags = []string{"focus-match"}
	out.Filter.Tags = map[string]bool{"focus-match": true}
	adoptFilterCarriers(out)
	out.RebuildRows()
	if got := visibleOutlinerTaskIDs(out); len(got) != 1 || !got[insideNested.ID] {
		t.Fatalf("focused tag facet rows = %v, want only %s", got, insideNested.ID)
	}

	out.Filter = query.NewFilterState()
	adoptFilterCarriers(out)
	out.Sort = query.SortState{Field: query.SortByTitle, Ascending: true}
	out.TreeSort = out.Sort
	out.RebuildRows()
	ordered := searchVisibleTaskIDs(out)
	if len(ordered) == 0 || ordered[0] == outsideBacklog.ID || !inside[ordered[0]] {
		t.Fatalf("focused title sort escaped branch: %v", ordered)
	}

	out.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: insideBacklog.ID}] = false
	out.RebuildRows()
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
	if len(out.Focus.CrossStack) != 1 || out.Focus.CrossStack[0].TaskID != root.ID {
		t.Fatalf("search restoration lost Focus: %+v", out.Focus.CrossStack)
	}

	insideInbox.WorkflowState = models.StateBacklog
	out.SelectOutlinerMode(chrome.ModeInbox)
	if len(out.Rows) != 0 || out.OutlinerEmptyMessage() != "No tasks are visible in this Focus. Press Esc to leave Focus." {
		t.Fatalf("empty Focus = rows %d message %q", len(out.Rows), out.OutlinerEmptyMessage())
	}
}
