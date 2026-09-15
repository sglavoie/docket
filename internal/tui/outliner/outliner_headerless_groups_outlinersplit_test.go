package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The contract these tests hold is that folding, navigation, group cursor
// restoration, and project targeting read the OutlinerRowGroup every row carries
// rather than the heading above it. There is now no heading over an expanded
// group to read instead: the project rides on each row's own compact label, the
// folded group's summary is the only project row the list draws, and every one
// of these keys is expected to answer from the group regardless.

// headerlessOutliner is newRowGroupFixture's cross-project All view, so a test
// names only the behavior it is about.
func headerlessOutliner(t *testing.T) (*OutlinerModel, rowGroupFixture) {
	t.Helper()
	fixture := newRowGroupFixture()
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	return outliner, fixture
}

func projectHeaderRows(m *OutlinerModel) []OutlinerRow {
	headers := make([]OutlinerRow, 0)
	for _, row := range m.Rows {
		if row.IsProjectHeader && row.IsCollapsibleHeader() {
			headers = append(headers, row)
		}
	}
	return headers
}

func cursorRow(t *testing.T, m *OutlinerModel) OutlinerRow {
	t.Helper()
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		t.Fatalf("cursor %d is outside the %d rows", m.Cursor, len(m.Rows))
	}
	return m.Rows[m.Cursor]
}

// TestProjectGroupsDrawAHeaderOnlyWhileFolded is the presentation claim the rest
// of the file leans on: an expanded group spends no row on a heading, a folded
// one is the summary that names it, and neither changes the group the rows
// carry.
func TestProjectGroupsDrawAHeaderOnlyWhileFolded(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)

	if headers := projectHeaderRows(outliner); len(headers) != 0 {
		t.Fatalf("expanded groups drew headers %+v, want none", headers)
	}
	for _, id := range []string{fixture.alphaBacklog.ID, fixture.alphaWaiting.ID, fixture.betaBacklog.ID} {
		if group := rowGroupFor(t, outliner, id); !group.ProjectGrouped() {
			t.Fatalf("task %q row carries group %+v, want a project half", id, group)
		}
	}

	group := rowGroupFor(t, outliner, fixture.alphaBacklog.ID)
	outliner.SetProjectGroupFolded(group, true)
	outliner.RebuildRows()
	if _, found := rowForTaskID(outliner, fixture.alphaBacklog.ID); found {
		t.Fatal("the folded group kept its task rows")
	}
	summaries := projectHeaderRows(outliner)
	if len(summaries) != 1 || summaries[0].Group != group {
		t.Fatalf("folded headers = %+v, want the one summary for %+v", summaries, group)
	}
}

// TestGroupStartsAreWhatAdjacentHeaderNavigationSteps holds n/N to the blocks
// rather than to the headings: every group is reachable, in row order, and the
// walk lands on the first row of a group that draws no heading at all.
func TestGroupStartsAreWhatAdjacentHeaderNavigationSteps(t *testing.T) {
	outliner, _ := headerlessOutliner(t)
	outliner.Cursor = 0

	visited := make([]OutlinerRowGroup, 0)
	seats := make([]int, 0)
	// One more step than there are rows, so the walk always ends on the wrap
	// back onto a seat it has already taken rather than on the row budget.
	for range len(outliner.Rows) + 1 {
		if _, ok := outliner.MoveCursorToAdjacentHeader(1); !ok {
			t.Fatal("n reported no group to step to")
		}
		if slices.Contains(seats, outliner.Cursor) {
			break
		}
		seats = append(seats, outliner.Cursor)
		row := cursorRow(t, outliner)
		if !outliner.rowStartsGroup(outliner.Cursor) {
			t.Fatalf("n parked on row %+v, which starts no block", row)
		}
		if row.Group.ProjectGrouped() {
			visited = append(visited, row.Group)
		}
	}

	want := outliner.VisibleProjectGroups()
	if len(want) == 0 {
		t.Fatal("the fixture reported no project groups to visit")
	}
	for _, group := range want {
		if !slices.Contains(visited, group) {
			t.Fatalf("a full pass of n never reached group %+v; it reached %+v", group, visited)
		}
	}

	// N retraces the same seats in reverse, which is what makes the wrap a walk
	// rather than two independent orders.
	back := make([]int, 0, len(seats))
	for range seats {
		if _, ok := outliner.MoveCursorToAdjacentHeader(-1); !ok {
			t.Fatal("N reported no group to step to")
		}
		back = append(back, outliner.Cursor)
	}
	slices.Reverse(back)
	if !slices.Equal(back, seats) {
		t.Fatalf("N retraced %v, want the reverse of n's %v", back, seats)
	}
}

// TestGroupStartIsTheFirstTaskNotTheHeading names the one visible difference from
// the heading form: with no heading drawn, the block starts on its first task,
// and the row after it is not a second start of the same block.
func TestGroupStartIsTheFirstTaskNotTheHeading(t *testing.T) {
	alphaSecond := primaryRowsTask("alpha-second", models.StateBacklog)
	fixture := newRowGroupFixture()
	fixture.alpha = primaryRowsProject("alpha", fixture.alphaBacklog, alphaSecond, fixture.alphaWaiting)
	outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	starts := make([]string, 0)
	for index, row := range outliner.Rows {
		if !outliner.rowStartsGroup(index) || !row.IsTaskRow() || row.Task == nil {
			continue
		}
		starts = append(starts, row.Task.ID)
	}
	if !slices.Contains(starts, fixture.alphaBacklog.ID) {
		t.Fatalf("group starts = %v, want alpha's first Backlog task among them", starts)
	}
	if slices.Contains(starts, alphaSecond.ID) {
		t.Fatalf("group starts = %v, want alpha's second task to continue the block it is in", starts)
	}
}

// TestGroupFoldFromATaskRowReturnsTheCursorToIt is h and l as one round trip: h
// on a task that has run out of hierarchy folds the group under it, and l on the
// summary puts the cursor back on the very task the fold hid.
func TestGroupFoldFromATaskRowReturnsTheCursorToIt(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)
	second := primaryRowsTask("alpha-second", models.StateBacklog)
	outliner.Population.Projects = []*models.Project{
		primaryRowsProject("alpha", fixture.alphaBacklog, second, fixture.alphaWaiting),
		fixture.beta,
	}
	outliner.Population.Invalidate()
	outliner.RebuildRows()

	focus := taskref.TaskIdentity{ProjectID: "alpha", TaskID: second.ID}
	if !outliner.MoveCursorToIdentity(focus) {
		t.Fatal("the second Backlog task has no row to stand on")
	}
	group := rowGroupFor(t, outliner, second.ID)

	if !outliner.CollapseOrMoveToParent() {
		t.Fatal("h on a root task reported no fold")
	}
	if !outliner.IsProjectGroupFolded(group) {
		t.Fatalf("h left group %+v expanded", group)
	}
	if row := cursorRow(t, outliner); !row.IsProjectHeader || row.Group != group {
		t.Fatalf("cursor after h = %+v, want the summary for %+v", row, group)
	}

	if !outliner.ExpandAtCursor() {
		t.Fatal("l on the summary reported no expansion")
	}
	if outliner.IsProjectGroupFolded(group) {
		t.Fatalf("l left group %+v folded", group)
	}
	if got := cursorTaskID(outliner); got != second.ID {
		t.Fatalf("cursor after l = %q, want the remembered %q", got, second.ID)
	}
	if _, remembered := outliner.Folds.groupFocus(group.ProjectKey); remembered {
		t.Fatal("the expansion kept a memory that outlives the fold that needed it")
	}
}

// TestExpandingAGroupFallsBackToItsFirstTask is the other half of that rule: a
// fold taken from the group's own summary stands on no task, so the expansion
// has nothing to return to and seats the cursor on the group's first task rather
// than on the row the fold left behind.
func TestExpandingAGroupFallsBackToItsFirstTask(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)
	group := rowGroupFor(t, outliner, fixture.alphaBacklog.ID)
	outliner.SetProjectGroupFolded(group, true)
	outliner.RebuildRows()
	if !outliner.moveCursorToGroupKey(group.ProjectKey) {
		t.Fatalf("the folded group %+v drew no summary to stand on", group)
	}

	if !outliner.ExpandAtCursor() {
		t.Fatal("l on the summary reported no expansion")
	}
	if got := cursorTaskID(outliner); got != fixture.alphaBacklog.ID {
		t.Fatalf("cursor after l = %q, want the group's first task %q", got, fixture.alphaBacklog.ID)
	}
}

// TestGroupFoldRefusesWhereThereIsNoGroup keeps the new h rung to the axis it
// belongs to: a single-project form carries no project half, so a root task
// there stays as silent as it was and p still refuses.
func TestGroupFoldRefusesWhereThereIsNoGroup(t *testing.T) {
	fixture := newRowGroupFixture()
	scoped, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
	scoped.PushProjectScope(fixture.alpha.ID)
	scoped.SelectOutlinerMode(chrome.ModeAll)

	if !scoped.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: fixture.alpha.ID, TaskID: fixture.alphaBacklog.ID}) {
		t.Fatal("the scoped view lost the task to stand on")
	}
	before := len(scoped.Rows)
	if scoped.CollapseOrMoveToParent() {
		t.Fatal("h folded something in a projection that owns no project group")
	}
	if len(scoped.Rows) != before {
		t.Fatalf("a refused h reshaped the projection: %d rows, want %d", len(scoped.Rows), before)
	}
	if scoped.ToggleVisibleProjectGroups() {
		t.Fatal("p reported a toggle in a projection that owns no project group")
	}
}

// TestCollapsedSummaryStillNamesItsProjectHeaderless is the project-targeting
// half: with no heading over an expanded group, the summary a folded one draws
// is what e, #, *, C, ; and Space resolve from, and it answers by the row's own
// project rather than by the view's.
func TestCollapsedSummaryStillNamesItsProjectHeaderless(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)
	if outliner.CursorProjectHeaderProject() != nil {
		t.Fatal("an expanded group offered a project header target")
	}

	group := rowGroupFor(t, outliner, fixture.betaBacklog.ID)
	outliner.SetProjectGroupFolded(group, true)
	outliner.RebuildRows()
	if !outliner.moveCursorToGroupKey(group.ProjectKey) {
		t.Fatalf("the folded group %+v drew no summary to stand on", group)
	}
	project := outliner.CursorProjectHeaderProject()
	if project == nil || project.ID != fixture.beta.ID {
		t.Fatalf("the summary named project %v, want %q", project, fixture.beta.ID)
	}
}

// TestBulkProjectGroupToggleSeatsTheCursorByGroup is p over the headerless form:
// the fold lands the cursor on its own group's summary and the expansion returns
// it to the task that summary was standing in for.
func TestBulkProjectGroupToggleSeatsTheCursorByGroup(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)
	focus := taskref.TaskIdentity{ProjectID: fixture.alpha.ID, TaskID: fixture.alphaWaiting.ID}
	if !outliner.MoveCursorToIdentity(focus) {
		t.Fatal("the Waiting task has no row to stand on")
	}
	group := rowGroupFor(t, outliner, fixture.alphaWaiting.ID)

	if !outliner.ToggleVisibleProjectGroups() {
		t.Fatal("p reported no fold over an all-expanded projection")
	}
	for _, other := range outliner.VisibleProjectGroups() {
		if !outliner.IsProjectGroupFolded(other) {
			t.Fatalf("p left group %+v expanded", other)
		}
	}
	if row := cursorRow(t, outliner); !row.IsProjectHeader || row.Group != group {
		t.Fatalf("cursor after p = %+v, want the summary for the cursor's own group %+v", row, group)
	}

	if !outliner.ToggleVisibleProjectGroups() {
		t.Fatal("p reported no expansion over an all-folded projection")
	}
	if got := cursorTaskID(outliner); got != fixture.alphaWaiting.ID {
		t.Fatalf("cursor after the second p = %q, want the remembered %q", got, fixture.alphaWaiting.ID)
	}
}

// TestGroupFoldKeepsASelectionItHides is the hidden-member rule one axis over: a
// project fold is a presentation change, so the explicit selection it covers
// survives it and comes back with the rows.
func TestGroupFoldKeepsASelectionItHides(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)
	hidden := taskref.TaskIdentity{ProjectID: fixture.beta.ID, TaskID: fixture.betaBacklog.ID}
	outliner.MultiSelectMode = true
	outliner.SelectTaskIdentity(hidden)

	group := rowGroupFor(t, outliner, fixture.betaBacklog.ID)
	outliner.SetProjectGroupFolded(group, true)
	outliner.RebuildRows()
	if _, found := rowForTaskID(outliner, fixture.betaBacklog.ID); found {
		t.Fatal("the fold left the selected row on screen, so this proves nothing")
	}
	if outliner.SelectedCount() != 1 || !outliner.IsTaskIdentitySelected(hidden) {
		t.Fatal("the fold dropped the selection it hid")
	}

	outliner.SetProjectGroupFolded(group, false)
	outliner.RebuildRows()
	if outliner.SelectedCount() != 1 || !outliner.IsTaskIdentitySelected(hidden) {
		t.Fatal("the expansion dropped the selection it restored")
	}
}

// TestReorderBoundsStopAtTheGroupHeaderless holds the reorder window to the
// group the cursor's row carries rather than to the heading above it, which is
// what keeps J/K inside one project when no heading separates two.
func TestReorderBoundsStopAtTheGroupHeaderless(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)
	index := -1
	for i, row := range outliner.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == fixture.alphaBacklog.ID {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("alpha's Backlog task has no row")
	}
	siblings := outliner.visibleSiblings(index)
	ids := make([]string, 0, len(siblings))
	for _, task := range siblings {
		ids = append(ids, task.ID)
	}
	if !slices.Equal(ids, []string{fixture.alphaBacklog.ID}) {
		t.Fatalf("reorder window = %v, want alpha's Backlog group alone", ids)
	}
}

// TestGroupNavigationVoidsAPendingRelocationCapture keeps n and N user cursor
// navigation: a step onto another group is a keypress, so the advance a
// committed mutation armed is dropped as it happens rather than re-armed by the
// reload that follows.
func TestGroupNavigationVoidsAPendingRelocationCapture(t *testing.T) {
	outliner, fixture := headerlessOutliner(t)
	focus := taskref.TaskIdentity{ProjectID: fixture.alpha.ID, TaskID: fixture.alphaBacklog.ID}
	if !outliner.MoveCursorToIdentity(focus) {
		t.Fatal("alpha's Backlog task has no row to stand on")
	}
	outliner.RelocationHistory.Pending = &OutlinerRelocationCapture{
		Task:          focus,
		Settled:       true,
		SettledCursor: outliner.Cursor,
	}

	before := outliner.Cursor
	if _, ok := outliner.MoveCursorToAdjacentHeader(1); !ok {
		t.Fatal("n reported no group to step to")
	}
	if outliner.Cursor == before {
		t.Fatal("n moved nothing, so this proves nothing about voiding")
	}
	if outliner.RelocationHistory.Pending != nil {
		t.Fatal("n left the pending relocation capture armed")
	}
}
