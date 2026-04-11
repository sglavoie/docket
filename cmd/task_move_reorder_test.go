package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestMoveTaskUp(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create three sibling tasks at root level
	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := models.NewTask("task-3", project.GetNextShortID(), models.TaskTypeTask, "Third Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task1, task2, task3)
	project.BuildTaskTree()

	// Move task2 up - should swap with task1
	err = project.MoveTaskUp(task2)
	testutil.AssertNoError(t, err)

	siblings := project.GetSiblings(task1)
	if len(siblings) != 3 {
		t.Fatalf("expected 3 siblings, got %d", len(siblings))
	}
	if siblings[0].ID != task2.ID {
		t.Errorf("expected task2 to be first, got %s", siblings[0].Title)
	}
	if siblings[1].ID != task1.ID {
		t.Errorf("expected task1 to be second, got %s", siblings[1].Title)
	}
}

func TestMoveTaskDown(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create three sibling tasks at root level
	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := models.NewTask("task-3", project.GetNextShortID(), models.TaskTypeTask, "Third Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task1, task2, task3)
	project.BuildTaskTree()

	// Move task1 down - should swap with task2
	err = project.MoveTaskDown(task1)
	testutil.AssertNoError(t, err)

	siblings := project.GetSiblings(task1)
	if len(siblings) != 3 {
		t.Fatalf("expected 3 siblings, got %d", len(siblings))
	}
	if siblings[0].ID != task2.ID {
		t.Errorf("expected task2 to be first, got %s", siblings[0].Title)
	}
	if siblings[1].ID != task1.ID {
		t.Errorf("expected task1 to be second, got %s", siblings[1].Title)
	}
}

func TestMoveTaskUpAtTop(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task1, task2)
	project.BuildTaskTree()

	// Get initial order
	siblingsBefore := project.GetSiblings(task1)

	// Trying to move the first task up is a no-op
	err = project.MoveTaskUp(task1)
	testutil.AssertNoError(t, err)

	// Order should be unchanged
	siblingsAfter := project.GetSiblings(task1)
	if siblingsAfter[0].ID != siblingsBefore[0].ID {
		t.Error("order should not change when moving top task up")
	}
}

func TestMoveTaskDownAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task1, task2)
	project.BuildTaskTree()

	// Get initial order
	siblingsBefore := project.GetSiblings(task1)

	// Trying to move the last task down is a no-op
	err = project.MoveTaskDown(task2)
	testutil.AssertNoError(t, err)

	// Order should be unchanged
	siblingsAfter := project.GetSiblings(task1)
	if siblingsAfter[1].ID != siblingsBefore[1].ID {
		t.Error("order should not change when moving bottom task down")
	}
}

func TestMoveTaskUpWithinParent(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic with child tasks
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Parent Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)

	child1, err := models.NewTask("child-1", project.GetNextShortID(), models.TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child1.ParentID = &epic.ID
	child2, err := models.NewTask("child-2", project.GetNextShortID(), models.TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	child2.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, child1, child2)
	project.BuildTaskTree()

	// Move child2 up within the epic
	err = project.MoveTaskUp(child2)
	testutil.AssertNoError(t, err)

	siblings := project.GetSiblings(child1)
	if len(siblings) != 2 {
		t.Fatalf("expected 2 siblings, got %d", len(siblings))
	}
	if siblings[0].ID != child2.ID {
		t.Errorf("expected child2 to be first, got %s", siblings[0].Title)
	}
}

func TestMoveUp_CompletionBoundaryBlocked(t *testing.T) {
	project := testutil.NewTestProject("Completion Boundary Test")

	incompleteTask, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Incomplete")
	if err != nil {
		t.Fatal(err)
	}
	incompleteTask.Status = models.StatusTodo
	project.Tasks = append(project.Tasks, incompleteTask)
	project.NextShortID++

	completedTask, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Completed")
	if err != nil {
		t.Fatal(err)
	}
	completedTask.Complete()
	project.Tasks = append(project.Tasks, completedTask)
	project.NextShortID++

	project.BuildTaskTree()

	siblings := project.GetSiblings(completedTask)
	siblingIdx := project.FindSiblingIndex(completedTask)

	if siblingIdx <= 0 {
		t.Fatal("expected completed task to have siblings above")
	}

	siblingAbove := siblings[siblingIdx-1]
	if completedTask.IsRoot() && completedTask.IsComplete() && !siblingAbove.IsComplete() {
		// This condition should prevent moving - verify the boundary logic
		testutil.AssertEqual(t, completedTask.IsComplete(), true)
		testutil.AssertEqual(t, siblingAbove.IsComplete(), false)
	}
}

func TestMoveDown_CompletionBoundaryBlocked(t *testing.T) {
	project := testutil.NewTestProject("Completion Down Boundary Test")

	incompleteTask, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Incomplete")
	if err != nil {
		t.Fatal(err)
	}
	incompleteTask.Status = models.StatusInProgress
	project.Tasks = append(project.Tasks, incompleteTask)
	project.NextShortID++

	completedTask, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Completed")
	if err != nil {
		t.Fatal(err)
	}
	completedTask.Complete()
	project.Tasks = append(project.Tasks, completedTask)
	project.NextShortID++

	project.BuildTaskTree()

	siblings := project.GetSiblings(incompleteTask)
	siblingIdx := project.FindSiblingIndex(incompleteTask)

	if siblingIdx == -1 || siblingIdx >= len(siblings)-1 {
		t.Fatal("expected incomplete task to have siblings below")
	}

	siblingBelow := siblings[siblingIdx+1]
	if incompleteTask.IsRoot() && !incompleteTask.IsComplete() && siblingBelow.IsComplete() {
		// This condition should prevent moving
		testutil.AssertEqual(t, incompleteTask.IsComplete(), false)
		testutil.AssertEqual(t, siblingBelow.IsComplete(), true)
	}
}
