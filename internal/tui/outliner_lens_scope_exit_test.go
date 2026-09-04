package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
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
	outlinerView := app.crossOutliner
	baseline := loadedProjectIDs(outlinerView)
	outlinerView.PushProjectScope("alpha")

	first := waitingLens(lensTestID(1), "Waiting", "")
	first = lensWithFacet(first, "+parked:only")
	applyLensOrFatal(t, outlinerView, first)
	if outlinerView.Lens.Pending != nil {
		t.Fatal("a scoped application must record its facets without loading")
	}
	frame := outlinerView.Lens.Active
	if !frame.ProjectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}
	origin := frame.Origin

	// Reapplying the very same definition under the scope: the live filter now
	// equals the incoming one, and the population is still the pre-lens one.
	applyLensOrFatal(t, outlinerView, first)
	if outlinerView.Lens.Pending != nil {
		t.Fatal("reapplying a lens under a project scope loaded a population")
	}
	if !outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("reapplying the same lens under scope discarded the owed population load")
	}

	// Switching to a different lens carrying identical project facets is the same
	// case: a new identity, the same unpaid population debt.
	second := waitingLens(lensTestID(2), "Held", "")
	second = lensWithFacet(second, "+parked:only")
	applyLensOrFatal(t, outlinerView, second)
	if outlinerView.Lens.Pending != nil {
		t.Fatal("switching lenses under a project scope loaded a population")
	}
	if outlinerView.Lens.Active.Name != "Held" {
		t.Fatalf("active lens = %q, want Held", outlinerView.Lens.Active.Name)
	}
	if !outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("an equivalent lens switch under scope discarded the owed population load")
	}
	if outlinerView.Lens.Active.Origin.Presentation.Mode != origin.Presentation.Mode {
		t.Fatal("the switch replaced the original pre-lens restoration frame")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, baseline) {
		t.Fatalf("population under scope = %v, want the untouched %v", ids, baseline)
	}

	handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true})
	if !handled {
		t.Fatal("the Project Hub clear-scope route was not handled")
	}
	if cmd == nil || outlinerView.Lens.Pending == nil {
		t.Fatal("the explicit exit skipped the population the live lens facets select")
	}
	if outlinerView.Scope.ProjectID != "alpha" {
		t.Fatal("the scope was dropped before the population it owes landed")
	}

	// A wholly failed read is transactional: same scope, same population, same
	// lens, and the facets still owed a load.
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		FailedLoads: []outliner.ProjectLoadFailure{{ProjectID: "gamma", Detail: "failed to load project gamma: boom"}},
		Generation:  outlinerView.Lens.Pending.Generation,
	}); published {
		t.Fatal("a wholly failed load published a scope exit")
	}
	if outlinerView.Scope.ProjectID != "alpha" || !outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("a wholly failed load did not leave the scope and the owed load as they were")
	}

	if _, cmd = app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); cmd == nil {
		t.Fatal("the retried exit started no load")
	}
	loaded := []*models.Project{primaryRowsProject("gamma", primaryRowsTask("gamma-waiting", models.StateWaiting))}
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects:   loaded,
		Generation: outlinerView.Lens.Pending.Generation,
	}); !published {
		t.Fatal("the landed exit published nothing")
	}
	if outlinerView.IsProjectScoped() {
		t.Fatal("the landed exit left the scope in place")
	}
	if outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("the landed exit left the population still owed")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("unscoped population = %v, want the [gamma] the lens facets select", ids)
	}

	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outlinerView.Scope.ProjectID != "alpha" {
		t.Fatalf("restored scope = %q, want the pre-lens alpha", outlinerView.Scope.ProjectID)
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, baseline) {
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
	outlinerView := app.crossOutliner
	base := slices.Clone(outlinerView.Population.Projects)
	baseline := loadedProjectIDs(outlinerView)

	parked := primaryRowsProject("parked", primaryRowsTask("parked-waiting", models.StateWaiting))
	parked.Tags = []string{models.ParkedProjectTag}

	outlinerView.PushProjectScope("alpha")
	parkedLens := waitingLens(lensTestID(1), "Parked", "")
	parkedLens = lensWithFacet(parkedLens, "+parked")
	applyLensOrFatal(t, outlinerView, parkedLens)
	frame := outlinerView.Lens.Active
	if !frame.ProjectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}
	origin := frame.Origin

	// A reload issued before the application lands afterwards carrying the
	// population the pre-lens filter selects. It is not the one the facets ask
	// for, so the debt is untouched and the frame now names that filter.
	outlinerView.Population.LoadGen++
	outlinerView.HandleCrossProjectsLoaded(outliner.CrossProjectsLoadedMsg{
		Projects:            base,
		Generation:          outlinerView.Population.LoadGen,
		LoadedProjectFilter: query.ProjectFilterFingerprint(&query.ProjectFilterState{}),
	})
	if !outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("a reload under the pre-lens filter paid the owed population load")
	}

	// The next reload reads under the live filter, which is the lens's own facets.
	// The command names that filter itself: the model it lands against may have
	// moved, so the read has to carry its own provenance.
	outlinerView.Population.LoadGen++
	result, ok := outlinerView.LoadCrossProjectsCmd()().(outliner.CrossProjectsLoadedMsg)
	if !ok {
		t.Fatal("the reload command produced no cross-projects result")
	}
	if result.LoadedProjectFilter != query.ProjectFilterFingerprint(&outlinerView.ProjectFilter) {
		t.Fatalf("reload provenance = %q, want the project filter it read under %q",
			result.LoadedProjectFilter, query.ProjectFilterFingerprint(&outlinerView.ProjectFilter))
	}
	result.Projects = append(slices.Clone(base), parked)
	result.FailedLoads = nil
	outlinerView.HandleCrossProjectsLoaded(result)
	if outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("a reload under the lens's own facets left the population still owed")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, append(slices.Clone(baseline), "parked")) {
		t.Fatalf("reloaded population = %v, want the parked-admitting one the live filter selects", ids)
	}

	// Switching to a lens whose facets equal the filter the scoped application
	// found is the stale-provenance case. The population in hand is the parked
	// one, so the base population this lens selects is owed a load.
	plain := waitingLens(lensTestID(2), "Plain", "")
	applyLensOrFatal(t, outlinerView, plain)
	if outlinerView.Lens.Pending != nil {
		t.Fatal("switching lenses under a project scope loaded a population")
	}
	if !outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("the switch cleared a population debt against a stale fingerprint")
	}

	handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true})
	if !handled {
		t.Fatal("the Project Hub clear-scope route was not handled")
	}
	if cmd == nil || outlinerView.Lens.Pending == nil {
		t.Fatal("the explicit exit skipped the population the live lens facets select")
	}

	// A wholly failed read is transactional: same scope, same population, and the
	// facets still owed a load.
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		FailedLoads: []outliner.ProjectLoadFailure{{ProjectID: "alpha", Detail: "failed to load project alpha: boom"}},
		Generation:  outlinerView.Lens.Pending.Generation,
	}); published {
		t.Fatal("a wholly failed load published a scope exit")
	}
	if outlinerView.Scope.ProjectID != "alpha" || !outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("a wholly failed load did not leave the scope and the owed load as they were")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, append(slices.Clone(baseline), "parked")) {
		t.Fatalf("population after a failed load = %v, want the untouched parked-admitting one", ids)
	}

	if _, cmd = app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); cmd == nil {
		t.Fatal("the retried exit started no load")
	}
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects:   base,
		Generation: outlinerView.Lens.Pending.Generation,
	}); !published {
		t.Fatal("the landed exit published nothing")
	}
	if outlinerView.IsProjectScoped() {
		t.Fatal("the landed exit left the scope in place")
	}
	if outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("the landed exit left the population still owed")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, baseline) {
		t.Fatalf("unscoped population = %v, want the %v the live lens selects", ids, baseline)
	}
	if outlinerView.Lens.Active.Origin.Presentation.Mode != origin.Presentation.Mode {
		t.Fatal("the repaired provenance replaced the original pre-lens restoration frame")
	}

	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outlinerView.Scope.ProjectID != "alpha" {
		t.Fatalf("restored scope = %q, want the pre-lens alpha", outlinerView.Scope.ProjectID)
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, baseline) {
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
func parkedFocusLensApp(t *testing.T, extra ...*models.Project) (*AppModel, *outliner.OutlinerModel, []*models.Project) {
	t.Helper()
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-waiting", models.StateWaiting))
	beta := primaryRowsProject("beta", primaryRowsTask("beta-waiting", models.StateWaiting))
	parked := primaryRowsProject("parked", primaryRowsTask("parked-waiting", models.StateWaiting))
	parked.Tags = []string{models.ParkedProjectTag}

	app := lensOperationApp(t)
	outlinerView := scopeTestOutliner(append([]*models.Project{alpha, beta, parked}, extra...)...)
	// The base task workspace holds no parked project; only the focus below
	// admits it.
	outlinerView.Population.Projects = []*models.Project{alpha, beta}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	app.crossOutliner = outlinerView
	outlinerView.SetSize(app.width, app.height-2)
	app.views.Register(presentation.ViewCrossProjectOutliner, outlinerView)

	everything := []*models.Project{alpha, beta, parked}
	outlinerView.BeginParkedProjectFocus(parked.ID)
	outlinerView.HandleCrossProjectsLoaded(outliner.CrossProjectsLoadedMsg{
		Projects:       everything,
		Generation:     outlinerView.Population.LoadGen,
		FocusProjectID: parked.ID,
	})
	if outlinerView.Scope.ProjectID != parked.ID || outlinerView.Scope.ParkedProjectID != parked.ID {
		t.Fatalf("parked focus did not settle: scope=%q exception=%q",
			outlinerView.Scope.ProjectID, outlinerView.Scope.ParkedProjectID)
	}
	return app, outlinerView, everything
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
		settle  func(app *AppModel, result outliner.LensProjectsLoadedMsg)
		loading bool
	}{
		{
			name: "navigation return first",
			settle: func(app *AppModel, result outliner.LensProjectsLoadedMsg) {
				app.Update(status.ViewDidAppearMsg{})
				app.Update(result)
			},
		},
		{
			name: "lens load first",
			settle: func(app *AppModel, result outliner.LensProjectsLoadedMsg) {
				app.Update(result)
				app.Update(status.ViewDidAppearMsg{})
			},
			// The return finds the exit already settled, so it starts the ordinary
			// eager reload it always does on becoming visible.
			loading: true,
		},
	}

	for _, ordering := range orderings {
		t.Run(ordering.name, func(t *testing.T) {
			app, outlinerView, everything := parkedFocusLensApp(t)

			lens := waitingLens(lensTestID(1), "Waiting", "")
			lens = lensWithFacet(lens, "+parked")
			applyLensOrFatal(t, outlinerView, lens)
			if outlinerView.Lens.Pending != nil {
				t.Fatal("a scoped application must record its facets without loading")
			}
			frame := outlinerView.Lens.Active
			if !frame.ProjectFacetsAwaitingLoad {
				t.Fatal("the scoped application recorded no owed population load")
			}

			if handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); !handled || cmd == nil {
				t.Fatal("clearing a parked scope under a project-filtering lens started no population load")
			}
			if outlinerView.Lens.Pending == nil {
				t.Fatal("the exit armed no pending application")
			}
			ordering.settle(app, outliner.LensProjectsLoadedMsg{
				Projects:   everything,
				Generation: outlinerView.Lens.Pending.Generation,
			})

			if outlinerView.IsProjectScoped() {
				t.Fatal("the settled exit left the scope in place")
			}
			if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, []string{"alpha", "beta", "parked"}) {
				t.Fatalf("population = %v, want every project the lens facets select", ids)
			}
			if ids := taskRowProjectIDs(outlinerView); !slices.Contains(ids, "parked") {
				t.Fatalf("rows = %v, want the parked project the lens admits", ids)
			}
			if outlinerView.Scope.ParkedReloadPending {
				t.Fatal("the exit armed a corrective reload no handler on this route claims")
			}
			if outlinerView.Scope.ParkedProjectID != "" {
				t.Fatal("a project the facets select is population, not a focus exception")
			}
			if outlinerView.Population.Loading != ordering.loading {
				t.Fatalf("population.loading = %t, want %t", outlinerView.Population.Loading, ordering.loading)
			}
			if !outlinerView.HasActiveLens() || outlinerView.Lens.Active != frame {
				t.Fatal("the exit disturbed the active lens")
			}
			if outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
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
	app, outlinerView, everything := parkedFocusLensApp(t)

	// An untagged-projects facet alone admits alpha and beta and rejects the
	// parked project, which carries the parked tag.
	lens := lensWithFacet(waitingLens(lensTestID(2), "Untagged", ""), "+#0")
	applyLensOrFatal(t, outlinerView, lens)

	if handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); !handled || cmd == nil {
		t.Fatal("clearing a parked scope under a project-filtering lens started no population load")
	}
	app.Update(outliner.LensProjectsLoadedMsg{
		Projects:   everything,
		Generation: outlinerView.Lens.Pending.Generation,
	})

	if outlinerView.IsProjectScoped() {
		t.Fatal("the settled exit left the scope in place")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, []string{"alpha", "beta"}) {
		t.Fatalf("population = %v, want the projects the lens facets select", ids)
	}
	if outlinerView.Scope.ParkedReloadPending {
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
	app, outlinerView, _ := parkedFocusLensApp(t, interloper)

	// An untagged-projects facet admits alpha and beta and rejects both parked
	// projects.
	lens := lensWithFacet(waitingLens(lensTestID(3), "Untagged", ""), "+#0")
	applyLensOrFatal(t, outlinerView, lens)
	frame := outlinerView.Lens.Active
	if !frame.ProjectFacetsAwaitingLoad {
		t.Fatal("the scoped application recorded no owed population load")
	}

	// A second parked project is requested from the surface while the first is
	// still the scope, so its load is in flight when the exit starts.
	if cmd := outlinerView.BeginParkedProjectFocus(interloper.ID); cmd == nil {
		t.Fatal("the second parked focus started no load")
	}
	if outlinerView.Scope.PendingParkedID != interloper.ID {
		t.Fatalf("pending parked focus = %q, want the requested %q", outlinerView.Scope.PendingParkedID, interloper.ID)
	}

	if handled, cmd := app.applyProjectScope(ApplyProjectScopeMsg{Clear: true}); !handled || cmd == nil {
		t.Fatal("clearing a parked scope under a project-filtering lens started no population load")
	}
	pending := outlinerView.Lens.Pending
	if pending == nil {
		t.Fatal("the exit armed no pending application")
	}
	if outlinerView.Scope.PendingParkedID != "" {
		t.Fatalf("the exit left the superseded focus request %q in flight", outlinerView.Scope.PendingParkedID)
	}

	// The selection the exit's read performs, minus the disk load the fixture has
	// no projects on disk for: the focus the model asked for is what decides
	// whether the interloper is appended to this population at all.
	population := outliner.SelectCrossScopeProjects(outlinerView.Runtime, outliner.CrossScopeOpts{
		ActiveOnly:     true,
		FocusProjectID: outlinerView.Scope.FocusRequest(),
		ProjectFilter:  pending.State.ProjectFilterState(),
	})
	app.Update(outliner.LensProjectsLoadedMsg{Projects: population, Generation: pending.Generation})

	if outlinerView.IsProjectScoped() {
		t.Fatal("the settled exit left the scope in place")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, []string{"alpha", "beta"}) {
		t.Fatalf("population = %v, want the projects the lens facets select", ids)
	}
	if ids := taskRowProjectIDs(outlinerView); slices.Contains(ids, interloper.ID) {
		t.Fatalf("rows = %v, want no project the superseded request appended", ids)
	}
	if outlinerView.Scope.ParkedProjectID != "" || outlinerView.Scope.PendingParkedID != "" {
		t.Fatalf("focus fields after the exit: adopted=%q pending=%q, want both cleared",
			outlinerView.Scope.ParkedProjectID, outlinerView.Scope.PendingParkedID)
	}
	if outlinerView.Scope.ParkedReloadPending {
		t.Fatal("the exit armed a corrective reload no handler on this route claims")
	}
	if !outlinerView.HasActiveLens() || outlinerView.Lens.Active != frame {
		t.Fatal("the exit disturbed the active lens")
	}
	if outlinerView.Lens.Active.ProjectFacetsAwaitingLoad {
		t.Fatal("the landed exit left the population still owed")
	}
}
