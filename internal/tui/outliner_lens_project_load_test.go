package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
)

func TestLensProjectLoadPublishesOnlyOnTheMatchingGeneration(t *testing.T) {
	outliner := lensLifecycleOutliner()
	baseline := slices.Clone(outliner.population.projects)

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked:only")
	cmd := applyLensOrFatal(t, outliner, lens)
	if cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	if outliner.hasActiveLens() {
		t.Fatal("nothing may publish before the load lands")
	}
	if outliner.projectFilter.ParkedMode == query.ProjectPopulationOnly {
		t.Fatal("the candidate filter must not be written to the live model")
	}
	if !slices.Equal(outliner.population.projects, baseline) {
		t.Fatal("the live population changed before the load landed")
	}

	generation := outliner.lens.pending.generation
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
	if outliner.projectFilter.ParkedMode != query.ProjectPopulationOnly {
		t.Fatal("publication did not install the lens project facets")
	}
	if ids := loadedProjectIDs(outliner); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("published population = %v, want [gamma]", ids)
	}
}

func TestLensProjectLoadIsSupersededByALaterRequest(t *testing.T) {
	outliner := lensLifecycleOutliner()

	first := waitingLens(lensTestID(1), "Waiting", "")
	first = lensWithFacet(first, "+parked:only")
	applyLensOrFatal(t, outliner, first)
	stale := outliner.lens.pending.generation

	second := waitingLens(lensTestID(2), "Included", "")
	second = lensWithFacet(second, "+parked")
	applyLensOrFatal(t, outliner, second)
	live := outliner.lens.pending.generation
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
	if outliner.lens.active.name != "Included" {
		t.Fatalf("published lens = %q, want Included", outliner.lens.active.name)
	}
}

func TestLensProjectLoadAbortsWhenNothingLoaded(t *testing.T) {
	outliner := lensLifecycleOutliner()
	baseline := slices.Clone(outliner.population.projects)

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked:only")
	applyLensOrFatal(t, outliner, lens)

	cmd, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		failedLoads: []projectLoadFailure{{ProjectID: "gamma", Detail: "failed to load project gamma: boom"}},
		generation:  outliner.lens.pending.generation,
	})
	if published {
		t.Fatal("a wholly failed load published a lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("a wholly failed load installed a lens")
	}
	if !slices.Equal(outliner.population.projects, baseline) {
		t.Fatal("a wholly failed load disturbed the live population")
	}
	if outliner.population.loading {
		t.Fatal("a wholly failed load left the loading state armed")
	}
	if cmd == nil {
		t.Fatal("a wholly failed load must report why")
	}
	if _, ok := cmd().(status.ErrorMsg); !ok {
		t.Fatal("a wholly failed load must raise an error")
	}
}

func TestLensProjectLoadPublishesPartialResultWithWarning(t *testing.T) {
	outliner := lensLifecycleOutliner()

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked:only")
	applyLensOrFatal(t, outliner, lens)

	cmd, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects:    []*models.Project{primaryRowsProject("gamma")},
		failedLoads: []projectLoadFailure{{ProjectID: "delta", Detail: "failed to load project delta: boom"}},
		generation:  outliner.lens.pending.generation,
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
	if _, ok := cmd().(status.InfoStatusMsg); !ok {
		t.Fatal("a partial load must warn rather than error")
	}
}

// TestLensProjectLoadCapturesBaselineBeforeIO pins the restoration point to the
// moment the application started rather than the moment its projects landed.
// Everything the user does while the load runs is a view they are passing
// through, not the view the lens replaced.
func TestLensProjectLoadCapturesBaselineBeforeIO(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.selectOutlinerMode(chrome.ModeInbox)
	outliner.folds.collapsedSections["baseline-fold"] = true
	baselineProjects := slices.Clone(outliner.population.projects)

	lens := waitingLens(lensTestID(1), "Parked", "")
	lens = lensWithFacet(lens, "+parked:only")
	if cmd := applyLensOrFatal(t, outliner, lens); cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	if got := outliner.lens.pending.origin.presentation.mode; got != chrome.ModeInbox {
		t.Fatalf("frozen baseline mode = %v, want %v", got, chrome.ModeInbox)
	}

	// The user keeps navigating and searching while the projects read. None of it
	// may rewrite the baseline the application already froze.
	outliner.selectOutlinerMode(chrome.ModeBacklog)
	delete(outliner.folds.collapsedSections, "baseline-fold")
	outliner.SetSearchQuery("needle")

	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: []*models.Project{
			primaryRowsProject("gamma", primaryRowsTask("gamma-task", models.StateWaiting)),
		},
		generation: outliner.lens.pending.generation,
	}); !published {
		t.Fatal("the matching generation did not publish")
	}

	if !listsurface.RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("Esc left the lens active")
	}
	if outliner.primaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-load %v", outliner.primaryMode, chrome.ModeInbox)
	}
	if !outliner.folds.collapsedSections["baseline-fold"] {
		t.Fatal("restoration lost the fold the baseline held before the load")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("restoration installed a search projection the frozen baseline never held")
	}
	if !slices.Equal(outliner.population.projects, baselineProjects) {
		t.Fatal("restoration did not return the pre-load population")
	}
}

// TestLensProjectLoadSupersededBySynchronousLensClearsPendingState covers the
// one supersession that changes nothing about the generation counter: a lens
// that needs no I/O publishes immediately, and the application still loading
// behind it must be settled rather than left holding the loading state.
func TestLensProjectLoadSupersededBySynchronousLensClearsPendingState(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.selectOutlinerMode(chrome.ModeInbox)
	baselineProjects := slices.Clone(outliner.population.projects)

	async := waitingLens(lensTestID(1), "Parked", "")
	async = lensWithFacet(async, "+parked:only")
	if cmd := applyLensOrFatal(t, outliner, async); cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	stale := outliner.lens.pending.generation
	// The 200ms delay has elapsed, so the spinner is on screen when the
	// synchronous lens supersedes the load.
	outliner.population.spinnerVisible = true

	if cmd := applyLensOrFatal(t, outliner, waitingLens(lensTestID(2), "Waiting", "")); cmd != nil {
		t.Fatal("an unchanged population must not start a load")
	}
	if outliner.lens.pending != nil {
		t.Fatal("a synchronous lens left an application pending")
	}
	if outliner.population.loading || outliner.population.spinnerVisible {
		t.Fatal("a synchronous lens left the loading state armed")
	}
	if outliner.lens.active.pendingGeneration != 0 {
		t.Fatal("the published frame still awaits a superseded generation")
	}

	if _, published := outliner.handleLensProjectsLoaded(lensProjectsLoadedMsg{
		projects: []*models.Project{primaryRowsProject("gamma")}, generation: stale,
	}); published {
		t.Fatal("the superseded request published")
	}
	if outliner.lens.active == nil || outliner.lens.active.name != "Waiting" {
		t.Fatal("the late result replaced the lens that superseded it")
	}
	if outliner.lens.pending != nil || outliner.population.loading || outliner.population.spinnerVisible {
		t.Fatal("the late result left pending or loading state armed")
	}

	if !listsurface.RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outliner.primaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", outliner.primaryMode, chrome.ModeInbox)
	}
	if !slices.Equal(outliner.population.projects, baselineProjects) {
		t.Fatal("a superseded load leaked its population into the restored view")
	}
}

func TestRestoringTheBaselineDiscardsAnUnpublishedApplication(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))

	switchTo := waitingLens(lensTestID(2), "Included", "")
	switchTo = lensWithFacet(switchTo, "+parked")
	applyLensOrFatal(t, outliner, switchTo)
	generation := outliner.lens.pending.generation

	outliner.restoreLensBaseline()
	if outliner.lens.pending != nil {
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
