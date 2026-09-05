package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestApplyNamedLensInstallsSavedProjectionAndEscRestoresBaseline(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	outlinerView.SelectOutlinerMode(chrome.ModeInbox)
	outlinerView.TreeSort = query.SortState{Field: query.SortByTitle, Ascending: true}
	outlinerView.Folds.CollapsedSections["baseline-fold"] = true
	outlinerView.PeekMode = true
	outlinerView.Selected[taskref.TaskIdentity{ProjectID: "beta", TaskID: "beta-inbox"}] = true
	baselineRows := searchVisibleTaskIDs(outlinerView)

	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))

	if !outlinerView.HasActiveLens() {
		t.Fatal("expected an active lens after ApplyNamedLens")
	}
	if outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("lens mode = %v, want %v", outlinerView.PrimaryMode, chrome.ModeAll)
	}
	if outlinerView.Lens.Query != "needle" {
		t.Fatalf("lens query = %q, want %q", outlinerView.Lens.Query, "needle")
	}
	if !outlinerView.Filter.HasStateFilter() {
		t.Fatal("expected the lens state facet to be installed")
	}
	if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, []string{"waiting", "beta-waiting"}) {
		t.Fatalf("lens rows = %v, want [waiting beta-waiting]", got)
	}
	if outlinerView.HasSearchProjection() {
		t.Fatal("a lens must not create an ordinary search projection")
	}
	// Peek is the view's, not the lens's, so it survives the application and is
	// closed here to prove restoration puts it back rather than merely leaving it.
	outlinerView.PeekMode = false

	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("Esc left the lens active")
	}
	if outlinerView.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want %v", outlinerView.PrimaryMode, chrome.ModeInbox)
	}
	if outlinerView.Lens.Query != "" {
		t.Fatalf("restored lens query = %q, want empty", outlinerView.Lens.Query)
	}
	if outlinerView.Filter.HasStateFilter() {
		t.Fatal("restoration kept the lens state facet")
	}
	if outlinerView.TreeSort.Field != query.SortByTitle || !outlinerView.TreeSort.Ascending {
		t.Fatalf("restored treeSort = %+v, want title ascending", outlinerView.TreeSort)
	}
	if !outlinerView.Folds.CollapsedSections["baseline-fold"] {
		t.Fatal("restoration dropped the baseline fold state")
	}
	if !outlinerView.PeekMode {
		t.Fatal("restoration dropped Peek")
	}
	if !outlinerView.Selected[taskref.TaskIdentity{ProjectID: "beta", TaskID: "beta-inbox"}] {
		t.Fatal("restoration dropped the baseline selection")
	}
	if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, baselineRows) {
		t.Fatalf("restored rows = %v, want %v", got, baselineRows)
	}
}

func TestApplyNamedLensRetainsScopeFocusAndCursorIdentity(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	outlinerView.PushProjectScope("alpha")
	outlinerView.Focus.CrossStack = []outliner.CrossProjectFocusEntry{{TaskID: "backlog", ProjectID: "alpha"}}

	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", ""))

	if outlinerView.Scope.ProjectID != "alpha" {
		t.Fatalf("scoped project = %q, want alpha", outlinerView.Scope.ProjectID)
	}
	if len(outlinerView.Focus.CrossStack) != 1 {
		t.Fatalf("focus stack depth = %d, want 1", len(outlinerView.Focus.CrossStack))
	}
	if outlinerView.Lens.Pending != nil {
		t.Fatal("a scoped application must not start a project load")
	}
}

func TestApplyNamedLensKeepsTheCursorOnASurvivingIdentity(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	if !outlinerView.MoveCursorToProjectTask("waiting", "alpha") {
		t.Fatal("cursor could not be seated on the waiting task")
	}

	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", ""))

	if identity := outlinerView.CurrentTaskIdentity(); identity.TaskID != "waiting" {
		t.Fatalf("cursor identity = %+v, want the surviving waiting task", identity)
	}
}

func TestApplyNamedLensFallsBackToTheNearestSurvivorWhenTheCursorTaskLeaves(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	if !outlinerView.MoveCursorToProjectTask("doing", "alpha") {
		t.Fatal("cursor could not be seated on the doing task")
	}

	// The Waiting lens drops the cursor's own task, so the existing
	// nearest-survivor fallback has to seat it on a task row that remains.
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", ""))

	identity := outlinerView.CurrentTaskIdentity()
	if identity.TaskID != "waiting" && identity.TaskID != "beta-waiting" {
		t.Fatalf("cursor identity = %+v, want a surviving lens row", identity)
	}
}

func TestApplyNamedLensUnderScopeRecordsProjectFacetsInert(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	outlinerView.PushProjectScope("alpha")

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked")
	applyLensOrFatal(t, outlinerView, lens)

	if outlinerView.ProjectFilter.ParkedMode != query.ProjectPopulationInclude {
		t.Fatal("the scoped application did not record the lens project facets")
	}
	if outlinerView.Lens.Pending != nil {
		t.Fatal("the recorded facets must stay inert under a scope: no load may start")
	}
	if ids := taskRowProjectIDs(outlinerView); len(ids) != 1 || ids[0] != "alpha" {
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
	outlinerView := app.crossOutliner
	baseline := loadedProjectIDs(outlinerView)
	outlinerView.SetSearchQuery("needle")
	outlinerView.PushProjectScope("alpha")

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked:only")
	applyLensOrFatal(t, outlinerView, lens)
	if outlinerView.Lens.Pending != nil {
		t.Fatal("a scoped application must record its facets without loading")
	}
	frame := outlinerView.Lens.Active
	savedFingerprint := frame.SavedFingerprint

	// Replacing one project scope with another is not becoming unscoped.
	outlinerView.PushProjectScope("beta")
	if outlinerView.Lens.Pending != nil {
		t.Fatal("replacing a project scope activated the lens project facets")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, baseline) {
		t.Fatalf("population after a scope replacement = %v, want the untouched %v", ids, baseline)
	}

	handled, cmd := app.applyProjectScope(outliner.ApplyProjectScopeMsg{Clear: true})
	if !handled {
		t.Fatal("the Project Hub clear-scope route was not handled")
	}
	if cmd == nil || outlinerView.Lens.Pending == nil {
		t.Fatal("clearing the scope under a project-filtering lens must load the population it selects")
	}
	if outlinerView.Scope.ProjectID != "beta" {
		t.Fatal("the scope was dropped onto the population it replaced instead of the one the lens selects")
	}

	// A wholly failed read leaves the transition where it was: same scope, same
	// population, same lens, and the facets still owed a load.
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		FailedLoads: []outliner.ProjectLoadFailure{{ProjectID: "gamma", Detail: "failed to load project gamma: boom"}},
		Generation:  outlinerView.Lens.Pending.Generation,
	}); published {
		t.Fatal("a wholly failed load published a scope exit")
	}
	if outlinerView.Scope.ProjectID != "beta" || !outlinerView.HasActiveLens() {
		t.Fatal("a wholly failed load did not leave the scope and the lens as they were")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, baseline) {
		t.Fatalf("population after a failed load = %v, want the untouched %v", ids, baseline)
	}

	if _, cmd = app.applyProjectScope(outliner.ApplyProjectScopeMsg{Clear: true}); cmd == nil {
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
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("unscoped population = %v, want the [gamma] the lens facets select", ids)
	}
	if ids := taskRowProjectIDs(outlinerView); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("unscoped rows = %v, want the [gamma] the lens facets select", ids)
	}

	// The exit publishes a population, never a definition: the live lens keeps its
	// identity, its saved fingerprint, and the pre-lens restoration point.
	if !outlinerView.HasActiveLens() || outlinerView.Lens.Active.Name != "Waiting" {
		t.Fatal("the exit disturbed the active lens")
	}
	if outlinerView.Lens.Active.SavedFingerprint != savedFingerprint {
		t.Fatal("the exit rewrote the saved definition the modified marker compares against")
	}
	// Scope is a lens-owned axis, so leaving one under a live lens is a genuine
	// divergence and the marker says so. That is the axis moving rather than the
	// definition: the saved fingerprint asserted just above is untouched, and
	// replacing the lens from the current view is what would adopt the new scope.
	if !outlinerView.LensModified() {
		t.Fatal("leaving a project scope under a live lens must mark it modified")
	}
	if outlinerView.Lens.Active != frame {
		t.Fatal("the exit replaced the original pre-lens restoration frame")
	}
	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outlinerView.Scope.ProjectID != "alpha" {
		t.Fatalf("restored scope = %q, want the pre-lens alpha", outlinerView.Scope.ProjectID)
	}
	if !outlinerView.HasSearchProjection() {
		t.Fatal("restoration dropped the pre-lens search projection")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, baseline) {
		t.Fatalf("restored population = %v, want the pre-lens %v", ids, baseline)
	}
}
