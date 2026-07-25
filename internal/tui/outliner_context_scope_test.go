package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// contextScopeOutliner builds a global Outliner over projects already loaded,
// with the Context resolver pointed at the same runtime catalog.
func contextScopeOutliner(runtime *config.RuntimeState, projects ...*models.Project) *OutlinerModel {
	m := NewOutlinerModel(nil, nil, runtime)
	m.SetSize(120, 40)
	m.loadedProjects = projects
	m.crossForceRebuild = true
	m.rebuildRows()
	return m
}

// TestContextScopeNarrowsEveryCrossProjectMode locks the session's core promise:
// an active Context is the single project-set authority, so the tree/All root,
// its Today section, Priority, the computed work horizons, and both review modes
// all enumerate only Context members.
func TestContextScopeNarrowsEveryCrossProjectMode(t *testing.T) {
	alpha := buildFilterParityProject("alpha", "Alpha")
	alpha.Context = "work"
	beta := buildFilterParityProject("beta", "Beta")
	beta.Context = "home"

	runtime := testConfigWithProjects(alpha, beta)
	runtime.Planning().SetDailyPlan([]config.DailyPlanItem{
		{TaskID: "alpha-today", ProjectID: "alpha", PlannedDate: "2026-07-23"},
		{TaskID: "beta-today", ProjectID: "beta", PlannedDate: "2026-07-23"},
	})

	m := contextScopeOutliner(runtime, alpha, beta)

	// Control: All spans both projects, so the narrowed assertions below are
	// meaningful regressions rather than vacuously true.
	m.selectOutlinerMode(ModeBacklog)
	if !containsProject(taskRowProjectIDs(m), "beta") {
		t.Fatalf("All-context Backlog omitted beta rows: %v", taskRowProjectIDs(m))
	}

	m.applyContextScope(RegularContextScope("work"))

	if got := m.scopeProjects(); len(got) != 1 || got[0].ID != "alpha" {
		t.Fatalf("scopeProjects under work context = %v, want [alpha]", projectIDsOf(got))
	}
	if got := m.workModeScopeProjects(); len(got) != 1 || got[0].ID != "alpha" {
		t.Fatalf("workModeScopeProjects under work context = %v, want [alpha]", projectIDsOf(got))
	}

	// The All root and its embedded Today section.
	m.selectOutlinerMode(ModeAll)
	assertAllProject(t, taskRowProjectIDs(m), "alpha")
	var todayIDs []string
	for _, row := range m.rows {
		if row.IsTodaySection && row.isTaskRow() {
			todayIDs = append(todayIDs, row.ProjectID)
		}
	}
	assertAllProject(t, todayIDs, "alpha")
	if len(todayIDs) != 1 {
		t.Fatalf("Today section rows = %d, want 1 (%v)", len(todayIDs), todayIDs)
	}

	for _, tc := range []struct {
		id OutlinerModeID
		// want is the expected number of task rows; review modes additionally
		// assert their section header counts sum to the same number, so a Context
		// narrowing that skipped the counting pass cannot pass silently.
		want   int
		review bool
	}{
		{id: ModePriorityLens, want: 3},
		{id: ModeNext, want: 1},
		{id: ModeBacklog, want: 1},
		{id: ModeDailyReview, want: 1, review: true},
		{id: ModeActivity, want: 3, review: true},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			m.selectOutlinerMode(tc.id)
			ids := taskRowProjectIDs(m)
			assertAllProject(t, ids, "alpha")
			if len(ids) != tc.want {
				t.Fatalf("%s rows = %d, want %d (%v)", tc.id, len(ids), tc.want, ids)
			}
			if tc.review {
				if sum := sectionCountSum(m); sum != len(ids) {
					t.Fatalf("%s section-count sum = %d, want row parity %d", tc.id, sum, len(ids))
				}
			}
		})
	}
}

// TestContextScopeHonorsSectionFilterAndJumperProjectSets asserts the surfaces
// that enumerate projects rather than rows — the F @ section filter selector and
// the task jumper — are fed the Context project set, not every loaded project.
func TestContextScopeHonorsSectionFilterAndJumperProjectSets(t *testing.T) {
	alpha := buildFilterParityProject("alpha", "Alpha")
	alpha.Context = "work"
	beta := buildFilterParityProject("beta", "Beta")
	beta.Context = "home"

	m := contextScopeOutliner(testConfigWithProjects(alpha, beta), alpha, beta)
	m.applyContextScope(RegularContextScope("work"))

	_, cmd := m.handleFilterPrefix("@")
	if cmd == nil {
		t.Fatal("F @ produced no command")
	}
	sectionMsg, ok := cmd().(ShowSectionFilterSelectorMsg)
	if !ok {
		t.Fatalf("F @ msg = %T, want ShowSectionFilterSelectorMsg", cmd())
	}
	if ids := projectIDsOf(sectionMsg.Projects); len(ids) != 1 || ids[0] != "alpha" {
		t.Fatalf("section filter selector projects = %v, want [alpha]", ids)
	}

	_, cmd = m.Update(syntheticKey('\\'))
	if cmd == nil {
		t.Fatal("jump key produced no command")
	}
	jumperMsg, ok := cmd().(ShowTaskJumperMsg)
	if !ok {
		t.Fatalf("jumper msg = %T, want ShowTaskJumperMsg", cmd())
	}
	if ids := projectIDsOf(jumperMsg.Projects); len(ids) != 1 || ids[0] != "alpha" {
		t.Fatalf("task jumper projects = %v, want [alpha]", ids)
	}
}

// TestCustomContextScopeNarrowsAndEmptyContextShowsNothing asserts a custom
// context scopes to its stored project IDs, and that a context resolving to zero
// projects renders empty rather than silently widening back to every project.
func TestCustomContextScopeNarrowsAndEmptyContextShowsNothing(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1")
	beta := scopeTestProject("beta", "Beta", "1")

	runtime := testConfigWithProjects(alpha, beta)
	runtime.Machine().SetCustomContext("focus", []string{"alpha"})
	runtime.Machine().SetCustomContext("empty", nil)

	m := contextScopeOutliner(runtime, alpha, beta)

	m.applyContextScope(CustomContextScope("focus"))
	if ids := scopeVisibleProjectIDs(m); !ids["alpha"] || ids["beta"] {
		t.Fatalf("custom context rows = %v, want alpha only", ids)
	}

	m.applyContextScope(CustomContextScope("empty"))
	if ids := scopeVisibleProjectIDs(m); len(ids) != 0 {
		t.Fatalf("empty custom context must render no rows, got %v", ids)
	}
}

// TestApplyContextScopeClearsRowAxesAndPreservesFilters locks the session's
// apply contract: a Context change drops the axes that name individual projects
// or rows (drill-in scope, zoom, multi-select, cursor, scroll) and preserves the
// user's view configuration (task filters, mode, search, sort, pinned-tag tab).
func TestApplyContextScopeClearsRowAxesAndPreservesFilters(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	alpha.Context = "work"
	beta := scopeTestProject("beta", "Beta", "1")

	m := contextScopeOutliner(testConfigWithProjects(alpha, beta), alpha, beta)

	m.pushProjectScope("beta")
	m.crossZoomStack = []CrossProjectZoomEntry{{TaskID: "beta-1", ProjectID: "beta"}}
	m.zoomStack = []ZoomEntry{{TaskID: "beta-1"}}
	m.multiSelectMode = true
	m.selectedTaskIdentities = map[TaskIdentity]bool{{ProjectID: "beta", TaskID: "beta-1"}: true}
	m.cursor = 1
	m.viewScrollOffset = 5

	// Preserved axes.
	m.filter.Priority = 1
	m.filter.Sections = map[string]bool{"inbox": true}
	m.filter.Actionable = true
	m.SetSearchQuery("alpha")
	m.setPinnedTagTabIndex(0)
	m.selectOutlinerMode(ModeBacklog)
	wantMode := m.activeOutlinerMode()
	// Set after the mode change, which owns its own ordering reset.
	m.sort = SortState{Field: SortByPriority}

	m.applyContextScope(RegularContextScope("work"))

	if m.scopedProjectID != "" || len(m.scopeStack) != 0 {
		t.Fatalf("drill-in scope survived context apply: id=%q stack=%d", m.scopedProjectID, len(m.scopeStack))
	}
	if len(m.zoomStack) != 0 || len(m.crossZoomStack) != 0 {
		t.Fatalf("zoom survived context apply: single=%d cross=%d", len(m.zoomStack), len(m.crossZoomStack))
	}
	if m.multiSelectMode || m.SelectedCount() != 0 {
		t.Fatalf("multi-select survived context apply: mode=%v count=%d", m.multiSelectMode, m.SelectedCount())
	}
	if m.cursor != 0 || m.viewScrollOffset != 0 {
		t.Fatalf("cursor/scroll not reset: cursor=%d scroll=%d", m.cursor, m.viewScrollOffset)
	}

	if m.filter.Priority != 1 || !m.filter.Actionable || len(m.filter.Sections) != 1 {
		t.Fatalf("task filters not preserved: %+v", m.filter)
	}
	if m.SearchQuery() != "alpha" {
		t.Fatalf("search not preserved: %q", m.SearchQuery())
	}
	if m.sort.Field != SortByPriority {
		t.Fatalf("sort not preserved: %+v", m.sort)
	}
	if got := m.activeOutlinerMode(); got != wantMode {
		t.Fatalf("mode not preserved: got %q, want %q", got, wantMode)
	}
}

// TestClearFiltersPreservesActiveContext locks the F x rule: the Context is
// scope, not a filter, so clearing task filters must leave it alone.
func TestClearFiltersPreservesActiveContext(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1")
	alpha.Context = "work"
	beta := scopeTestProject("beta", "Beta", "1")

	m := contextScopeOutliner(testConfigWithProjects(alpha, beta), alpha, beta)
	m.applyContextScope(RegularContextScope("work"))
	m.filter.Priority = 1

	m.handleFilterPrefix("x")

	if m.activeContext != RegularContextScope("work") {
		t.Fatalf("F x cleared the active context: %+v", m.activeContext)
	}
	if m.filter.Priority != 0 {
		t.Fatalf("F x must still clear task filters, priority=%d", m.filter.Priority)
	}
	if ids := scopeVisibleProjectIDs(m); !ids["alpha"] || ids["beta"] {
		t.Fatalf("context narrowing lost after F x: %v", ids)
	}
}

// TestContextScopeChromeNamesTheContext asserts the Context renders as scope
// chrome — the title and status bar name it and count its projects — rather than
// as another entry in the filter badge list.
func TestContextScopeChromeNamesTheContext(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1")
	alpha.Context = "work"
	beta := scopeTestProject("beta", "Beta", "1")

	m := contextScopeOutliner(testConfigWithProjects(alpha, beta), alpha, beta)
	m.applyContextScope(RegularContextScope("work"))

	title := ansi.Strip(m.renderOutlinerTitle(120))
	if !strings.Contains(title, "Context: work") {
		t.Fatalf("title missing context scope chrome:\n%s", title)
	}
	if !strings.Contains(title, "(1 project)") {
		t.Fatalf("title must count context projects, got:\n%s", title)
	}
	if strings.Contains(title, "ctx:") {
		t.Fatalf("context must be scope chrome, not a filter badge:\n%s", title)
	}
	if got := m.crossStatusBarLabel(); !strings.Contains(got, "· work ·") {
		t.Fatalf("status bar label missing context name: %q", got)
	}
}

// TestProjectDrillInOverridesContextScope asserts a drill-in scope wins over the
// Context, so drilling into a non-member project still shows it instead of
// rendering an empty view.
func TestProjectDrillInOverridesContextScope(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1")
	alpha.Context = "work"
	beta := scopeTestProject("beta", "Beta", "1")

	m := contextScopeOutliner(testConfigWithProjects(alpha, beta), alpha, beta)
	m.applyContextScope(RegularContextScope("work"))

	m.pushProjectScope("beta")
	if ids := scopeVisibleProjectIDs(m); !ids["beta"] || ids["alpha"] {
		t.Fatalf("drill-in into a non-member project must show it, got %v", ids)
	}

	m.popProjectScope()
	if ids := scopeVisibleProjectIDs(m); !ids["alpha"] || ids["beta"] {
		t.Fatalf("context narrowing not restored after pop, got %v", ids)
	}
}

// tagScopeFixture builds a work/home project pair whose tag catalogs and
// task-tag counts both differ, so a tag selector built from every loaded project
// is distinguishable from one built from the active Context's projects — by tag
// membership (alpha-* vs beta-*) and by the count of the shared tag.
func tagScopeFixture() (alpha, beta *models.Project) {
	alpha = &models.Project{
		ID: "alpha", Name: "Alpha", Context: "work",
		Tags:  []string{"alpha-catalog"},
		Tasks: []*models.Task{{ID: "alpha-1", Title: "Alpha task", Tags: []string{"alpha-task", "shared"}}},
	}
	alpha.BuildTaskTree()

	beta = &models.Project{
		ID: "beta", Name: "Beta", Context: "home",
		Tags: []string{"beta-catalog"},
		Tasks: []*models.Task{
			{ID: "beta-1", Title: "Beta task", Tags: []string{"beta-task", "shared"}},
			{ID: "beta-2", Title: "Beta task 2", Tags: []string{"shared"}},
		},
	}
	beta.BuildTaskTree()
	return alpha, beta
}

var (
	allContextTagChoices  = []string{"alpha-catalog:0", "alpha-task:1", "beta-catalog:0", "beta-task:1", "shared:3"}
	workContextTagChoices = []string{"alpha-catalog:0", "alpha-task:1", "shared:1"}
)

func tagFilterMsgFromCmd(t *testing.T, cmd tea.Cmd) ShowTagFilterMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("tag selector entry point produced no command")
	}
	raw := cmd()
	msg, ok := raw.(ShowTagFilterMsg)
	if !ok {
		t.Fatalf("tag selector msg = %T, want ShowTagFilterMsg", raw)
	}
	return msg
}

// TestContextScopeNarrowsKeyboardTagSelector asserts both keyboard tag-picker
// entry points offer only the tags of the active Context's projects, and count
// only that set's task usages. The All-context control runs first so a narrowed
// expectation cannot pass vacuously.
func TestContextScopeNarrowsKeyboardTagSelector(t *testing.T) {
	alpha, beta := tagScopeFixture()

	entries := []struct {
		name string
		open func(*testing.T, *OutlinerModel) ShowTagFilterMsg
	}{
		{"F#", func(t *testing.T, m *OutlinerModel) ShowTagFilterMsg {
			_, cmd := m.handleFilterPrefix("#")
			return tagFilterMsgFromCmd(t, cmd)
		}},
		{"T", func(t *testing.T, m *OutlinerModel) ShowTagFilterMsg {
			_, cmd := m.Update(syntheticKey('T'))
			return tagFilterMsgFromCmd(t, cmd)
		}},
	}

	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			m := contextScopeOutliner(testConfigWithProjects(alpha, beta), alpha, beta)

			if got := summarizeTagInfos(entry.open(t, m).Tags); !reflect.DeepEqual(got, allContextTagChoices) {
				t.Fatalf("All-context %s tags = %v, want %v", entry.name, got, allContextTagChoices)
			}

			m.applyContextScope(RegularContextScope("work"))
			if got := summarizeTagInfos(entry.open(t, m).Tags); !reflect.DeepEqual(got, workContextTagChoices) {
				t.Fatalf("work-context %s tags = %v, want %v", entry.name, got, workContextTagChoices)
			}
		})
	}
}

// TestContextScopeNarrowsCommandPaletteTagSelector is the palette half of the
// same parity: the "filter by tag" command must offer the identical scoped
// catalog the keyboard does, never a tag that only exists outside the Context.
func TestContextScopeNarrowsCommandPaletteTagSelector(t *testing.T) {
	alpha, beta := tagScopeFixture()

	app := newTestAppModel()
	setTestProjects(app.runtime, []*models.Project{alpha, beta})
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.SetSize(120, 40)
	app.crossOutliner.loadedProjects = []*models.Project{alpha, beta}
	app.pushView(ViewCrossProjectOutliner)

	open := func(t *testing.T) ShowTagFilterMsg {
		t.Helper()
		command := findCommand(app.filterCommands(), "filter by tag")
		if command == nil {
			t.Fatal("missing 'filter by tag' palette command")
		}
		raw := command.Handler()
		msg, ok := raw.(ShowTagFilterMsg)
		if !ok {
			t.Fatalf("palette msg = %T, want ShowTagFilterMsg", raw)
		}
		return msg
	}

	if got := summarizeTagInfos(open(t).Tags); !reflect.DeepEqual(got, allContextTagChoices) {
		t.Fatalf("All-context palette tags = %v, want %v", got, allContextTagChoices)
	}

	app.crossOutliner.applyContextScope(RegularContextScope("work"))
	if got := summarizeTagInfos(open(t).Tags); !reflect.DeepEqual(got, workContextTagChoices) {
		t.Fatalf("work-context palette tags = %v, want %v", got, workContextTagChoices)
	}
}

// TestRefreshContextScopeDropsArchivedMembers asserts the resolved member set is
// recomputed against the catalog, so archiving a member removes it from scope
// instead of leaving a stale admitted ID behind.
func TestRefreshContextScopeDropsArchivedMembers(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1")
	alpha.Context = "work"
	beta := scopeTestProject("beta", "Beta", "1")
	beta.Context = "work"

	runtime := testConfigWithProjects(alpha, beta)
	m := contextScopeOutliner(runtime, alpha, beta)
	m.applyContextScope(RegularContextScope("work"))
	if ids := scopeVisibleProjectIDs(m); !ids["alpha"] || !ids["beta"] {
		t.Fatalf("both work projects should be in scope, got %v", ids)
	}

	beta.Archived = true
	setTestProjects(runtime, []*models.Project{alpha, beta})
	m.refreshContextScopeProjects()
	m.crossForceRebuild = true
	m.rebuildRows()

	if ids := scopeVisibleProjectIDs(m); !ids["alpha"] || ids["beta"] {
		t.Fatalf("archived member still in scope, got %v", ids)
	}
}
