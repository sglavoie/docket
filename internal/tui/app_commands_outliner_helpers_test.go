package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// minimalAppModel returns a bare AppModel suitable for helper unit tests that
// only care about view-stack routing and outliner pointers.
func minimalAppModel() *AppModel {
	return &AppModel{
		views:     NewViewRegistry(),
		viewStack: []PresentationID{ViewDashboard},
	}
}

func TestActiveOutliner_PaletteOriginPicksSingle(t *testing.T) {
	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.crossOutliner = minimalOutliner()
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	if got := app.activeOutliner(); got != app.outliner {
		t.Fatalf("activeOutliner = %p, want single outliner %p", got, app.outliner)
	}
}

func TestActiveOutliner_PaletteOriginPicksCross(t *testing.T) {
	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.crossOutliner = minimalOutliner()
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewCommandPalette}

	if got := app.activeOutliner(); got != app.crossOutliner {
		t.Fatalf("activeOutliner = %p, want cross outliner %p", got, app.crossOutliner)
	}
}

func TestActiveOutliner_FallbackWhenOriginIsNotOutliner(t *testing.T) {
	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.viewStack = []PresentationID{ViewDashboard}

	if got := app.activeOutliner(); got != app.outliner {
		t.Fatalf("activeOutliner fallback = %p, want single outliner %p", got, app.outliner)
	}

	app.outliner = nil
	app.crossOutliner = minimalOutliner()
	if got := app.activeOutliner(); got != app.crossOutliner {
		t.Fatalf("activeOutliner fallback = %p, want cross outliner %p", got, app.crossOutliner)
	}
}

func TestActiveOutliner_NilWhenBothMissing(t *testing.T) {
	app := minimalAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	if got := app.activeOutliner(); got != nil {
		t.Fatalf("activeOutliner = %v, want nil", got)
	}
}

func TestOutlinerTarget_SingleProjectPaletteOrigin(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", models.SectionInbox)

	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.outliner.project = proj
	app.outliner.rows = []outlinerRow{{task: task, ProjectID: proj.ID}}
	app.outliner.cursor = 0
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	gotTask, gotProj := app.outlinerTarget()
	if gotTask != task || gotProj != proj {
		t.Fatalf("outlinerTarget = (%v, %v), want (%v, %v)", gotTask, gotProj, task, proj)
	}
}

func TestOutlinerTarget_CrossProjectPaletteOrigin(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	projB := &models.Project{ID: "proj-B"}
	a1 := makeTask("a1", "A1", "")
	b1 := makeTask("b1", "B1", "")

	app := minimalAppModel()
	app.crossOutliner = minimalOutliner()
	app.crossOutliner.loadedProjects = []*models.Project{projA, projB}
	app.crossOutliner.rows = []outlinerRow{
		{task: a1, ProjectID: projA.ID},
		{IsSeparator: true},
		{task: b1, ProjectID: projB.ID},
	}
	app.crossOutliner.cursor = 2
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewCommandPalette}

	gotTask, gotProj := app.outlinerTarget()
	if gotTask != b1 || gotProj != projB {
		t.Fatalf("outlinerTarget = (%v, %v), want (%v, %v)", gotTask, gotProj, b1, projB)
	}
}

func TestOutlinerTarget_HeaderAndSeparatorReturnNil(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", "")

	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.outliner.project = proj
	app.outliner.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{IsSeparator: true},
		{task: task, ProjectID: proj.ID},
	}
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	for _, tc := range []struct {
		name   string
		cursor int
	}{
		{"header", 0},
		{"separator", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.outliner.cursor = tc.cursor
			gotTask, gotProj := app.outlinerTarget()
			if gotTask != nil || gotProj != nil {
				t.Fatalf("outlinerTarget = (%v, %v), want (nil, nil)", gotTask, gotProj)
			}
		})
	}
}

func TestOutlinerTarget_CrossProjectStaleID(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	orphan := makeTask("orphan", "Orphan", "")

	app := minimalAppModel()
	app.crossOutliner = minimalOutliner()
	app.crossOutliner.loadedProjects = []*models.Project{projA}
	app.crossOutliner.rows = []outlinerRow{{task: orphan, ProjectID: "proj-missing"}}
	app.crossOutliner.cursor = 0
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewCommandPalette}

	gotTask, gotProj := app.outlinerTarget()
	if gotTask != nil || gotProj != nil {
		t.Fatalf("outlinerTarget stale = (%v, %v), want (nil, nil)", gotTask, gotProj)
	}
}

func TestOutlinerTarget_NilWhenNoOutliner(t *testing.T) {
	app := minimalAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	gotTask, gotProj := app.outlinerTarget()
	if gotTask != nil || gotProj != nil {
		t.Fatalf("outlinerTarget = (%v, %v), want (nil, nil)", gotTask, gotProj)
	}
}

func TestOutlinerCursorProject_SinglePaletteOrigin(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", "")

	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.outliner.project = proj
	app.outliner.rows = []outlinerRow{{task: task, ProjectID: proj.ID}}
	app.outliner.cursor = 0
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	if got := app.outlinerCursorProject(); got != proj {
		t.Fatalf("outlinerCursorProject = %v, want %v", got, proj)
	}
}

func TestOutlinerCursorProject_CrossPaletteOrigin(t *testing.T) {
	projA := &models.Project{ID: "proj-A"}
	projB := &models.Project{ID: "proj-B"}
	a := makeTask("a", "A", "")
	b := makeTask("b", "B", "")

	app := minimalAppModel()
	app.crossOutliner = minimalOutliner()
	app.crossOutliner.loadedProjects = []*models.Project{projA, projB}
	app.crossOutliner.rows = []outlinerRow{
		{task: a, ProjectID: projA.ID},
		{IsSeparator: true},
		{task: b, ProjectID: projB.ID},
	}
	app.crossOutliner.cursor = 2
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewCommandPalette}

	if got := app.outlinerCursorProject(); got != projB {
		t.Fatalf("outlinerCursorProject = %v, want %v", got, projB)
	}
}

func TestOutlinerCursorProject_HeaderSeparatorAndStale(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", "")

	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.outliner.project = proj
	app.outliner.rows = []outlinerRow{
		{IsSectionHeader: true, SectionName: models.SectionInbox},
		{IsSeparator: true},
		{task: task, ProjectID: proj.ID},
	}
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	for _, tc := range []struct {
		name   string
		cursor int
	}{
		{"header", 0},
		{"separator", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.outliner.cursor = tc.cursor
			if got := app.outlinerCursorProject(); got != nil {
				t.Fatalf("outlinerCursorProject = %v, want nil", got)
			}
		})
	}

	// Stale cross-project ID.
	projA := &models.Project{ID: "proj-A"}
	orphan := makeTask("orphan", "Orphan", "")
	app2 := minimalAppModel()
	app2.crossOutliner = minimalOutliner()
	app2.crossOutliner.loadedProjects = []*models.Project{projA}
	app2.crossOutliner.rows = []outlinerRow{{task: orphan, ProjectID: "proj-missing"}}
	app2.crossOutliner.cursor = 0
	app2.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewCommandPalette}

	if got := app2.outlinerCursorProject(); got != nil {
		t.Fatalf("outlinerCursorProject stale = %v, want nil", got)
	}
}

func TestOutlinerCursorProject_NilWhenNoOutliner(t *testing.T) {
	app := minimalAppModel()
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	if got := app.outlinerCursorProject(); got != nil {
		t.Fatalf("outlinerCursorProject = %v, want nil", got)
	}
}

func TestRunOutlinerCmd_NilReturnsNil(t *testing.T) {
	app := minimalAppModel()
	if got := app.runOutlinerCmd(nil); got != nil {
		t.Fatalf("runOutlinerCmd(nil) = %v, want nil", got)
	}
}

func TestRunOutlinerCmd_RefreshDispatchesToActiveOutliner(t *testing.T) {
	proj := &models.Project{ID: "proj"}
	task := makeTask("t", "T", "")

	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.outliner.project = proj
	app.outliner.rows = []outlinerRow{{task: task, ProjectID: proj.ID}}
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	cmd := func() tea.Msg { return outlinerRefreshMsg{} }
	if got := app.runOutlinerCmd(cmd); got != nil {
		t.Fatalf("runOutlinerCmd refresh = %v, want nil", got)
	}
}

func TestRunOutlinerCmd_BatchWalksRecursively(t *testing.T) {
	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	wantStatus := StatusMsg("hello")
	cmd := func() tea.Msg {
		return tea.BatchMsg{
			func() tea.Msg { return outlinerRefreshMsg{} },
			func() tea.Msg { return wantStatus },
		}
	}

	got := app.runOutlinerCmd(cmd)
	if got != wantStatus {
		t.Fatalf("runOutlinerCmd batch = %v, want %v", got, wantStatus)
	}
}

func TestRunOutlinerCmd_BatchAllRefreshReturnsNil(t *testing.T) {
	app := minimalAppModel()
	app.outliner = minimalOutliner()
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	cmd := func() tea.Msg {
		return tea.BatchMsg{
			func() tea.Msg { return outlinerRefreshMsg{} },
			func() tea.Msg { return outlinerRefreshMsg{} },
		}
	}
	if got := app.runOutlinerCmd(cmd); got != nil {
		t.Fatalf("runOutlinerCmd batch-all-refresh = %v, want nil", got)
	}
}

func TestRunOutlinerCmd_PassthroughForOtherMessages(t *testing.T) {
	app := minimalAppModel()
	want := StatusMsg("passthrough")
	cmd := func() tea.Msg { return want }

	if got := app.runOutlinerCmd(cmd); got != want {
		t.Fatalf("runOutlinerCmd passthrough = %v, want %v", got, want)
	}
}
