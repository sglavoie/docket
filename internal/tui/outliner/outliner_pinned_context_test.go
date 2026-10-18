package outliner

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// A window that opens inside a lane spends its first screen row naming that
// lane and the project the first row belongs to. These are the contracts over
// that one row: when it is drawn, what it may not cost, and that every reader of
// the window still reads the window the render settled on.

const (
	pinnedContextWidth  = 100
	pinnedContextHeight = 20
	// pinnedContextLaneTasks is enough work per project and lane that either
	// lane alone overflows the window, so a cursor can sit deep inside one.
	pinnedContextLaneTasks = 30
)

// pinnedContextFixture is All over two projects, each holding one long Doing
// lane and one long Backlog lane.
func pinnedContextFixture(t *testing.T) *OutlinerModel {
	t.Helper()
	build := func(slug string) *models.Project {
		tasks := make([]*models.Task, 0, 2*pinnedContextLaneTasks)
		for index := range pinnedContextLaneTasks {
			tasks = append(tasks,
				primaryRowsTask(fmt.Sprintf("%s-doing-%02d", slug, index), models.StateDoing),
				primaryRowsTask(fmt.Sprintf("%s-backlog-%02d", slug, index), models.StateBacklog),
			)
		}
		return primaryRowsProject(slug, tasks...)
	}
	model, _ := primaryRowsOutliner(build("alpha"), build("beta"))
	model.SetSize(pinnedContextWidth, pinnedContextHeight)
	model.SelectOutlinerMode(chrome.ModeAll)
	if len(model.Rows) < 4*pinnedContextLaneTasks {
		t.Fatalf("the fixture built %d rows", len(model.Rows))
	}
	return model
}

// pinnedContextLines renders the list and returns its plain screen rows.
func pinnedContextLines(model *OutlinerModel) (outlinerViewport, []string) {
	viewport := model.ComputeOutlinerViewport(model.Width(), 0)
	return viewport, strings.Split(ansi.Strip(model.renderOutlinerRows(viewport)), "\n")
}

func pinnedContextSeat(t *testing.T, model *OutlinerModel, taskID, projectID string) {
	t.Helper()
	if !model.MoveCursorToProjectTask(taskID, projectID) {
		t.Fatalf("the cursor did not land on %s/%s", projectID, taskID)
	}
}

func TestPinnedContextIsDrawnOnlyWhenTheLaneHeaderIsAboveTheWindow(t *testing.T) {
	model := pinnedContextFixture(t)

	viewport, lines := pinnedContextLines(model)
	if viewport.pinned || viewport.startIdx != 0 {
		t.Fatalf("an unscrolled window reported %+v", viewport)
	}
	if strings.Contains(lines[0], "›") {
		t.Fatalf("an unscrolled window drew a pinned row: %q", lines[0])
	}

	pinnedContextSeat(t, model, "beta-doing-20", "beta")
	viewport, lines = pinnedContextLines(model)
	if !viewport.pinned {
		t.Fatalf("a window deep inside Doing reported no pinned row: %+v", viewport)
	}
	header := model.Rows[viewport.pinnedHeader]
	if !header.IsSectionHeader || viewport.pinnedHeader >= viewport.startIdx {
		t.Fatalf("the pinned row stands in for row %d (%+v) with the window at %d", viewport.pinnedHeader, header, viewport.startIdx)
	}
	want := fmt.Sprintf("▾  Doing › [beta]  (%d) ─", header.SectionCount)
	if !strings.HasPrefix(lines[0], want) {
		t.Fatalf("the pinned row = %q, want it to open %q", lines[0], want)
	}
	if got := viewport.endIdx - viewport.startIdx; got != viewport.VisibleRows-1 {
		t.Fatalf("a pinned window holds %d rows of %d screen rows, want one fewer", got, viewport.VisibleRows)
	}
	if len(lines) != viewport.VisibleRows {
		t.Fatalf("the list drew %d screen rows, want the %d it was given", len(lines), viewport.VisibleRows)
	}

	// The window opening exactly on a lane header has nothing scrolled past.
	backlog := -1
	for index, row := range model.Rows {
		if row.IsSectionHeader && index > viewport.pinnedHeader {
			backlog = index
			break
		}
	}
	if backlog < 0 {
		t.Fatal("the fixture drew no second lane")
	}
	model.Cursor, model.ViewScrollOffset = backlog, backlog
	viewport, lines = pinnedContextLines(model)
	if viewport.pinned || viewport.startIdx != backlog || strings.Contains(lines[0], "›") {
		t.Fatalf("a window opening on its lane header reported %+v and drew %q", viewport, lines[0])
	}
	if got := viewport.endIdx - viewport.startIdx; got != viewport.VisibleRows {
		t.Fatalf("an unpinned window holds %d rows, want all %d", got, viewport.VisibleRows)
	}
}

func TestPinnedContextNeverCoversTheCursorAndIsExactlyTheRowWidth(t *testing.T) {
	model := pinnedContextFixture(t)
	pinnedContextSeat(t, model, "alpha-doing-00", "alpha")
	seenPinned := false
	for range len(model.Rows) {
		viewport := model.ComputeOutlinerViewport(model.Width(), 0)
		if model.Cursor < viewport.startIdx || model.Cursor >= viewport.endIdx {
			t.Fatalf("the cursor at row %d is outside the window %d..%d (pinned %t)",
				model.Cursor, viewport.startIdx, viewport.endIdx, viewport.pinned)
		}
		if viewport.drawnRows() > viewport.VisibleRows {
			t.Fatalf("the window draws %d screen rows of %d", viewport.drawnRows(), viewport.VisibleRows)
		}
		if viewport.pinned {
			seenPinned = true
			line := model.renderPinnedContextRow(viewport)
			if got := lipgloss.Width(line); got != viewport.rowWidth {
				t.Fatalf("the pinned row is %d columns wide, want %d: %q", got, viewport.rowWidth, ansi.Strip(line))
			}
		}
		if model.Cursor == len(model.Rows)-1 {
			break
		}
		model.Cursor++
	}
	if !seenPinned {
		t.Fatal("walking the whole projection never drew a pinned row")
	}

	// A lane name the row cannot hold is cut to the row, never past it.
	pinnedContextSeat(t, model, "beta-doing-20", "beta")
	viewport := model.ComputeOutlinerViewport(model.Width(), 0)
	viewport.rowWidth = 12
	if got := lipgloss.Width(model.renderPinnedContextRow(viewport)); got != 12 {
		t.Fatalf("a narrow pinned row is %d columns wide, want 12", got)
	}
}

func TestPinnedContextHoldsStillWhileTheCursorMovesInsideOneLane(t *testing.T) {
	model := pinnedContextFixture(t)
	pinnedContextSeat(t, model, "beta-doing-20", "beta")
	viewport, lines := pinnedContextLines(model)
	offset, pinned := model.ViewScrollOffset, lines[0]

	// A frame that changes nothing writes the offset it found.
	if again := model.ComputeOutlinerViewport(model.Width(), 0); again != viewport || model.ViewScrollOffset != offset {
		t.Fatalf("an idle frame moved the window from %+v at %d to %+v at %d", viewport, offset, again, model.ViewScrollOffset)
	}

	for range 3 {
		model.Cursor--
		_, lines = pinnedContextLines(model)
		if model.ViewScrollOffset != offset {
			t.Fatalf("moving the cursor up inside the window scrolled it from %d to %d", offset, model.ViewScrollOffset)
		}
		if lines[0] != pinned {
			t.Fatalf("the pinned row changed from %q to %q", pinned, lines[0])
		}
	}
}

func TestPinnedContextIsWithheldWhereNoLaneHeaderScrolledAway(t *testing.T) {
	t.Run("ranked list", func(t *testing.T) {
		model := pinnedContextFixture(t)
		searchRankedList(model, "doing")
		if !model.ShowsRankedList() || len(model.Rows) < pinnedContextLaneTasks {
			t.Fatalf("the ranked list holds %d rows", len(model.Rows))
		}
		model.Cursor = len(model.Rows) - 1
		if viewport, lines := pinnedContextLines(model); viewport.pinned || viewport.startIdx == 0 || strings.Contains(lines[0], "›") {
			t.Fatalf("a scrolled ranked list reported %+v and drew %q", viewport, lines[0])
		}
	})

	t.Run("focused single-project form", func(t *testing.T) {
		model := pinnedContextFixture(t)
		model.PushProjectScope("beta")
		model.RebuildRows()
		if model.groupsRowsByProject() {
			t.Fatal("the scoped projection is still project-grouped")
		}
		model.Cursor = len(model.Rows) - 1
		if viewport, lines := pinnedContextLines(model); viewport.pinned || viewport.startIdx == 0 || strings.Contains(lines[0], "›") {
			t.Fatalf("a scrolled single-project form reported %+v and drew %q", viewport, lines[0])
		}
	})

	t.Run("zero-group first row", func(t *testing.T) {
		model := pinnedContextFixture(t)
		pinnedContextSeat(t, model, "beta-doing-20", "beta")
		viewport := model.ComputeOutlinerViewport(model.Width(), 0)
		if !viewport.pinned {
			t.Fatalf("the starting window is not pinned: %+v", viewport)
		}
		model.Rows[viewport.startIdx].Group = OutlinerRowGroup{}
		if again := model.ComputeOutlinerViewport(model.Width(), 0); again.pinned {
			t.Fatalf("a window opening on a row with no group reported %+v", again)
		}
	})

	t.Run("an index the rows no longer bear out", func(t *testing.T) {
		model := pinnedContextFixture(t)
		pinnedContextSeat(t, model, "beta-doing-20", "beta")
		viewport := model.ComputeOutlinerViewport(model.Width(), 0)
		model.Rows[viewport.pinnedHeader].IsSectionHeader = false
		if again := model.ComputeOutlinerViewport(model.Width(), 0); again.pinned {
			t.Fatalf("a stale header index was trusted: %+v", again)
		}
	})
}

// TestPinnedContextGivesTheRowBackAtTheEndOfTheList covers the one window the
// smaller height cannot improve: the cursor on the last row, with the row the
// pinned line would have left first being a lane header itself.
func TestPinnedContextGivesTheRowBackAtTheEndOfTheList(t *testing.T) {
	model := pinnedContextFixture(t)
	model.Cursor = len(model.Rows) - 1
	rows := model.ComputeOutlinerViewport(model.Width(), 0).VisibleRows
	// Cut the document so the last full window starts one row above a header.
	header := -1
	for index := len(model.Rows) - 1; index >= 0; index-- {
		if model.Rows[index].IsSectionHeader {
			header = index
			break
		}
	}
	if header <= 0 || header+rows-1 > len(model.Rows) {
		t.Fatalf("the last lane header at %d leaves no window of %d rows", header, rows)
	}
	model.Rows = model.Rows[:header+rows-1]
	model.Cursor, model.ViewScrollOffset = len(model.Rows)-1, 0

	viewport := model.ComputeOutlinerViewport(model.Width(), 0)
	if viewport.pinned || viewport.startIdx != header-1 || viewport.endIdx != len(model.Rows) {
		t.Fatalf("the last window = %+v, want every screen row spent on rows from %d", viewport, header-1)
	}
	if again := model.ComputeOutlinerViewport(model.Width(), 0); again != viewport {
		t.Fatalf("the same frame drew %+v and then %+v", viewport, again)
	}
}

// TestScrollAnchorMeasuresTheWindowItLandsIn starts an anchor from a window
// that pins nothing and lets it scroll the lane's header away: the row
// the pin costs belongs to the destination, so one press has to settle.
func TestScrollAnchorMeasuresTheWindowItLandsIn(t *testing.T) {
	for _, anchor := range []struct {
		id        operation.OperationID
		rowsAbove func(height int) int
	}{
		{operation.OperationCoreScrollCursorTop, func(int) int { return 0 }},
		{operation.OperationCoreScrollCursorMiddle, func(height int) int { return (height - 1) / 2 }},
	} {
		model := pinnedContextFixture(t)
		start := model.ComputeOutlinerViewport(model.Width(), 0)
		if start.pinned || start.startIdx != 0 {
			t.Fatalf("the starting window is not the unpinned top: %+v", start)
		}
		// The last row the unscrolled window draws: both anchors scroll down to
		// honor it, far enough that the header leaves. The bottom anchor has no
		// such start, since that row is already where it would put the cursor.
		model.Cursor = start.endIdx - 1
		cursor := model.Cursor
		if before := model.ComputeOutlinerViewport(model.Width(), 0); before != start {
			t.Fatalf("moving inside the window redrew it as %+v", before)
		}

		model.ViewScrollOffset = model.CursorScrollOffset(anchor.id)
		drawn := model.ComputeOutlinerViewport(model.Width(), 0)
		if !drawn.pinned || model.Cursor != cursor {
			t.Fatalf("%s left the window %+v with the cursor on %d", anchor.id, drawn, model.Cursor)
		}
		if height := drawn.endIdx - drawn.startIdx; height != start.endIdx-start.startIdx-1 {
			t.Fatalf("%s drew %d rows beneath the pinned row, want one fewer than %d",
				anchor.id, height, start.endIdx-start.startIdx)
		}
		if got, want := cursor-drawn.startIdx, anchor.rowsAbove(drawn.endIdx-drawn.startIdx); got != want {
			t.Fatalf("%s put %d rows above the cursor beneath the new pinned row, want %d", anchor.id, got, want)
		}
		if again := model.CursorScrollOffset(anchor.id); again != model.ViewScrollOffset {
			t.Fatalf("%s needs a second press: it would move the window from %d to %d",
				anchor.id, model.ViewScrollOffset, again)
		}
	}
}

// TestPinnedContextWindowIsTheOneEveryReaderReads holds the window's other
// readers to a pinned frame: the cached window, the three scroll anchors, and
// the hosted-panel loan.
func TestPinnedContextWindowIsTheOneEveryReaderReads(t *testing.T) {
	model := pinnedContextFixture(t)
	pinnedContextSeat(t, model, "beta-doing-20", "beta")
	viewport := model.ComputeOutlinerViewport(model.Width(), 0)
	if !viewport.pinned {
		t.Fatalf("the starting window is not pinned: %+v", viewport)
	}
	if start, end := model.VisibleRowWindow(); start != viewport.startIdx || end != viewport.endIdx {
		t.Fatalf("VisibleRowWindow = %d..%d, want the drawn %d..%d", start, end, viewport.startIdx, viewport.endIdx)
	}

	cursor := model.Cursor
	for _, anchor := range []struct {
		id        operation.OperationID
		rowsAbove func(height int) int
	}{
		{operation.OperationCoreScrollCursorTop, func(int) int { return 0 }},
		{operation.OperationCoreScrollCursorMiddle, func(height int) int { return (height - 1) / 2 }},
		{operation.OperationCoreScrollCursorBottom, func(height int) int { return height - 1 }},
	} {
		model.ViewScrollOffset = model.CursorScrollOffset(anchor.id)
		drawn := model.ComputeOutlinerViewport(model.Width(), 0)
		if !drawn.pinned || model.Cursor != cursor {
			t.Fatalf("%s left the window %+v with the cursor on %d", anchor.id, drawn, model.Cursor)
		}
		if got, want := cursor-drawn.startIdx, anchor.rowsAbove(drawn.endIdx-drawn.startIdx); got != want {
			t.Fatalf("%s put %d rows above the cursor beneath the pinned row, want %d", anchor.id, got, want)
		}
		if model.CursorScrollOffset(anchor.id) != model.ViewScrollOffset {
			t.Fatalf("%s is not settled: it would move the window from %d to %d",
				anchor.id, model.ViewScrollOffset, model.CursorScrollOffset(anchor.id))
		}
	}

	model.ViewScrollOffset = model.CursorScrollOffset(operation.OperationCoreScrollCursorMiddle)
	before := model.ComputeOutlinerViewport(model.Width(), 0)
	model.SetHostedBudget(pinnedContextWidth, pinnedContextHeight-6)
	during := model.ComputeOutlinerViewport(model.Width(), 0)
	if during.startIdx != before.startIdx || !during.pinned {
		t.Fatalf("the loan moved a pinned window from %+v to %+v", before, during)
	}
	model.ReleaseHostedBudget(pinnedContextWidth, pinnedContextHeight)
	if after := model.ComputeOutlinerViewport(model.Width(), 0); after != before {
		t.Fatalf("the release came back to %+v, want %+v", after, before)
	}
}

func TestPinnedProjectContextInInbox(t *testing.T) {
	tasks := make([]*models.Task, 0, 60)
	for index := range 60 {
		tasks = append(tasks, primaryRowsTask(fmt.Sprintf("beta-inbox-%02d", index), models.StateInbox))
	}
	model, _ := primaryRowsOutliner(primaryRowsProject("alpha", primaryRowsTask("alpha-task", models.StateInbox)), primaryRowsProject("beta", tasks...))
	model.SelectOutlinerMode(chrome.ModeInbox)
	for _, width := range []int{60, 80, 100} {
		model.SetSize(width, 20)
		pinnedContextSeat(t, model, "beta-inbox-40", "beta")
		viewport, lines := pinnedContextLines(model)
		if !viewport.pinned || !strings.Contains(lines[0], "[beta]") {
			t.Fatalf("width %d lost project context: %+v %q", width, viewport, lines[0])
		}
		if viewport.startIdx > model.Cursor || viewport.endIdx <= model.Cursor {
			t.Fatal("context hid cursor")
		}
		if lipgloss.Width(model.renderPinnedContextRow(viewport)) != viewport.rowWidth {
			t.Fatal("context width changed")
		}
	}
}
