package cmd

import (
	"fmt"
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// newStateTestCmd returns a bare command carrying the mutation output flags so
// runTaskState/runAppTransition can read --json/--quiet.
func newStateTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "state"}
	addOutputFlags(cmd)
	return cmd
}

// findTodoLeaf returns a todo leaf task from the seeded project.
func findTodoLeaf(t *testing.T, project *models.Project) *models.Task {
	t.Helper()
	for _, task := range project.Tasks {
		if task.State() == models.StateReady && len(task.Children) == 0 {
			return task
		}
	}
	t.Fatalf("no todo leaf task found in seeded project")
	return nil
}

func setupStateTest(t *testing.T) (*testDeps, *models.Project) {
	t.Helper()
	deps, project := setupTestDepsWithProject(t, "State Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })

	return deps, project
}

// TestTaskStateTransitionsThroughAppLayer proves `tt task state` routes a
// transition through the application service and persists the new workflow state
// via the repository (fallback save path under the in-memory test repository).
func TestTaskStateTransitionsThroughAppLayer(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)
	project.BuildTaskTree()

	cmd := newStateTestCmd()
	err := runTaskState(cmd, []string{"waiting", fmt.Sprintf("%d", task.ShortID)})
	testutil.AssertNoError(t, err)

	saved := deps.Repository.Projects[project.ID]
	got := saved.GetTaskByID(task.ID)
	if got == nil {
		t.Fatalf("task %s missing from saved project", task.ID)
	}
	if got.State() != models.StateWaiting {
		t.Errorf("state = %q, want %q", got.State(), models.StateWaiting)
	}
}

// TestTaskStateConvenienceRoutesThroughApp proves a convenience command (ready)
// transitions the task to its fixed target state.
func TestTaskStateConvenienceRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)
	project.BuildTaskTree()

	cmd := newStateTestCmd()
	err := runAppTransition(cmd, []string{fmt.Sprintf("%d", task.ShortID)}, models.StateNext)
	testutil.AssertNoError(t, err)

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if got.State() != models.StateNext {
		t.Errorf("state = %q, want %q", got.State(), models.StateNext)
	}
}

func TestTaskStateQualifiedDuplicateUsesOwningProject(t *testing.T) {
	root := t.TempDir()
	alpha := newFileBackedProject(t, root, "state-alpha", "State Alpha")
	beta := newFileBackedProject(t, root, "state-beta", "State Beta")
	const duplicateID = "duplicate-state-task"
	addTestTask(t, alpha, duplicateID, "Alpha duplicate", models.StateReady)
	addTestTask(t, beta, duplicateID, "Beta duplicate", models.StateReady)
	setupFileBackedWorkflowCLI(t, root, alpha, []*models.Project{alpha, beta}, nil, nil)

	err := runAppTransition(newStateTestCmd(), []string{beta.Slug + ":" + duplicateID}, models.StateWaiting)
	testutil.AssertNoError(t, err)

	reloadWorkflowProject(t, alpha)
	reloadWorkflowProject(t, beta)
	if got := alpha.GetTaskByID(duplicateID).State(); got != models.StateReady {
		t.Fatalf("alpha duplicate state = %q, want ready", got)
	}
	if got := beta.GetTaskByID(duplicateID).State(); got != models.StateWaiting {
		t.Fatalf("beta duplicate state = %q, want waiting", got)
	}
}

// TestTaskStateRejectsInvalidState reports an invalid target state without
// mutating anything.
func TestTaskStateRejectsInvalidState(t *testing.T) {
	_, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	cmd := newStateTestCmd()
	err := runTaskState(cmd, []string{"bogus", fmt.Sprintf("%d", task.ShortID)})
	if err == nil {
		t.Fatal("expected an error for an invalid workflow state")
	}
}

func TestTaskStateCompletesCanonicalTargetFirst(t *testing.T) {
	got, directive := completeTaskStateArgs(taskStateCmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion directive = %v", directive)
	}
	if !slices.Equal(got, stateCompletions) {
		t.Fatalf("state completions = %v, want %v", got, stateCompletions)
	}
	for _, retired := range []string{"todo", "blocked", "standby", "maybe"} {
		if slices.Contains(got, retired) {
			t.Fatalf("canonical state completion includes retired lifecycle value %q", retired)
		}
	}
}
