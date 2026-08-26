package tui

import (
	"fmt"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// lensLifecycleOutliner builds one two-project cross-project scope whose tasks
// span the states, priorities, and titles the lifecycle assertions narrow on.
func lensLifecycleOutliner() *OutlinerModel {
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	backlog.Title = "Needle in the backlog"
	doing := primaryRowsTask("doing", models.StateDoing)
	doing.Title = "Unrelated in flight"
	waiting := primaryRowsTask("waiting", models.StateWaiting)
	waiting.Title = "Needle held up"
	waiting.Priority = models.PriorityHigh
	alpha := primaryRowsProject("alpha", backlog, doing, waiting)

	betaWaiting := primaryRowsTask("beta-waiting", models.StateWaiting)
	betaWaiting.Title = "Beta needle"
	betaInbox := primaryRowsTask("beta-inbox", models.StateInbox)
	betaInbox.Title = "Beta triage"
	beta := primaryRowsProject("beta", betaWaiting, betaInbox)

	return scopeTestOutliner(alpha, beta)
}

// waitingLens is the starter-shaped definition the lifecycle tests apply: a
// named mode, a real facet, and a narrowing query.
func waitingLens(id, name, query string) config.NamedLens {
	state := lensState{
		mode:          ModeAll,
		query:         query,
		filter:        NewFilterState(),
		projectFilter: newProjectFilterState(),
		sort:          NewSortState(),
	}
	states, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		panic(err)
	}
	state.filter.setStateFilter(states, true)
	return state.namedLens(id, name)
}

// lensTestID mints the canonical lowercase hyphenated UUID form the config
// layer is the sole judge of, so the lifecycle tests exercise the same identity
// rule a persisted lens does.
func lensTestID(n int) string {
	return fmt.Sprintf("6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6d%02d", n)
}

func applyLensOrFatal(t *testing.T, outliner *OutlinerModel, lens config.NamedLens) tea.Cmd {
	t.Helper()
	cmd, err := outliner.ApplyNamedLens(lens)
	if err != nil {
		t.Fatalf("ApplyNamedLens(%q): %v", lens.Name, err)
	}
	return cmd
}

func TestApplyNamedLensInstallsSavedProjectionAndEscRestoresBaseline(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.selectOutlinerMode(ModeInbox)
	outliner.treeSort = SortState{Field: SortByTitle, Ascending: true}
	outliner.collapsedSections["baseline-fold"] = true
	outliner.peekMode = true
	outliner.selectedTaskIdentities[TaskIdentity{ProjectID: "beta", TaskID: "beta-inbox"}] = true
	baselineRows := searchVisibleTaskIDs(outliner)

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	if !outliner.hasActiveLens() {
		t.Fatal("expected an active lens after ApplyNamedLens")
	}
	if outliner.primaryMode != ModeAll {
		t.Fatalf("lens mode = %v, want %v", outliner.primaryMode, ModeAll)
	}
	if outliner.lensQuery != "needle" {
		t.Fatalf("lens query = %q, want %q", outliner.lensQuery, "needle")
	}
	if !outliner.filter.hasStateFilter() {
		t.Fatal("expected the lens state facet to be installed")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"waiting", "beta-waiting"}) {
		t.Fatalf("lens rows = %v, want [waiting beta-waiting]", got)
	}
	if outliner.hasSearchProjection() {
		t.Fatal("a lens must not create an ordinary search projection")
	}
	// Peek is the view's, not the lens's, so it survives the application and is
	// closed here to prove restoration puts it back rather than merely leaving it.
	outliner.peekMode = false

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("Esc left the lens active")
	}
	if outliner.primaryMode != ModeInbox {
		t.Fatalf("restored mode = %v, want %v", outliner.primaryMode, ModeInbox)
	}
	if outliner.lensQuery != "" {
		t.Fatalf("restored lens query = %q, want empty", outliner.lensQuery)
	}
	if outliner.filter.hasStateFilter() {
		t.Fatal("restoration kept the lens state facet")
	}
	if outliner.treeSort.Field != SortByTitle || !outliner.treeSort.Ascending {
		t.Fatalf("restored treeSort = %+v, want title ascending", outliner.treeSort)
	}
	if !outliner.collapsedSections["baseline-fold"] {
		t.Fatal("restoration dropped the baseline fold state")
	}
	if !outliner.peekMode {
		t.Fatal("restoration dropped Peek")
	}
	if !outliner.selectedTaskIdentities[TaskIdentity{ProjectID: "beta", TaskID: "beta-inbox"}] {
		t.Fatal("restoration dropped the baseline selection")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, baselineRows) {
		t.Fatalf("restored rows = %v, want %v", got, baselineRows)
	}
}

func TestApplyNamedLensRetainsScopeFocusAndCursorIdentity(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.pushProjectScope("alpha")
	outliner.crossFocusStack = []CrossProjectFocusEntry{{TaskID: "backlog", ProjectID: "alpha"}}

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))

	if outliner.scopedProjectID != "alpha" {
		t.Fatalf("scoped project = %q, want alpha", outliner.scopedProjectID)
	}
	if len(outliner.crossFocusStack) != 1 {
		t.Fatalf("focus stack depth = %d, want 1", len(outliner.crossFocusStack))
	}
	if outliner.pendingLens != nil {
		t.Fatal("a scoped application must not start a project load")
	}
}

func TestApplyNamedLensKeepsTheCursorOnASurvivingIdentity(t *testing.T) {
	outliner := lensLifecycleOutliner()
	if !outliner.moveCursorToProjectTask("waiting", "alpha") {
		t.Fatal("cursor could not be seated on the waiting task")
	}

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))

	if identity := outliner.currentTaskIdentity(); identity.TaskID != "waiting" {
		t.Fatalf("cursor identity = %+v, want the surviving waiting task", identity)
	}
}

func TestApplyNamedLensFallsBackToTheNearestSurvivorWhenTheCursorTaskLeaves(t *testing.T) {
	outliner := lensLifecycleOutliner()
	if !outliner.moveCursorToProjectTask("doing", "alpha") {
		t.Fatal("cursor could not be seated on the doing task")
	}

	// The Waiting lens drops the cursor's own task, so the existing
	// nearest-survivor fallback has to seat it on a task row that remains.
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))

	identity := outliner.currentTaskIdentity()
	if identity.TaskID != "waiting" && identity.TaskID != "beta-waiting" {
		t.Fatalf("cursor identity = %+v, want a surviving lens row", identity)
	}
}

func TestApplyNamedLensUnderScopeRecordsProjectFacetsInert(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.pushProjectScope("alpha")

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens.Projects.ParkedMode = config.LensPopulationInclude
	applyLensOrFatal(t, outliner, lens)

	if outliner.projectFilter.ParkedMode != projectPopulationInclude {
		t.Fatal("the scoped application did not record the lens project facets")
	}
	if outliner.pendingLens != nil {
		t.Fatal("the recorded facets must stay inert under a scope: no load may start")
	}
	if ids := taskRowProjectIDs(outliner); len(ids) != 1 || ids[0] != "alpha" {
		t.Fatalf("scoped population = %v, want [alpha]", ids)
	}
}

func TestSwitchingLensesRetainsTheOriginalBaseline(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.selectOutlinerMode(ModeInbox)

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))
	first := outliner.activeLens.origin
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(2), "Held", ""))

	if outliner.activeLens.name != "Held" {
		t.Fatalf("active lens = %q, want Held", outliner.activeLens.name)
	}
	if outliner.activeLens.origin.presentation.mode != first.presentation.mode {
		t.Fatal("a switch replaced the original restoration point")
	}
	if outliner.lensQuery != "" {
		t.Fatalf("switch kept the previous narrowing query %q", outliner.lensQuery)
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("one Esc must leave the lens entirely")
	}
	if outliner.primaryMode != ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", outliner.primaryMode, ModeInbox)
	}
}

func TestApplyNamedLensFromAcceptedSearchRestoresThatSearch(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.SetSearchQuery("needle")
	if !outliner.hasSearchProjection() {
		t.Fatal("expected an ordinary search projection")
	}
	searchRows := searchVisibleTaskIDs(outliner)

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))
	if outliner.hasSearchProjection() {
		t.Fatal("the lens must replace the ordinary search projection")
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if !outliner.hasSearchProjection() {
		t.Fatal("restoration did not hand the search projection back")
	}
	if outliner.SearchQuery() != "needle" {
		t.Fatalf("restored search query = %q, want needle", outliner.SearchQuery())
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, searchRows) {
		t.Fatalf("restored search rows = %v, want %v", got, searchRows)
	}
	// The search frame is whole again, so its own Esc still restores the ordinary
	// projection underneath it.
	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the restored search projection")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("the restored search projection did not dismiss")
	}
}

func TestLensModifiedTracksOwnedAxesOnly(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	if outliner.lensModified() {
		t.Fatal("a freshly applied lens is not modified")
	}
	if got := outliner.activeLensLabel(); got != "Lens: Waiting" {
		t.Fatalf("label = %q, want %q", got, "Lens: Waiting")
	}

	// Navigation, folds, selection, and Peek are the view, not the lens.
	outliner.cursor = 0
	outliner.collapsedSections["some-section"] = true
	outliner.peekMode = true
	outliner.selectedTaskIdentities[TaskIdentity{ProjectID: "alpha", TaskID: "waiting"}] = true
	if outliner.lensModified() {
		t.Fatal("cursor, folds, selection, and Peek must not mark a lens modified")
	}

	outliner.filter.Priority = int(models.PriorityHigh)
	if !outliner.lensModified() {
		t.Fatal("a facet change must mark the lens modified")
	}
	if got := outliner.activeLensLabel(); got != "Lens: Waiting*" {
		t.Fatalf("label = %q, want %q", got, "Lens: Waiting*")
	}

	outliner.filter.Priority = 0
	if outliner.lensModified() {
		t.Fatal("reverting the facet must clear the marker")
	}
}

func TestLensModifiedTracksModeQuerySortAndProjectFacets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*OutlinerModel)
	}{
		{"mode", func(o *OutlinerModel) { o.selectOutlinerMode(ModeInbox) }},
		{"query", func(o *OutlinerModel) { o.setLensQuery("other") }},
		{"sort", func(o *OutlinerModel) { o.treeSort = SortState{Field: SortByTitle, Ascending: true} }},
		{"project facets", func(o *OutlinerModel) { o.projectFilter.Untagged = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outliner := lensLifecycleOutliner()
			applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))
			tc.modify(outliner)
			if !outliner.lensModified() {
				t.Fatalf("changing %s must mark the lens modified", tc.name)
			}
		})
	}
}

func TestOrdinaryFiltersAndSortRemainEditableUnderALensQuery(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	outliner.filter.Priority = int(models.PriorityHigh)
	outliner.treeSort = SortState{Field: SortByTitle, Ascending: true}
	outliner.sort = outliner.treeSort
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()

	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"waiting"}) {
		t.Fatalf("rows under lens query plus priority facet = %v, want [waiting]", got)
	}
	if outliner.lensQuery != "needle" {
		t.Fatalf("editing filters disturbed the narrowing query: %q", outliner.lensQuery)
	}
	if !outliner.hasActiveLens() {
		t.Fatal("editing filters must not dismiss the lens")
	}
}

func TestApplyNamedLensRefusesInvalidDefinitionsWithoutTouchingTheView(t *testing.T) {
	for _, tc := range []struct {
		name string
		lens func() config.NamedLens
	}{
		{"unknown mode", func() config.NamedLens {
			lens := waitingLens(lensTestID(1), "Waiting", "")
			lens.Mode = "nonsense"
			return lens
		}},
		{"unknown facet token", func() config.NamedLens {
			lens := waitingLens(lensTestID(1), "Waiting", "")
			lens.Tasks.DueDate = "sometime"
			return lens
		}},
		{"unhonorable query", func() config.NamedLens {
			return waitingLens(lensTestID(1), "Waiting", "!9")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outliner := lensLifecycleOutliner()
			outliner.selectOutlinerMode(ModeInbox)
			before := searchVisibleTaskIDs(outliner)

			cmd, err := outliner.ApplyNamedLens(tc.lens())
			if err == nil {
				t.Fatal("expected the definition to refuse")
			}
			if cmd != nil {
				t.Fatal("a refusal must return no command")
			}
			if outliner.hasActiveLens() || outliner.pendingLens != nil {
				t.Fatal("a refusal must leave no lens state behind")
			}
			if outliner.primaryMode != ModeInbox || outliner.lensQuery != "" {
				t.Fatal("a refusal must leave the current projection untouched")
			}
			if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, before) {
				t.Fatalf("rows = %v, want the untouched %v", got, before)
			}
		})
	}
}

func TestLensProjectLoadPublishesOnlyOnTheMatchingGeneration(t *testing.T) {
	outliner := lensLifecycleOutliner()
	baseline := slices.Clone(outliner.loadedProjects)

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens.Projects.ParkedMode = config.LensPopulationOnly
	cmd := applyLensOrFatal(t, outliner, lens)
	if cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	if outliner.hasActiveLens() {
		t.Fatal("nothing may publish before the load lands")
	}
	if outliner.projectFilter.ParkedMode == projectPopulationOnly {
		t.Fatal("the candidate filter must not be written to the live model")
	}
	if !slices.Equal(outliner.loadedProjects, baseline) {
		t.Fatal("the live population changed before the load landed")
	}

	generation := outliner.pendingLens.generation
	loaded := []*models.Project{primaryRowsProject("gamma", primaryRowsTask("gamma-task", models.StateWaiting))}

	// A stale result from an earlier generation publishes nothing.
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: loaded, generation: generation - 1,
	}); published {
		t.Fatal("a stale generation published a lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("a stale result installed a lens")
	}

	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: loaded, generation: generation,
	}); !published {
		t.Fatal("the matching generation did not publish")
	}
	if !outliner.hasActiveLens() {
		t.Fatal("the matching generation did not install the lens")
	}
	if outliner.projectFilter.ParkedMode != projectPopulationOnly {
		t.Fatal("publication did not install the lens project facets")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("published population = %v, want [gamma]", ids)
	}
}

func TestLensProjectLoadIsSupersededByALaterRequest(t *testing.T) {
	outliner := lensLifecycleOutliner()

	first := waitingLens(lensTestID(1), "Waiting", "")
	first.Projects.ParkedMode = config.LensPopulationOnly
	applyLensOrFatal(t, outliner, first)
	stale := outliner.pendingLens.generation

	second := waitingLens(lensTestID(2), "Included", "")
	second.Projects.ParkedMode = config.LensPopulationInclude
	applyLensOrFatal(t, outliner, second)
	live := outliner.pendingLens.generation
	if live == stale {
		t.Fatal("a superseding request must take its own generation")
	}

	loaded := []*models.Project{primaryRowsProject("gamma")}
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: loaded, generation: stale,
	}); published {
		t.Fatal("the superseded request published")
	}
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: loaded, generation: live,
	}); !published {
		t.Fatal("the newest request did not publish")
	}
	if outliner.activeLens.name != "Included" {
		t.Fatalf("published lens = %q, want Included", outliner.activeLens.name)
	}
}

func TestLensProjectLoadAbortsWhenNothingLoaded(t *testing.T) {
	outliner := lensLifecycleOutliner()
	baseline := slices.Clone(outliner.loadedProjects)

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens.Projects.ParkedMode = config.LensPopulationOnly
	applyLensOrFatal(t, outliner, lens)

	cmd, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		failedLoads: []projectLoadFailure{{ProjectID: "gamma", Detail: "failed to load project gamma: boom"}},
		generation:  outliner.pendingLens.generation,
	})
	if published {
		t.Fatal("a wholly failed load published a lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("a wholly failed load installed a lens")
	}
	if !slices.Equal(outliner.loadedProjects, baseline) {
		t.Fatal("a wholly failed load disturbed the live population")
	}
	if outliner.crossLoading {
		t.Fatal("a wholly failed load left the loading state armed")
	}
	if cmd == nil {
		t.Fatal("a wholly failed load must report why")
	}
	if _, ok := cmd().(ErrorMsg); !ok {
		t.Fatal("a wholly failed load must raise an error")
	}
}

func TestLensProjectLoadPublishesPartialResultWithWarning(t *testing.T) {
	outliner := lensLifecycleOutliner()

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens.Projects.ParkedMode = config.LensPopulationOnly
	applyLensOrFatal(t, outliner, lens)

	cmd, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects:    []*models.Project{primaryRowsProject("gamma")},
		failedLoads: []projectLoadFailure{{ProjectID: "delta", Detail: "failed to load project delta: boom"}},
		generation:  outliner.pendingLens.generation,
	})
	if !published {
		t.Fatal("a partial load must publish the projects that read")
	}
	if !outliner.hasActiveLens() {
		t.Fatal("a partial load did not install the lens")
	}
	if cmd == nil {
		t.Fatal("a partial load must report the projects it dropped")
	}
	if _, ok := cmd().(InfoStatusMsg); !ok {
		t.Fatal("a partial load must warn rather than error")
	}
}

// TestLensProjectLoadCapturesBaselineBeforeIO pins the restoration point to the
// moment the application started rather than the moment its projects landed.
// Everything the user does while the load runs is a view they are passing
// through, not the view the lens replaced.
func TestLensProjectLoadCapturesBaselineBeforeIO(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.selectOutlinerMode(ModeInbox)
	outliner.collapsedSections["baseline-fold"] = true
	baselineProjects := slices.Clone(outliner.loadedProjects)

	lens := waitingLens(lensTestID(1), "Parked", "")
	lens.Projects.ParkedMode = config.LensPopulationOnly
	if cmd := applyLensOrFatal(t, outliner, lens); cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	if got := outliner.pendingLens.origin.presentation.mode; got != ModeInbox {
		t.Fatalf("frozen baseline mode = %v, want %v", got, ModeInbox)
	}

	// The user keeps navigating and searching while the projects read. None of it
	// may rewrite the baseline the application already froze.
	outliner.selectOutlinerMode(ModeBacklog)
	delete(outliner.collapsedSections, "baseline-fold")
	outliner.SetSearchQuery("needle")

	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: []*models.Project{
			primaryRowsProject("gamma", primaryRowsTask("gamma-task", models.StateWaiting)),
		},
		generation: outliner.pendingLens.generation,
	}); !published {
		t.Fatal("the matching generation did not publish")
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("Esc left the lens active")
	}
	if outliner.primaryMode != ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-load %v", outliner.primaryMode, ModeInbox)
	}
	if !outliner.collapsedSections["baseline-fold"] {
		t.Fatal("restoration lost the fold the baseline held before the load")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("restoration installed a search projection the frozen baseline never held")
	}
	if !slices.Equal(outliner.loadedProjects, baselineProjects) {
		t.Fatal("restoration did not return the pre-load population")
	}
}

// TestLensProjectLoadSupersededBySynchronousLensClearsPendingState covers the
// one supersession that changes nothing about the generation counter: a lens
// that needs no I/O publishes immediately, and the application still loading
// behind it must be settled rather than left holding the loading state.
func TestLensProjectLoadSupersededBySynchronousLensClearsPendingState(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.selectOutlinerMode(ModeInbox)
	baselineProjects := slices.Clone(outliner.loadedProjects)

	async := waitingLens(lensTestID(1), "Parked", "")
	async.Projects.ParkedMode = config.LensPopulationOnly
	if cmd := applyLensOrFatal(t, outliner, async); cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	stale := outliner.pendingLens.generation
	// The 200ms delay has elapsed, so the spinner is on screen when the
	// synchronous lens supersedes the load.
	outliner.crossSpinnerVisible = true

	if cmd := applyLensOrFatal(t, outliner, waitingLens(lensTestID(2), "Waiting", "")); cmd != nil {
		t.Fatal("an unchanged population must not start a load")
	}
	if outliner.pendingLens != nil {
		t.Fatal("a synchronous lens left an application pending")
	}
	if outliner.crossLoading || outliner.crossSpinnerVisible {
		t.Fatal("a synchronous lens left the loading state armed")
	}
	if outliner.activeLens.pendingGeneration != 0 {
		t.Fatal("the published frame still awaits a superseded generation")
	}

	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: []*models.Project{primaryRowsProject("gamma")}, generation: stale,
	}); published {
		t.Fatal("the superseded request published")
	}
	if outliner.activeLens == nil || outliner.activeLens.name != "Waiting" {
		t.Fatal("the late result replaced the lens that superseded it")
	}
	if outliner.pendingLens != nil || outliner.crossLoading || outliner.crossSpinnerVisible {
		t.Fatal("the late result left pending or loading state armed")
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outliner.primaryMode != ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", outliner.primaryMode, ModeInbox)
	}
	if !slices.Equal(outliner.loadedProjects, baselineProjects) {
		t.Fatal("a superseded load leaked its population into the restored view")
	}
}

func TestRestoringTheBaselineDiscardsAnUnpublishedApplication(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))

	switchTo := waitingLens(lensTestID(2), "Included", "")
	switchTo.Projects.ParkedMode = config.LensPopulationInclude
	applyLensOrFatal(t, outliner, switchTo)
	generation := outliner.pendingLens.generation

	outliner.restoreLensBaseline()
	if outliner.pendingLens != nil {
		t.Fatal("restoration left an application pending")
	}

	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: []*models.Project{primaryRowsProject("gamma")}, generation: generation,
	}); published {
		t.Fatal("a load landing after restoration reinstalled a lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("a load landing after restoration installed a lens")
	}
}

func TestLensQueryEditingNarrowsWithoutASearchProjection(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))

	outliner.ActivateSearch()
	if !outliner.editingLensQuery() {
		t.Fatal("slash under a lens must edit the narrowing query")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("lens query editing must not create a search projection")
	}

	typeLensQuery(t, outliner, "beta")
	if outliner.lensQuery != "beta" {
		t.Fatalf("lens query = %q, want beta", outliner.lensQuery)
	}
	if outliner.SearchQuery() != "" {
		t.Fatalf("the ordinary search query must stay empty, got %q", outliner.SearchQuery())
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"beta-waiting"}) {
		t.Fatalf("narrowed rows = %v, want [beta-waiting]", got)
	}

	outliner.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if outliner.editingLensQuery() || outliner.IsSearchMode() {
		t.Fatal("Enter did not leave the query editor")
	}
	if outliner.lensQuery != "beta" {
		t.Fatalf("Enter did not accept the query: %q", outliner.lensQuery)
	}
	if !outliner.lensModified() {
		t.Fatal("an accepted narrowing query must mark the lens modified")
	}
}

func TestLensQueryEditingEscRestoresTheQueryItOpenedOn(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	outliner.ActivateSearch()
	typeLensQuery(t, outliner, "z")
	outliner.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})

	if outliner.editingLensQuery() || outliner.IsSearchMode() {
		t.Fatal("Esc did not leave the query editor")
	}
	if outliner.lensQuery != "needle" {
		t.Fatalf("Esc left the query at %q, want the original needle", outliner.lensQuery)
	}
	if !outliner.hasActiveLens() {
		t.Fatal("abandoning the query edit must not dismiss the lens")
	}
	if outliner.lensModified() {
		t.Fatal("an abandoned query edit must leave the lens unmodified")
	}
}

func TestStartupAndGlobalResetRetainNoLensFrame(t *testing.T) {
	fresh := NewOutlinerModel(nil, nil, testConfigWithProjects())
	if fresh.hasActiveLens() || fresh.pendingLens != nil || fresh.lensQuery != "" {
		t.Fatal("a fresh Outliner must open with no lens")
	}

	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))
	outliner.resetToGlobalAll()

	if outliner.hasActiveLens() || outliner.pendingLens != nil {
		t.Fatal("the global reset retained a lens frame")
	}
	if outliner.lensQuery != "" {
		t.Fatalf("the global reset retained the narrowing query %q", outliner.lensQuery)
	}
	if outliner.primaryMode != ModeAll || outliner.filter.hasStateFilter() {
		t.Fatal("the global reset did not clear the lens-installed axes")
	}
}

// The frame holds the installed identity and fingerprint, never the live view:
// diverging an axis moves the capture and leaves the frame exactly where the
// application put it, which is the whole of how the modified marker is decided.
func TestTheActiveFrameKeepsTheInstalledIdentityAndFingerprint(t *testing.T) {
	outliner := lensLifecycleOutliner()
	saved := waitingLens(lensTestID(1), "Waiting", "needle")
	applyLensOrFatal(t, outliner, saved)

	installed := outliner.activeLens.savedFingerprint
	if outliner.lensModified() {
		t.Fatal("a freshly applied lens reported itself modified")
	}

	// Diverging the live view must not move the installed fingerprint.
	outliner.filter.Priority = int(models.PriorityHigh)

	if outliner.activeLens.lensID != saved.ID || outliner.activeLens.name != saved.Name {
		t.Fatalf("frame identity = %q/%q, want %q/%q",
			outliner.activeLens.lensID, outliner.activeLens.name, saved.ID, saved.Name)
	}
	if outliner.activeLens.savedFingerprint != installed {
		t.Fatal("the installed fingerprint absorbed a live edit")
	}
	if !outliner.lensModified() {
		t.Fatal("a diverged view did not report itself modified")
	}
}

// typeLensQuery drives the lens query editor one rune at a time through the
// ordinary key path, so the test exercises the same input loop the user does.
func typeLensQuery(t *testing.T, outliner *OutlinerModel, query string) {
	t.Helper()
	for _, r := range query {
		outliner.handleKeyMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func loadedProjectIDs(outliner *OutlinerModel) []string {
	ids := make([]string, 0, len(outliner.loadedProjects))
	for _, project := range outliner.loadedProjects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return ids
}
