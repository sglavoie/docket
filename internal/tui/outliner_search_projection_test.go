package tui

import (
	"maps"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// The ranked list is the one presentation that leaves the mode's lanes and the
// user's folds behind. Scope is not a clause it can leave: the search runs
// inside the frame it was typed in. Facets still gate it, and the deferred and
// terminal populations open only when the query says so.
func TestTheRankedListLeavesModeAndFoldsButHonorsScopeAndFacets(t *testing.T) {
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Title = "Ordinary parent"
	child := primaryRowsTask("child", models.StateWaiting)
	child.Title = "Child title"
	child.Description = "contains Needle"
	child.ParentID = &parent.ID
	deferred := primaryRowsTask("deferred", models.StateSomeday)
	deferred.Title = "Needle deferred"
	deferred.DeferredUntil = &tomorrow
	inbox := primaryRowsTask("inbox", models.StateInbox)
	inbox.Title = "Needle inbox"
	inbox.Priority = models.PriorityLow
	done := primaryRowsTask("done", models.StateDone)
	done.Title = "Needle done"
	canceled := primaryRowsTask("canceled", models.StateCanceled)
	canceled.Title = "Needle canceled"
	alpha := primaryRowsProject("alpha", parent, child, deferred, inbox, done, canceled)

	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Needle other project"
	beta := primaryRowsProject("beta", other)

	outliner := scopeTestOutliner(alpha, beta)
	outliner.pushProjectScope(alpha.ID)
	outliner.selectOutlinerMode(ModeBacklog)
	outliner.expandedNodes[TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}] = false
	searchRankedList(outliner, "needle")

	got := qualifiedVisibleTaskIDs(outliner)
	want := []TaskIdentity{
		{ProjectID: alpha.ID, TaskID: inbox.ID},
		{ProjectID: alpha.ID, TaskID: child.ID},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ranked list rows = %v, want the scoped open rows %v", got, want)
	}
	if outliner.rows[1].Breadcrumb != "Ordinary parent ›" {
		t.Fatalf("child breadcrumb = %q, want preserved ancestor context", outliner.rows[1].Breadcrumb)
	}

	// The deferred task is reachable from the same list, but only because the
	// query asked for it. Admission has one rule and the query is it.
	searchRankedList(outliner, "needle @@any")
	if got := qualifiedVisibleTaskIDs(outliner); !slices.Equal(got,
		[]TaskIdentity{{ProjectID: alpha.ID, TaskID: deferred.ID}}) {
		t.Fatalf("deferred query = %v, want the deferred hit alone", got)
	}
	searchRankedList(outliner, "needle")

	// A facet is an AND gate applied before scoring, so it narrows the same ranked
	// list without disturbing its order. Only the low-priority task clears the
	// threshold: an unset priority is effectively below Low.
	outliner.filter.Priority = models.PriorityLow
	outliner.adoptFilterCarriers()
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if got := qualifiedVisibleTaskIDs(outliner); !slices.Equal(got,
		[]TaskIdentity{{ProjectID: alpha.ID, TaskID: inbox.ID}}) {
		t.Fatalf("search under the priority facet = %v, want only the low-priority hit", got)
	}
}

func TestScopeWideSearchRanksFieldsFuzzyScoreAndStableSourceOrder(t *testing.T) {
	titleStrong := primaryRowsTask("title-strong", models.StateBacklog)
	titleStrong.Title = "Needle"
	titleWeak := primaryRowsTask("title-weak", models.StateBacklog)
	titleWeak.Title = "Need a little extra"
	metadataFirst := primaryRowsTask("metadata-first", models.StateBacklog)
	metadataFirst.Description = "Needle"
	metadataTie := primaryRowsTask("metadata-tie", models.StateBacklog)
	metadataTie.Tags = []string{"needle"}
	project := primaryRowsProject("alpha", titleWeak, metadataFirst, metadataTie, titleStrong)
	project.Slug = "alpha"

	matches := searchProjectionMatchesWithin([]*models.Project{project}, "needle", nil, fuzzy.Matcher{}, nil)
	got := searchMatchTaskIDs(matches)
	want := []string{titleStrong.ID, titleWeak.ID, metadataFirst.ID, metadataTie.ID}
	if !slices.Equal(got, want) {
		t.Fatalf("ranked search = %v, want title then metadata score/source order %v", got, want)
	}

	exact := primaryRowsTask("exact", models.StateBacklog)
	exact.ShortID = 7
	exact.Title = "No shared words"
	project.Tasks = append(project.Tasks, exact)
	if got := searchMatchTaskIDs(searchProjectionMatchesWithin([]*models.Project{project}, "alpha#7", nil, fuzzy.Matcher{}, nil)); !slices.Equal(got, []string{exact.ID}) {
		t.Fatalf("exact alpha#7 lookup = %v, want %s", got, exact.ID)
	}
	if got := searchProjectionMatchesWithin([]*models.Project{project}, "alpha#999", nil, fuzzy.Matcher{}, nil); len(got) != 0 {
		t.Fatalf("missing exact qualifier returned %v", searchMatchTaskIDs(got))
	}
}

func TestScopeWideSearchRestoresProjectionExactlyAndFallsBackAfterDeletion(t *testing.T) {
	first := primaryRowsTask("first", models.StateInbox)
	first.Title = "Needle first"
	second := primaryRowsTask("second", models.StateInbox)
	second.Title = "Needle second"
	third := primaryRowsTask("third", models.StateInbox)
	third.Title = "Needle third"
	first.Tags = []string{"saved"}
	second.Tags = []string{"saved"}
	third.Tags = []string{"saved"}
	project := primaryRowsProject("alpha", first, second, third)
	outliner := scopeTestOutliner(project)
	outliner.selectOutlinerMode(ModeInbox)
	outliner.filter.Tags = map[string]bool{"saved": true}
	outliner.adoptFilterCarriers()
	outliner.collapsedSections = map[string]bool{"saved-section": true}
	outliner.expandedNodes = map[TaskIdentity]bool{{ProjectID: project.ID, TaskID: first.ID}: false}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if !outliner.moveCursorToProjectTask(second.ID, project.ID) {
		t.Fatal("failed to seed cursor")
	}
	outliner.viewScrollOffset = 4
	wantFilter := copyFilterState(outliner.filter)
	wantCollapsed := maps.Clone(outliner.collapsedSections)
	wantExpanded := maps.Clone(outliner.expandedNodes)

	searchRankedList(outliner, "needle")
	filterResult := (&AppModel{crossOutliner: outliner}).executeFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationFilterPriority},
		Value:      models.PriorityHigh,
	})
	// The facet applies to the live projection rather than refusing, and Esc
	// still restores the snapshot taken at entry — including this edit.
	if filterResult.Status != "Filtering High priority" ||
		outliner.filter.Priority != models.PriorityHigh {
		t.Fatalf("filter refused during search: result=%+v priority=%d",
			filterResult, outliner.filter.Priority)
	}
	outliner.filter.Tags = map[string]bool{"changed": true}
	outliner.adoptFilterCarriers()
	outliner.collapsedSections["changed-section"] = true
	outliner.expandedNodes[TaskIdentity{ProjectID: project.ID, TaskID: third.ID}] = false
	// The mode is a live axis under a held query, and it is also part of the
	// frame the query was typed in, so moving it here is exactly what Esc has to
	// put back below.
	if _, _ = outliner.selectOutlinerMode(ModeDue); outliner.primaryMode != ModeDue {
		t.Fatalf("mode refused to change during search, staying %s", outliner.primaryMode)
	}
	outliner.SetSearchActive(true)
	outliner.SetSearchInputValue("needle")
	_, _ = outliner.handleKeyMsg(syntheticSpecialKey(tea.KeyEsc))

	if outliner.primaryMode != ModeInbox ||
		outliner.filter.Priority != wantFilter.Priority ||
		!maps.Equal(outliner.filter.Tags, wantFilter.Tags) ||
		!maps.Equal(outliner.collapsedSections, wantCollapsed) ||
		!maps.Equal(outliner.expandedNodes, wantExpanded) ||
		outliner.viewScrollOffset != 4 {
		t.Fatalf("restored state differs: mode=%s filter=%v collapsed=%v expanded=%v scroll=%d",
			outliner.primaryMode, outliner.filter.Tags, outliner.collapsedSections,
			outliner.expandedNodes, outliner.viewScrollOffset)
	}
	if outliner.hasSearchProjection() || outliner.HasActiveSearch() || outliner.SearchQuery() != "" {
		t.Fatalf("Esc left search active: projection=%v input=%v query=%q",
			outliner.hasSearchProjection(), outliner.HasActiveSearch(), outliner.SearchQuery())
	}
	if task, selectedProject := outliner.getTaskRef(); task == nil || task.ID != second.ID || selectedProject != project {
		t.Fatalf("restored cursor = %v/%v, want alpha/second", selectedProject, task)
	}

	searchRankedList(outliner, "needle")
	project.Tasks = []*models.Task{first, third}
	project.BuildTaskTree()
	outliner.ClearSearch()
	if task, _ := outliner.getTaskRef(); task == nil || task.ID != third.ID {
		t.Fatalf("deleted cursor fallback = %v, want nearest following task %s", task, third.ID)
	}
}

func TestScopeWideSearchRefreshesAfterMutationsAndShowsNoMatches(t *testing.T) {
	matching := primaryRowsTask("matching", models.StateBacklog)
	matching.Title = "Needle before edit"
	appearing := primaryRowsTask("appearing", models.StateBacklog)
	appearing.Title = "Initially absent"
	project := primaryRowsProject("alpha", matching, appearing)
	outliner := scopeTestOutliner(project)
	searchRankedList(outliner, "needle")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{matching.ID}) {
		t.Fatalf("initial search = %v", got)
	}

	matching.Title = "No longer matches"
	appearing.Description = "Needle added by mutation"
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{appearing.ID}) {
		t.Fatalf("refreshed search = %v, want appearing task", got)
	}

	appearing.Description = "Removed again"
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if len(outliner.rows) != 0 {
		t.Fatalf("no-match projection retained rows: %v", searchVisibleTaskIDs(outliner))
	}
	if got := outliner.outlinerEmptyMessage(); got != `No tasks match search "needle". Press Esc to clear.` {
		t.Fatalf("no-match message = %q", got)
	}
}

func TestScopeWideSearchAppliesSmartCaseAndFallsBackWhenEmpty(t *testing.T) {
	upper := primaryRowsTask("upper", models.StateBacklog)
	upper.Title = "Ship API client"
	lower := primaryRowsTask("lower", models.StateBacklog)
	lower.Title = "Rework api docs"
	exact := primaryRowsTask("exact", models.StateBacklog)
	exact.ShortID = 42
	exact.Title = "Nothing in common"
	project := primaryRowsProject("alpha", upper, lower, exact)
	outliner := scopeTestOutliner(project)

	searchRankedList(outliner, "api")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{upper.ID, lower.ID}) {
		t.Fatalf("lowercase search = %v, want both casings", got)
	}
	if outliner.searchCaseRelaxed || outliner.searchCaseSensitive() {
		t.Fatal("lowercase query was treated as case-sensitive")
	}

	searchRankedList(outliner, "API")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{upper.ID}) {
		t.Fatalf("uppercase search = %v, want only the exact-case title", got)
	}
	if outliner.searchCaseRelaxed || !outliner.searchCaseSensitive() {
		t.Fatalf("narrowing search relaxed=%v sensitive=%v, want a case-sensitive pass",
			outliner.searchCaseRelaxed, outliner.searchCaseSensitive())
	}

	searchRankedList(outliner, "Docs")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{lower.ID}) {
		t.Fatalf("fallback search = %v, want the case-insensitive retry to match", got)
	}
	if !outliner.searchCaseRelaxed || outliner.searchCaseSensitive() {
		t.Fatalf("fallback search relaxed=%v sensitive=%v, want a relaxed pass",
			outliner.searchCaseRelaxed, outliner.searchCaseSensitive())
	}

	searchRankedList(outliner, "Zebra")
	if len(outliner.rows) != 0 {
		t.Fatalf("unmatchable query kept rows %v", searchVisibleTaskIDs(outliner))
	}
	if outliner.searchCaseRelaxed {
		t.Fatal("an empty insensitive retry still claimed the case was ignored")
	}

	searchRankedList(outliner, "Alpha#42")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{exact.ID}) {
		t.Fatalf("capitalized project#id shorthand = %v, want %s", got, exact.ID)
	}

	outliner.ClearSearch()
	if outliner.searchCaseRelaxed {
		t.Fatal("ClearSearch left the relaxed flag set")
	}
}

func qualifiedVisibleTaskIDs(outliner *OutlinerModel) []TaskIdentity {
	result := make([]TaskIdentity, 0)
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil {
			result = append(result, TaskIdentity{ProjectID: row.ProjectID, TaskID: row.task.ID})
		}
	}
	return result
}

// searchRankedList holds a query and shows it as the flat ranked list. The tier,
// score, and seating assertions are all about that presentation; a query alone
// narrows the mode's own lanes instead, which the D7 tests cover separately.
func searchRankedList(outliner *OutlinerModel, query string) {
	outliner.SetSearchQuery(query)
	if !outliner.showsRankedList() {
		outliner.toggleRankedList()
	}
}

func searchVisibleTaskIDs(outliner *OutlinerModel) []string {
	result := make([]string, 0)
	for _, identity := range qualifiedVisibleTaskIDs(outliner) {
		result = append(result, identity.TaskID)
	}
	return result
}

func searchMatchTaskIDs(matches []outlinerSearchMatch) []string {
	result := make([]string, len(matches))
	for i, match := range matches {
		result[i] = match.task.ID
	}
	return result
}
