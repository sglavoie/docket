package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
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
	facets, err := models.ParseQuery("$" + string(models.StateWaiting))
	if err != nil {
		panic(err)
	}
	state := lensState{
		mode:   chrome.ModeAll,
		facets: facets,
		query:  query,
		sort:   NewSortState(),
	}
	return state.namedLens(id, name)
}

// lensWithFacet adds one canonical token to a saved definition's narrowing. A
// lens carries one query string rather than a filter struct, so a fixture names
// an extra facet by spelling it rather than by writing a field.
func lensWithFacet(lens config.NamedLens, token string) config.NamedLens {
	lens.Query = strings.TrimSpace(lens.Query + " " + token)
	return lens
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
	outliner.selectOutlinerMode(chrome.ModeInbox)
	outliner.treeSort = SortState{Field: SortByTitle, Ascending: true}
	outliner.collapsedSections["baseline-fold"] = true
	outliner.peekMode = true
	outliner.selectedTaskIdentities[taskref.TaskIdentity{ProjectID: "beta", TaskID: "beta-inbox"}] = true
	baselineRows := searchVisibleTaskIDs(outliner)

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	if !outliner.hasActiveLens() {
		t.Fatal("expected an active lens after ApplyNamedLens")
	}
	if outliner.primaryMode != chrome.ModeAll {
		t.Fatalf("lens mode = %v, want %v", outliner.primaryMode, chrome.ModeAll)
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
	if outliner.primaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want %v", outliner.primaryMode, chrome.ModeInbox)
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
	if !outliner.selectedTaskIdentities[taskref.TaskIdentity{ProjectID: "beta", TaskID: "beta-inbox"}] {
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
	lens = lensWithFacet(lens, "+parked")
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
	lens = lensWithFacet(lens, "+parked:only")
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
