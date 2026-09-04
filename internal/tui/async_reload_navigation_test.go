package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

func newSession2Project(t *testing.T, dataDir, name string) *models.Project {
	t.Helper()
	task, err := models.NewGenericTask(name+"-task", 1, name+" task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = models.StateBacklog
	project, err := models.NewProject(name+"-project", name, filepath.Join(dataDir, "projects", models.Slugify(name)))
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", project.Path, err)
	}
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	task.Project = project
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	return project
}

func newSession2OutlinerApp(t *testing.T, dataDir string, projects ...*models.Project) (*AppModel, *outliner.OutlinerModel) {
	t.Helper()
	app := newTestAppModel()
	seed := newTestConfigAt(dataDir)
	setTestProjects(seed, projects)
	app.runtime = testRuntimeAt(dataDir, seed)
	app.configManager = config.NewManager(config.ConfigPathForDataDir(dataDir))
	ledger := workspaceLedgerFromConfig(app.runtime)
	app.appService = newTUIAppService(dataDir, app.configManager.RuntimeDependencies(), ledger)
	app.deps = newShellDeps(ledger)
	setTestAppProjects(app, projects...)

	cross := outliner.NewOutlinerModel(nil, app.configManager, app.runtime)
	cross.Population.Projects = projects
	cross.SelectOutlinerMode(chrome.ModeAll)
	cross.Population.Invalidate()
	cross.CrossProjectRebuildRows()
	app.crossOutliner = cross
	app.viewNav.views.Register(presentation.ViewCrossProjectOutliner, cross)
	setTestPresentationPath(app, presentation.ViewUnknown, presentation.ViewCrossProjectOutliner)
	return app, cross
}

func runSession2AppCommand(t *testing.T, app *AppModel, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 200 {
			t.Fatal("command queue did not settle")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		_, followup := app.Update(msg)
		if followup != nil {
			queue = append(queue, followup)
		}
	}
}

func session2TaskRow(t *testing.T, cross *outliner.OutlinerModel, projectID, taskID string) int {
	t.Helper()
	for i, row := range cross.Rows {
		if row.IsTaskRow() && row.ProjectID == projectID && row.Task != nil && row.Task.ID == taskID {
			return i
		}
	}
	t.Fatalf("task row %s/%s not found", projectID, taskID)
	return -1
}

func TestHandleWithRefreshDefersCrossReloadWhileCovered(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "covered")
	app, cross := newSession2OutlinerApp(t, dataDir, project)
	pushTestPresentationOverlay(app, presentation.ViewTaskComments)

	cmd := app.handleWithRefresh(false, true)

	if cmd != nil {
		t.Fatal("covered Outliner reload should defer until it is revealed")
	}
	if cross.Population.Loading {
		t.Fatal("covered Outliner must not start a load whose completion cannot be routed")
	}
	if len(cross.Population.Projects) != 1 {
		t.Fatalf("covered Outliner cache changed before reveal: %d projects", len(cross.Population.Projects))
	}
}

func TestValidateCurrentProjectReturnsReloadCommand(t *testing.T) {
	dataDir := t.TempDir()
	stale := newSession2Project(t, dataDir, "stale")
	survivor := newSession2Project(t, dataDir, "survivor")
	app, cross := newSession2OutlinerApp(t, dataDir, stale, survivor)
	app.currentProject = stale

	// Simulate a CLI deleting the current project behind the running TUI.
	setTestAppProjects(app, survivor)

	handled, cmd := app.handleGlobalKeyPress(syntheticKey('z'))
	if !handled || cmd == nil {
		t.Fatalf("stale-project guard = handled %v cmd %v, want returned reload batch", handled, cmd)
	}
	if app.currentProject != nil {
		t.Fatal("stale current project was not cleared")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("fallback view = %s, want %s", app.currentView(), presentation.ViewCrossProjectOutliner)
	}
	if !cross.Population.Loading {
		t.Fatal("fallback Outliner reload was not armed")
	}

	runSession2AppCommand(t, app, cmd)

	if cross.Population.Loading {
		t.Fatal("returned stale-project reload command did not settle")
	}
	if len(cross.Population.Projects) != 1 || cross.Population.Projects[0].ID != survivor.ID {
		t.Fatalf("fallback loaded projects = %#v, want only survivor", cross.Population.Projects)
	}
}

func TestBackMsgReturnsAppearanceReloadCommand(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "back")
	app, cross := newSession2OutlinerApp(t, dataDir, project)
	pushTestPresentationOverlay(app, presentation.ViewTaskComments)
	generation := cross.Population.LoadGen

	_, cmd := app.Update(status.BackMsg{})
	if cmd == nil {
		t.Fatal("BackMsg dropped popView's ViewDidAppearMsg command")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Fatalf("revealed view = %s, want %s", app.currentView(), presentation.ViewCrossProjectOutliner)
	}

	runSession2AppCommand(t, app, cmd)

	if cross.Population.LoadGen != generation+1 {
		t.Fatalf("Back reload generation = %d, want %d", cross.Population.LoadGen, generation+1)
	}
	if cross.Population.Loading {
		t.Fatal("Back-triggered appearance reload did not settle")
	}
}

func TestCoveredCrossProjectLoadCompletionSettlesAuthoritativeOutliner(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "covered completion")
	app, cross := newSession2OutlinerApp(t, dataDir, project)

	_ = cross.ReloadCrossProjects()
	generation := cross.Population.LoadGen
	pushTestPresentationOverlay(app, presentation.ViewTaskComments)

	raw := cross.LoadCrossProjectsCmd()()
	loaded, ok := raw.(outliner.CrossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("load command returned %T, want crossProjectsLoadedMsg", raw)
	}
	_, followup := app.Update(loaded)
	if followup != nil {
		runSession2AppCommand(t, app, followup)
	}

	if cross.Population.Loading {
		t.Fatal("covered load completion left the authoritative Outliner loading")
	}
	if len(cross.Population.Projects) != 1 || cross.Population.Projects[0].ID != project.ID {
		t.Fatalf("covered load completion did not update the Outliner cache: %#v", cross.Population.Projects)
	}

	_, popCmd := app.Update(status.BackMsg{})
	if popCmd == nil {
		t.Fatal("returning to the Outliner did not emit its appearance reload")
	}
	runSession2AppCommand(t, app, popCmd)

	if cross.Population.LoadGen != generation+1 {
		t.Fatalf("appearance reload generation = %d, want %d", cross.Population.LoadGen, generation+1)
	}
	if cross.Population.Loading {
		t.Fatal("appearance reload after covered completion did not settle")
	}
}

func TestCrossProjectColdLoadAlwaysRendersLoadingFrame(t *testing.T) {
	outlinerView := outliner.NewOutlinerModel(nil, nil, nil)
	outlinerView.SetSize(100, 30)
	if cmd := outlinerView.Init(); cmd == nil {
		t.Fatal("cold Outliner did not start its initial load")
	}

	content := outlinerView.View().Content
	if content == "" || !strings.Contains(content, "Loading projects") {
		t.Fatalf("cold loading frame = %q, want non-empty loading state", content)
	}
}

func TestViewDidAppearDoesNotSupersedePendingReloadAndPreservesCursor(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "cursor")
	app, cross := newSession2OutlinerApp(t, dataDir, project)
	task := project.Tasks[0]
	cross.Cursor = session2TaskRow(t, cross, project.ID, task.ID)

	loadCmd := cross.ReloadCrossProjects()
	generation := cross.Population.LoadGen
	savedCursor := cross.Cursor
	_, duplicateCmd := app.Update(status.ViewDidAppearMsg{})

	if duplicateCmd != nil {
		t.Fatal("appearance signal should not launch a second load while one is pending")
	}
	if cross.Population.LoadGen != generation {
		t.Fatalf("appearance signal advanced generation to %d, want %d", cross.Population.LoadGen, generation)
	}
	if cross.Cursor != savedCursor {
		t.Fatalf("pending reload cursor changed synchronously: got %d want %d", cross.Cursor, savedCursor)
	}

	runSession2AppCommand(t, app, loadCmd)

	if cross.Population.Loading {
		t.Fatal("pending reload did not settle")
	}
	row := cross.Rows[cross.Cursor]
	if !row.IsTaskRow() || row.Task == nil || row.ProjectID != project.ID || row.Task.ID != task.ID {
		t.Fatalf("cursor identity was not restored after reload: %#v", row)
	}
}

func TestCrossProjectRefreshKeepsAcceptedSnapshotVisible(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "visible refresh")
	app, cross := newSession2OutlinerApp(t, dataDir, project)
	cross.Population.SnapshotReady = true
	cross.SetSize(100, 30)
	cross.Cursor = session2TaskRow(t, cross, project.ID, project.Tasks[0].ID)
	cross.ViewScrollOffset = 1
	projectsBefore := cross.Population.Projects
	rowsBefore := len(cross.Rows)

	loadCmd := cross.ReloadCrossProjects()
	if loadCmd == nil || !cross.Population.Loading {
		t.Fatal("refresh did not start")
	}
	if len(cross.Population.Projects) != len(projectsBefore) || len(cross.Rows) != rowsBefore ||
		len(projectsBefore) > 0 && cross.Population.Projects[0] != projectsBefore[0] {
		t.Fatalf("refresh cleared accepted content: projects=%d rows=%d", len(cross.Population.Projects), len(cross.Rows))
	}
	if cross.ViewScrollOffset != 1 {
		t.Fatalf("refresh reset scroll offset to %d", cross.ViewScrollOffset)
	}
	content := app.View().Content
	if content == "" || strings.Contains(content, "Unknown view") || !strings.Contains(content, project.Tasks[0].Title) {
		t.Fatalf("refresh frame did not retain task content:\n%s", content)
	}

	_, spinnerCmd := cross.Update(outliner.CrossSpinnerDelayMsg{})
	if spinnerCmd != nil || cross.Population.SpinnerVisible {
		t.Fatal("background refresh replaced content with the initial-load spinner")
	}
	if content = app.View().Content; content == "" || strings.Contains(content, "Unknown view") {
		t.Fatalf("delayed refresh frame became invalid:\n%s", content)
	}

	// Rendering a short list legitimately clamps its viewport to the top. Set a
	// non-zero offset immediately before the accepted result to isolate the
	// reload handler's preservation behavior.
	cross.ViewScrollOffset = 1
	runSession2AppCommand(t, app, loadCmd)
	if cross.Population.Loading {
		t.Fatal("refresh did not settle")
	}
	if cross.ViewScrollOffset != 1 {
		t.Fatalf("accepted refresh changed scroll offset to %d", cross.ViewScrollOffset)
	}
}

func TestCrossProjectReloadMissingCursorTaskPrefersFollowingTask(t *testing.T) {
	original := scopeTestProject("alpha", "Alpha", "before", "removed", "after")
	cross := outliner.NewOutlinerModel(nil, nil, nil)
	cross.Population.Projects = []*models.Project{original}
	cross.Population.SnapshotReady = true
	cross.PushProjectScope(original.ID)
	cross.Population.Invalidate()
	cross.CrossProjectRebuildRows()
	cross.Cursor = session2TaskRow(t, cross, original.ID, "removed")
	cross.ViewScrollOffset = 1

	_ = cross.ReloadCrossProjects()
	generation := cross.Population.LoadGen
	replacement := scopeTestProject("alpha", "Alpha", "before", "after")
	cross.HandleCrossProjectsLoaded(outliner.CrossProjectsLoadedMsg{
		Projects:   []*models.Project{replacement},
		Generation: generation,
	})

	if cross.Cursor < 0 || cross.Cursor >= len(cross.Rows) {
		t.Fatalf("cursor %d is outside %d rebuilt rows", cross.Cursor, len(cross.Rows))
	}
	row := cross.Rows[cross.Cursor]
	if !row.IsTaskRow() || row.Task == nil || row.Task.ID != "after" {
		t.Fatalf("fallback cursor = %#v, want following task", row)
	}
	if cross.ViewScrollOffset != 1 {
		t.Fatalf("fallback changed scroll offset to %d", cross.ViewScrollOffset)
	}
}
