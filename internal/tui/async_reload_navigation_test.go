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
)

func TestCrossProjectColdLoadAlwaysRendersLoadingFrame(t *testing.T) {
	outliner := NewOutlinerModel(nil, nil, nil)
	outliner.SetSize(100, 30)
	if cmd := outliner.Init(); cmd == nil {
		t.Fatal("cold Outliner did not start its initial load")
	}

	content := outliner.View().Content
	if content == "" || !strings.Contains(content, "Loading projects") {
		t.Fatalf("cold loading frame = %q, want non-empty loading state", content)
	}
}

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

func newSession2OutlinerApp(t *testing.T, dataDir string, projects ...*models.Project) (*AppModel, *OutlinerModel) {
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

	cross := NewOutlinerModel(nil, app.configManager, app.runtime)
	cross.loadedProjects = projects
	cross.selectOutlinerMode(ModeAll)
	cross.crossForceRebuild = true
	cross.crossProjectRebuildRows()
	app.crossOutliner = cross
	app.views.Register(ViewCrossProjectOutliner, cross)
	setTestPresentationPath(app, ViewUnknown, ViewCrossProjectOutliner)
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

func session2TaskRow(t *testing.T, cross *OutlinerModel, projectID, taskID string) int {
	t.Helper()
	for i, row := range cross.rows {
		if row.isTaskRow() && row.ProjectID == projectID && row.task != nil && row.task.ID == taskID {
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
	pushTestPresentationOverlay(app, ViewTaskComments)

	cmd := app.handleWithRefresh(false, true)

	if cmd != nil {
		t.Fatal("covered Outliner reload should defer until it is revealed")
	}
	if cross.crossLoading {
		t.Fatal("covered Outliner must not start a load whose completion cannot be routed")
	}
	if len(cross.loadedProjects) != 1 {
		t.Fatalf("covered Outliner cache changed before reveal: %d projects", len(cross.loadedProjects))
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
	if app.currentView() != ViewCrossProjectOutliner {
		t.Fatalf("fallback view = %s, want %s", app.currentView(), ViewCrossProjectOutliner)
	}
	if !cross.crossLoading {
		t.Fatal("fallback Outliner reload was not armed")
	}

	runSession2AppCommand(t, app, cmd)

	if cross.crossLoading {
		t.Fatal("returned stale-project reload command did not settle")
	}
	if len(cross.loadedProjects) != 1 || cross.loadedProjects[0].ID != survivor.ID {
		t.Fatalf("fallback loaded projects = %#v, want only survivor", cross.loadedProjects)
	}
}

func TestBackMsgReturnsAppearanceReloadCommand(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "back")
	app, cross := newSession2OutlinerApp(t, dataDir, project)
	pushTestPresentationOverlay(app, ViewTaskComments)
	generation := cross.crossLoadGen

	_, cmd := app.Update(BackMsg{})
	if cmd == nil {
		t.Fatal("BackMsg dropped popView's ViewDidAppearMsg command")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Fatalf("revealed view = %s, want %s", app.currentView(), ViewCrossProjectOutliner)
	}

	runSession2AppCommand(t, app, cmd)

	if cross.crossLoadGen != generation+1 {
		t.Fatalf("Back reload generation = %d, want %d", cross.crossLoadGen, generation+1)
	}
	if cross.crossLoading {
		t.Fatal("Back-triggered appearance reload did not settle")
	}
}

func TestViewDidAppearDoesNotSupersedePendingReloadAndPreservesCursor(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "cursor")
	app, cross := newSession2OutlinerApp(t, dataDir, project)
	task := project.Tasks[0]
	cross.cursor = session2TaskRow(t, cross, project.ID, task.ID)

	loadCmd := cross.reloadCrossProjects()
	generation := cross.crossLoadGen
	savedCursor := cross.cursor
	_, duplicateCmd := app.Update(ViewDidAppearMsg{})

	if duplicateCmd != nil {
		t.Fatal("appearance signal should not launch a second load while one is pending")
	}
	if cross.crossLoadGen != generation {
		t.Fatalf("appearance signal advanced generation to %d, want %d", cross.crossLoadGen, generation)
	}
	if cross.cursor != savedCursor {
		t.Fatalf("pending reload cursor changed synchronously: got %d want %d", cross.cursor, savedCursor)
	}

	runSession2AppCommand(t, app, loadCmd)

	if cross.crossLoading {
		t.Fatal("pending reload did not settle")
	}
	row := cross.rows[cross.cursor]
	if !row.isTaskRow() || row.task == nil || row.ProjectID != project.ID || row.task.ID != task.ID {
		t.Fatalf("cursor identity was not restored after reload: %#v", row)
	}
}

func TestCrossProjectRefreshKeepsAcceptedSnapshotVisible(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "visible refresh")
	app, cross := newSession2OutlinerApp(t, dataDir, project)
	cross.crossSnapshotReady = true
	cross.SetSize(100, 30)
	cross.cursor = session2TaskRow(t, cross, project.ID, project.Tasks[0].ID)
	cross.viewScrollOffset = 1
	projectsBefore := cross.loadedProjects
	rowsBefore := len(cross.rows)

	loadCmd := cross.reloadCrossProjects()
	if loadCmd == nil || !cross.crossLoading {
		t.Fatal("refresh did not start")
	}
	if len(cross.loadedProjects) != len(projectsBefore) || len(cross.rows) != rowsBefore ||
		len(projectsBefore) > 0 && cross.loadedProjects[0] != projectsBefore[0] {
		t.Fatalf("refresh cleared accepted content: projects=%d rows=%d", len(cross.loadedProjects), len(cross.rows))
	}
	if cross.viewScrollOffset != 1 {
		t.Fatalf("refresh reset scroll offset to %d", cross.viewScrollOffset)
	}
	content := app.View().Content
	if content == "" || strings.Contains(content, "Unknown view") || !strings.Contains(content, project.Tasks[0].Title) {
		t.Fatalf("refresh frame did not retain task content:\n%s", content)
	}

	_, spinnerCmd := cross.Update(crossSpinnerDelayMsg{})
	if spinnerCmd != nil || cross.crossSpinnerVisible {
		t.Fatal("background refresh replaced content with the initial-load spinner")
	}
	if content = app.View().Content; content == "" || strings.Contains(content, "Unknown view") {
		t.Fatalf("delayed refresh frame became invalid:\n%s", content)
	}

	// Rendering a short list legitimately clamps its viewport to the top. Set a
	// non-zero offset immediately before the accepted result to isolate the
	// reload handler's preservation behavior.
	cross.viewScrollOffset = 1
	runSession2AppCommand(t, app, loadCmd)
	if cross.crossLoading {
		t.Fatal("refresh did not settle")
	}
	if cross.viewScrollOffset != 1 {
		t.Fatalf("accepted refresh changed scroll offset to %d", cross.viewScrollOffset)
	}
}

func TestCrossProjectReloadMissingCursorTaskPrefersFollowingTask(t *testing.T) {
	original := scopeTestProject("alpha", "Alpha", "before", "removed", "after")
	cross := NewOutlinerModel(nil, nil, nil)
	cross.loadedProjects = []*models.Project{original}
	cross.crossSnapshotReady = true
	cross.pushProjectScope(original.ID)
	cross.crossForceRebuild = true
	cross.crossProjectRebuildRows()
	cross.cursor = session2TaskRow(t, cross, original.ID, "removed")
	cross.viewScrollOffset = 1

	_ = cross.reloadCrossProjects()
	generation := cross.crossLoadGen
	replacement := scopeTestProject("alpha", "Alpha", "before", "after")
	cross.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   []*models.Project{replacement},
		generation: generation,
	})

	if cross.cursor < 0 || cross.cursor >= len(cross.rows) {
		t.Fatalf("cursor %d is outside %d rebuilt rows", cross.cursor, len(cross.rows))
	}
	row := cross.rows[cross.cursor]
	if !row.isTaskRow() || row.task == nil || row.task.ID != "after" {
		t.Fatalf("fallback cursor = %#v, want following task", row)
	}
	if cross.viewScrollOffset != 1 {
		t.Fatalf("fallback changed scroll offset to %d", cross.viewScrollOffset)
	}
}

func TestCoveredCrossProjectLoadCompletionSettlesAuthoritativeOutliner(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "covered completion")
	app, cross := newSession2OutlinerApp(t, dataDir, project)

	_ = cross.reloadCrossProjects()
	generation := cross.crossLoadGen
	pushTestPresentationOverlay(app, ViewTaskComments)

	raw := cross.loadCrossProjectsCmd()()
	loaded, ok := raw.(crossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("load command returned %T, want crossProjectsLoadedMsg", raw)
	}
	_, followup := app.Update(loaded)
	if followup != nil {
		runSession2AppCommand(t, app, followup)
	}

	if cross.crossLoading {
		t.Fatal("covered load completion left the authoritative Outliner loading")
	}
	if len(cross.loadedProjects) != 1 || cross.loadedProjects[0].ID != project.ID {
		t.Fatalf("covered load completion did not update the Outliner cache: %#v", cross.loadedProjects)
	}

	_, popCmd := app.Update(BackMsg{})
	if popCmd == nil {
		t.Fatal("returning to the Outliner did not emit its appearance reload")
	}
	runSession2AppCommand(t, app, popCmd)

	if cross.crossLoadGen != generation+1 {
		t.Fatalf("appearance reload generation = %d, want %d", cross.crossLoadGen, generation+1)
	}
	if cross.crossLoading {
		t.Fatal("appearance reload after covered completion did not settle")
	}
}
