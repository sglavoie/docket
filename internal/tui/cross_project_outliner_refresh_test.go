package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func newCrossProjectRefreshPrimaryBundle() PrimaryViewBundle {
	cfg := newTestConfig()
	bundle := NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg))
	bundle.projectService = NewProjectService()
	return bundle
}

// TestHandleWithRefreshTriggersCrossProjectReload verifies the S2 contract:
// when handleWithRefresh runs with reload=true and the outliner is in
// cross-project mode (project==nil), reloadCrossProjects() is invoked so
// the disk-backed loadedProjects cache is refreshed. The saved cursor is
// preserved across the synchronous portion of the reload.
func TestHandleWithRefreshTriggersCrossProjectReload(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)

	loaded := []*models.Project{
		{ID: "alpha", Name: "Alpha"},
		{ID: "beta", Name: "Beta"},
	}
	// The global Outliner root owns the cross-project cache, so the active
	// cross-project instance is m.crossOutliner (activeCrossProjectOutliner
	// prefers it). Install the hand-built cross-mode model there.
	app.outliner = nil
	app.crossOutliner = &OutlinerModel{
		PrimaryViewBundle:    newCrossProjectRefreshPrimaryBundle(),
		InteractiveViewState: InteractiveViewState{cursor: 7},
		project:              nil, // cross-project mode
		loadedProjects:       loaded,
	}

	cmd := app.handleWithRefresh(false, true, "")

	if cmd == nil {
		t.Fatal("expected non-nil cmd containing cross-project reload")
	}
	if app.crossOutliner.loadedProjects != nil {
		t.Error("expected loadedProjects to be cleared by reloadCrossProjects")
	}
	if !app.crossOutliner.crossLoading {
		t.Error("expected crossLoading=true after startCrossProjectLoad")
	}
	if app.crossOutliner.cursor != 7 {
		t.Errorf("expected cursor preserved at 7, got %d", app.crossOutliner.cursor)
	}
}

// TestHandleWithRefreshNoReloadSkipsCrossProjectReload verifies that with
// reload=false the cross-project cache is left intact.
func TestHandleWithRefreshNoReloadSkipsCrossProjectReload(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)

	loaded := []*models.Project{{ID: "alpha", Name: "Alpha"}}
	app.outliner = &OutlinerModel{
		PrimaryViewBundle: newCrossProjectRefreshPrimaryBundle(),
		project:           nil,
		loadedProjects:    loaded,
	}

	_ = app.handleWithRefresh(false, false, "")

	if app.outliner.loadedProjects == nil {
		t.Error("expected loadedProjects untouched when reload=false")
	}
	if app.outliner.crossLoading {
		t.Error("expected crossLoading=false when reload=false")
	}
}

// TestHandleWithRefreshTriggersCrossOutlinerInstanceReload verifies that
// when the cross-project outliner is stored in m.crossOutliner (the real
// instance used by ViewCrossProjectOutliner) rather than in m.outliner,
// handleWithRefresh(reload=true) still kicks off a cross-project reload
// against m.crossOutliner. Before the fix, refreshAllViews()/handleWithRefresh
// only inspected m.outliner, leaving the distinct cross-project view stale.
func TestHandleWithRefreshTriggersCrossOutlinerInstanceReload(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.outliner = nil

	loaded := []*models.Project{
		{ID: "alpha", Name: "Alpha"},
		{ID: "beta", Name: "Beta"},
	}
	app.crossOutliner = &OutlinerModel{
		PrimaryViewBundle:    newCrossProjectRefreshPrimaryBundle(),
		InteractiveViewState: InteractiveViewState{cursor: 7},
		project:              nil, // cross-project mode
		loadedProjects:       loaded,
	}

	cmd := app.handleWithRefresh(false, true, "")

	if cmd == nil {
		t.Fatal("expected non-nil cmd containing cross-project reload")
	}
	if app.crossOutliner.loadedProjects != nil {
		t.Error("expected crossOutliner.loadedProjects to be cleared by reloadCrossProjects")
	}
	if !app.crossOutliner.crossLoading {
		t.Error("expected crossOutliner.crossLoading=true after startCrossProjectLoad")
	}
	if app.crossOutliner.cursor != 7 {
		t.Errorf("expected cursor preserved at 7, got %d", app.crossOutliner.cursor)
	}
}

// TestHandleWithRefreshSingleProjectOutlinerSkipsCrossReload verifies that
// when the outliner is in single-project mode (project!=nil), the new
// cross-project reload branch is a no-op.
func TestHandleWithRefreshSingleProjectOutlinerSkipsCrossReload(t *testing.T) {
	app, _, project := newTestAppModelWithMockPersisterAndProject(t)

	loaded := []*models.Project{{ID: "alpha", Name: "Alpha"}}
	app.outliner = &OutlinerModel{
		PrimaryViewBundle: newCrossProjectRefreshPrimaryBundle(),
		project:           project,
		loadedProjects:    loaded,
	}

	_ = app.handleWithRefresh(false, true, "")

	if app.outliner.loadedProjects == nil {
		t.Error("expected loadedProjects untouched in single-project mode")
	}
	if app.outliner.crossLoading {
		t.Error("expected crossLoading=false in single-project mode")
	}
}
