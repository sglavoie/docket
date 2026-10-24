package outliner

import (
	"slices"
	"testing"
	"time"

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

// hiddenAttentionFixture pairs visible work owed review or gone stale with the
// same two attention facts on a directly hidden task and on a child hidden only
// by inheritance, beside a quiet hidden task neither question admits.
func hiddenAttentionFixture() *OutlinerModel {
	old := time.Now().AddDate(0, 0, -(models.StaleTaskThresholdDays + 30))
	visibleReview := primaryRowsTask("seen-review", models.StateBacklog)
	visibleReview.Review = true
	visibleStale := primaryRowsTask("seen-stale", models.StateBacklog)
	visibleStale.UpdatedAt = old
	flagged := primaryRowsTask("hid-review", models.StateBacklog)
	flagged.Hidden, flagged.Review = true, true
	staleHidden := primaryRowsTask("hid-stale", models.StateBacklog)
	staleHidden.Hidden, staleHidden.UpdatedAt = true, old
	parent := primaryRowsTask("hid-parent", models.StateBacklog)
	parent.Hidden = true
	child := primaryRowsTask("hid-child", models.StateBacklog)
	child.ParentID = &parent.ID
	child.Review, child.UpdatedAt = true, old
	return scopeTestOutliner(primaryRowsProject("alpha",
		visibleReview, visibleStale, flagged, staleHidden, parent, child))
}

func hiddenIncludeQuery(t *testing.T) models.Query {
	t.Helper()
	clauses := query.PopulationClauses(models.QueryFieldHidden, query.ProjectPopulationInclude)
	if len(clauses) != 1 {
		t.Fatalf("+hidden spelled %d clauses", len(clauses))
	}
	return models.Query{Clauses: clauses}
}

// TestSearchAttentionPredicatesNeverAdmitHiddenWork: ?review and ?stale answer
// exactly as the canonical query clauses do, so hidden work, direct or
// inherited, is never owed attention even where fH has opened its population,
// and the negations admit it as work owing none. Text search and the stored
// =review flag still reach hidden work once the facet opens it.
func TestSearchAttentionPredicatesNeverAdmitHiddenWork(t *testing.T) {
	cases := []struct {
		label  string
		facet  func(*testing.T) models.Query
		search string
		want   []string
	}{
		{"base ?review", nil, "?review", []string{"seen-review"}},
		{"base ?stale", nil, "?stale", []string{"seen-stale"}},
		{"base -?review", nil, "-?review", []string{"seen-stale"}},
		{"include ?review", hiddenIncludeQuery, "?review", []string{"seen-review"}},
		{"include ?stale", hiddenIncludeQuery, "?stale", []string{"seen-stale"}},
		{"include -?review", hiddenIncludeQuery, "-?review",
			[]string{"seen-stale", "hid-review", "hid-stale", "hid-parent", "hid-child"}},
		{"include -?stale", hiddenIncludeQuery, "-?stale",
			[]string{"seen-review", "hid-review", "hid-stale", "hid-parent", "hid-child"}},
		{"only ?review", hiddenOnlyQuery, "?review", nil},
		{"only ?stale", hiddenOnlyQuery, "?stale", nil},
		{"only -?review", hiddenOnlyQuery, "-?review",
			[]string{"hid-review", "hid-stale", "hid-parent", "hid-child"}},
		{"only -?stale", hiddenOnlyQuery, "-?stale",
			[]string{"hid-review", "hid-stale", "hid-parent", "hid-child"}},
		{"only =review", hiddenOnlyQuery, "=review", []string{"hid-review", "hid-child"}},
		{"only text", hiddenOnlyQuery, "hid", []string{"hid-review", "hid-stale", "hid-parent", "hid-child"}},
	}
	for _, tc := range cases {
		outliner := hiddenAttentionFixture()
		if tc.facet != nil {
			outliner.SetFilterQuery(tc.facet(t))
		}
		searchRankedList(outliner, tc.search)
		requireVisibleTaskSet(t, outliner, tc.label, tc.want...)
	}
}
