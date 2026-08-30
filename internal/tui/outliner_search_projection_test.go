package tui

import (
	"maps"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/util"
)

func TestScopeWideSearchIgnoresModeAndFoldsButHonorsScopeAndFacets(t *testing.T) {
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

	outliner.SetSearchQuery("needle")
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
	if _, _ = outliner.selectOutlinerMode(ModeDue); outliner.primaryMode != ModeInbox {
		t.Fatalf("mode changed during search to %s", outliner.primaryMode)
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

// searchGateProjects builds one two-project scope whose task order is the source
// order a sigil-only query must preserve.
func searchGateProjects() (*models.Project, *models.Project) {
	today := time.Now()
	later := today.AddDate(0, 0, 3)

	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Title = "Pin candidate"
	pinned.Pinned = true
	pinned.Tags = []string{"backend"}
	priorityOne := primaryRowsTask("priority-one", models.StateBacklog)
	priorityOne.Title = "Ship auth"
	priorityOne.Priority = models.PriorityHigh
	priorityOne.DueDate = &today
	contexted := primaryRowsTask("contexted", models.StateBacklog)
	contexted.Title = "Client work"
	contexted.Context = "client-acme"
	contexted.DueDate = &later
	plannedToday := primaryRowsTask("planned-today", models.StateBacklog)
	plannedToday.Title = "Planned today"
	plannedNext := primaryRowsTask("planned-next", models.StateBacklog)
	plannedNext.Title = "Planned next"
	finished := primaryRowsTask("finished", models.StateDone)
	finished.Title = "Old auth work"
	alpha := primaryRowsProject("alpha", pinned, priorityOne, contexted, plannedToday, plannedNext, finished)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta auth"
	betaTask.Tags = []string{"backend"}
	beta := primaryRowsProject("beta", betaTask)

	return alpha, beta
}

func searchGateOutliner() *OutlinerModel {
	alpha, beta := searchGateProjects()
	outliner := scopeTestOutliner(alpha, beta)
	outliner.runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-today"},
	})
	outliner.runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-next"},
	})
	return outliner
}

func TestSearchSigilsGateTheProjectionAndAndTogether(t *testing.T) {
	exactDay := time.Now().AddDate(0, 0, 3).Format(util.DateFormat)
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"pinned", "*pin", []string{"pinned"}},
		{"unpinned", "*unpin", []string{"priority-one", "contexted", "planned-today", "planned-next", "beta-task"}},
		{"priority equality", "!1", []string{"priority-one"}},
		{"tag prefix", "#back", []string{"pinned", "beta-task"}},
		{"excluded tag", "-#backend", []string{"priority-one", "contexted", "planned-today", "planned-next"}},
		{"context prefix", "##client", []string{"contexted"}},
		{"due bucket", "@today", []string{"priority-one"}},
		{"missing due date", "@none", []string{"pinned", "planned-today", "planned-next", "beta-task"}},
		{"exact due day", "@" + exactDay, []string{"contexted"}},
		{"terminal destination", "$done", []string{"finished"}},
		{"planning today", "$today", []string{"planned-today"}},
		{"planning next", "$next", []string{"planned-next"}},
		{"backlog includes planning members", "$backlog", []string{
			"pinned", "priority-one", "contexted", "planned-today", "planned-next", "beta-task",
		}},
		{"project prefix", "%bet", []string{"beta-task"}},
		{"predicate and free text", "#backend auth", []string{"beta-task"}},
		{"two predicates", "!1 @today", []string{"priority-one"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outliner := searchGateOutliner()
			outliner.SetSearchQuery(tt.query)
			if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, tt.want) {
				t.Fatalf("search %q = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestSearchTerminalTasksNeedAnExplicitTerminalDestination(t *testing.T) {
	outliner := searchGateOutliner()
	outliner.SetSearchQuery("auth")
	if got := searchVisibleTaskIDs(outliner); slices.Contains(got, "finished") {
		t.Fatalf("plain text search = %v, want the done task excluded", got)
	}
	outliner.SetSearchQuery("$done auth")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"finished"}) {
		t.Fatalf("$done search = %v, want the done task revealed", got)
	}
}

func TestSearchProjectSigilNarrowsWithinFocusRatherThanEscapingIt(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Title = "Parent"
	child := primaryRowsTask("child", models.StateBacklog)
	child.Title = "Child"
	child.ParentID = &parent.ID
	outside := primaryRowsTask("outside", models.StateBacklog)
	outside.Title = "Outside"
	alpha := primaryRowsProject("alpha", parent, child, outside)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta"
	beta := primaryRowsProject("beta", betaTask)

	outliner := scopeTestOutliner(alpha, beta)
	outliner.pushFocusFrame(TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}, 0, 0)

	outliner.SetSearchQuery("%alpha")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{child.ID}) {
		t.Fatalf("%%alpha under Focus = %v, want the focused descendant only", got)
	}
	outliner.SetSearchQuery("%beta")
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("%%beta under Focus = %v, want no escape from the focused branch", got)
	}
}

func TestSearchProjectQualifierSurvivesButProjectSigilIsADifferentQuery(t *testing.T) {
	target := primaryRowsTask("target", models.StateBacklog)
	target.Title = "Exact qualifier target"
	target.ShortID = 7
	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Tagged seven"
	other.Tags = []string{"7"}
	alpha := primaryRowsProject("alpha", target, other)
	outliner := scopeTestOutliner(alpha)

	outliner.SetSearchQuery("alpha#7")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{target.ID}) {
		t.Fatalf("alpha#7 = %v, want the exact short-ID lookup", got)
	}
	outliner.SetSearchQuery("%alpha #7")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{other.ID}) {
		t.Fatalf("%%alpha #7 = %v, want the tag gate", got)
	}
}

func TestSearchRefusesRecognizedAndWrongValuesWithRegistryGuidance(t *testing.T) {
	outliner := searchGateOutliner()
	for _, query := range []string{"!9", "$ready", "##0", "auth >4"} {
		outliner.SetSearchQuery(query)
		if len(outliner.rows) != 0 {
			t.Fatalf("search %q kept rows %v", query, searchVisibleTaskIDs(outliner))
		}
		guidance := outliner.searchGate.guidanceText()
		if guidance == "" {
			t.Fatalf("search %q produced no guidance", query)
		}
		if got := outliner.outlinerEmptyMessage(); got != guidance+". Press Esc to clear." {
			t.Fatalf("empty message = %q, want the guidance sentence", got)
		}
		if got := outliner.classifyEmptyCause(); got != outlinerEmptySearchInvalid {
			t.Fatalf("empty cause = %v, want the invalid-search cause", got)
		}
	}
	outliner.SetSearchQuery("!9")
	want := textgrammar.InvalidPriorityError("9").Error()
	if got := outliner.searchGate.guidanceText(); got != want {
		t.Fatalf("!9 guidance = %q, want the registry sentence %q", got, want)
	}
}

func TestSearchCaseRelaxationRetryAdmitsTheSameGatedSet(t *testing.T) {
	outliner := searchGateOutliner()
	outliner.SetSearchQuery("#backend Auth")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"beta-task"}) {
		t.Fatalf("relaxed gated search = %v, want the backend-tagged match", got)
	}
	if !outliner.searchCaseRelaxed {
		t.Fatal("the retry did not report a relaxed pass")
	}
	if outliner.searchRemainder != "Auth" {
		t.Fatalf("remainder = %q, want the sigil lifted out", outliner.searchRemainder)
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
