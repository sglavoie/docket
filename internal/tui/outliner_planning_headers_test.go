package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestPlanningRowsShowProjectHeadersWithoutChangingDurableOrder(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
	alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
	betaOne := primaryRowsTask("beta-one", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
	beta := primaryRowsProject("beta", betaOne)
	outliner, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaOne.ID},
		{ProjectID: beta.ID, TaskID: betaOne.ID},
		{ProjectID: alpha.ID, TaskID: alphaTwo.ID},
	})

	for _, mode := range []OutlinerModeID{ModeAll, ModeNext} {
		t.Run(string(mode), func(t *testing.T) {
			outliner.selectOutlinerMode(mode)
			if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{
				alphaOne.ID, betaOne.ID, alphaTwo.ID,
			}) {
				t.Fatalf("Next task order = %v, want durable interleaved order", got)
			}

			var headers []string
			for _, row := range outliner.rows {
				if row.IsProjectHeader && row.IsNextSection {
					headers = append(headers, row.ProjectID)
				}
			}
			if !slices.Equal(headers, []string{alpha.ID, beta.ID, alpha.ID}) {
				t.Fatalf("Next project headers = %v, want one per durable-order project run", headers)
			}
		})
	}
}

func TestPlanningProjectHeadersCollapseAndDisappearInProjectScope(t *testing.T) {
	alphaOne := primaryRowsTask("alpha-one", models.StateBacklog)
	alphaTwo := primaryRowsTask("alpha-two", models.StateBacklog)
	betaOne := primaryRowsTask("beta-one", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaOne, alphaTwo)
	beta := primaryRowsProject("beta", betaOne)
	outliner, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaOne.ID},
		{ProjectID: beta.ID, TaskID: betaOne.ID},
		{ProjectID: alpha.ID, TaskID: alphaTwo.ID},
	})
	outliner.selectOutlinerMode(ModeNext)

	for index, row := range outliner.rows {
		if row.IsProjectHeader && row.IsNextSection && row.ProjectID == alpha.ID {
			outliner.cursor = index
			break
		}
	}
	if !outliner.collapseOrMoveToParent() {
		t.Fatal("left on a Next project header did not collapse its project runs")
	}
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaOne.ID}) {
		t.Fatalf("collapsed Next tasks = %v, want only beta", got)
	}

	outliner.pushProjectScope(alpha.ID)
	for _, row := range outliner.rows {
		if row.IsProjectHeader && row.IsNextSection {
			t.Fatalf("project-scoped Next rendered a redundant project header: %+v", row)
		}
	}
}

func TestTodayRowsUseTheSameProjectHeaderContract(t *testing.T) {
	alphaTask := primaryRowsTask("alpha-today", models.StateBacklog)
	betaTask := primaryRowsTask("beta-today", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: beta.ID, TaskID: betaTask.ID},
		{ProjectID: alpha.ID, TaskID: alphaTask.ID},
	})
	outliner.selectOutlinerMode(ModeAll)

	var headers []string
	for _, row := range outliner.rows {
		if row.IsProjectHeader && row.IsTodaySection {
			headers = append(headers, row.ProjectID)
		}
	}
	if !slices.Equal(headers, []string{beta.ID, alpha.ID}) {
		t.Fatalf("Today project headers = %v, want durable-order project runs", headers)
	}
}
