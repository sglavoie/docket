package cmd

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newHierarchyTestCmd returns a bare command carrying the mutation output flags so
// the hierarchy/type-change adapters can read --json/--quiet.
func newHierarchyTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "hier"}
	addOutputFlags(cmd)
	return cmd
}

// seedTwoRootLeaves replaces the seeded project's tasks with two known root leaf
// tasks in a deterministic order and persists them, so an ordering test starts from
// a stable state.
func seedTwoRootLeaves(t *testing.T, deps *testutil.TestDeps, project *models.Project) (*models.Task, *models.Task) {
	t.Helper()
	first, err := models.NewTask("route-first", 1, models.TaskTypeTask, "first")
	testutil.AssertNoError(t, err)
	second, err := models.NewTask("route-second", 2, models.TaskTypeTask, "second")
	testutil.AssertNoError(t, err)
	project.Tasks = []*models.Task{first, second}
	project.NextShortID = 3
	project.BuildTaskTree()
	testutil.AssertNoError(t, deps.Repository.Save(project))
	return first, second
}

// TestPromoteRoutesThroughApp proves `tt task promote` raises a leaf task's type
// through the application-layer ChangeTaskType use case and persists it via the
// repository (fallback save path under the in-memory test repository).
func TestPromoteRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	leaf, err := models.NewTask("route-leaf", 1, models.TaskTypeTask, "root leaf")
	testutil.AssertNoError(t, err)
	project.Tasks = []*models.Task{leaf}
	project.NextShortID = 2
	project.BuildTaskTree()
	testutil.AssertNoError(t, deps.Repository.Save(project))

	cmd := newHierarchyTestCmd()
	err = runTaskPromote(cmd, []string{"1"})
	testutil.AssertNoError(t, err)

	got := deps.Repository.Projects[project.ID].GetTaskByID(leaf.ID)
	if got == nil {
		t.Fatalf("task %s missing from saved project", leaf.ID)
	}
	if got.Type != models.TaskTypeStory {
		t.Errorf("type = %q, want story", got.Type)
	}
}

// TestPromoteEpicRejected proves an ineligible explicit-ID promote surfaces the
// model's rejection and persists nothing.
func TestPromoteEpicRejected(t *testing.T) {
	deps, project := setupStateTest(t)
	epic, err := models.NewTask("route-epic", 1, models.TaskTypeEpic, "Epic")
	testutil.AssertNoError(t, err)
	project.Tasks = []*models.Task{epic}
	project.NextShortID = 2
	project.BuildTaskTree()
	testutil.AssertNoError(t, deps.Repository.Save(project))

	cmd := newHierarchyTestCmd()
	if err := runTaskPromote(cmd, []string{"1"}); err == nil {
		t.Fatal("expected error promoting an epic")
	}
	got := deps.Repository.Projects[project.ID].GetTaskByID(epic.ID)
	if got.Type != models.TaskTypeEpic {
		t.Errorf("type = %q, want epic (unchanged)", got.Type)
	}
}

// TestMoveUpRoutesThroughApp proves `tt task move up` reorders siblings through the
// application-layer MoveTaskUp use case.
func TestMoveUpRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	_, second := seedTwoRootLeaves(t, deps, project)

	cmd := newHierarchyTestCmd()
	err := runTaskMoveUp(cmd, []string{fmt.Sprintf("%d", second.ShortID)})
	testutil.AssertNoError(t, err)

	saved := deps.Repository.Projects[project.ID]
	if saved.Tasks[0].ID != "route-second" || saved.Tasks[1].ID != "route-first" {
		t.Fatalf("order = [%s %s], want [route-second route-first]", saved.Tasks[0].ID, saved.Tasks[1].ID)
	}
}

// TestMoveUpAtTopRejected proves an ineligible explicit-ID move surfaces the guard
// error from the application layer.
func TestMoveUpAtTopRejected(t *testing.T) {
	deps, project := setupStateTest(t)
	first, _ := seedTwoRootLeaves(t, deps, project)

	cmd := newHierarchyTestCmd()
	if err := runTaskMoveUp(cmd, []string{fmt.Sprintf("%d", first.ShortID)}); err == nil {
		t.Fatal("expected error moving the top task up")
	}
}

// TestMoveToSameProjectRejected proves the cross-project move adapter rejects a move
// whose destination is the current project before touching the application layer.
func TestMoveToSameProjectRejected(t *testing.T) {
	_, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	taskMoveToForce = true
	t.Cleanup(func() { taskMoveToForce = false })

	err := runTaskMoveTo(taskMoveToCmd, []string{fmt.Sprintf("%d", task.ShortID), project.Slug})
	if err == nil {
		t.Fatal("expected error moving a task to its own project")
	}
}
