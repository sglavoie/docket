package outliner

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// A bounded panel borrows rows from the list underneath it and gives them back.
// These are the contracts over that loan: what the borrowing may not cost, what
// the repayment must restore, and where the line between a loan and a terminal
// resize falls. The window itself is still ComputeOutlinerViewport's — every
// assertion below reads the offset the render settled on, never one this file
// computed.

// hostBudgetFixture is one project of plain backlog tasks, enough of them that
// the window has somewhere to be other than the top.
func hostBudgetFixture(t *testing.T) *OutlinerModel {
	t.Helper()
	const count = hostBudgetTaskCount
	tasks := make([]*models.Task, 0, count)
	for index := range count {
		tasks = append(tasks, renderFixtureTask(
			t,
			fmt.Sprintf("task-%03d", index),
			index+1,
			fmt.Sprintf("Task %03d", index),
			models.StateBacklog,
			0,
			models.StateBacklog,
		))
	}
	project := renderFixtureProject(t, "project", "project", "Project", tasks)
	model := NewOutlinerModel(project, nil, newTestConfig())
	model.SetSize(hostBudgetWidth, hostBudgetHeight)
	model.RebuildRows()
	if len(model.Rows) < count {
		t.Fatalf("the fixture built %d rows for %d tasks", len(model.Rows), count)
	}
	return model
}

const (
	hostBudgetWidth  = 100
	hostBudgetHeight = 33
	// hostBudgetPanelRows is a plausible bounded panel: a good third of the
	// content area, which is what makes a shortened window a different window.
	hostBudgetPanelRows = 12
	// hostBudgetTaskCount and hostBudgetSeatedCursor are one starting position
	// every case below shares: a document far longer than the screen, read from
	// well inside it, so the window always has somewhere to be other than the top.
	hostBudgetTaskCount    = 120
	hostBudgetSeatedCursor = 90
)

// seatHostBudgetWindow puts the cursor on one task and lets the render settle
// the window around it, which is the state every case below starts from.
func seatHostBudgetWindow(t *testing.T, model *OutlinerModel) {
	t.Helper()
	model.Cursor = hostBudgetSeatedCursor
	model.ComputeOutlinerViewport(model.Width(), 0)
	if model.ViewScrollOffset == 0 {
		t.Fatalf("the window is still at the top with the cursor on row %d, so there is no position to lose", model.Cursor)
	}
}

// TestHostedBudgetRoundTripRestoresTheWindow is the behavior the seam exists
// for. The rows, the folds and the cursor are all unchanged, so the window the
// release hands back is the window the loan started from — not merely one the
// cursor happens to be visible in.
func TestHostedBudgetRoundTripRestoresTheWindow(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)
	cursor, before := model.Cursor, model.ViewScrollOffset
	identity := model.CurrentTaskIdentity()

	model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight-hostBudgetPanelRows)
	model.ComputeOutlinerViewport(model.Width(), 0)
	if model.Cursor != cursor {
		t.Fatalf("the loan moved the cursor from row %d to row %d", cursor, model.Cursor)
	}
	if got := model.CurrentTaskIdentity(); got != identity {
		t.Fatalf("the loan re-seated the cursor on %+v, want the task it was on %+v", got, identity)
	}

	model.ReleaseHostedBudget(hostBudgetWidth, hostBudgetHeight)
	model.ComputeOutlinerViewport(model.Width(), 0)
	if model.ViewScrollOffset != before {
		t.Fatalf("the release came back to offset %d, want the %d the loan started from", model.ViewScrollOffset, before)
	}
	if model.Cursor != cursor {
		t.Fatalf("the release moved the cursor from row %d to row %d", cursor, model.Cursor)
	}
}

// TestHostedBudgetKeepsTheCursorVisibleWhileShortened is the other half: the
// window survives the loan, but not at the price of the row the cursor is on. A
// cursor near the bottom of the old window is outside the shortened one, and the
// one window authority is what moves it back into view.
func TestHostedBudgetKeepsTheCursorVisibleWhileShortened(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)
	before := model.ViewScrollOffset

	model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight-hostBudgetPanelRows)
	viewport := model.ComputeOutlinerViewport(model.Width(), 0)
	if model.Cursor < viewport.startIdx || model.Cursor >= viewport.endIdx {
		t.Fatalf("the shortened window [%d,%d) does not draw the cursor on row %d",
			viewport.startIdx, viewport.endIdx, model.Cursor)
	}
	if model.ViewScrollOffset <= before {
		t.Fatalf("offset %d did not move for a window %d rows shorter, so the clamp was not what was measured",
			model.ViewScrollOffset, hostBudgetPanelRows)
	}
	if model.ViewScrollOffset != model.Cursor-viewport.VisibleRows+1 {
		t.Fatalf("offset %d is further than keeping the cursor visible needs (%d)",
			model.ViewScrollOffset, model.Cursor-viewport.VisibleRows+1)
	}
}

// TestHostedBudgetSuccessiveHeightsAnchorToOneLoan is the repeated-resizing
// case: a panel that grows, shrinks and grows again is one loan, so each budget
// is measured against the window the first one borrowed from rather than against
// whatever the previous budget's clamp settled on.
func TestHostedBudgetSuccessiveHeightsAnchorToOneLoan(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)
	before := model.ViewScrollOffset

	for _, panel := range []int{6, 14, 20, 8, 14} {
		model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight-panel)
		model.ComputeOutlinerViewport(model.Width(), 0)
	}
	model.ReleaseHostedBudget(hostBudgetWidth, hostBudgetHeight)
	model.ComputeOutlinerViewport(model.Width(), 0)

	if model.ViewScrollOffset != before {
		t.Fatalf("five height changes settled at offset %d, want the %d the first one borrowed from",
			model.ViewScrollOffset, before)
	}
}

// TestHostedBudgetFollowsItsAnchorRowThroughAChangedProjection is why the anchor
// is a qualified identity and not a row count. Work committed from the panel
// above lands in the projection underneath it, and the view the user gets back
// is the one they were reading — the same task on the same screen row — rather
// than the same numbered slice of a document that grew.
func TestHostedBudgetFollowsItsAnchorRowThroughAChangedProjection(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)
	before := model.ViewScrollOffset
	anchorIdentity := model.TaskIdentityForRow(model.Rows[before])
	if !anchorIdentity.Valid() {
		t.Fatalf("row %d is not a task row, so the fixture does not exercise the anchor", before)
	}

	model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight-hostBudgetPanelRows)
	model.ComputeOutlinerViewport(model.Width(), 0)

	// One task captured ahead of everything the window was showing: every row
	// below it is one further down the document than the loan recorded.
	captured := renderFixtureTask(t, "captured", 900, "Task captured while the panel was open", models.StateBacklog, 0, models.StateBacklog)
	model.Project.Tasks = append([]*models.Task{captured}, model.Project.Tasks...)
	model.Project.BuildTaskTree()
	model.RebuildRows()
	model.MoveCursorToIdentity(model.CurrentTaskIdentity())

	model.ReleaseHostedBudget(hostBudgetWidth, hostBudgetHeight)
	model.ComputeOutlinerViewport(model.Width(), 0)

	if got := model.TaskIdentityForRow(model.Rows[model.ViewScrollOffset]); got != anchorIdentity {
		t.Fatalf("the release seated the window on %+v, want the row it was anchored to %+v", got, anchorIdentity)
	}
	if model.ViewScrollOffset == before {
		t.Fatalf("offset %d is the row number rather than the row: the projection grew above it", before)
	}
}

// TestHostedBudgetAnchorSurvivesARowItCannotFind is the ladder's second rung.
// With the anchor row gone from the projection the cursor is the position worth
// keeping, so the row the user was standing on comes back to the screen row it
// was on.
func TestHostedBudgetAnchorSurvivesARowItCannotFind(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)
	before := model.ViewScrollOffset
	cursorRow := model.Cursor - before
	cursorIdentity := model.CurrentTaskIdentity()

	model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight-hostBudgetPanelRows)
	model.ComputeOutlinerViewport(model.Width(), 0)

	// The anchor row itself leaves, which is what a completion or a move
	// committed from the panel does to the row a window happened to start on.
	anchored := model.Rows[before].Task.ID
	remaining := make([]*models.Task, 0, len(model.Project.Tasks))
	for _, task := range model.Project.Tasks {
		if task.ID != anchored {
			remaining = append(remaining, task)
		}
	}
	model.Project.Tasks = remaining
	model.Project.BuildTaskTree()
	model.RebuildRows()
	model.MoveCursorToIdentity(cursorIdentity)

	model.ReleaseHostedBudget(hostBudgetWidth, hostBudgetHeight)
	model.ComputeOutlinerViewport(model.Width(), 0)

	if got := model.Cursor - model.ViewScrollOffset; got != cursorRow {
		t.Fatalf("the release drew the cursor on screen row %d, want the %d it was on", got, cursorRow)
	}
}

// TestHostedBudgetLeavesEveryOtherAxisAlone states what the seam may not touch.
// A loan is a height change, so the narrowing, the folds, the scope, the
// selection and the rows on the other side of it are the ones the user left
// standing: nothing here is a state rollback.
func TestHostedBudgetLeavesEveryOtherAxisAlone(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)

	model.SetSearchQuery("Task 09")
	model.Folds.setSection("backlog", true)
	identity := model.CurrentTaskIdentity()
	model.ToggleCursorSelection()
	rows := len(model.Rows)

	model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight-hostBudgetPanelRows)
	model.ComputeOutlinerViewport(model.Width(), 0)
	model.ReleaseHostedBudget(hostBudgetWidth, hostBudgetHeight)
	model.ComputeOutlinerViewport(model.Width(), 0)

	if got := model.SearchQuery(); got != "Task 09" {
		t.Fatalf("the round trip left the query as %q", got)
	}
	if !model.Folds.CollapsedSections["backlog"] {
		t.Fatal("the round trip reopened a collapsed section")
	}
	if !model.IsTaskIdentitySelected(identity) {
		t.Fatal("the round trip dropped the visual selection")
	}
	if len(model.Rows) != rows {
		t.Fatalf("the round trip rebuilt the projection: %d rows, want %d", len(model.Rows), rows)
	}
}

// TestTerminalResizeEndsTheLoanRatherThanRepayingIt is the line between the two
// acts. A terminal that changed size while a panel was open is showing a
// different amount of the document, and the window it dropped stays dropped: the
// release that follows repays the loan the new size took out, not the one the
// old size did.
func TestTerminalResizeEndsTheLoanRatherThanRepayingIt(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)
	before := model.ViewScrollOffset

	model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight-hostBudgetPanelRows)
	model.ComputeOutlinerViewport(model.Width(), 0)

	// The shell's own order on a resize: every view is sized, and the panel's
	// host is then handed the rows the panel leaves it at the new size.
	model.SetSize(hostBudgetWidth, hostBudgetHeight+10)
	if model.ViewScrollOffset != 0 {
		t.Fatalf("a terminal resize left offset %d, which is not the drop SetSize declares", model.ViewScrollOffset)
	}
	model.SetHostedBudget(hostBudgetWidth, hostBudgetHeight+10-hostBudgetPanelRows)
	resized := model.ComputeOutlinerViewport(model.Width(), 0)
	if model.Cursor < resized.startIdx || model.Cursor >= resized.endIdx {
		t.Fatalf("the resized window [%d,%d) does not draw the cursor on row %d",
			resized.startIdx, resized.endIdx, model.Cursor)
	}

	model.ReleaseHostedBudget(hostBudgetWidth, hostBudgetHeight+10)
	viewport := model.ComputeOutlinerViewport(model.Width(), 0)

	if model.ViewScrollOffset == before {
		t.Fatalf("the release put back offset %d, which the terminal resize had already dropped", before)
	}
	// The loan the release repays is the one the resized terminal took out, and
	// that one started at the top: what comes back is a window scrolled no
	// further than drawing the cursor needs.
	if model.ViewScrollOffset != model.Cursor-viewport.VisibleRows+1 {
		t.Fatalf("the release settled at offset %d, want the %d a window taken out at the top clamps to",
			model.ViewScrollOffset, model.Cursor-viewport.VisibleRows+1)
	}
}

// TestReleaseWithNoLoanIsAnOrdinaryResize keeps the fallback honest: a view told
// to take its whole area back without ever having lent any is being resized, and
// answers the way SetSize does.
func TestReleaseWithNoLoanIsAnOrdinaryResize(t *testing.T) {
	model := hostBudgetFixture(t)
	seatHostBudgetWindow(t, model)

	model.ReleaseHostedBudget(hostBudgetWidth, hostBudgetHeight)
	if model.ViewScrollOffset != 0 {
		t.Fatalf("a release with no loan behind it kept offset %d", model.ViewScrollOffset)
	}
}
