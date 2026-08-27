package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// applyFilterForComposition drives one facet through the production handler, so
// every assertion below reads the same path a chord press reaches.
func applyFilterForComposition(
	t *testing.T,
	outliner *OutlinerModel,
	id OperationID,
	value any,
) string {
	t.Helper()
	result := (&AppModel{crossOutliner: outliner}).executeFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: id},
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
	applyFilterForComposition(t, outliner, operationFilterTags, []string{"backend"})
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
	applyFilterForComposition(t, outliner, operationFilterTags, []string{"urgent"})
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("disagreeing sigil and facet = %v, want the AND gate to admit nothing", got)
	}
	if cause := outliner.classifyEmptyCause(); cause != outlinerEmptySearchFiltered {
		t.Fatalf("empty cause = %v, want the facet-filtered cause", cause)
	}
}

// TestSearchAdmitsTerminalWorkOnlyUnderAnExplicitTerminalFacet proves the
// widening in buildSearchProjectionRows is keyed on hasExplicitTerminalFilter
// and nothing else: `f$done` reveals completed work with no `$done` sigil typed,
// while a nonterminal state facet leaves the default exclusion in place.
func TestSearchAdmitsTerminalWorkOnlyUnderAnExplicitTerminalFacet(t *testing.T) {
	outliner := searchGateOutliner()
	outliner.SetSearchQuery("auth")
	if got := searchVisibleTaskIDs(outliner); slices.Contains(got, "finished") {
		t.Fatalf("plain search = %v, want completed work excluded by default", got)
	}

	// The facet applies rather than refusing, which is the composition itself.
	if status := applyFilterForComposition(t, outliner,
		operationFilterStates, []string{string(models.StateDone)}); status != "Filtering 1 state(s)" {
		t.Fatalf("f$done status under search = %q, want the applied facet's own sentence", status)
	}
	if !outliner.hasExplicitTerminalFilter() {
		t.Fatal("f$done did not register as an explicit terminal filter")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"finished"}) {
		t.Fatalf("search under f$done = %v, want the completed auth task alone", got)
	}

	applyFilterForComposition(t, outliner, operationFilterStates, []string{string(models.StateBacklog)})
	if outliner.hasExplicitTerminalFilter() {
		t.Fatal("f$backlog registered as an explicit terminal filter")
	}
	if got := searchVisibleTaskIDs(outliner); slices.Contains(got, "finished") {
		t.Fatalf("search under f$backlog = %v, want completed work excluded again", got)
	}
}

// TestSearchSortReordersWithinRelevanceTiersOnly pins the comparator's bound: a
// sort is free inside a tier and powerless across tiers, so the highest-priority
// task in the description tier still ranks below every title hit.
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
	outliner.SetSearchQuery("needle")
	baseline := searchVisibleTaskIDs(outliner)
	if !slices.Equal(baseline, []string{"title-low", "title-high", "described"}) {
		t.Fatalf("unsorted search = %v, want both title hits ranked above the description hit", baseline)
	}

	result := app.executeSortOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationSortPriority},
	})
	if result.Err != nil {
		t.Fatalf("sort by priority under search failed: %v", result.Err)
	}
	if outliner.sort.Field != SortByPriority || !outliner.sort.Ascending {
		t.Fatalf("sort state = %+v, want ascending priority installed directly", outliner.sort)
	}
	// The title tier reorders; the description hit stays last despite carrying
	// the same High priority that floated title-high to the top of its own tier.
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got,
		[]string{"title-high", "title-low", "described"}) {
		t.Fatalf("sorted search = %v, want a within-tier reorder that never crosses a tier", got)
	}

	result = app.executeSortOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationSortClear},
	})
	if result.Status != "Sort cleared" {
		t.Fatalf("clear sort status = %q, want the cleared sentence", result.Status)
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, baseline) {
		t.Fatalf("cleared search order = %v, want the ranked baseline %v", got, baseline)
	}
}

// TestPopulationFacetsStillRefuseUnderASearchProjection keeps the F* half of the
// split honest at both ends: the chord stops resolving while a projection is
// live, and the handler that a stale route could still reach refuses by name.
func TestPopulationFacetsStillRefuseUnderASearchProjection(t *testing.T) {
	app := newFilterChordTestApp(t)
	ctx := app.operationCommandContext()
	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		InputStrategySimplified, []string{"F", "#"}, ctx,
	); resolution != ChordTerminal {
		t.Fatalf("F# resolution outside search = %v, want a terminal chord", resolution)
	}

	app.crossOutliner.SetSearchQuery("critical")
	if !app.crossOutliner.hasSearchProjection() {
		t.Fatal("the search projection did not start")
	}
	searchCtx := app.operationCommandContext()
	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		InputStrategySimplified, []string{"F", "#"}, searchCtx,
	); resolution != ChordNoMatch {
		t.Fatalf("F# resolution under search = %v, want no match", resolution)
	}
	// The row facets keep resolving on the same context, which is the whole point
	// of the split: one family composes and the other does not.
	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		InputStrategySimplified, []string{"f", "#"}, searchCtx,
	); resolution != ChordTerminal {
		t.Fatalf("f# resolution under search = %v, want a terminal chord", resolution)
	}

	result := app.executeProjectFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationFilterProjectTags},
		Value:      []string{"backend"},
	})
	if result.Status != "Exit search before changing filters" {
		t.Fatalf("F# handler under search = %+v, want the refusal", result)
	}
	if len(app.crossOutliner.projectFilter.Tags) != 0 {
		t.Fatalf("the refused population facet still wrote %v", app.crossOutliner.projectFilter.Tags)
	}
}

// TestSearchProjectionNeverAppliesLensNarrowing is an assertion over the branch
// order, not only the rows: buildCanonicalModeRows returns at the search branch
// above compileLensNarrowing, so a live projection leaves lensGate untouched.
// Session 9's candidate boundary is therefore not a place to hang lens logic.
func TestSearchProjectionNeverAppliesLensNarrowing(t *testing.T) {
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
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"needle"}) {
		t.Fatalf("lens-narrowed rows = %v, want the narrowing applied on the canonical path", got)
	}

	// Nil it deliberately: only compileLensNarrowing writes this field, so a
	// still-nil gate after the rebuild proves the search branch returned first.
	outliner.lensGate = nil
	outliner.SetSearchQuery("beta")
	if !outliner.hasSearchProjection() {
		t.Fatal("the search projection did not start under a lens")
	}
	if outliner.lensGate != nil {
		t.Fatal("the search branch compiled the lens narrowing")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"other"}) {
		t.Fatalf("search under a lens = %v, want the query alone deciding membership", got)
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
