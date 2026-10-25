package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
)

// TestDefaultOrderIsPriorityThenManual holds the default tree order and the J/K
// window it implies: priority leads, stored order breaks ties, and a reorder
// only trades places inside one priority band, since a swap across bands would
// rewrite stored order without moving a row.
func TestDefaultOrderIsPriorityThenManual(t *testing.T) {
	first := primaryRowsTask("first-high", models.StateBacklog)
	first.Priority = models.PriorityHigh
	low := primaryRowsTask("low", models.StateBacklog)
	low.Priority = models.PriorityLow
	second := primaryRowsTask("second-high", models.StateBacklog)
	second.Priority = models.PriorityHigh
	unset := primaryRowsTask("unset", models.StateBacklog)
	project := primaryRowsProject("alpha", first, low, second, unset)

	outliner, _ := primaryRowsOutliner(project)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	if got, want := visibleTaskIDs(outliner), []string{first.ID, second.ID, low.ID, unset.ID}; !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}

	outliner.MoveCursorToTask(second.ID)
	siblings := outliner.visibleSiblings(outliner.Cursor)
	ids := make([]string, 0, len(siblings))
	for _, task := range siblings {
		ids = append(ids, task.ID)
	}
	if want := []string{first.ID, second.ID}; !slices.Equal(ids, want) {
		t.Fatalf("reorder window = %v, want the priority-1 band %v", ids, want)
	}

	outliner.ReorderTask(1)
	if got, want := visibleTaskIDs(outliner), []string{first.ID, second.ID, low.ID, unset.ID}; !slices.Equal(got, want) {
		t.Fatalf("J across a band boundary moved rows to %v, want %v", got, want)
	}
	outliner.ReorderTask(-1)
	if got, want := visibleTaskIDs(outliner), []string{second.ID, first.ID, low.ID, unset.ID}; !slices.Equal(got, want) {
		t.Fatalf("K inside the band gave rows %v, want %v", got, want)
	}
}

// prioritizedTask seeds a task at a priority, where 0 leaves it unset.
func prioritizedTask(id string, state models.WorkflowState, priority int) *models.Task {
	task := primaryRowsTask(id, state)
	task.Priority = priority
	return task
}

// sectionTaskIDs names the qualified task rows a section drew, in render order.
func sectionTaskIDs(m *OutlinerModel, in func(OutlinerRow) bool) []string {
	var ids []string
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && in(row) {
			ids = append(ids, row.ProjectID+"/"+row.Task.ID)
		}
	}
	return ids
}

// sectionProjectOrder names the project blocks a section drew, in render order.
func sectionProjectOrder(m *OutlinerModel, in func(OutlinerRow) bool) []string {
	var order []string
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && in(row) && !slices.Contains(order, row.ProjectID) {
			order = append(order, row.ProjectID)
		}
	}
	return order
}

func inDoingLane(row OutlinerRow) bool {
	return row.Group.SectionKey == stateGroupCollapsePrefix+string(models.StateGroupDoing)
}

// TestLifecycleLaneRanksProjectBlocksByTheirBestTask holds that a lane's
// project blocks follow the active order, the project order breaking a tie, and
// that an explicit descending sort reverses them.
func TestLifecycleLaneRanksProjectBlocksByTheirBestTask(t *testing.T) {
	alpha := primaryRowsProject("alpha", prioritizedTask("alpha-unset", models.StateDoing, 0))
	beta := primaryRowsProject("beta", prioritizedTask("beta-p2", models.StateDoing, 2))
	gamma := primaryRowsProject("gamma", prioritizedTask("gamma-p1", models.StateDoing, 1))
	zeta := primaryRowsProject("zeta",
		prioritizedTask("zeta-p3", models.StateDoing, 3),
		prioritizedTask("zeta-p1", models.StateDoing, 1))

	outliner, _ := primaryRowsOutliner(alpha, beta, gamma, zeta)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	if got, want := sectionProjectOrder(outliner, inDoingLane), []string{"gamma", "zeta", "beta", "alpha"}; !slices.Equal(got, want) {
		t.Fatalf("Doing blocks = %v, want %v", got, want)
	}
	if got, want := sectionTaskIDs(outliner, inDoingLane), []string{
		"gamma/gamma-p1", "zeta/zeta-p1", "zeta/zeta-p3", "beta/beta-p2", "alpha/alpha-unset",
	}; !slices.Equal(got, want) {
		t.Fatalf("Doing rows = %v, want %v", got, want)
	}

	outliner.Sort = query.SortState{Field: query.SortByPriority, Ascending: false}
	outliner.RebuildRows()
	if got, want := sectionProjectOrder(outliner, inDoingLane), []string{"alpha", "zeta", "beta", "gamma"}; !slices.Equal(got, want) {
		t.Fatalf("Doing blocks under o! descending = %v, want %v", got, want)
	}
}

// TestInboxRanksProjectBlocksByTheirBestTask covers the tree-built modes, which
// rank a block by the best task anywhere in its tree.
func TestInboxRanksProjectBlocksByTheirBestTask(t *testing.T) {
	alpha := primaryRowsProject("alpha", prioritizedTask("alpha-p3", models.StateInbox, 3))
	beta := primaryRowsProject("beta",
		prioritizedTask("beta-unset", models.StateInbox, 0),
		prioritizedTask("beta-p1", models.StateInbox, 1))

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeInbox)
	if got, want := projectGroupOrder(outliner), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Fatalf("Inbox blocks = %v, want %v", got, want)
	}
}

// TestPlanningSectionsOrderByPriorityThenPlanningOrder holds Today and Next in
// both forms: the All lanes and the dedicated modes.
func TestPlanningSectionsOrderByPriorityThenPlanningOrder(t *testing.T) {
	for _, kind := range []struct {
		name    string
		mode    chrome.OutlinerModeID
		inLane  func(OutlinerRow) bool
		setPlan func(*OutlinerModel, []config.ProjectTaskLocator)
	}{
		{"Today", chrome.ModeToday, func(row OutlinerRow) bool { return row.IsTodaySection },
			func(m *OutlinerModel, order []config.ProjectTaskLocator) { m.Runtime.Planning().SetToday(order) }},
		{"Next", chrome.ModeNext, func(row OutlinerRow) bool { return row.IsNextSection },
			func(m *OutlinerModel, order []config.ProjectTaskLocator) { m.Runtime.Planning().SetNext(order) }},
	} {
		t.Run(kind.name, func(t *testing.T) {
			alphaUnset := prioritizedTask("alpha-unset", models.StateBacklog, 0)
			alphaP1 := prioritizedTask("alpha-p1", models.StateBacklog, 1)
			betaP1 := prioritizedTask("beta-p1", models.StateBacklog, 1)
			betaP1Late := prioritizedTask("beta-p1-late", models.StateBacklog, 1)
			betaP2 := prioritizedTask("beta-p2", models.StateBacklog, 2)
			alpha := primaryRowsProject("alpha", alphaUnset, alphaP1)
			beta := primaryRowsProject("beta", betaP1, betaP1Late, betaP2)
			outliner, _ := primaryRowsOutliner(alpha, beta)
			kind.setPlan(outliner, []config.ProjectTaskLocator{
				{ProjectID: alpha.ID, TaskID: alphaUnset.ID},
				{ProjectID: beta.ID, TaskID: betaP2.ID},
				{ProjectID: beta.ID, TaskID: betaP1Late.ID},
				{ProjectID: beta.ID, TaskID: betaP1.ID},
				{ProjectID: alpha.ID, TaskID: alphaP1.ID},
			})
			// Priority leads and planning order breaks the beta-p1 tie; both blocks
			// lead with a p1, so the project order breaks theirs.
			want := []string{
				"alpha/alpha-p1", "alpha/alpha-unset",
				"beta/beta-p1-late", "beta/beta-p1", "beta/beta-p2",
			}

			outliner.SelectOutlinerMode(chrome.ModeAll)
			if got := sectionTaskIDs(outliner, kind.inLane); !slices.Equal(got, want) {
				t.Fatalf("All %s lane = %v, want %v", kind.name, got, want)
			}
			outliner.SelectOutlinerMode(kind.mode)
			if got := sectionTaskIDs(outliner, kind.inLane); !slices.Equal(got, want) {
				t.Fatalf("%s mode = %v, want %v", kind.name, got, want)
			}

			// J/K stays in the cursor's band: beta-p1-late trades only with
			// beta-p1, and beta-p1 has no peer below it.
			order := outliner.globalTodayOrder()
			if kind.mode == chrome.ModeNext {
				order = outliner.globalNextOrder()
			}
			outliner.MoveCursorToProjectTask(betaP1Late.ID, beta.ID)
			if got := outliner.visiblePlanningMembers(order, 1); len(got) != 1 || got[0].TaskID != betaP1.ID {
				t.Fatalf("planning run below beta-p1-late = %v, want its one band peer", got)
			}
			outliner.MoveCursorToProjectTask(betaP1.ID, beta.ID)
			if got := outliner.visiblePlanningMembers(order, 1); len(got) != 0 {
				t.Fatalf("planning run below beta-p1 = %v, want none across the band", got)
			}
		})
	}
}

// TestReviewAndPinnedFollowTheActiveOrder holds the two promoted lanes that
// used to keep their own order.
func TestReviewAndPinnedFollowTheActiveOrder(t *testing.T) {
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	older := now.Add(-time.Hour)
	pinnedNewUnset := prioritizedTask("pinned-new-unset", models.StateBacklog, 0)
	pinnedNewUnset.Pinned, pinnedNewUnset.PinnedAt = true, &now
	pinnedOldP1 := prioritizedTask("pinned-old-p1", models.StateBacklog, 1)
	pinnedOldP1.Pinned, pinnedOldP1.PinnedAt = true, &older
	pinnedOldUnset := prioritizedTask("pinned-old-unset", models.StateBacklog, 0)
	pinnedOldUnset.Pinned, pinnedOldUnset.PinnedAt = true, &older
	reviewUnset := prioritizedTask("review-unset", models.StateBacklog, 0)
	reviewUnset.Review = true
	reviewP2 := prioritizedTask("review-p2", models.StateBacklog, 2)
	reviewP2.Review = true
	alpha := primaryRowsProject("alpha", pinnedNewUnset, pinnedOldP1, pinnedOldUnset, reviewUnset, reviewP2)

	outliner, _ := primaryRowsOutliner(alpha)
	outliner.SelectOutlinerMode(chrome.ModeAll)
	inPinned := func(row OutlinerRow) bool { return row.IsPinnedSection }
	if got, want := sectionTaskIDs(outliner, inPinned), []string{
		"alpha/pinned-old-p1", "alpha/pinned-new-unset", "alpha/pinned-old-unset",
	}; !slices.Equal(got, want) {
		t.Fatalf("Pinned = %v, want priority first and pin order breaking ties", got)
	}
	inReview := func(row OutlinerRow) bool { return row.PromotedTag == reviewLaneKey }
	if got, want := sectionTaskIDs(outliner, inReview), []string{"alpha/review-p2", "alpha/review-unset"}; !slices.Equal(got, want) {
		t.Fatalf("Review = %v, want %v", got, want)
	}

	outliner.MoveCursorToTask(pinnedNewUnset.ID)
	if got := outliner.visiblePinnedSiblings(-1); len(got) != 0 {
		t.Fatalf("pinned siblings above pinned-new-unset = %v, want none across the band", got)
	}
	if got := outliner.visiblePinnedSiblings(1); len(got) != 1 {
		t.Fatalf("pinned siblings below pinned-new-unset = %v, want its one band peer", got)
	}

	outliner.Sort = query.SortState{Field: query.SortByTitle, Ascending: false}
	outliner.RebuildRows()
	if got, want := sectionTaskIDs(outliner, inReview), []string{"alpha/review-unset", "alpha/review-p2"}; !slices.Equal(got, want) {
		t.Fatalf("Review under title descending = %v, want %v", got, want)
	}
}

// TestDueBreaksASameDayTieByPriority holds the Due tiebreak and that it reaches
// the block order.
func TestDueBreaksASameDayTieByPriority(t *testing.T) {
	alphaTask := groupingDueTask("alpha-p2", 0)
	alphaTask.Priority = 2
	betaTask := groupingDueTask("beta-p1", 0)
	betaTask.Priority = 1
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeDue)
	if got, want := projectGroupOrder(outliner), []string{"beta", "alpha"}; !slices.Equal(got, want) {
		t.Fatalf("Due blocks = %v, want the p1 block first", got)
	}
}
