package tui

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

func TestUITestFixtureNames(t *testing.T) {
	got := BuiltinFixtureNames()
	want := []string{"busy", "checklist", "empty", "fresh", "inbox-triage", "multi-project", "narrow", "parked", "project-mix", "release", "rich", "signals", "stale", "workflow"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuiltinFixtureNames() = %v, want %v", got, want)
	}
}

func TestUITestFixtureUnknown(t *testing.T) {
	driver, ws, err := NewFixtureDriver("does-not-exist", 100, 30)
	if err == nil {
		if ws != nil {
			_ = ws.Cleanup()
		}
		t.Fatal("NewFixtureDriver(unknown) expected error, got nil")
	}
	if driver != nil || ws != nil {
		t.Fatalf("NewFixtureDriver(unknown) returned non-nil driver/ws on error: %v %v", driver, ws)
	}
}

func TestUITestFixtureBuildsAndRenders(t *testing.T) {
	for _, name := range BuiltinFixtureNames() {
		t.Run(name, func(t *testing.T) {
			width, height := 100, 30
			if name == FixtureNarrow {
				width, height = 62, 24
			}
			driver, ws, err := NewFixtureDriver(name, width, height)
			if err != nil {
				t.Fatalf("NewFixtureDriver(%q) error: %v", name, err)
			}
			t.Cleanup(func() { _ = ws.Cleanup() })

			// The workspace must live in a temp dir, not real user data.
			if !strings.HasPrefix(ws.Root, os.TempDir()) {
				t.Fatalf("workspace root %q is not under temp dir %q", ws.Root, os.TempDir())
			}
			if ws.Runtime.DataDir() != ws.DataDir || !strings.HasPrefix(ws.DataDir, ws.Root) {
				t.Fatalf("runtime DataDir %q not rooted in workspace %q", ws.Runtime.DataDir(), ws.Root)
			}

			obs := driver.Observe(ObserveOptions{})
			if strings.TrimSpace(obs.PlainScreen) == "" {
				t.Fatalf("fixture %q rendered an empty screen", name)
			}
			if strings.Contains(obs.PlainScreen, "\x1b") {
				t.Fatalf("fixture %q screen still contains ANSI escapes", name)
			}

			// Cleanup must remove the temp tree.
			root := ws.Root
			if err := ws.Cleanup(); err != nil {
				t.Fatalf("Cleanup() error: %v", err)
			}
			if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
				t.Fatalf("Cleanup() left temp dir behind: stat err = %v", statErr)
			}
		})
	}
}

func TestUITestFixtureEmptyHasNoUserTasks(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureEmpty, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver(empty) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	for _, p := range driver.App().taskWorkspaceProjects() {
		if len(p.Tasks) != 0 {
			t.Fatalf("empty fixture project %q unexpectedly has %d tasks", p.Name, len(p.Tasks))
		}
	}
}

func TestUITestFixtureParkedSplitsTaskWorkspaceFromCatalog(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureParked, 100, 35)
	if err != nil {
		t.Fatalf("NewFixtureDriver(parked) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	runtime := driver.App().runtime
	parked := projectByFixtureName(runtime.AvailableProjects(), parkedFixtureParkedName)
	if parked == nil {
		t.Fatalf("%q is not an available project", parkedFixtureParkedName)
	}
	if !models.ProjectTagsParked(parked.Tags) {
		t.Fatalf("parked project tags = %v, want the reserved parked tag", parked.Tags)
	}
	if runtime.TaskWorkspaceProject(parked.ID) != nil {
		t.Fatalf("%q is in the base task workspace despite being parked", parked.Name)
	}

	active := projectByFixtureName(runtime.TaskWorkspaceProjects(), parkedFixtureWorkspaceName)
	if active == nil {
		t.Fatalf("%q is missing from the base task workspace", parkedFixtureWorkspaceName)
	}
	if models.ProjectTagsParked(active.Tags) {
		t.Fatalf("base workspace project tags = %v, want no parked tag", active.Tags)
	}

	screen := driver.Observe(ObserveOptions{}).PlainScreen
	if !strings.Contains(screen, "Ship the delivery checklist") {
		t.Fatalf("Active Projects is missing the unparked project's work:\n%s", screen)
	}
	if strings.Contains(screen, "Revisit the paused migration") {
		t.Fatalf("Active Projects leaked a parked project's work:\n%s", screen)
	}
}

func projectByFixtureName(projects []*models.Project, name string) *models.Project {
	for _, project := range projects {
		if project != nil && project.Name == name {
			return project
		}
	}
	return nil
}

func TestFreshFixtureBootstrapsStableVisibleSystemInbox(t *testing.T) {
	driver, ws, fixtureErr := NewFixtureDriver(FixtureFresh, 100, 35)
	if fixtureErr != nil {
		t.Fatalf("NewFixtureDriver(fresh) error: %v", fixtureErr)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	inbox := driver.App().runtime.SystemUnfiledProject()
	if inbox == nil {
		t.Fatal("fresh install has no system Inbox")
	}
	if inbox.Name != config.SystemUnfiledName || inbox.Archived {
		t.Fatalf("system Inbox metadata = %+v", inbox)
	}
	active := driver.App().runtime.TaskWorkspaceProjects()
	if len(active) != 1 || active[0].ID != inbox.ID {
		t.Fatalf("fresh active projects = %+v, want visible system Inbox", active)
	}
	if _, err := os.Stat(storage.TasksFilePath(inbox.Path)); err != nil {
		t.Fatalf("system Inbox tasks file: %v", err)
	}

	reloaded, reloadErr := config.LoadRuntimeState(ws.DataDir)
	if reloadErr != nil {
		t.Fatalf("reload runtime: %v", reloadErr)
	}
	relaunched, err := NewAppModel(reloaded, config.NewManager(ws.ConfigPath))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	if got := relaunched.runtime.SystemUnfiledProject(); got == nil || got.ID != inbox.ID {
		t.Fatalf("system Inbox identity changed across launch: before=%q after=%v", inbox.ID, got)
	}
}

func TestContextualQuickCapturePersistsInOwningProject(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	target := driver.App().currentProject
	inbox := driver.App().runtime.SystemUnfiledProject()
	if target == nil || inbox == nil || target.ID == inbox.ID {
		t.Fatalf("fixture projects: target=%v inbox=%v", target, inbox)
	}
	targetCount := len(target.Tasks)
	inboxCount := len(inbox.Tasks)

	driver.dispatch(driver.App().captureComposerMessage(target, driver.App().taskWorkspaceProjects()))
	if driver.App().currentView() != presentation.ViewTaskComposer {
		t.Fatalf("current view = %s, want QuickCapture", driver.App().currentView())
	}
	const title = "Contextual capture stays with Rich Project"
	for _, action := range []UITestAction{
		{Type: UITestActionText, Text: title},
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	} {
		if _, actionErr := driver.ApplyAction(action, ObserveOptions{}); actionErr != nil {
			t.Fatal(actionErr)
		}
	}

	if err = storage.LoadProject(target); err != nil {
		t.Fatalf("reload target project: %v", err)
	}
	if err = storage.LoadProject(inbox); err != nil {
		t.Fatalf("reload system Inbox: %v", err)
	}
	if len(target.Tasks) != targetCount+1 {
		t.Fatalf("target task count = %d, want %d", len(target.Tasks), targetCount+1)
	}
	if len(inbox.Tasks) != inboxCount {
		t.Fatalf("system Inbox task count = %d, want %d", len(inbox.Tasks), inboxCount)
	}

	var captured *models.Task
	for _, task := range target.Tasks {
		if task.Title == title {
			captured = task
			break
		}
	}
	if captured == nil {
		t.Fatalf("captured task %q not found in target project", title)
	}
	if captured.State() != models.StateInbox {
		t.Fatalf("captured task section = %q, want Inbox", captured.State())
	}
	for _, task := range inbox.Tasks {
		if task.Title == title {
			t.Fatalf("captured task %q was persisted in the system Inbox", title)
		}
	}
}

func TestUITestFixtureRichLoadsTasksFromDisk(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("NewFixtureDriver(rich) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	current := driver.App().currentProject
	if current == nil {
		t.Fatal("rich fixture has no current project")
	}
	if len(current.Tasks) == 0 {
		t.Fatal("rich fixture current project has no tasks after load from disk")
	}

	// Tasks must have round-tripped through storage: the on-disk tasks.md
	// exists inside the isolated workspace.
	tasksPath := storage.TasksFilePath(current.Path)
	if !strings.HasPrefix(tasksPath, ws.DataDir) {
		t.Fatalf("project tasks path %q not inside workspace data dir %q", tasksPath, ws.DataDir)
	}
	if _, statErr := os.Stat(tasksPath); statErr != nil {
		t.Fatalf("expected tasks.md on disk at %q: %v", tasksPath, statErr)
	}
}

func TestUITestFixtureRichLoadsGlobalAndProjectNotesFromDisk(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("NewFixtureDriver(rich) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	project := driver.App().currentProject
	if project == nil {
		t.Fatal("rich fixture has no current project")
	}
	globalNotes, err := storage.ListNotesFrom(storage.GlobalNotesDir(ws.DataDir))
	if err != nil {
		t.Fatalf("load rich Global notes: %v", err)
	}
	projectNotes, err := storage.ListNotes(project)
	if err != nil {
		t.Fatalf("load rich project notes: %v", err)
	}
	if len(globalNotes) != 1 || globalNotes[0].Title != "Workspace launch brief" {
		t.Fatalf("rich Global notes = %#v", globalNotes)
	}
	if len(projectNotes) != 1 ||
		projectNotes[0].Title != "Rich Project delivery plan" ||
		!strings.Contains(projectNotes[0].Body, "authentication milestone") {
		t.Fatalf("rich project notes = %#v", projectNotes)
	}
}

func TestUITestWorkspaceCopyIsIsolated(t *testing.T) {
	src := newSourceWorkspace(t)
	srcProject := src.Runtime.TaskWorkspaceProjects()[0]
	srcTasksPath := storage.TasksFilePath(srcProject.Path)
	srcBefore := mustReadFile(t, srcTasksPath)
	sourceCatalog, err := config.NewDomainFileStore(src.DataDir).LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog(source): %v", err)
	}
	sourceWorkspaceID := sourceCatalog.WorkspaceID
	sourceMachine := config.NewMachineState()
	sourceMachine.Theme = "light"
	sourceStore := config.NewDomainFileStore(src.DataDir)
	if bindErr := sourceStore.BindWorkspaceID(sourceWorkspaceID); bindErr != nil {
		t.Fatalf("BindWorkspaceID(source): %v", bindErr)
	}
	if saveErr := sourceStore.SaveMachine(sourceMachine); saveErr != nil {
		t.Fatalf("SaveMachine(source): %v", saveErr)
	}

	driver, ws, err := NewCopiedWorkspaceDriver(src.ConfigPath, 100, 30)
	if err != nil {
		t.Fatalf("NewCopiedWorkspaceDriver error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	// The copy lives in its own temp tree, distinct from the source.
	if ws.DataDir == src.DataDir {
		t.Fatal("copied workspace shares the source data dir")
	}
	if driver.App().runtime.DataDir() != ws.DataDir {
		t.Fatalf("driver runtime DataDir = %q, want copied dir %q", driver.App().runtime.DataDir(), ws.DataDir)
	}
	if got := driver.App().runtime.Catalog().WorkspaceID; got == "" || got == sourceWorkspaceID {
		t.Fatalf("copied workspace ID = %q, want a fresh identity distinct from %q", got, sourceWorkspaceID)
	}
	if got, want := driver.App().runtime.Machine().Theme, config.NewMachineState().Theme; got != want {
		t.Fatalf("copied workspace machine theme = %q, want isolated default %q", got, want)
	}

	// The copied project points at the temp copy and its tasks.md matches source.
	copied := driver.App().projectByID(srcProject.ID)
	if copied == nil {
		t.Fatal("copied config is missing the source project")
	}
	copiedTasksPath := storage.TasksFilePath(copied.Path)
	if !strings.HasPrefix(copiedTasksPath, ws.DataDir) {
		t.Fatalf("copied project path %q not inside copy data dir %q", copiedTasksPath, ws.DataDir)
	}
	if got := mustReadFile(t, copiedTasksPath); got != srcBefore {
		t.Fatal("copied tasks.md differs from source")
	}
}

func TestUITestWorkspaceDoesNotMutateSource(t *testing.T) {
	src := newSourceWorkspace(t)
	srcProject := src.Runtime.TaskWorkspaceProjects()[0]
	srcTasksPath := storage.TasksFilePath(srcProject.Path)
	// Persistence is the decomposed domain files, not the legacy monolithic
	// .tt.json (which Manager.Save no longer writes), so the source "config" to
	// guard against mutation is the workspace-catalog domain artifact.
	srcCatalogPath := config.WorkspaceCatalogPath(src.DataDir)
	srcConfigBefore := mustReadFile(t, srcCatalogPath)
	srcTasksBefore := mustReadFile(t, srcTasksPath)

	driver, ws, err := NewCopiedWorkspaceDriver(src.ConfigPath, 100, 30)
	if err != nil {
		t.Fatalf("NewCopiedWorkspaceDriver error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	// Write through the copied project — the isolation guarantee is that this
	// never reaches the source tree.
	copied := driver.App().projectByID(srcProject.ID)
	if copied == nil {
		t.Fatal("copied config is missing the source project")
	}
	newTask, err := models.NewGenericTask("copy-only-task", copied.NextShortID, "Copy-only task")
	if err != nil {
		t.Fatalf("NewTask error: %v", err)
	}
	copied.Tasks = append(copied.Tasks, newTask)
	copied.BuildTaskTree()
	if err := storage.SaveProject(copied); err != nil {
		t.Fatalf("SaveProject(copied) error: %v", err)
	}

	if got := mustReadFile(t, srcTasksPath); got != srcTasksBefore {
		t.Fatal("source tasks.md was mutated by copied-workspace writes")
	}
	if got := mustReadFile(t, srcCatalogPath); got != srcConfigBefore {
		t.Fatal("source config was mutated by copied-workspace launch")
	}
}

// newSourceWorkspace builds an isolated workspace with one project + tasks on
// disk and its config file written, to act as the "real user data" source for
// copied-workspace tests. It is cleaned up automatically.
func newSourceWorkspace(t *testing.T) *uitestWorkspace {
	t.Helper()
	ws, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	project, err := ws.addProject("Source Project", func(p *models.Project) {
		epic := newFixtureTask(p, "Source epic")
		epic.Priority = 1
		story := newFixtureTask(p, "Source story")
		story.ParentID = &epic.ID
	})
	if err != nil {
		t.Fatalf("addProject error: %v", err)
	}
	ws.setCurrentProject(project.ID)
	if err := ws.persistSyncedDomains(); err != nil {
		t.Fatalf("persistSyncedDomains error: %v", err)
	}
	return ws
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(data)
}

// TestFixtureDueInstantSurvivesTheMidnightMinute pins the wall-clock hazard that
// broke every busy-fixture build for one minute a day: a raw time.Now() offset
// carries seconds the schema-13 codec renders away, and the "00:00" left behind
// is the date-only signal it refuses to spell as a due_time. Truncating to the
// minute is what the grammar produces, so midnight becomes an honest date-only
// due instead of an unrepresentable timed one.
func TestFixtureDueInstantSurvivesTheMidnightMinute(t *testing.T) {
	// The offsets buildBusyFixture and the release fixture hand setDue.
	offsets := []time.Duration{-3 * 24 * time.Hour, -2 * 24 * time.Hour, -24 * time.Hour, 12 * time.Hour}
	// A base 22.5s past noon sends the +12h offset into the midnight minute,
	// which is the instant the failing run happened to occupy.
	base := time.Date(2026, 8, 27, 12, 0, 22, 500_000_000, time.Local)

	ws, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	for _, offset := range offsets {
		due := fixtureDueInstant(base, offset)
		if due.Second() != 0 || due.Nanosecond() != 0 {
			t.Errorf("fixtureDueInstant(base, %v) = %v, want a whole minute", offset, due)
		}
		project, err := ws.addProject(fmt.Sprintf("Due %v", offset), func(p *models.Project) {
			task := newFixtureTask(p, "Timed due")
			task.ApplyWorkflowState(models.StateBacklog)
			task.DueDate = &due
		})
		if err != nil {
			t.Fatalf("addProject with due %v (offset %v): %v", due, offset, err)
		}
		reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
		if err := storage.LoadProject(reloaded); err != nil {
			t.Fatalf("LoadProject after due %v: %v", due, err)
		}
	}
}
