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

// TestScopedLensActivatesProjectFacetsOnExplicitScopeExit walks the Project Hub
// clear-scope route while a project-filtering lens stays active. Project facets
// are population selectors rather than row predicates, so the facets a scoped
// application could only record must select the population the moment the model
// becomes unscoped — and not one moment earlier, since replacing one project
// scope with another leaves them just as inert as applying under one did.
func TestScopedLensActivatesProjectFacetsOnExplicitScopeExit(t *testing.T) {
	app := lensOperationApp(t)
	outliner := app.crossOutliner
	baseline := loadedProjectIDs(outliner)
	outliner.SetSearchQuery("needle")
	outliner.pushProjectScope("alpha")

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens.Projects.ParkedMode = config.LensPopulationOnly
	applyLensOrFatal(t, outliner, lens)
	if outliner.pendingLens != nil {
		t.Fatal("a scoped application must record its facets without loading")
	}
	frame := outliner.activeLens
	savedFingerprint := frame.savedFingerprint

	// Replacing one project scope with another is not becoming unscoped.
	outliner.pushProjectScope("beta")
	if outliner.pendingLens != nil {
		t.Fatal("replacing a project scope activated the lens project facets")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, baseline) {
		t.Fatalf("population after a scope replacement = %v, want the untouched %v", ids, baseline)
	}

	handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true})
	if !handled {
		t.Fatal("the Project Hub clear-scope route was not handled")
	}
	if cmd == nil || outliner.pendingLens == nil {
		t.Fatal("clearing the scope under a project-filtering lens must load the population it selects")
	}
	if outliner.scopedProjectID != "beta" {
		t.Fatal("the scope was dropped onto the population it replaced instead of the one the lens selects")
	}

	// A wholly failed read leaves the transition where it was: same scope, same
	// population, same lens, and the facets still owed a load.
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		failedLoads: []projectLoadFailure{{ProjectID: "gamma", Detail: "failed to load project gamma: boom"}},
		generation:  outliner.pendingLens.generation,
	}); published {
		t.Fatal("a wholly failed load published a scope exit")
	}
	if outliner.scopedProjectID != "beta" || !outliner.hasActiveLens() {
		t.Fatal("a wholly failed load did not leave the scope and the lens as they were")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, baseline) {
		t.Fatalf("population after a failed load = %v, want the untouched %v", ids, baseline)
	}

	if _, cmd = app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); cmd == nil {
		t.Fatal("the retried exit started no load")
	}
	loaded := []*models.Project{primaryRowsProject("gamma", primaryRowsTask("gamma-waiting", models.StateWaiting))}
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects:   loaded,
		generation: outliner.pendingLens.generation,
	}); !published {
		t.Fatal("the landed exit published nothing")
	}
	if outliner.isProjectScoped() {
		t.Fatal("the landed exit left the scope in place")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("unscoped population = %v, want the [gamma] the lens facets select", ids)
	}
	if ids := taskRowProjectIDs(outliner); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("unscoped rows = %v, want the [gamma] the lens facets select", ids)
	}

	// The exit publishes a population, never a definition: the live lens keeps its
	// identity, its saved fingerprint, and the pre-lens restoration point.
	if !outliner.hasActiveLens() || outliner.activeLens.name != "Waiting" {
		t.Fatal("the exit disturbed the active lens")
	}
	if outliner.activeLens.savedFingerprint != savedFingerprint {
		t.Fatal("the exit rewrote the saved definition the modified marker compares against")
	}
	// Scope is a lens-owned axis, so leaving one under a live lens is a genuine
	// divergence and the marker says so. That is the axis moving rather than the
	// definition: the saved fingerprint asserted just above is untouched, and
	// replacing the lens from the current view is what would adopt the new scope.
	if !outliner.lensModified() {
		t.Fatal("leaving a project scope under a live lens must mark it modified")
	}
	if outliner.activeLens != frame {
		t.Fatal("the exit replaced the original pre-lens restoration frame")
	}
	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outliner.scopedProjectID != "alpha" {
		t.Fatalf("restored scope = %q, want the pre-lens alpha", outliner.scopedProjectID)
	}
	if !outliner.hasSearchProjection() {
		t.Fatal("restoration dropped the pre-lens search projection")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, baseline) {
		t.Fatalf("restored population = %v, want the pre-lens %v", ids, baseline)
	}
}

// TestScopedEquivalentLensSwitchRetainsDeferredPopulationLoad pins the
// provenance of the loaded population against the equality between consecutive
// lens filters. A scoped application writes a project filter without reading the
// population it selects, so reapplying that lens — or switching to another one
// carrying identical project facets — compares equal to a filter no population
// ever answered to. The debt belongs to the population, not to the comparison,
// so the explicit scope exit still owes that load.
func TestScopedEquivalentLensSwitchRetainsDeferredPopulationLoad(t *testing.T) {
	app := lensOperationApp(t)
	outliner := app.crossOutliner
	baseline := loadedProjectIDs(outliner)
	outliner.pushProjectScope("alpha")

	first := waitingLens(lensTestID(1), "Waiting", "")
	first.Projects.ParkedMode = config.LensPopulationOnly
	applyLensOrFatal(t, outliner, first)
	if outliner.pendingLens != nil {
		t.Fatal("a scoped application must record its facets without loading")
	}
	frame := outliner.activeLens
	if !frame.projectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}
	origin := frame.origin

	// Reapplying the very same definition under the scope: the live filter now
	// equals the incoming one, and the population is still the pre-lens one.
	applyLensOrFatal(t, outliner, first)
	if outliner.pendingLens != nil {
		t.Fatal("reapplying a lens under a project scope loaded a population")
	}
	if !outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("reapplying the same lens under scope discarded the owed population load")
	}

	// Switching to a different lens carrying identical project facets is the same
	// case: a new identity, the same unpaid population debt.
	second := waitingLens(lensTestID(2), "Held", "")
	second.Projects.ParkedMode = config.LensPopulationOnly
	applyLensOrFatal(t, outliner, second)
	if outliner.pendingLens != nil {
		t.Fatal("switching lenses under a project scope loaded a population")
	}
	if outliner.activeLens.name != "Held" {
		t.Fatalf("active lens = %q, want Held", outliner.activeLens.name)
	}
	if !outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("an equivalent lens switch under scope discarded the owed population load")
	}
	if outliner.activeLens.origin.presentation.mode != origin.presentation.mode {
		t.Fatal("the switch replaced the original pre-lens restoration frame")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, baseline) {
		t.Fatalf("population under scope = %v, want the untouched %v", ids, baseline)
	}

	handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true})
	if !handled {
		t.Fatal("the Project Hub clear-scope route was not handled")
	}
	if cmd == nil || outliner.pendingLens == nil {
		t.Fatal("the explicit exit skipped the population the live lens facets select")
	}
	if outliner.scopedProjectID != "alpha" {
		t.Fatal("the scope was dropped before the population it owes landed")
	}

	// A wholly failed read is transactional: same scope, same population, same
	// lens, and the facets still owed a load.
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		failedLoads: []projectLoadFailure{{ProjectID: "gamma", Detail: "failed to load project gamma: boom"}},
		generation:  outliner.pendingLens.generation,
	}); published {
		t.Fatal("a wholly failed load published a scope exit")
	}
	if outliner.scopedProjectID != "alpha" || !outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("a wholly failed load did not leave the scope and the owed load as they were")
	}

	if _, cmd = app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); cmd == nil {
		t.Fatal("the retried exit started no load")
	}
	loaded := []*models.Project{primaryRowsProject("gamma", primaryRowsTask("gamma-waiting", models.StateWaiting))}
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects:   loaded,
		generation: outliner.pendingLens.generation,
	}); !published {
		t.Fatal("the landed exit published nothing")
	}
	if outliner.isProjectScoped() {
		t.Fatal("the landed exit left the scope in place")
	}
	if outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("the landed exit left the population still owed")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("unscoped population = %v, want the [gamma] the lens facets select", ids)
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outliner.scopedProjectID != "alpha" {
		t.Fatalf("restored scope = %q, want the pre-lens alpha", outliner.scopedProjectID)
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, baseline) {
		t.Fatalf("restored population = %v, want the pre-lens %v", ids, baseline)
	}
}

// TestScopedLensPopulationProvenanceTracksOrdinaryReloads pins the provenance of
// the live population to the reloads that actually replace it. A lens applied
// under a project scope records project facets it cannot load, and an ordinary
// reload beside it publishes a population under whatever filter it read. If the
// frame keeps naming the filter the scoped application merely found, a later lens
// whose facets happen to equal that abandoned filter clears a debt nothing paid,
// and the explicit scope exit then shows a population the live lens never
// selected.
func TestScopedLensPopulationProvenanceTracksOrdinaryReloads(t *testing.T) {
	app := lensOperationApp(t)
	outliner := app.crossOutliner
	base := slices.Clone(outliner.loadedProjects)
	baseline := loadedProjectIDs(outliner)

	parked := primaryRowsProject("parked", primaryRowsTask("parked-waiting", models.StateWaiting))
	parked.Tags = []string{models.ParkedProjectTag}

	outliner.pushProjectScope("alpha")
	parkedLens := waitingLens(lensTestID(1), "Parked", "")
	parkedLens.Projects.ParkedMode = config.LensPopulationInclude
	applyLensOrFatal(t, outliner, parkedLens)
	frame := outliner.activeLens
	if !frame.projectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}
	origin := frame.origin

	// A reload issued before the application lands afterwards carrying the
	// population the pre-lens filter selects. It is not the one the facets ask
	// for, so the debt is untouched and the frame now names that filter.
	outliner.crossLoadGen++
	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:            base,
		generation:          outliner.crossLoadGen,
		loadedProjectFilter: projectFilterFingerprint(&ProjectFilterState{}),
	})
	if !outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("a reload under the pre-lens filter paid the owed population load")
	}

	// The next reload reads under the live filter, which is the lens's own facets.
	// The command names that filter itself: the model it lands against may have
	// moved, so the read has to carry its own provenance.
	outliner.crossLoadGen++
	result, ok := outliner.loadCrossProjectsCmd()().(crossProjectsLoadedMsg)
	if !ok {
		t.Fatal("the reload command produced no cross-projects result")
	}
	if result.loadedProjectFilter != projectFilterFingerprint(&outliner.projectFilter) {
		t.Fatalf("reload provenance = %q, want the project filter it read under %q",
			result.loadedProjectFilter, projectFilterFingerprint(&outliner.projectFilter))
	}
	result.projects = append(slices.Clone(base), parked)
	result.failedLoads = nil
	outliner.handleCrossProjectsLoaded(result)
	if outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("a reload under the lens's own facets left the population still owed")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, append(slices.Clone(baseline), "parked")) {
		t.Fatalf("reloaded population = %v, want the parked-admitting one the live filter selects", ids)
	}

	// Switching to a lens whose facets equal the filter the scoped application
	// found is the stale-provenance case. The population in hand is the parked
	// one, so the base population this lens selects is owed a load.
	plain := waitingLens(lensTestID(2), "Plain", "")
	applyLensOrFatal(t, outliner, plain)
	if outliner.pendingLens != nil {
		t.Fatal("switching lenses under a project scope loaded a population")
	}
	if !outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("the switch cleared a population debt against a stale fingerprint")
	}

	handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true})
	if !handled {
		t.Fatal("the Project Hub clear-scope route was not handled")
	}
	if cmd == nil || outliner.pendingLens == nil {
		t.Fatal("the explicit exit skipped the population the live lens facets select")
	}

	// A wholly failed read is transactional: same scope, same population, and the
	// facets still owed a load.
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		failedLoads: []projectLoadFailure{{ProjectID: "alpha", Detail: "failed to load project alpha: boom"}},
		generation:  outliner.pendingLens.generation,
	}); published {
		t.Fatal("a wholly failed load published a scope exit")
	}
	if outliner.scopedProjectID != "alpha" || !outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("a wholly failed load did not leave the scope and the owed load as they were")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, append(slices.Clone(baseline), "parked")) {
		t.Fatalf("population after a failed load = %v, want the untouched parked-admitting one", ids)
	}

	if _, cmd = app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); cmd == nil {
		t.Fatal("the retried exit started no load")
	}
	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects:   base,
		generation: outliner.pendingLens.generation,
	}); !published {
		t.Fatal("the landed exit published nothing")
	}
	if outliner.isProjectScoped() {
		t.Fatal("the landed exit left the scope in place")
	}
	if outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("the landed exit left the population still owed")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, baseline) {
		t.Fatalf("unscoped population = %v, want the %v the live lens selects", ids, baseline)
	}
	if outliner.activeLens.origin.presentation.mode != origin.presentation.mode {
		t.Fatal("the repaired provenance replaced the original pre-lens restoration frame")
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outliner.scopedProjectID != "alpha" {
		t.Fatalf("restored scope = %q, want the pre-lens alpha", outliner.scopedProjectID)
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, baseline) {
		t.Fatalf("restored population = %v, want the pre-lens %v", ids, baseline)
	}
}

// parkedFocusLensApp builds an AppModel whose Outliner holds a two-project base
// workspace plus one parked project the base workspace never admits, then adopts
// that parked project as an explicit focus exactly as the lazy focus load does:
// the scope is pushed by the load result and the exception that keeps the
// project in the snapshot is recorded there. Extra projects join the catalog
// without joining the base workspace or the returned population, which is what
// lets a caller keep a second focusable project reachable. It returns the app,
// its Outliner, and the population a later load under a parked-admitting filter
// returns.
func parkedFocusLensApp(t *testing.T, extra ...*models.Project) (*AppModel, *OutlinerModel, []*models.Project) {
	t.Helper()
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-waiting", models.StateWaiting))
	beta := primaryRowsProject("beta", primaryRowsTask("beta-waiting", models.StateWaiting))
	parked := primaryRowsProject("parked", primaryRowsTask("parked-waiting", models.StateWaiting))
	parked.Tags = []string{models.ParkedProjectTag}

	app := lensOperationApp(t)
	outliner := scopeTestOutliner(append([]*models.Project{alpha, beta, parked}, extra...)...)
	// The base task workspace holds no parked project; only the focus below
	// admits it.
	outliner.loadedProjects = []*models.Project{alpha, beta}
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	app.crossOutliner = outliner
	outliner.SetSize(app.width, app.height-2)
	app.views.Register(ViewCrossProjectOutliner, outliner)

	everything := []*models.Project{alpha, beta, parked}
	outliner.beginParkedProjectFocus(parked.ID)
	outliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:       everything,
		generation:     outliner.crossLoadGen,
		focusProjectID: parked.ID,
	})
	if outliner.scopedProjectID != parked.ID || outliner.parkedFocusProjectID != parked.ID {
		t.Fatalf("parked focus did not settle: scope=%q exception=%q",
			outliner.scopedProjectID, outliner.parkedFocusProjectID)
	}
	return app, outliner, everything
}

// TestParkedScopedLensExitKeepsSelectedPopulation pins the deferred exit from a
// parked-project scope. The exit's load and the navigation return are batched,
// and a batch has no result order, so the exit has to settle the same way
// whichever arrives first. The parked-focus exception belongs to the base
// workspace the scope replaced, never to the population the lens's own project
// facets just selected: a lens that admits the parked project must keep it, one
// that does not must shed it, and neither may leave a corrective reload armed
// that nothing on this route ever claims.
func TestParkedScopedLensExitKeepsSelectedPopulation(t *testing.T) {
	orderings := []struct {
		name    string
		settle  func(app *AppModel, result lensProjectsLoadedMsg)
		loading bool
	}{
		{
			name: "navigation return first",
			settle: func(app *AppModel, result lensProjectsLoadedMsg) {
				app.Update(ViewDidAppearMsg{})
				app.Update(result)
			},
		},
		{
			name: "lens load first",
			settle: func(app *AppModel, result lensProjectsLoadedMsg) {
				app.Update(result)
				app.Update(ViewDidAppearMsg{})
			},
			// The return finds the exit already settled, so it starts the ordinary
			// eager reload it always does on becoming visible.
			loading: true,
		},
	}

	for _, ordering := range orderings {
		t.Run(ordering.name, func(t *testing.T) {
			app, outliner, everything := parkedFocusLensApp(t)

			lens := waitingLens(lensTestID(1), "Waiting", "")
			lens.Projects.ParkedMode = config.LensPopulationInclude
			applyLensOrFatal(t, outliner, lens)
			if outliner.pendingLens != nil {
				t.Fatal("a scoped application must record its facets without loading")
			}
			frame := outliner.activeLens
			if !frame.projectFacetsAwaitingLoad {
				t.Fatal("the scoped application recorded no owed population load")
			}

			if handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); !handled || cmd == nil {
				t.Fatal("clearing a parked scope under a project-filtering lens started no population load")
			}
			if outliner.pendingLens == nil {
				t.Fatal("the exit armed no pending application")
			}
			ordering.settle(app, lensProjectsLoadedMsg{
				projects:   everything,
				generation: outliner.pendingLens.generation,
			})

			if outliner.isProjectScoped() {
				t.Fatal("the settled exit left the scope in place")
			}
			if ids := loadedProjectIDs(outliner); !slices.Equal(ids, []string{"alpha", "beta", "parked"}) {
				t.Fatalf("population = %v, want every project the lens facets select", ids)
			}
			if ids := taskRowProjectIDs(outliner); !slices.Contains(ids, "parked") {
				t.Fatalf("rows = %v, want the parked project the lens admits", ids)
			}
			if outliner.parkedFocusReloadPending {
				t.Fatal("the exit armed a corrective reload no handler on this route claims")
			}
			if outliner.parkedFocusProjectID != "" {
				t.Fatal("a project the facets select is population, not a focus exception")
			}
			if outliner.crossLoading != ordering.loading {
				t.Fatalf("crossLoading = %t, want %t", outliner.crossLoading, ordering.loading)
			}
			if !outliner.hasActiveLens() || outliner.activeLens != frame {
				t.Fatal("the exit disturbed the active lens")
			}
			if outliner.activeLens.projectFacetsAwaitingLoad {
				t.Fatal("the landed exit left the population still owed")
			}
		})
	}
}

// TestParkedScopedLensExitShedsAProjectItsFacetsReject is the other half of the
// same settlement: the exit's load carries the focus request, so the population
// it returns holds the parked project whether or not the facets asked for it.
// One the facets reject leaves with the scope, and still owes no reload.
func TestParkedScopedLensExitShedsAProjectItsFacetsReject(t *testing.T) {
	app, outliner, everything := parkedFocusLensApp(t)

	lens := waitingLens(lensTestID(2), "Untagged", "")
	// Untagged alone is an active project facet that admits alpha and beta and
	// rejects the parked project, which carries the parked tag.
	lens.Projects.Untagged = true
	applyLensOrFatal(t, outliner, lens)

	if handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); !handled || cmd == nil {
		t.Fatal("clearing a parked scope under a project-filtering lens started no population load")
	}
	app.Update(lensProjectsLoadedMsg{
		projects:   everything,
		generation: outliner.pendingLens.generation,
	})

	if outliner.isProjectScoped() {
		t.Fatal("the settled exit left the scope in place")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, []string{"alpha", "beta"}) {
		t.Fatalf("population = %v, want the projects the lens facets select", ids)
	}
	if outliner.parkedFocusReloadPending {
		t.Fatal("shedding a rejected project armed a reload the population already answered")
	}
}

// TestParkedScopedLensExitDoesNotLeakPendingFocus pins the other parked project
// a lens-backed exit can be asked about: one whose focus request is still in
// flight when the scope is cleared. That request is superseded by the exit, so it
// must not choose the focus the exit's own read carries — otherwise its project
// is appended to the population as an exception the settlement never evaluates,
// and a project the live facets reject survives the exit.
func TestParkedScopedLensExitDoesNotLeakPendingFocus(t *testing.T) {
	interloper := primaryRowsProject("interloper", primaryRowsTask("interloper-waiting", models.StateWaiting))
	interloper.Tags = []string{models.ParkedProjectTag}
	app, outliner, _ := parkedFocusLensApp(t, interloper)

	lens := waitingLens(lensTestID(3), "Untagged", "")
	// Untagged admits alpha and beta and rejects both parked projects.
	lens.Projects.Untagged = true
	applyLensOrFatal(t, outliner, lens)
	frame := outliner.activeLens
	if !frame.projectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}

	// A second parked project is requested from the surface while the first is
	// still the scope, so its load is in flight when the exit starts.
	if cmd := outliner.beginParkedProjectFocus(interloper.ID); cmd == nil {
		t.Fatal("the second parked focus started no load")
	}
	if outliner.pendingParkedFocusID != interloper.ID {
		t.Fatalf("pending parked focus = %q, want the requested %q", outliner.pendingParkedFocusID, interloper.ID)
	}

	if handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); !handled || cmd == nil {
		t.Fatal("clearing a parked scope under a project-filtering lens started no population load")
	}
	pending := outliner.pendingLens
	if pending == nil {
		t.Fatal("the exit armed no pending application")
	}
	if outliner.pendingParkedFocusID != "" {
		t.Fatalf("the exit left the superseded focus request %q in flight", outliner.pendingParkedFocusID)
	}

	// The selection the exit's read performs, minus the disk load the fixture has
	// no projects on disk for: the focus the model asked for is what decides
	// whether the interloper is appended to this population at all.
	population := selectCrossScopeProjects(outliner.runtime, CrossScopeOpts{
		ActiveOnly:     true,
		FocusProjectID: outliner.focusRequestForLoad(),
		ProjectFilter:  pending.state.projectFilter,
	})
	app.Update(lensProjectsLoadedMsg{projects: population, generation: pending.generation})

	if outliner.isProjectScoped() {
		t.Fatal("the settled exit left the scope in place")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, []string{"alpha", "beta"}) {
		t.Fatalf("population = %v, want the projects the lens facets select", ids)
	}
	if ids := taskRowProjectIDs(outliner); slices.Contains(ids, interloper.ID) {
		t.Fatalf("rows = %v, want no project the superseded request appended", ids)
	}
	if outliner.parkedFocusProjectID != "" || outliner.pendingParkedFocusID != "" {
		t.Fatalf("focus fields after the exit: adopted=%q pending=%q, want both cleared",
			outliner.parkedFocusProjectID, outliner.pendingParkedFocusID)
	}
	if outliner.parkedFocusReloadPending {
		t.Fatal("the exit armed a corrective reload no handler on this route claims")
	}
	if !outliner.hasActiveLens() || outliner.activeLens != frame {
		t.Fatal("the exit disturbed the active lens")
	}
	if outliner.activeLens.projectFacetsAwaitingLoad {
		t.Fatal("the landed exit left the population still owed")
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
	outliner.adoptFilterCarriers()
	if !outliner.lensModified() {
		t.Fatal("a facet change must mark the lens modified")
	}
	if got := outliner.activeLensLabel(); got != "Lens: Waiting*" {
		t.Fatalf("label = %q, want %q", got, "Lens: Waiting*")
	}

	outliner.filter.Priority = 0
	outliner.adoptFilterCarriers()
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
	outliner.adoptFilterCarriers()
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
	outliner.adoptFilterCarriers()

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
