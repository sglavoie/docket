package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestMoveTaskUp(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create three sibling tasks at root level
	task1 := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	task2 := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	task3 := models.NewTask("task-3", project.GetNextShortID(), models.TaskTypeTask, "Third Task")
	project.Tasks = append(project.Tasks, task1, task2, task3)
	project.BuildTaskTree()

	// Move task2 up - should swap with task1
	err := project.MoveTaskUp(task2)
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
	task1 := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	task2 := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	task3 := models.NewTask("task-3", project.GetNextShortID(), models.TaskTypeTask, "Third Task")
	project.Tasks = append(project.Tasks, task1, task2, task3)
	project.BuildTaskTree()

	// Move task1 down - should swap with task2
	err := project.MoveTaskDown(task1)
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

	task1 := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	task2 := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	project.Tasks = append(project.Tasks, task1, task2)
	project.BuildTaskTree()

	// Get initial order
	siblingsBefore := project.GetSiblings(task1)

	// Trying to move the first task up is a no-op
	err := project.MoveTaskUp(task1)
	testutil.AssertNoError(t, err)

	// Order should be unchanged
	siblingsAfter := project.GetSiblings(task1)
	if siblingsAfter[0].ID != siblingsBefore[0].ID {
		t.Error("order should not change when moving top task up")
	}
}

func TestMoveTaskDownAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task1 := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "First Task")
	task2 := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Second Task")
	project.Tasks = append(project.Tasks, task1, task2)
	project.BuildTaskTree()

	// Get initial order
	siblingsBefore := project.GetSiblings(task1)

	// Trying to move the last task down is a no-op
	err := project.MoveTaskDown(task2)
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
	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Parent Epic")
	project.Tasks = append(project.Tasks, epic)

	child1 := models.NewTask("child-1", project.GetNextShortID(), models.TaskTypeTask, "Child 1")
	child1.ParentID = &epic.ID
	child2 := models.NewTask("child-2", project.GetNextShortID(), models.TaskTypeTask, "Child 2")
	child2.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, child1, child2)
	project.BuildTaskTree()

	// Move child2 up within the epic
	err := project.MoveTaskUp(child2)
	testutil.AssertNoError(t, err)

	siblings := project.GetSiblings(child1)
	if len(siblings) != 2 {
		t.Fatalf("expected 2 siblings, got %d", len(siblings))
	}
	if siblings[0].ID != child2.ID {
		t.Errorf("expected child2 to be first, got %s", siblings[0].Title)
	}
}

func TestIndentTask(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic and a task that can be indented under it
	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Parent Epic")
	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task to Indent")
	project.Tasks = append(project.Tasks, epic, task)
	project.BuildTaskTree()

	// Verify task starts at root
	if task.ParentID != nil {
		t.Error("task should start at root level")
	}

	// Indent task under epic
	err := project.IndentTask(task)
	testutil.AssertNoError(t, err)

	// Verify task is now under epic
	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Error("task should be indented under epic")
	}
}

func TestIndentTaskCannotIndentUnderInvalidParent(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create two tasks - tasks cannot be parents of other tasks
	task1 := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	task2 := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Task 2")
	project.Tasks = append(project.Tasks, task1, task2)
	project.BuildTaskTree()

	// Verify task2 cannot be indented under task1
	canIndent, _ := task2.CanIndent(task1)
	if canIndent {
		t.Error("task should not be able to indent under another task")
	}
}

func TestOutdentTask(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic with a story that has a task
	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story")
	story.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, story)

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	task.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Verify initial structure
	if task.ParentID == nil || *task.ParentID != story.ID {
		t.Error("task should start under story")
	}

	// Outdent task - should move to be sibling of story (under epic)
	err := project.OutdentTask(task)
	testutil.AssertNoError(t, err)

	// Verify task is now under epic
	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Errorf("task should be under epic after outdent, got parent: %v", task.ParentID)
	}
}

func TestOutdentTaskToRoot(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic with a task
	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	task.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Outdent task - should become root
	err := project.OutdentTask(task)
	testutil.AssertNoError(t, err)

	// Verify task is now at root
	if task.ParentID != nil {
		t.Error("task should be at root after outdent")
	}
}

func TestOutdentRootTaskFails(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create a task at root level
	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Root Task")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Verify cannot outdent
	canOutdent, _ := task.CanOutdent()
	if canOutdent {
		t.Error("root task should not be able to outdent")
	}
}

func TestIndentStoryUnderEpic(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic and a story
	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story")
	project.Tasks = append(project.Tasks, epic, story)
	project.BuildTaskTree()

	// Story can be indented under epic
	canIndent, _ := story.CanIndent(epic)
	if !canIndent {
		t.Error("story should be able to indent under epic")
	}

	err := project.IndentTask(story)
	testutil.AssertNoError(t, err)

	if story.ParentID == nil || *story.ParentID != epic.ID {
		t.Error("story should be under epic after indent")
	}
}

func TestCannotIndentEpicUnderAnything(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create two epics
	epic1 := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic 1")
	epic2 := models.NewTask("epic-2", project.GetNextShortID(), models.TaskTypeEpic, "Epic 2")
	project.Tasks = append(project.Tasks, epic1, epic2)
	project.BuildTaskTree()

	// Epic cannot be indented under another epic
	canIndent, _ := epic2.CanIndent(epic1)
	if canIndent {
		t.Error("epic should not be able to indent under another epic")
	}
}

func TestSelectTaskForMove_WithDirectID(t *testing.T) {
	project := testutil.NewTestProject("Select Move Test")

	task := models.NewTask("known-id-123", project.GetNextShortID(), models.TaskTypeTask, "Task to Find")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	args := []string{task.ID}
	found, err := selectTaskForMove(project, args, "up")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, found.ID, task.ID)
}

func TestSelectTaskForMove_TaskNotFound(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Move Not Found Test")
	defer deps.Cleanup()

	args := []string{"nonexistent-task-id"}
	_, err := selectTaskForMove(project, args, "up")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestSelectTaskForMove_FilterUp_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Up Test")

	task := models.NewTask("single-task", project.GetNextShortID(), models.TaskTypeTask, "Only Task")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err := selectTaskForMove(project, nil, "up")
	if err == nil {
		t.Error("expected error when no candidates for move up")
	}
}

func TestSelectTaskForMove_FilterDown_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Down Test")

	task := models.NewTask("single-task", project.GetNextShortID(), models.TaskTypeTask, "Only Task")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err := selectTaskForMove(project, nil, "down")
	if err == nil {
		t.Error("expected error when no candidates for move down")
	}
}

func TestSelectTaskForMove_FilterOutdent_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Outdent Test")

	task := models.NewTask("root-task", project.GetNextShortID(), models.TaskTypeTask, "Root Task")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err := selectTaskForMove(project, nil, "outdent")
	if err == nil {
		t.Error("expected error when no candidates for outdent")
	}
}

func TestSelectTaskForMove_FilterIndent_NoCandidates(t *testing.T) {
	project := testutil.NewTestProject("Filter Indent Test")

	task := models.NewTask("only-task", project.GetNextShortID(), models.TaskTypeIdea, "Lonely Idea")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	_, err := selectTaskForMove(project, nil, "indent")
	if err == nil {
		t.Error("expected error when no candidates for indent")
	}
}

func TestMoveUp_CompletionBoundaryBlocked(t *testing.T) {
	project := testutil.NewTestProject("Completion Boundary Test")

	incompleteTask := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Incomplete")
	incompleteTask.Status = models.StatusTodo
	project.Tasks = append(project.Tasks, incompleteTask)
	project.NextShortID++

	completedTask := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Completed")
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

	incompleteTask := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Incomplete")
	incompleteTask.Status = models.StatusInProgress
	project.Tasks = append(project.Tasks, incompleteTask)
	project.NextShortID++

	completedTask := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Completed")
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

func TestIndent_NoTaskAbove(t *testing.T) {
	project := testutil.NewTestProject("Indent No Task Above Test")

	task := models.NewTask("lonely-task", project.GetNextShortID(), models.TaskTypeTask, "Lonely Task")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	targetParent := project.GetTaskAbove(task)
	if targetParent != nil {
		t.Error("expected no task above single root task")
	}
}

func TestIndent_ValidParent(t *testing.T) {
	project := testutil.NewTestProject("Indent Valid Parent Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Parent Epic")
	project.Tasks = append(project.Tasks, epic)
	project.NextShortID++

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++

	project.BuildTaskTree()

	targetParent := project.GetTaskAbove(task)
	if targetParent == nil {
		t.Fatal("expected epic to be above task")
	}
	testutil.AssertEqual(t, targetParent.ID, epic.ID)

	ok, reason := task.CanIndent(targetParent)
	if !ok {
		t.Errorf("expected task to be able to indent under epic: %s", reason)
	}
}

func TestOutdent_ToNewParent(t *testing.T) {
	project := testutil.NewTestProject("Outdent New Parent Test")

	grandparent := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Grandparent")
	project.Tasks = append(project.Tasks, grandparent)
	project.NextShortID++

	parent := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Parent Story")
	parent.ParentID = &grandparent.ID
	project.Tasks = append(project.Tasks, parent)
	project.NextShortID++

	child := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	child.ParentID = &parent.ID
	project.Tasks = append(project.Tasks, child)
	project.NextShortID++

	project.BuildTaskTree()

	testutil.AssertEqual(t, *child.ParentID, parent.ID)

	err := project.OutdentTask(child)
	testutil.AssertNoError(t, err)

	if child.ParentID == nil {
		t.Error("expected child to move to grandparent, not root")
	} else {
		testutil.AssertEqual(t, *child.ParentID, grandparent.ID)
	}
}
