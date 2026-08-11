package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Fa shows archived projects in the ordinary Outliner, so every direct key is
// pressed against an archived task the same way a user would. The tests below
// compare the tasks.md bytes on disk before and after each key, which proves a
// refusal emitted no persistence mutation at all rather than one that happened
// to leave the inspected field alone.

const (
	archivedGuardActiveProjectName   = "Active Delivery"
	archivedGuardArchivedProjectName = "Retired Platform"
	archivedGuardActiveTaskTitle     = "Confirm the release window"
	archivedGuardArchivedTaskTitle   = "Revisit the retired migration"
)

// archivedGuardSettleTimeout budgets the task transaction the driver's default
// per-command drain is too short to finish inline.
const archivedGuardSettleTimeout = 2 * time.Second

type archivedGuardEnv struct {
	workspace *uitestWorkspace
	driver    *UITestDriver
	active    *models.Project
	archived  *models.Project
}

// newArchivedGuardEnv builds a real on-disk workspace holding one active and one
// archived project, launches the app over it, and turns on the Fa population
// facet so the archived tasks are visible rows.
func newArchivedGuardEnv(t *testing.T) *archivedGuardEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	active, err := workspace.addProject(archivedGuardActiveProjectName, func(p *models.Project) {
		task := newFixtureTask(p, archivedGuardActiveTaskTitle)
		task.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		t.Fatalf("add active project: %v", err)
	}
	archived, err := workspace.addProject(archivedGuardArchivedProjectName, func(p *models.Project) {
		p.Archived = true
		task := newFixtureTask(p, archivedGuardArchivedTaskTitle)
		task.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		t.Fatalf("add archived project: %v", err)
	}
	driver, err := workspace.launch(120, 40)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}

	env := &archivedGuardEnv{workspace: workspace, driver: driver, active: active, archived: archived}
	env.press(t, "F", "a")
	out := env.outliner(t)
	if out.projectFilter.ArchivedMode != projectPopulationInclude {
		t.Fatal("Fa did not turn on the archived population facet")
	}
	if !visibleOutlinerTaskIDs(out)[env.taskID(t, archived, archivedGuardArchivedTaskTitle)] {
		t.Fatalf("Fa showed no archived task rows (rows=%d)", len(out.rows))
	}
	return env
}

func (env *archivedGuardEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

func (env *archivedGuardEnv) outliner(t *testing.T) *OutlinerModel {
	t.Helper()
	out, ok := env.driver.App().outlinerPrimary()
	if !ok || out == nil {
		t.Fatal("the Outliner is not the active route")
	}
	return out
}

func (env *archivedGuardEnv) taskID(t *testing.T, project *models.Project, title string) string {
	t.Helper()
	loaded := env.reload(t, project)
	for _, task := range loaded.Tasks {
		if task.Title == title {
			return task.ID
		}
	}
	t.Fatalf("project %q has no task titled %q", project.Name, title)
	return ""
}

// focusTask walks the cursor with the public navigation keys so the test fails
// if the archived rows ever stop being reachable through ordinary movement.
func (env *archivedGuardEnv) focusTask(t *testing.T, taskID string) {
	t.Helper()
	env.press(t, "home")
	for range len(env.outliner(t).rows) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return
		}
		env.press(t, "down")
	}
	t.Fatalf("the cursor never reached task %s", taskID)
}

func (env *archivedGuardEnv) reload(t *testing.T, project *models.Project) *models.Project {
	t.Helper()
	fresh, err := models.NewProject(project.ID, project.Name, "")
	if err != nil {
		t.Fatalf("rebuild project %q: %v", project.Name, err)
	}
	fresh.Path = project.Path
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("load project %q: %v", project.Name, err)
	}
	return fresh
}

func (env *archivedGuardEnv) tasksFileBytes(t *testing.T, project *models.Project) string {
	t.Helper()
	content, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks file for %q: %v", project.Name, err)
	}
	return string(content)
}

// act sends one direct key sequence and then resolves whatever the 25ms drain
// budget abandoned, so a task transaction has genuinely finished — or genuinely
// never started — before the assertions read the files back.
func (env *archivedGuardEnv) act(t *testing.T, keys ...string) {
	t.Helper()
	env.driver.App().err = nil
	env.driver.pendingFutures = nil
	env.press(t, keys...)
	env.driver.Wait(archivedGuardSettleTimeout)
}

func (env *archivedGuardEnv) archivedRejection(t *testing.T, label string) {
	t.Helper()
	err := env.driver.App().err
	if err == nil {
		t.Fatalf("%s produced no rejection (status %q)", label, env.driver.App().statusMessage)
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Fatalf("%s rejection = %q, want the archived lifecycle refusal", label, err)
	}
}

// Each of these keys reaches a different execution route: the state chord and
// the priority key are registry-resolved task operations, edit and reorder are
// delegated core operations whose bindings skip registry target resolution, and
// pin is a bulk-capable direct mutation.
func TestArchivedTaskRefusesMutatingDirectKeys(t *testing.T) {
	cases := []struct {
		name string
		keys []string
	}{
		{name: "set state", keys: []string{"w", "i"}},
		{name: "set priority", keys: []string{"!", "1"}},
		{name: "set due date", keys: []string{"@"}},
		{name: "set tags", keys: []string{"#"}},
		{name: "toggle pin", keys: []string{"P"}},
		{name: "edit title", keys: []string{"e"}},
		{name: "edit description", keys: []string{"E"}},
		{name: "toggle today", keys: []string{"w", "t"}},
		{name: "reorder down", keys: []string{"J"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := newArchivedGuardEnv(t)
			env.focusTask(t, env.taskID(t, env.archived, archivedGuardArchivedTaskTitle))
			before := env.tasksFileBytes(t, env.archived)

			env.act(t, testCase.keys...)

			env.archivedRejection(t, testCase.name)
			if after := env.tasksFileBytes(t, env.archived); after != before {
				t.Fatalf("%s rewrote the archived project's tasks file", testCase.name)
			}
			if view := env.driver.App().currentView(); view != ViewCrossProjectOutliner {
				t.Fatalf("%s left the Outliner for %s", testCase.name, view)
			}
		})
	}
}

// Peek is the read-only counterpart: Fa exists so archived work can be read, so
// the guard must not turn the archived rows into inert ones.
func TestArchivedTaskKeepsReadOnlyDirectKeys(t *testing.T) {
	env := newArchivedGuardEnv(t)
	env.focusTask(t, env.taskID(t, env.archived, archivedGuardArchivedTaskTitle))

	env.act(t, "enter")

	if err := env.driver.App().err; err != nil {
		t.Fatalf("peek on an archived task failed: %v", err)
	}
	if !env.outliner(t).peekMode {
		t.Fatal("peek did not open over an archived task")
	}
}

// A bulk key over a mixed selection has to refuse the whole operation. Checking
// the active project's file too is the point: a per-target guard would still
// have written the active half.
func TestArchivedMixedSelectionRefusesBulkDirectKeys(t *testing.T) {
	env := newArchivedGuardEnv(t)
	activeTaskID := env.taskID(t, env.active, archivedGuardActiveTaskTitle)
	archivedTaskID := env.taskID(t, env.archived, archivedGuardArchivedTaskTitle)

	env.focusTask(t, activeTaskID)
	env.press(t, "V")
	env.focusSelectionThrough(t, archivedTaskID)

	out := env.outliner(t)
	selected := out.SelectedTaskIdentities()
	if len(selected) < 2 {
		t.Fatalf("the visual selection holds %d tasks, want the active and archived pair", len(selected))
	}
	activeBefore := env.tasksFileBytes(t, env.active)
	archivedBefore := env.tasksFileBytes(t, env.archived)

	env.act(t, "w", "i")

	env.archivedRejection(t, "bulk set state")
	if after := env.tasksFileBytes(t, env.active); after != activeBefore {
		t.Fatal("the refused bulk operation still mutated the active project")
	}
	if after := env.tasksFileBytes(t, env.archived); after != archivedBefore {
		t.Fatal("the refused bulk operation still mutated the archived project")
	}
}

// focusSelectionThrough extends an open visual selection down to the named task.
func (env *archivedGuardEnv) focusSelectionThrough(t *testing.T, taskID string) {
	t.Helper()
	for range len(env.outliner(t).rows) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return
		}
		env.press(t, "down")
	}
	t.Fatalf("the visual selection never reached task %s", taskID)
}

// The guard is scoped to archived owners only, so showing them must not disturb
// the tasks a user can still change.
func TestArchivedPopulationLeavesActiveTasksMutable(t *testing.T) {
	env := newArchivedGuardEnv(t)
	activeTaskID := env.taskID(t, env.active, archivedGuardActiveTaskTitle)
	env.focusTask(t, activeTaskID)

	env.act(t, "w", "i")

	if err := env.driver.App().err; err != nil {
		t.Fatalf("the active task refused a state change: %v", err)
	}
	loaded := env.reload(t, env.active)
	task := loaded.GetTaskByID(activeTaskID)
	if task == nil {
		t.Fatalf("task %s vanished from the active project", activeTaskID)
	}
	if task.State() != models.StateInbox {
		t.Fatalf("the active task persisted state %s, want inbox", task.State())
	}
}
