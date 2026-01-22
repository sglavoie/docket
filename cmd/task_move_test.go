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
