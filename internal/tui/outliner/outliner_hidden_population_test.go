package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// hiddenPopulationFixture holds one visible task, a flagged parent whose child
// stores nothing and is hidden by inheritance, and a hidden task owed review.
func hiddenPopulationFixture() *OutlinerModel {
	visible := primaryRowsTask("needle-visible", models.StateBacklog)
	parent := primaryRowsTask("needle-parent", models.StateBacklog)
	parent.Hidden = true
	child := primaryRowsTask("needle-child", models.StateBacklog)
	child.ParentID = &parent.ID
	flagged := primaryRowsTask("needle-flagged", models.StateBacklog)
	flagged.Hidden, flagged.Review = true, true
	inbox := primaryRowsTask("needle-inbox", models.StateInbox)
	inbox.Hidden = true
	return scopeTestOutliner(primaryRowsProject("alpha", visible, parent, child, flagged, inbox))
}

func hiddenOnlyQuery(t *testing.T) models.Query {
	t.Helper()
	clauses := query.PopulationClauses(models.QueryFieldHidden, query.ProjectPopulationOnly)
	if len(clauses) != 1 {
		t.Fatalf("+hidden:only spelled %d clauses", len(clauses))
	}
	return models.Query{Clauses: clauses}
}

func requireVisibleTaskSet(t *testing.T, outliner *OutlinerModel, label string, want ...string) {
	t.Helper()
	got := visibleTaskIDs(outliner)
	slices.Sort(got)
	got = slices.Compact(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("%s rows = %v, want %v", label, got, want)
	}
}

func TestHiddenWorkIsAbsentFromEveryModeUntilTheFacetOpensIt(t *testing.T) {
	outliner := hiddenPopulationFixture()
	for _, mode := range []chrome.OutlinerModeID{chrome.ModeAll, chrome.ModeBacklog, chrome.ModeInbox, chrome.ModeRecent} {
		outliner.SelectOutlinerMode(mode)
		outliner.RebuildRows()
		for _, id := range visibleTaskIDs(outliner) {
			if id != "needle-visible" {
				t.Fatalf("mode %v drew hidden task %s", mode, id)
			}
		}
	}

	outliner.SelectOutlinerMode(chrome.ModeAll)
	outliner.RebuildRows()
	requireVisibleTaskSet(t, outliner, "All", "needle-visible")
	outliner.SetFilterQuery(hiddenOnlyQuery(t))
	outliner.RebuildRows()
	if !outliner.AdmitsHiddenPopulation() {
		t.Fatal("+hidden:only did not open the hidden population")
	}
	if got := outliner.Filter.HiddenMode; got != query.ProjectPopulationOnly {
		t.Fatalf("facet cache HiddenMode = %v, want only", got)
	}
	requireVisibleTaskSet(t, outliner, "+hidden:only", "needle-parent", "needle-child", "needle-flagged", "needle-inbox")
	for _, row := range outliner.Rows {
		if row.IsTaskRow() && row.PromotedTag == reviewLaneKey {
			t.Fatalf("hidden task %s was promoted into the Review lane", row.Task.ID)
		}
	}
}

func TestSearchExcludesHiddenWorkUnlessTheFacetOpensIt(t *testing.T) {
	outliner := hiddenPopulationFixture()
	searchRankedList(outliner, "needle")
	requireVisibleTaskSet(t, outliner, "ranked search", "needle-visible")

	outliner = hiddenPopulationFixture()
	outliner.SetFilterQuery(hiddenOnlyQuery(t))
	searchRankedList(outliner, "needle")
	requireVisibleTaskSet(t, outliner, "ranked search under +hidden:only",
		"needle-parent", "needle-child", "needle-flagged", "needle-inbox")
}

// TestReviewPassSkipsHiddenWorkEvenUnderTheFacet: fH puts hidden work on
// screen, but a review pass never records it as a successor, since hidden work
// is never owed attention.
func TestReviewPassSkipsHiddenWorkEvenUnderTheFacet(t *testing.T) {
	source := reviewFlagged("source")
	source.Hidden = true
	next := reviewFlagged("next")
	next.Hidden = true
	project := primaryRowsProject("alpha", source, next)
	outliner := reviewAdvanceOutliner(project)
	outliner.SetFilterQuery(hiddenOnlyQuery(t))
	outliner.RebuildRows()
	index, ok := outliner.RowIndexForTask(taskIdentityOf(source, project))
	if !ok {
		t.Fatal("+hidden:only did not draw the hidden source")
	}
	if _, ok := outliner.RowIndexForTask(taskIdentityOf(next, project)); !ok {
		t.Fatal("+hidden:only did not draw the hidden successor")
	}
	outliner.Cursor = index
	capture := outliner.CaptureOutlinerRelocation([]taskref.TaskIdentity{taskIdentityOf(source, project)})
	outliner.PlanReviewAdvance(capture)
	if len(capture.ReviewSuccessors) != 0 {
		t.Fatalf("review pass recorded hidden successors %v", capture.ReviewSuccessors)
	}
}

func taskIdentityOf(task *models.Task, project *models.Project) taskref.TaskIdentity {
	return taskref.For(task, project)
}
