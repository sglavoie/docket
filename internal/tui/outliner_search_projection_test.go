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

func TestScopeWideSearchIgnoresModeFacetsAndFoldsButHonorsScope(t *testing.T) {
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
	outliner.filter.Priority = models.PriorityHigh
	outliner.expandedNodes[TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}] = false
	outliner.SetSearchQuery("needle")

	got := qualifiedVisibleTaskIDs(outliner)
	want := []TaskIdentity{
		{ProjectID: alpha.ID, TaskID: deferred.ID},
		{ProjectID: alpha.ID, TaskID: inbox.ID},
		{ProjectID: alpha.ID, TaskID: child.ID},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("scope-wide search rows = %v, want ranked scoped nonterminal rows %v", got, want)
	}
	if outliner.rows[2].Breadcrumb != "Ordinary parent ›" {
		t.Fatalf("child breadcrumb = %q, want preserved ancestor context", outliner.rows[2].Breadcrumb)
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

	matches := searchProjectionMatchesWithin([]*models.Project{project}, "needle", nil, fuzzy.Matcher{})
	got := searchMatchTaskIDs(matches)
	want := []string{titleStrong.ID, titleWeak.ID, metadataFirst.ID, metadataTie.ID}
	if !slices.Equal(got, want) {
		t.Fatalf("ranked search = %v, want title then metadata score/source order %v", got, want)
	}

	exact := primaryRowsTask("exact", models.StateBacklog)
	exact.ShortID = 7
	exact.Title = "No shared words"
	project.Tasks = append(project.Tasks, exact)
	if got := searchMatchTaskIDs(searchProjectionMatchesWithin([]*models.Project{project}, "alpha#7", nil, fuzzy.Matcher{})); !slices.Equal(got, []string{exact.ID}) {
		t.Fatalf("exact alpha#7 lookup = %v, want %s", got, exact.ID)
	}
	if got := searchProjectionMatchesWithin([]*models.Project{project}, "alpha#999", nil, fuzzy.Matcher{}); len(got) != 0 {
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

	outliner.SetSearchQuery("needle")
	filterResult := (&AppModel{crossOutliner: outliner}).executeFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationFilterPriority},
		Value:      models.PriorityHigh,
	})
	if filterResult.Status != "Exit search before changing filters" ||
		outliner.filter.Priority != wantFilter.Priority {
		t.Fatalf("filter changed during search: result=%+v priority=%d",
			filterResult, outliner.filter.Priority)
	}
	outliner.filter.Tags = map[string]bool{"changed": true}
	outliner.collapsedSections["changed-section"] = true
	outliner.expandedNodes[TaskIdentity{ProjectID: project.ID, TaskID: third.ID}] = false
	if _, _ = outliner.selectOutlinerMode(ModeDue); outliner.primaryMode != ModeInbox {
		t.Fatalf("mode changed during search to %s", outliner.primaryMode)
	}
	outliner.SetSearchActive(true)
	outliner.SetSearchInputValue("needle")
	_, _ = outliner.handleKeyMsg(syntheticSpecialKey(tea.KeyEsc))

	if outliner.primaryMode != ModeInbox ||
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

	outliner.SetSearchQuery("needle")
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
	outliner.SetSearchQuery("needle")
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

	outliner.SetSearchQuery("api")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{upper.ID, lower.ID}) {
		t.Fatalf("lowercase search = %v, want both casings", got)
	}
	if outliner.searchCaseRelaxed || outliner.searchCaseSensitive() {
		t.Fatal("lowercase query was treated as case-sensitive")
	}

	outliner.SetSearchQuery("API")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{upper.ID}) {
		t.Fatalf("uppercase search = %v, want only the exact-case title", got)
	}
	if outliner.searchCaseRelaxed || !outliner.searchCaseSensitive() {
		t.Fatalf("narrowing search relaxed=%v sensitive=%v, want a case-sensitive pass",
			outliner.searchCaseRelaxed, outliner.searchCaseSensitive())
	}

	outliner.SetSearchQuery("Docs")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{lower.ID}) {
		t.Fatalf("fallback search = %v, want the case-insensitive retry to match", got)
	}
	if !outliner.searchCaseRelaxed || outliner.searchCaseSensitive() {
		t.Fatalf("fallback search relaxed=%v sensitive=%v, want a relaxed pass",
			outliner.searchCaseRelaxed, outliner.searchCaseSensitive())
	}

	outliner.SetSearchQuery("Zebra")
	if len(outliner.rows) != 0 {
		t.Fatalf("unmatchable query kept rows %v", searchVisibleTaskIDs(outliner))
	}
	if outliner.searchCaseRelaxed {
		t.Fatal("an empty insensitive retry still claimed the case was ignored")
	}

	outliner.SetSearchQuery("Alpha#42")
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
