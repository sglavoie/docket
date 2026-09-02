package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

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
	first = lensWithFacet(first, "+parked:only")
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
	second = lensWithFacet(second, "+parked:only")
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
	parkedLens = lensWithFacet(parkedLens, "+parked")
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
	app.views.Register(presentation.ViewCrossProjectOutliner, outliner)

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
			lens = lensWithFacet(lens, "+parked")
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

	// An untagged-projects facet alone admits alpha and beta and rejects the
	// parked project, which carries the parked tag.
	lens := lensWithFacet(waitingLens(lensTestID(2), "Untagged", ""), "+#0")
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

	// An untagged-projects facet admits alpha and beta and rejects both parked
	// projects.
	lens := lensWithFacet(waitingLens(lensTestID(3), "Untagged", ""), "+#0")
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
		ProjectFilter:  pending.state.projectFilterState(),
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
