package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
)

func TestLensProjectLoadPublishesOnlyOnTheMatchingGeneration(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	baseline := slices.Clone(outlinerView.Population.Projects)

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked:only")
	cmd := applyLensOrFatal(t, outlinerView, lens)
	if cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("nothing may publish before the load lands")
	}
	if outlinerView.ProjectFilter.ParkedMode == query.ProjectPopulationOnly {
		t.Fatal("the candidate filter must not be written to the live model")
	}
	if !slices.Equal(outlinerView.Population.Projects, baseline) {
		t.Fatal("the live population changed before the load landed")
	}

	generation := outlinerView.Lens.Pending.Generation
	loaded := []*models.Project{primaryRowsProject("gamma", primaryRowsTask("gamma-task", models.StateWaiting))}

	// A stale result from an earlier generation publishes nothing.
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects: loaded, Generation: generation - 1,
	}); published {
		t.Fatal("a stale generation published a lens")
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("a stale result installed a lens")
	}

	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects: loaded, Generation: generation,
	}); !published {
		t.Fatal("the matching generation did not publish")
	}
	if !outlinerView.HasActiveLens() {
		t.Fatal("the matching generation did not install the lens")
	}
	if outlinerView.ProjectFilter.ParkedMode != query.ProjectPopulationOnly {
		t.Fatal("publication did not install the lens project facets")
	}
	if ids := loadedProjectIDs(outlinerView); !slices.Equal(ids, []string{"gamma"}) {
		t.Fatalf("published population = %v, want [gamma]", ids)
	}
}

func TestLensProjectLoadIsSupersededByALaterRequest(t *testing.T) {
	outlinerView := lensLifecycleOutliner()

	first := waitingLens(lensTestID(1), "Waiting", "")
	first = lensWithFacet(first, "+parked:only")
	applyLensOrFatal(t, outlinerView, first)
	stale := outlinerView.Lens.Pending.Generation

	second := waitingLens(lensTestID(2), "Included", "")
	second = lensWithFacet(second, "+parked")
	applyLensOrFatal(t, outlinerView, second)
	live := outlinerView.Lens.Pending.Generation
	if live == stale {
		t.Fatal("a superseding request must take its own generation")
	}

	loaded := []*models.Project{primaryRowsProject("gamma")}
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects: loaded, Generation: stale,
	}); published {
		t.Fatal("the superseded request published")
	}
	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects: loaded, Generation: live,
	}); !published {
		t.Fatal("the newest request did not publish")
	}
	if outlinerView.Lens.Active.Name != "Included" {
		t.Fatalf("published lens = %q, want Included", outlinerView.Lens.Active.Name)
	}
}

func TestLensProjectLoadAbortsWhenNothingLoaded(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	baseline := slices.Clone(outlinerView.Population.Projects)

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked:only")
	applyLensOrFatal(t, outlinerView, lens)

	cmd, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		FailedLoads: []outliner.ProjectLoadFailure{{ProjectID: "gamma", Detail: "failed to load project gamma: boom"}},
		Generation:  outlinerView.Lens.Pending.Generation,
	})
	if published {
		t.Fatal("a wholly failed load published a lens")
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("a wholly failed load installed a lens")
	}
	if !slices.Equal(outlinerView.Population.Projects, baseline) {
		t.Fatal("a wholly failed load disturbed the live population")
	}
	if outlinerView.Population.Loading {
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
	outlinerView := lensLifecycleOutliner()

	lens := waitingLens(lensTestID(1), "Waiting", "")
	lens = lensWithFacet(lens, "+parked:only")
	applyLensOrFatal(t, outlinerView, lens)

	cmd, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects:    []*models.Project{primaryRowsProject("gamma")},
		FailedLoads: []outliner.ProjectLoadFailure{{ProjectID: "delta", Detail: "failed to load project delta: boom"}},
		Generation:  outlinerView.Lens.Pending.Generation,
	})
	if !published {
		t.Fatal("a partial load must publish the projects that read")
	}
	if !outlinerView.HasActiveLens() {
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
	outlinerView := lensLifecycleOutliner()
	outlinerView.SelectOutlinerMode(chrome.ModeInbox)
	outlinerView.Folds.CollapsedSections["baseline-fold"] = true
	baselineProjects := slices.Clone(outlinerView.Population.Projects)

	lens := waitingLens(lensTestID(1), "Parked", "")
	lens = lensWithFacet(lens, "+parked:only")
	if cmd := applyLensOrFatal(t, outlinerView, lens); cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	if got := outlinerView.Lens.Pending.Origin.Presentation.Mode; got != chrome.ModeInbox {
		t.Fatalf("frozen baseline mode = %v, want %v", got, chrome.ModeInbox)
	}

	// The user keeps navigating and searching while the projects read. None of it
	// may rewrite the baseline the application already froze.
	outlinerView.SelectOutlinerMode(chrome.ModeBacklog)
	delete(outlinerView.Folds.CollapsedSections, "baseline-fold")
	outlinerView.SetSearchQuery("needle")

	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects: []*models.Project{
			primaryRowsProject("gamma", primaryRowsTask("gamma-task", models.StateWaiting)),
		},
		Generation: outlinerView.Lens.Pending.Generation,
	}); !published {
		t.Fatal("the matching generation did not publish")
	}

	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("Esc left the lens active")
	}
	if outlinerView.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-load %v", outlinerView.PrimaryMode, chrome.ModeInbox)
	}
	if !outlinerView.Folds.CollapsedSections["baseline-fold"] {
		t.Fatal("restoration lost the fold the baseline held before the load")
	}
	if outlinerView.HasSearchProjection() {
		t.Fatal("restoration installed a search projection the frozen baseline never held")
	}
	if !slices.Equal(outlinerView.Population.Projects, baselineProjects) {
		t.Fatal("restoration did not return the pre-load population")
	}
}

// TestLensProjectLoadSupersededBySynchronousLensClearsPendingState covers the
// one supersession that changes nothing about the generation counter: a lens
// that needs no I/O publishes immediately, and the application still loading
// behind it must be settled rather than left holding the loading state.
func TestLensProjectLoadSupersededBySynchronousLensClearsPendingState(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	outlinerView.SelectOutlinerMode(chrome.ModeInbox)
	baselineProjects := slices.Clone(outlinerView.Population.Projects)

	async := waitingLens(lensTestID(1), "Parked", "")
	async = lensWithFacet(async, "+parked:only")
	if cmd := applyLensOrFatal(t, outlinerView, async); cmd == nil {
		t.Fatal("a project-population change must start a load")
	}
	stale := outlinerView.Lens.Pending.Generation
	// The 200ms delay has elapsed, so the spinner is on screen when the
	// synchronous lens supersedes the load.
	outlinerView.Population.SpinnerVisible = true

	if cmd := applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(2), "Waiting", "")); cmd != nil {
		t.Fatal("an unchanged population must not start a load")
	}
	if outlinerView.Lens.Pending != nil {
		t.Fatal("a synchronous lens left an application pending")
	}
	if outlinerView.Population.Loading || outlinerView.Population.SpinnerVisible {
		t.Fatal("a synchronous lens left the loading state armed")
	}
	if outlinerView.Lens.Active.PendingGeneration != 0 {
		t.Fatal("the published frame still awaits a superseded generation")
	}

	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects: []*models.Project{primaryRowsProject("gamma")}, Generation: stale,
	}); published {
		t.Fatal("the superseded request published")
	}
	if outlinerView.Lens.Active == nil || outlinerView.Lens.Active.Name != "Waiting" {
		t.Fatal("the late result replaced the lens that superseded it")
	}
	if outlinerView.Lens.Pending != nil || outlinerView.Population.Loading || outlinerView.Population.SpinnerVisible {
		t.Fatal("the late result left pending or loading state armed")
	}

	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected the Esc cascade to consume the active lens")
	}
	if outlinerView.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", outlinerView.PrimaryMode, chrome.ModeInbox)
	}
	if !slices.Equal(outlinerView.Population.Projects, baselineProjects) {
		t.Fatal("a superseded load leaked its population into the restored view")
	}
}

func TestRestoringTheBaselineDiscardsAnUnpublishedApplication(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", ""))

	switchTo := waitingLens(lensTestID(2), "Included", "")
	switchTo = lensWithFacet(switchTo, "+parked")
	applyLensOrFatal(t, outlinerView, switchTo)
	generation := outlinerView.Lens.Pending.Generation

	outlinerView.RestoreLensBaseline()
	if outlinerView.Lens.Pending != nil {
		t.Fatal("restoration left an application pending")
	}

	if _, published := outlinerView.HandleLensProjectsLoaded(outliner.LensProjectsLoadedMsg{
		Projects: []*models.Project{primaryRowsProject("gamma")}, Generation: generation,
	}); published {
		t.Fatal("a load landing after restoration reinstalled a lens")
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("a load landing after restoration installed a lens")
	}
}
