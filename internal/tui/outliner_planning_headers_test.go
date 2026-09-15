package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func TestPlanningRowsGroupEachProjectUnderOneGroup(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
	alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
	betaOne := primaryRowsTask("beta-one", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
	beta := primaryRowsProject("beta", betaOne)
	beta.Pinned = true
	outlinerView, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaOne.ID},
		{ProjectID: beta.ID, TaskID: betaOne.ID},
		{ProjectID: alpha.ID, TaskID: alphaTwo.ID},
	})

	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeNext} {
		t.Run(string(mode), func(t *testing.T) {
			outlinerView.SelectOutlinerMode(mode)
			if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{
				betaOne.ID, alphaOne.ID, alphaTwo.ID,
			}) {
				t.Fatalf("Next task order = %v, want rows grouped under pinned-first project headers", got)
			}

			// The lane's groups are read off its rows: an open group draws no
			// heading, so the group each row carries is the whole of what says
			// where one project's run ends and the next begins.
			groups, counts := planningLaneGroups(outlinerView, func(row outliner.OutlinerRow) bool {
				return row.IsNextSection
			})
			if !slices.Equal(groups, []string{beta.ID, alpha.ID}) {
				t.Fatalf("Next project groups = %v, want pinned projects first", groups)
			}
			if counts[alpha.ID] != 2 || counts[beta.ID] != 1 {
				t.Fatalf("Next project counts = %v, want alpha=2 and beta=1", counts)
			}
		})
	}
}

func TestPlanningProjectGroupsCollapseAndDisappearInProjectScope(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
	alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
	betaOne := primaryRowsTask("beta-one", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
	beta := primaryRowsProject("beta", betaOne)
	outlinerView, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaOne.ID},
		{ProjectID: beta.ID, TaskID: betaOne.ID},
		{ProjectID: alpha.ID, TaskID: alphaTwo.ID},
	})
	outlinerView.SelectOutlinerMode(chrome.ModeNext)

	// Folding starts from a task row inside the group, since the group draws no
	// header until the fold it would name is already closed.
	seated := false
	for index, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.IsNextSection && row.Group.ProjectID == alpha.ID {
			outlinerView.Cursor, seated = index, true
			break
		}
	}
	if !seated {
		t.Fatalf("Next carries no alpha row to fold from: %+v", outlinerView.Rows)
	}
	if !outlinerView.CollapseOrMoveToParent() {
		t.Fatal("left on a Next row did not collapse its project group")
	}
	if got := visibleTaskIDs(outlinerView); !slices.Equal(got, []string{betaOne.ID}) {
		t.Fatalf("collapsed Next tasks = %v, want only beta", got)
	}

	outlinerView.PushProjectScope(alpha.ID)
	for _, row := range outlinerView.Rows {
		if row.IsProjectHeader && row.IsNextSection {
			t.Fatalf("project-scoped Next rendered a redundant project header: %+v", row)
		}
	}
}

func TestTodayRowsUseTheSameProjectGroupContract(t *testing.T) {
	alphaTask := primaryRowsTask("alpha-today", models.StateBacklog)
	betaTask := primaryRowsTask("beta-today", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	beta.Pinned = true
	outlinerView, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaTask.ID},
		{ProjectID: beta.ID, TaskID: betaTask.ID},
	})
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	groups, _ := planningLaneGroups(outlinerView, func(row outliner.OutlinerRow) bool {
		return row.IsTodaySection
	})
	if !slices.Equal(groups, []string{beta.ID, alpha.ID}) {
		t.Fatalf("Today project groups = %v, want pinned projects first", groups)
	}
}

// planningLaneGroups returns the project groups one planning lane shows, in row
// order, and how many task rows each holds. A folded group states both on its
// summary and an open one on every row it holds, so both are counted from the
// group the rows carry rather than from a heading that may not be drawn.
func planningLaneGroups(
	view *outliner.OutlinerModel,
	inLane func(outliner.OutlinerRow) bool,
) (groups []string, counts map[string]int) {
	counts = make(map[string]int)
	for _, row := range view.Rows {
		if !inLane(row) || !row.Group.ProjectGrouped() {
			continue
		}
		if !slices.Contains(groups, row.Group.ProjectID) {
			groups = append(groups, row.Group.ProjectID)
		}
		switch {
		case row.IsProjectHeader:
			counts[row.Group.ProjectID] += row.SectionCount
		case row.IsTaskRow():
			counts[row.Group.ProjectID]++
		}
	}
	return groups, counts
}
