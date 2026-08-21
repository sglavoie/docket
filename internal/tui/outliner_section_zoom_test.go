package tui

import (
	"maps"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func sectionHeaderNames(outliner *OutlinerModel) []string {
	names := make([]string, 0)
	for _, row := range outliner.rows {
		if row.IsSectionHeader {
			names = append(names, row.SectionName)
		}
	}
	return names
}

func sectionHeaderIndex(t *testing.T, outliner *OutlinerModel, name string) int {
	t.Helper()
	for index, row := range outliner.rows {
		if row.IsSectionHeader && row.SectionName == name {
			return index
		}
	}
	t.Fatalf("projection has no %q section header, sections are %v", name, sectionHeaderNames(outliner))
	return -1
}

func zoomIntoSection(t *testing.T, outliner *OutlinerModel, name string) {
	t.Helper()
	outliner.cursor = sectionHeaderIndex(t, outliner, name)
	if !outliner.enterSectionZoom(outliner.rows[outliner.cursor]) {
		t.Fatalf("zoom into %q was refused", name)
	}
}

func TestSectionZoomNarrowsAllToOneSectionAndRestoresViewport(t *testing.T) {
	now := time.Now()
	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Pinned = true
	pinned.PinnedAt = &now
	planned := primaryRowsTask("planned", models.StateBacklog)
	doing := primaryRowsTask("doing", models.StateDoing)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	project := primaryRowsProject("alpha", pinned, planned, doing, backlog)
	outliner, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    planned.ID,
	}})
	outliner.selectOutlinerMode(ModeAll)

	fullRows := len(outliner.rows)
	doingLabel := models.StateGroupDoing.Label()
	doingIndex := sectionHeaderIndex(t, outliner, doingLabel)
	outliner.cursor = doingIndex
	outliner.viewScrollOffset = 3

	if !outliner.enterSectionZoom(outliner.rows[doingIndex]) {
		t.Fatal("the Doing header refused the zoom")
	}
	if names := sectionHeaderNames(outliner); len(names) != 1 || names[0] != doingLabel {
		t.Fatalf("zoomed sections = %v, want %q alone", names, doingLabel)
	}
	if row := outliner.rows[0]; !row.IsSectionHeader || row.SectionCount != 1 {
		t.Fatalf("zoomed row 0 = %+v, want the Doing header carrying its count", row)
	}
	if outliner.cursor != 0 {
		t.Fatalf("zoomed cursor = %d, want the header the zoom was entered from", outliner.cursor)
	}
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task.ID != doing.ID {
			t.Fatalf("zoomed projection kept %q, which lives outside Doing", row.task.ID)
		}
	}
	if outliner.zoom.name != doingLabel {
		t.Fatalf("zoom name = %q, want the section's display name", outliner.zoom.name)
	}

	if !outliner.exitSectionZoom() {
		t.Fatal("an active zoom refused to exit")
	}
	if outliner.zoom.active() {
		t.Fatal("the zoom stayed active after exiting")
	}
	if len(outliner.rows) != fullRows {
		t.Fatalf("unzoomed row count = %d, want the pre-zoom %d", len(outliner.rows), fullRows)
	}
	if outliner.cursor != doingIndex || outliner.viewScrollOffset != 3 {
		t.Fatalf(
			"unzoomed viewport = cursor %d scroll %d, want cursor %d scroll 3",
			outliner.cursor, outliner.viewScrollOffset, doingIndex,
		)
	}
	if outliner.exitSectionZoom() {
		t.Fatal("exiting an inactive zoom reported a change")
	}
}

func TestSectionZoomOverridesTheSectionFoldWithoutWritingIt(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	project := primaryRowsProject("alpha", doing, backlog)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeAll)

	doingLabel := models.StateGroupDoing.Label()
	outliner.setSectionRowCollapsed(outliner.rows[sectionHeaderIndex(t, outliner, doingLabel)], true)
	outliner.rebuildRows()
	foldsBefore := maps.Clone(outliner.collapsedSections)

	zoomIntoSection(t, outliner, doingLabel)
	if len(outliner.rows) < 2 {
		t.Fatalf("zoomed collapsed section emitted %d rows, want its tasks too", len(outliner.rows))
	}
	if outliner.isSectionRowCollapsed(outliner.rows[0]) {
		t.Fatal("the zoomed section still reads collapsed")
	}
	if outliner.collapseOrMoveToParent() {
		t.Fatal("h folded the zoomed section")
	}
	if outliner.toggleVisibleSections() {
		t.Fatal("S folded the zoomed section")
	}
	if !maps.Equal(outliner.collapsedSections, foldsBefore) {
		t.Fatalf("the zoom changed fold state: got %v, want %v", outliner.collapsedSections, foldsBefore)
	}

	if !outliner.exitSectionZoom() {
		t.Fatal("an active zoom refused to exit")
	}
	restored := outliner.rows[sectionHeaderIndex(t, outliner, doingLabel)]
	if !outliner.isSectionRowCollapsed(restored) {
		t.Fatal("the fold the user set did not survive the zoom")
	}
}

func TestSectionZoomOnPinnedEmitsNoSeparator(t *testing.T) {
	now := time.Now()
	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Pinned = true
	pinned.PinnedAt = &now
	other := primaryRowsTask("other", models.StateBacklog)
	project := primaryRowsProject("alpha", pinned, other)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeAll)

	zoomIntoSection(t, outliner, pinnedSectionName)
	for index, row := range outliner.rows {
		if row.IsSeparator {
			t.Fatalf("zoomed Pinned kept the separator at row %d: %v", index, sectionHeaderNames(outliner))
		}
		if row.isTaskRow() && row.task.ID != pinned.ID {
			t.Fatalf("zoomed Pinned kept %q, which is not pinned", row.task.ID)
		}
	}
	if row := outliner.rows[0]; !row.IsSectionHeader || !row.IsPinnedSection {
		t.Fatalf("zoomed row 0 = %+v, want the Pinned header", row)
	}
}

func TestSectionZoomNarrowsComputedDueAndRecentBuckets(t *testing.T) {
	now := time.Now()
	dueToday := primaryRowsTask("due-today", models.StateBacklog)
	dueToday.DueDate = &now
	dueLater := primaryRowsTask("due-later", models.StateBacklog)
	laterDue := now.AddDate(0, 0, 2)
	dueLater.DueDate = &laterDue
	dueProject := primaryRowsProject("alpha", dueToday, dueLater)
	dueOutliner, _ := primaryRowsOutliner(dueProject)
	dueOutliner.selectOutlinerMode(ModeDue)

	dueSections := len(sectionHeaderNames(dueOutliner))
	zoomIntoSection(t, dueOutliner, "Today")
	if names := sectionHeaderNames(dueOutliner); len(names) != 1 || names[0] != "Today" {
		t.Fatalf("zoomed Due sections = %v, want Today alone", names)
	}
	for _, row := range dueOutliner.rows {
		if row.isTaskRow() && row.task.ID != dueToday.ID {
			t.Fatalf("zoomed Due Today kept %q", row.task.ID)
		}
	}
	if !dueOutliner.exitSectionZoom() {
		t.Fatal("the Due zoom refused to exit")
	}
	if names := sectionHeaderNames(dueOutliner); len(names) != dueSections {
		t.Fatalf("unzoomed Due sections = %v, want the pre-zoom %d", names, dueSections)
	}

	recentProject := primaryRowsProject(
		"beta",
		recentTask("touched-today", models.StateBacklog, 0),
		recentTask("touched-yesterday", models.StateBacklog, 1),
	)
	recentOutliner, _ := primaryRowsOutliner(recentProject)
	recentOutliner.selectOutlinerMode(ModeRecent)

	zoomIntoSection(t, recentOutliner, "Yesterday")
	if names := sectionHeaderNames(recentOutliner); len(names) != 1 || names[0] != "Yesterday" {
		t.Fatalf("zoomed Recent sections = %v, want Yesterday alone", names)
	}
	for _, row := range recentOutliner.rows {
		if row.isTaskRow() && row.task.ID != "touched-yesterday" {
			t.Fatalf("zoomed Recent Yesterday kept %q", row.task.ID)
		}
	}
}

func TestReenteringTheSameSectionZoomKeepsTheRetainedViewport(t *testing.T) {
	first := primaryRowsTask("first", models.StateDoing)
	second := primaryRowsTask("second", models.StateDoing)
	project := primaryRowsProject("alpha", first, second)
	outliner, _ := primaryRowsOutliner(project)
	outliner.selectOutlinerMode(ModeAll)

	doingLabel := models.StateGroupDoing.Label()
	doingIndex := sectionHeaderIndex(t, outliner, doingLabel)
	outliner.cursor = doingIndex
	outliner.viewScrollOffset = 5
	if !outliner.enterSectionZoom(outliner.rows[doingIndex]) {
		t.Fatal("the Doing header refused the zoom")
	}

	zoomBefore := outliner.zoom
	rowsBefore := len(outliner.rows)
	outliner.cursor = 1
	if !outliner.enterSectionZoom(outliner.rows[0]) {
		t.Fatal("re-entering the active zoom was refused")
	}
	if outliner.zoom != zoomBefore {
		t.Fatalf("re-entry replaced the retained viewport: got %+v, want %+v", outliner.zoom, zoomBefore)
	}
	if outliner.cursor != 1 || len(outliner.rows) != rowsBefore {
		t.Fatalf(
			"re-entry moved the view: cursor %d rows %d, want cursor 1 rows %d",
			outliner.cursor, len(outliner.rows), rowsBefore,
		)
	}

	if !outliner.exitSectionZoom() {
		t.Fatal("an active zoom refused to exit")
	}
	if outliner.cursor != doingIndex || outliner.viewScrollOffset != 5 {
		t.Fatalf(
			"unzoomed viewport = cursor %d scroll %d, want cursor %d scroll 5",
			outliner.cursor, outliner.viewScrollOffset, doingIndex,
		)
	}
}
