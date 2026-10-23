package outliner

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// An expanded cross-project group holding exactly one top-level task draws no
// header: that task's row leads with the project's `[slug]` and keeps the same
// depth as other project tasks. Folding brings the header back and l merges
// it again. These tests hold that presentation and the keys that read the
// group off the row rather than off a header.

// mergedSingletonFixture is All over alpha, whose Backlog lane holds one root
// with a child and whose Waiting lane holds one task, and beta-long, whose
// Backlog lane holds one task: every project group in view is a singleton.
func mergedSingletonFixture(t *testing.T) (*OutlinerModel, *models.Task, *models.Task) {
	t.Helper()
	root := primaryRowsTask("alpha-root", models.StateBacklog)
	child := primaryRowsTask("alpha-child", models.StateBacklog)
	child.ParentID = &root.ID
	alpha := primaryRowsProject("alpha", root, child, primaryRowsTask("alpha-waiting", models.StateWaiting))
	beta := primaryRowsProject("beta-long", primaryRowsTask("beta-backlog", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	return outliner, root, child
}

func collapsibleProjectHeaders(m *OutlinerModel) []OutlinerRow {
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

func mustRowForTask(t *testing.T, m *OutlinerModel, taskID string) (int, OutlinerRow) {
	t.Helper()
	for index, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			return index, row
		}
	}
	t.Fatalf("task %q has no row", taskID)
	return 0, OutlinerRow{}
}

func TestSingletonProjectGroupMergesIntoItsRow(t *testing.T) {
	outliner, root, child := mergedSingletonFixture(t)

	if headers := collapsibleProjectHeaders(outliner); len(headers) != 0 {
		t.Fatalf("expanded singleton groups drew headers %+v, want none", headers)
	}
	leads := 0
	for _, row := range outliner.Rows {
		if row.LeadsMergedGroup {
			leads++
		}
	}
	if groups := outliner.VisibleProjectGroups(); leads != len(groups) {
		t.Fatalf("%d merged rows over %d groups, want one each", leads, len(groups))
	}

	rootIndex, rootRow := mustRowForTask(t, outliner, root.ID)
	if !rootRow.LeadsMergedGroup || rootRow.Prefix != connectorSpace {
		t.Fatalf("root row = %+v, want it to lead the merged group at the task's depth", rootRow)
	}
	_, childRow := mustRowForTask(t, outliner, child.ID)
	if childRow.LeadsMergedGroup || childRow.Prefix != connectorSpace+connectorLastItem {
		t.Fatalf("child row prefix %q leads=%t, want one level beneath its root", childRow.Prefix, childRow.LeadsMergedGroup)
	}
	if childRow.Group != rootRow.Group {
		t.Fatalf("child group %+v, want the merged group %+v", childRow.Group, rootRow.Group)
	}
	if !outliner.rowStartsGroup(rootIndex) {
		t.Fatal("n/N would not stop on the merged row")
	}

	line := ansi.Strip(outliner.RenderOutlinerTaskRow(rootRow, false, 120, rootIndex, nil))
	if !strings.Contains(line, "[alpha] 🞑 alpha-root") {
		t.Fatalf("merged row %q does not lead with its project", line)
	}
	childLine := ansi.Strip(outliner.RenderOutlinerTaskRow(childRow, false, 120, rootIndex+1, nil))
	if strings.Contains(childLine, "[alpha]") {
		t.Fatalf("child row %q repeated the project", childLine)
	}
}

func TestProjectGroupWithTwoRootsKeepsItsHeader(t *testing.T) {
	alpha := primaryRowsProject("alpha",
		primaryRowsTask("alpha-one", models.StateBacklog),
		primaryRowsTask("alpha-two", models.StateBacklog),
	)
	beta := primaryRowsProject("beta", primaryRowsTask("beta-one", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)

	headers := collapsibleProjectHeaders(outliner)
	if len(headers) != 1 || headers[0].ProjectID != alpha.ID || headers[0].SectionCount != 2 {
		t.Fatalf("headers = %+v, want alpha's (2) alone", headers)
	}
	for _, id := range []string{"alpha-one", "alpha-two"} {
		if _, row := mustRowForTask(t, outliner, id); row.LeadsMergedGroup || row.Prefix == "" {
			t.Fatalf("row %q = %+v, want it beneath alpha's header", id, row)
		}
	}
	if _, row := mustRowForTask(t, outliner, "beta-one"); !row.LeadsMergedGroup {
		t.Fatalf("beta's one task did not merge: %+v", row)
	}
}

func TestSingletonProjectLabelAlignsWithProjectHeaders(t *testing.T) {
	alpha := primaryRowsProject("alpha",
		primaryRowsTask("alpha-one", models.StateBacklog),
		primaryRowsTask("alpha-two", models.StateBacklog),
	)
	beta := primaryRowsProject("beta", primaryRowsTask("beta-one", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	for _, pins := range [][2]bool{{false, false}, {true, true}, {true, false}, {false, true}} {
		alpha.Pinned, beta.Pinned = pins[0], pins[1]
		for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeBacklog} {
			outliner.SelectOutlinerMode(mode)
			for _, width := range []int{40, 80, 120} {
				for _, selected := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/pinned=%v/width=%d/selected=%t", mode, pins, width, selected), func(t *testing.T) {
						layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, width)
						header := collapsibleProjectHeaders(outliner)[0]
						headerLine := ansi.Strip(outliner.RenderOutlinerProjectHeaderRow(header, selected, width, 0))
						index, row := mustRowForTask(t, outliner, "beta-one")
						line := ansi.Strip(outliner.RenderOutlinerTaskRow(row, selected, width, index, layout))
						label := strings.Index(line, "[beta]")
						want := strings.Index(headerLine, "[alpha]")
						if label < 0 || want < 0 || ansi.StringWidth(line[:label]) != ansi.StringWidth(headerLine[:want]) {
							t.Fatalf("project labels are misaligned: header %q, task %q", headerLine, line)
						}
						badge := strings.TrimSpace(ansi.Strip(outliner.renderTaskStatusIcon(row.Task)))
						if strings.Index(line, badge) <= label {
							t.Fatalf("checkbox must follow the project: %q", line)
						}
					})
				}
			}
		}
	}
}

func TestMergedRowFoldsToItsHeaderAndMergesBack(t *testing.T) {
	outliner, root, _ := mergedSingletonFixture(t)
	identity := taskref.TaskIdentity{ProjectID: "beta-long", TaskID: "beta-backlog"}
	if !outliner.MoveCursorToIdentity(identity) {
		t.Fatal("beta's task has no row")
	}
	group := rowGroupFor(t, outliner, identity.TaskID)

	if !outliner.CollapseOrMoveToParent() {
		t.Fatal("h on a merged row reported no fold")
	}
	if !outliner.IsProjectGroupFolded(group) {
		t.Fatalf("h left group %+v expanded", group)
	}
	if row := cursorRow(t, outliner); !row.IsProjectHeader || row.Group != group || row.SectionCount != 1 {
		t.Fatalf("cursor after h = %+v, want the (1) summary for %+v", row, group)
	}

	if !outliner.ExpandAtCursor() {
		t.Fatal("l on the summary reported no expansion")
	}
	if row := cursorRow(t, outliner); !row.LeadsMergedGroup || row.Task == nil || row.Task.ID != identity.TaskID {
		t.Fatalf("cursor after l = %+v, want the merged row again", row)
	}
	for _, header := range collapsibleProjectHeaders(outliner) {
		if header.Group == group {
			t.Fatal("l left the singleton's header drawn")
		}
	}

	// A merged root with children folds them first: the group is h's last rung.
	if !outliner.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: "alpha", TaskID: root.ID}) {
		t.Fatal("alpha's root has no row")
	}
	rootGroup := rowGroupFor(t, outliner, root.ID)
	if !outliner.CollapseOrMoveToParent() || outliner.IsProjectGroupFolded(rootGroup) {
		t.Fatal("the first h folded the group rather than the root's children")
	}
	if !outliner.CollapseOrMoveToParent() || !outliner.IsProjectGroupFolded(rootGroup) {
		t.Fatal("the second h did not fold the merged group")
	}
}

func TestToggleProjectGroupsFoldsSingletonsToo(t *testing.T) {
	outliner, _, _ := mergedSingletonFixture(t)
	groups := outliner.VisibleProjectGroups()

	if !outliner.ToggleVisibleProjectGroups() {
		t.Fatal("p reported no toggle")
	}
	if headers := collapsibleProjectHeaders(outliner); len(headers) != len(groups) {
		t.Fatalf("p drew %d summaries over %d groups", len(headers), len(groups))
	}
	if !outliner.ToggleVisibleProjectGroups() {
		t.Fatal("the second p reported no toggle")
	}
	if headers := collapsibleProjectHeaders(outliner); len(headers) != 0 {
		t.Fatalf("the second p left summaries %+v drawn", headers)
	}
}

// A long label elsewhere in the list must not create a gap between a merged
// row's own label and its state badge.
func TestMergedRowLabelsUseOnlyTheirOwnWidth(t *testing.T) {
	outliner, root, _ := mergedSingletonFixture(t)
	_, beta := mustRowForTask(t, outliner, "beta-backlog")
	beta.Task.Context = "mobile-app"
	for _, width := range []int{60, 80, 120, 180} {
		layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, width)
		index, row := mustRowForTask(t, outliner, root.ID)
		line := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, width, index, layout))
		if !strings.Contains(line, "[alpha] 🞑 alpha-root") {
			t.Fatalf("width %d: merged row reserves another task's label width: %q", width, line)
		}
	}
}

func TestGroupedRowsDoNotReserveSpaceForOtherTasksContexts(t *testing.T) {
	plain := primaryRowsTask("plain", models.StateBacklog)
	contextual := primaryRowsTask("contextual", models.StateBacklog)
	contextual.Context = "mobile-app"
	alpha := primaryRowsProject("alpha", plain, contextual)
	beta := primaryRowsProject("beta", primaryRowsTask("beta-one", models.StateBacklog))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	for _, width := range []int{60, 80, 120, 180} {
		layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, width)
		for _, task := range []*models.Task{plain, contextual} {
			index, row := mustRowForTask(t, outliner, task.ID)
			line := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, width, index, layout))
			want := "🞑 " + task.Title
			if task.Context != "" {
				want = "🞑 " + ansi.Strip(outliner.taskIdentityLabel(row, width)) + " " + task.Title
			}
			if !strings.Contains(line, want) {
				t.Fatalf("width %d: grouped row has an empty metadata gap: %q, want %q", width, line, want)
			}
		}
	}
}

// A merged row's descendants hang beneath its state badge, not beneath the
// project label the row leads with, so they read as its children.
func TestMergedRowDescendantsHangBeneathItsBadge(t *testing.T) {
	outliner, root, child := mergedSingletonFixture(t)
	root.Context = "infra"
	for _, width := range []int{60, 80, 120} {
		layout := outliner.ComputeOutlinerRowLayout(outliner.Rows, width)
		rootIndex, rootRow := mustRowForTask(t, outliner, root.ID)
		childIndex, childRow := mustRowForTask(t, outliner, child.ID)
		if childRow.MergedLead == nil || childRow.MergedLead.ID != root.ID {
			t.Fatalf("child row = %+v, want it to remember its merged lead", childRow)
		}
		parent := ansi.Strip(outliner.RenderOutlinerTaskRow(rootRow, false, width, rootIndex, layout))
		line := ansi.Strip(outliner.RenderOutlinerTaskRow(childRow, false, width, childIndex, layout))
		badge := ansi.StringWidth(parent[:strings.Index(parent, "🞑")])
		connector := ansi.StringWidth(line[:strings.Index(line, connectorLastItem)])
		if connector != badge {
			t.Fatalf("width %d: child connector at column %d, want the lead's badge column %d:\n%s\n%s", width, connector, badge, parent, line)
		}
	}
}

func TestMergedRootChecklistRowsSitBeneathIt(t *testing.T) {
	outliner, _, _ := mergedSingletonFixture(t)
	task := outliner.Population.Projects[1].Tasks[0]
	task.Checklist = []models.ChecklistItem{{Text: "step one"}}
	outliner.Folds.ExpandedChecklists = map[taskref.TaskIdentity]bool{
		{ProjectID: "beta-long", TaskID: task.ID}: true,
	}
	outliner.Population.Invalidate()
	outliner.RebuildRows()

	index, row := mustRowForTask(t, outliner, task.ID)
	if !row.LeadsMergedGroup {
		t.Fatalf("checklist owner = %+v, want it to lead its group", row)
	}
	item := outliner.Rows[index+1]
	if !item.IsChecklistItem || item.LeadsMergedGroup || item.Prefix != connectorSpace+connectorLastItem || item.Group != row.Group {
		t.Fatalf("checklist row = %+v, want it one level beneath its owner in the same group", item)
	}
}

func TestLaunchFocusSeatsOnTheFirstMergedRow(t *testing.T) {
	outliner, _, _ := mergedSingletonFixture(t)
	if !outliner.FocusFirstLaunchRow() {
		t.Fatal("launch focus found no task")
	}
	if row := cursorRow(t, outliner); !row.LeadsMergedGroup {
		t.Fatalf("launch focus seated on %+v, want the first merged row", row)
	}
}

// TestPinnedContextNamesAMergedGroupsProject covers a window opening deep in a
// merged root's subtree: the lane header has scrolled away and no project
// header was ever drawn, yet the pinned row still names both.
func TestPinnedContextNamesAMergedGroupsProject(t *testing.T) {
	root := primaryRowsTask("solo-root", models.StateDoing)
	tasks := []*models.Task{root}
	for index := range 40 {
		child := primaryRowsTask(fmt.Sprintf("solo-child-%02d", index), models.StateDoing)
		child.ParentID = &root.ID
		tasks = append(tasks, child)
	}
	solo := primaryRowsProject("solo", tasks...)
	other := primaryRowsProject("other", primaryRowsTask("other-task", models.StateBacklog))
	model, _ := primaryRowsOutliner(solo, other)
	model.SetSize(pinnedContextWidth, pinnedContextHeight)
	model.SelectOutlinerMode(chrome.ModeAll)
	if _, row := mustRowForTask(t, model, root.ID); !row.LeadsMergedGroup {
		t.Fatalf("the subtree's root did not merge: %+v", row)
	}

	pinnedContextSeat(t, model, "solo-child-30", "solo")
	viewport, lines := pinnedContextLines(model)
	if !viewport.pinned {
		t.Fatalf("a window deep inside the merged subtree reported no pinned row: %+v", viewport)
	}
	if !strings.Contains(lines[0], "Doing") || !strings.Contains(lines[0], "[solo]") {
		t.Fatalf("pinned row %q, want the lane and the merged group's project", lines[0])
	}
}

// TestRelocationRecoveryLandsOnTheRowAGroupMergedInto covers a departure that
// shrinks its group to one task: the header the capture saw is gone after the
// rebuild, and the sweep still lands on the survivor, now leading the group.
func TestRelocationRecoveryLandsOnTheRowAGroupMergedInto(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateInbox)
	alphaTwo := primaryRowsTask("alpha-two", models.StateInbox)
	alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
	beta := primaryRowsProject("beta", primaryRowsTask("beta-one", models.StateInbox))
	outliner := inboxModeOutliner(t, alpha, beta)
	if headers := collapsibleProjectHeaders(outliner); len(headers) != 1 || headers[0].ProjectID != alpha.ID {
		t.Fatalf("headers = %+v, want alpha's alone before the departure", headers)
	}

	cursorToTaskRow(t, outliner, alphaOne.ID)
	departAfterCapture(t, outliner, alphaOne, alpha, models.StateBacklog)

	requireCursorTask(t, outliner, taskref.For(alphaTwo, alpha))
	if row := cursorRow(t, outliner); !row.LeadsMergedGroup {
		t.Fatalf("cursor row = %+v, want the survivor leading its merged group", row)
	}
	if headers := collapsibleProjectHeaders(outliner); len(headers) != 0 {
		t.Fatalf("headers = %+v, want none once both groups hold one task", headers)
	}
}
