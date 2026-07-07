package tui

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestUITestFixtureNames(t *testing.T) {
	got := BuiltinFixtureNames()
	want := []string{"alerts", "empty", "narrow", "rich", "workflow"}
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
			if ws.Config.DataDir != ws.DataDir || !strings.HasPrefix(ws.DataDir, ws.Root) {
				t.Fatalf("config DataDir %q not rooted in workspace %q", ws.Config.DataDir, ws.Root)
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

	for _, p := range driver.App().config.GetActiveProjects() {
		if len(p.Tasks) != 0 {
			t.Fatalf("empty fixture project %q unexpectedly has %d tasks", p.Name, len(p.Tasks))
		}
	}
}

func TestUITestFixtureRichLoadsTasksFromDisk(t *testing.T) {
	driver, ws, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("NewFixtureDriver(rich) error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	current := driver.App().config.GetCurrentProject()
	if current == nil {
		t.Fatal("rich fixture has no current project")
	}
	if len(current.Tasks) == 0 {
		t.Fatal("rich fixture current project has no tasks after load from disk")
	}

	// Tasks must have round-tripped through storage: the on-disk tasks.md
	// exists inside the isolated workspace.
	tasksPath := storage.GetTasksFilePath(current)
	if !strings.HasPrefix(tasksPath, ws.DataDir) {
		t.Fatalf("project tasks path %q not inside workspace data dir %q", tasksPath, ws.DataDir)
	}
	if _, statErr := os.Stat(tasksPath); statErr != nil {
		t.Fatalf("expected tasks.md on disk at %q: %v", tasksPath, statErr)
	}
}

func TestUITestWorkspaceCopyIsIsolated(t *testing.T) {
	src := newSourceWorkspace(t)
	srcProject := src.Config.Projects[0]
	srcTasksPath := storage.GetTasksFilePath(srcProject)
	srcBefore := mustReadFile(t, srcTasksPath)

	driver, ws, err := NewCopiedWorkspaceDriver(src.ConfigPath, 100, 30)
	if err != nil {
		t.Fatalf("NewCopiedWorkspaceDriver error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	// The copy lives in its own temp tree, distinct from the source.
	if ws.DataDir == src.DataDir {
		t.Fatal("copied workspace shares the source data dir")
	}
	if driver.App().config.DataDir != ws.DataDir {
		t.Fatalf("driver config DataDir = %q, want copied dir %q", driver.App().config.DataDir, ws.DataDir)
	}

	// The copied project points at the temp copy and its tasks.md matches source.
	copied := driver.App().config.GetProject(srcProject.ID)
	if copied == nil {
		t.Fatal("copied config is missing the source project")
	}
	copiedTasksPath := storage.GetTasksFilePath(copied)
	if !strings.HasPrefix(copiedTasksPath, ws.DataDir) {
		t.Fatalf("copied project path %q not inside copy data dir %q", copiedTasksPath, ws.DataDir)
	}
	if got := mustReadFile(t, copiedTasksPath); got != srcBefore {
		t.Fatal("copied tasks.md differs from source")
	}
}

func TestUITestWorkspaceDoesNotMutateSource(t *testing.T) {
	src := newSourceWorkspace(t)
	srcProject := src.Config.Projects[0]
	srcTasksPath := storage.GetTasksFilePath(srcProject)
	srcConfigBefore := mustReadFile(t, src.ConfigPath)
	srcTasksBefore := mustReadFile(t, srcTasksPath)

	driver, ws, err := NewCopiedWorkspaceDriver(src.ConfigPath, 100, 30)
	if err != nil {
		t.Fatalf("NewCopiedWorkspaceDriver error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	// Write through the copied project — the isolation guarantee is that this
	// never reaches the source tree.
	copied := driver.App().config.GetProject(srcProject.ID)
	if copied == nil {
		t.Fatal("copied config is missing the source project")
	}
	newTask, err := models.NewTask("copy-only-task", copied.NextShortID, models.TaskTypeTask, "Copy-only task")
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
	if got := mustReadFile(t, src.ConfigPath); got != srcConfigBefore {
		t.Fatal("source config was mutated by copied-workspace launch")
	}
}

func TestUITestWorkspaceIsolatesExternalProject(t *testing.T) {
	ws, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	// A project stored OUTSIDE the source data dir (a custom project location).
	externalDir := t.TempDir()
	project, err := models.NewProject(uuid.New().String(), "External Project", "")
	if err != nil {
		t.Fatalf("NewProject error: %v", err)
	}
	project.Path = externalDir
	project.EnsureDefaultSections()
	task := newFixtureTask(project, models.TaskTypeTask, "External task")
	task.Priority = 1
	project.BuildTaskTree()
	if err = storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(external) error: %v", err)
	}
	ws.Config.Projects = append(ws.Config.Projects, project)
	ws.Config.CurrentProject = project.ID
	if err = ws.Manager.Save(ws.Config); err != nil {
		t.Fatalf("Manager.Save error: %v", err)
	}

	externalTasksPath := storage.GetTasksFilePath(project)
	externalBefore := mustReadFile(t, externalTasksPath)

	driver, copyWs, err := NewCopiedWorkspaceDriver(ws.ConfigPath, 100, 30)
	if err != nil {
		t.Fatalf("NewCopiedWorkspaceDriver error: %v", err)
	}
	t.Cleanup(func() { _ = copyWs.Cleanup() })

	// The external project must be relocated INTO the copy, not left pointed at
	// the source tree, and its tasks.md must match the source content.
	copied := driver.App().config.GetProject(project.ID)
	if copied == nil {
		t.Fatal("copied config is missing the external project")
	}
	if !strings.HasPrefix(copied.Path, copyWs.DataDir) {
		t.Fatalf("external project path %q not relocated under copy data dir %q", copied.Path, copyWs.DataDir)
	}
	copiedTasksPath := storage.GetTasksFilePath(copied)
	if got := mustReadFile(t, copiedTasksPath); got != externalBefore {
		t.Fatal("copied external tasks.md differs from source")
	}

	// Writing through the copied project must not touch the original external
	// tasks.md — the core isolation guarantee for custom project locations.
	newTask, err := models.NewTask("copy-only-task", copied.NextShortID, models.TaskTypeTask, "Copy-only task")
	if err != nil {
		t.Fatalf("NewTask error: %v", err)
	}
	copied.Tasks = append(copied.Tasks, newTask)
	copied.BuildTaskTree()
	if err := storage.SaveProject(copied); err != nil {
		t.Fatalf("SaveProject(copied) error: %v", err)
	}
	if got := mustReadFile(t, externalTasksPath); got != externalBefore {
		t.Fatal("source external tasks.md was mutated by copied-workspace writes")
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
		epic := newFixtureTask(p, models.TaskTypeEpic, "Source epic")
		epic.Priority = 1
		story := newFixtureTask(p, models.TaskTypeStory, "Source story")
		story.ParentID = &epic.ID
	})
	if err != nil {
		t.Fatalf("addProject error: %v", err)
	}
	ws.Config.CurrentProject = project.ID
	if err := ws.Manager.Save(ws.Config); err != nil {
		t.Fatalf("Manager.Save error: %v", err)
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
