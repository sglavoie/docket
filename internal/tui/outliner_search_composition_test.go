package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

// applyFilterForComposition drives one facet through the production handler, so
// every assertion below reads the same path a chord press reaches.
func applyFilterForComposition(
	t *testing.T,
	outliner *OutlinerModel,
	id operation.OperationID,
	value any,
) string {
	t.Helper()
	result := (&AppModel{crossOutliner: outliner}).executeFilterOperation(operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: id},
		Value:      value,
	})
	if result.Err != nil {
		t.Fatalf("filter %s(%v) failed: %v", id, value, result.Err)
	}
	return result.Status
}

// TestSearchFacetsGateCandidatesBeforeScoringAndAndWithTheSigils pins the shape
// of the composition rather than only its result: the facet narrows
// searchCandidateBoundary, which is the pre-scoring channel the sigils already
// use, so a facet can never be a post-ranking trim of rows that were scored
// anyway.
func TestSearchFacetsGateCandidatesBeforeScoringAndAndWithTheSigils(t *testing.T) {
	outliner := searchGateOutliner()

	outliner.SetSearchQuery("auth")
	baseline := searchVisibleTaskIDs(outliner)
	for _, id := range []string{"priority-one", "beta-task"} {
		if !slices.Contains(baseline, id) {
			t.Fatalf("unfaceted search = %v, want it to contain %s", baseline, id)
		}
	}

	// The tag facet is an AND gate: only the backend-tagged auth match survives,
	// and the backend-tagged task that does not match the query stays out.
	applyFilterForComposition(t, outliner, operation.OperationFilterTags, []string{"backend"})
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"beta-task"}) {
		t.Fatalf("search under the tag facet = %v, want the backend-tagged auth match alone", got)
	}

	// The gate is applied to the candidate set, before any task is scored.
	candidates := outliner.searchCandidateBoundary(
		outliner.projectsInCatalogOrder(outliner.workModeScopeProjects()))
	if candidates == nil {
		t.Fatal("an active facet left the candidate boundary scope-wide")
	}
	for projectID, tasks := range candidates {
		for _, task := range tasks {
			if !slices.Contains(task.Tags, "backend") {
				t.Fatalf("candidate boundary for %s admitted untagged task %s", projectID, task.ID)
			}
		}
	}

	// A sigil and a facet AND together rather than either winning.
	outliner.SetSearchQuery("#backend auth")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"beta-task"}) {
		t.Fatalf("agreeing sigil and facet = %v, want the one task both admit", got)
	}
	applyFilterForComposition(t, outliner, operation.OperationFilterTags, []string{"urgent"})
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("disagreeing sigil and facet = %v, want the AND gate to admit nothing", got)
	}
	if cause := outliner.classifyEmptyCause(); cause != outlinerEmptySearchFiltered {
		t.Fatalf("empty cause = %v, want the facet-filtered cause", cause)
	}
}

// TestSearchAdmitsTerminalWorkOnlyWhenTheQuerySaysSo proves terminal admission
// is one rule read through one accessor: `f$done` reveals completed work with no
// `$done` sigil typed because the facets are part of the query, while a
// nonterminal state facet leaves the default exclusion in place.
func TestSearchAdmitsTerminalWorkOnlyWhenTheQuerySaysSo(t *testing.T) {
	outliner := searchGateOutliner()
	searchRankedList(outliner, "auth")
	if got := searchVisibleTaskIDs(outliner); slices.Contains(got, "finished") {
		t.Fatalf("plain search = %v, want completed work excluded by default", got)
	}

	// The facet applies rather than refusing, which is the composition itself.
	if status := applyFilterForComposition(t, outliner,
		operation.OperationFilterStates, []string{string(models.StateDone)}); status != "Filtering 1 state(s)" {
		t.Fatalf("f$done status under search = %q, want the applied facet's own sentence", status)
	}
	if !outliner.admitsTerminalPopulation() {
		t.Fatal("f$done did not open the terminal population")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"finished"}) {
		t.Fatalf("search under f$done = %v, want the completed auth task alone", got)
	}

	applyFilterForComposition(t, outliner, operation.OperationFilterStates, []string{string(models.StateBacklog)})
	if outliner.admitsTerminalPopulation() {
		t.Fatal("f$backlog opened the terminal population")
	}
	if got := searchVisibleTaskIDs(outliner); slices.Contains(got, "finished") {
		t.Fatalf("search under f$backlog = %v, want completed work excluded again", got)
	}
}

// TestQueryAloneNeverFreezesAnAxis is D7 stated as a test: with a query held,
// the mode changes, the population facets apply, and the rows stay the mode's
// own lanes rather than a flat list nobody asked for.
func TestQueryAloneNeverFreezesAnAxis(t *testing.T) {
	outliner := searchGateOutliner()
	outliner.SetSearchQuery("auth")
	if outliner.showsRankedList() {
		t.Fatal("typing a query opened the flat ranked list")
	}
	if _, _ = outliner.selectOutlinerMode(chrome.ModeBacklog); outliner.primaryMode != chrome.ModeBacklog {
		t.Fatalf("mode under a held query = %v, want the mode change to apply", outliner.primaryMode)
	}
	if !outliner.hasSearchProjection() {
		t.Fatal("changing the mode dropped the held query")
	}
	if status := applyFilterForComposition(t, outliner,
		operation.OperationFilterStates, []string{string(models.StateBacklog)}); status == "" {
		t.Fatal("a row facet under a held query produced no status")
	}
}

// TestSearchSortReordersWithinRelevanceTiersOnly pins the comparator's bound: a
// sort is free inside a tier and powerless across tiers, so the highest-priority
// task in the description tier still ranks below every title hit. Tiers are the
// flat ranked list's own ordering, so that is the presentation under test.
func TestSearchSortReordersWithinRelevanceTiersOnly(t *testing.T) {
	titleLow := primaryRowsTask("title-low", models.StateBacklog)
	titleLow.Title = "Needle groundwork"
	titleLow.Priority = models.PriorityLow
	titleHigh := primaryRowsTask("title-high", models.StateBacklog)
	titleHigh.Title = "Needle rollout"
	titleHigh.Priority = models.PriorityHigh
	described := primaryRowsTask("described", models.StateBacklog)
	described.Title = "Unrelated heading"
	described.Description = "buried needle in the body"
	described.Priority = models.PriorityHigh
	alpha := primaryRowsProject("alpha", titleLow, titleHigh, described)

	outliner := scopeTestOutliner(alpha)
	app := &AppModel{crossOutliner: outliner}
	searchRankedList(outliner, "needle")
	baseline := searchVisibleTaskIDs(outliner)
	if !slices.Equal(baseline, []string{"title-low", "title-high", "described"}) {
		t.Fatalf("unsorted search = %v, want both title hits ranked above the description hit", baseline)
	}

	result := app.executeSortOperation(operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationSortPriority},
	})
	if result.Err != nil {
		t.Fatalf("sort by priority under search failed: %v", result.Err)
	}
	if outliner.sort.Field != query.SortByPriority || !outliner.sort.Ascending {
		t.Fatalf("sort state = %+v, want ascending priority installed directly", outliner.sort)
	}
	// The title tier reorders; the description hit stays last despite carrying
	// the same High priority that floated title-high to the top of its own tier.
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got,
		[]string{"title-high", "title-low", "described"}) {
		t.Fatalf("sorted search = %v, want a within-tier reorder that never crosses a tier", got)
	}

	result = app.executeSortOperation(operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationSortClear},
	})
	if result.Status != "Sort cleared" {
		t.Fatalf("clear sort status = %q, want the cleared sentence", result.Status)
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, baseline) {
		t.Fatalf("cleared search order = %v, want the ranked baseline %v", got, baseline)
	}
}

// TestPopulationFacetsRefuseOnlyUnderTheRankedList keeps the F* half of the
// split honest at both ends: the chord stops resolving while the flat list
// stands, and the handler that a stale route could still reach refuses by name.
// A query that is merely narrowing freezes neither, which is D7.
func TestPopulationFacetsRefuseOnlyUnderTheRankedList(t *testing.T) {
	app := newFilterChordTestApp(t)
	ctx := app.operationCommandContext()
	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		operation.InputStrategySimplified, []string{"F", "#"}, ctx,
	); resolution != operation.ChordTerminal {
		t.Fatalf("F# resolution outside search = %v, want a terminal chord", resolution)
	}

	app.crossOutliner.SetSearchQuery("critical")
	if !app.crossOutliner.hasSearchProjection() {
		t.Fatal("the search projection did not start")
	}
	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		operation.InputStrategySimplified, []string{"F", "#"}, app.operationCommandContext(),
	); resolution != operation.ChordTerminal {
		t.Fatalf("F# resolution under a narrowing query = %v, want it still live", resolution)
	}

	app.crossOutliner.toggleRankedList()
	searchCtx := app.operationCommandContext()
	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		operation.InputStrategySimplified, []string{"F", "#"}, searchCtx,
	); resolution != operation.ChordNoMatch {
		t.Fatalf("F# resolution under search = %v, want no match", resolution)
	}
	// The row facets keep resolving on the same context, which is the whole point
	// of the split: one family composes and the other does not.
	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		operation.InputStrategySimplified, []string{"f", "#"}, searchCtx,
	); resolution != operation.ChordTerminal {
		t.Fatalf("f# resolution under search = %v, want a terminal chord", resolution)
	}

	result := app.executeProjectFilterOperation(operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationFilterProjectTags},
		Value:      []string{"backend"},
	})
	if result.Status != "Exit search before changing filters" {
		t.Fatalf("F# handler under search = %+v, want the refusal", result)
	}
	if len(app.crossOutliner.projectFilter.Tags) != 0 {
		t.Fatalf("the refused population facet still wrote %v", app.crossOutliner.projectFilter.Tags)
	}
}

// TestOneNarrowingQueryCompilesAtATime is an assertion over the compile path,
// not only the rows: both carriers reach compileActiveNarrowing, and a lens owns
// the query while one is active, so exactly one gate is ever written per
// rebuild. Session 9's candidate boundary is therefore not a place to hang lens
// logic.
func TestOneNarrowingQueryCompilesAtATime(t *testing.T) {
	needle := primaryRowsTask("needle", models.StateBacklog)
	needle.Title = "Needle groundwork"
	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Beta rollout"
	alpha := primaryRowsProject("alpha", needle, other)

	outliner := scopeTestOutliner(alpha)
	outliner.activeLens = &activeLensFrame{lensID: "lens", name: "Groundwork"}
	outliner.setLensQuery("needle")
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	if outliner.lensGate == nil {
		t.Fatal("the canonical path did not compile the lens narrowing")
	}
	if outliner.searchGate != nil {
		t.Fatal("the lens rebuild also wrote the search gate")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"needle"}) {
		t.Fatalf("lens-narrowed rows = %v, want the narrowing applied on the canonical path", got)
	}

	// Without a lens the same call site compiles the search query instead, and
	// the rows are still the mode's own.
	outliner.activeLens = nil
	outliner.lensGate = nil
	outliner.setLensQuery("")
	outliner.SetSearchQuery("beta")
	if outliner.lensGate != nil {
		t.Fatal("a lens-free rebuild compiled a lens narrowing")
	}
	if outliner.searchGate == nil {
		t.Fatal("the search query was never compiled")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"other"}) {
		t.Fatalf("narrowing search = %v, want the query alone deciding membership", got)
	}
}

// TestSlashUnderALensEditsTheLensQueryRatherThanStartingASearch is the other
// half of that boundary: the key never mints a scope-wide projection beneath a
// lens, so no rebuild can reach both narrowings at once.
func TestSlashUnderALensEditsTheLensQueryRatherThanStartingASearch(t *testing.T) {
	task := primaryRowsTask("task", models.StateBacklog)
	task.Title = "Needle groundwork"
	outliner := scopeTestOutliner(primaryRowsProject("alpha", task))
	outliner.activeLens = &activeLensFrame{lensID: "lens", name: "Groundwork"}
	outliner.setLensQuery("needle")

	_ = outliner.ActivateSearch()
	if !outliner.editingLensQuery() {
		t.Fatal("/ under a lens did not open the lens query editor")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("/ under a lens started a scope-wide search projection")
	}
	if got := outliner.SearchInputValue(); got != "needle" {
		t.Fatalf("lens query editor seeded with %q, want the lens's own query", got)
	}
	if got := outliner.SearchQuery(); got != "" {
		t.Fatalf("ordinary search query = %q, want it left empty under a lens", got)
	}

	_, _ = outliner.handleKeyMsg(syntheticSpecialKey(tea.KeyEsc))
	if outliner.editingLensQuery() {
		t.Fatal("Esc did not close the lens query editor")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("Esc left a search projection behind")
	}
}
