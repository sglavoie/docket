package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestIndentTask(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic and a task that can be indented under it
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Parent Epic")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task to Indent")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic, task)
	project.BuildTaskTree()

	// Verify task starts at root
	if task.ParentID != nil {
		t.Error("task should start at root level")
	}

	// Indent task under epic
	err = project.IndentTask(task)
	testutil.AssertNoError(t, err)

	// Verify task is now under epic
	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Error("task should be indented under epic")
	}
}

func TestIndentTaskCannotIndentUnderInvalidParent(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create two tasks - tasks cannot be parents of other tasks
	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
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
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)

	story, err := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	story.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, story)

	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Verify initial structure
	if task.ParentID == nil || *task.ParentID != story.ID {
		t.Error("task should start under story")
	}

	// Outdent task - should move to be sibling of story (under epic)
	err = project.OutdentTask(task)
	testutil.AssertNoError(t, err)

	// Verify task is now under epic
	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Errorf("task should be under epic after outdent, got parent: %v", task.ParentID)
	}
}

func TestOutdentTaskToRoot(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic with a task
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)

	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Outdent task - should become root
	err = project.OutdentTask(task)
	testutil.AssertNoError(t, err)

	// Verify task is now at root
	if task.ParentID != nil {
		t.Error("task should be at root after outdent")
	}
}

func TestOutdentRootTaskFails(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create a task at root level
	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Root Task")
	if err != nil {
		t.Fatal(err)
	}
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
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	story, err := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic, story)
	project.BuildTaskTree()

	// Story can be indented under epic
	canIndent, _ := story.CanIndent(epic)
	if !canIndent {
		t.Error("story should be able to indent under epic")
	}

	err = project.IndentTask(story)
	testutil.AssertNoError(t, err)

	if story.ParentID == nil || *story.ParentID != epic.ID {
		t.Error("story should be under epic after indent")
	}
}

func TestCannotIndentEpicUnderAnything(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create two epics
	epic1, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic 1")
	if err != nil {
		t.Fatal(err)
	}
	epic2, err := models.NewTask("epic-2", project.GetNextShortID(), models.TaskTypeEpic, "Epic 2")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic1, epic2)
	project.BuildTaskTree()

	// Epic cannot be indented under another epic
	canIndent, _ := epic2.CanIndent(epic1)
	if canIndent {
		t.Error("epic should not be able to indent under another epic")
	}
}

func TestIndent_NoTaskAbove(t *testing.T) {
	project := testutil.NewTestProject("Indent No Task Above Test")

	task, err := models.NewTask("lonely-task", project.GetNextShortID(), models.TaskTypeTask, "Lonely Task")
	if err != nil {
		t.Fatal(err)
	}
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

	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Parent Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)
	project.NextShortID++

	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	if err != nil {
		t.Fatal(err)
	}
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

	grandparent, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Grandparent")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, grandparent)
	project.NextShortID++

	parent, err := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Parent Story")
	if err != nil {
		t.Fatal(err)
	}
	parent.ParentID = &grandparent.ID
	project.Tasks = append(project.Tasks, parent)
	project.NextShortID++

	child, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	if err != nil {
		t.Fatal(err)
	}
	child.ParentID = &parent.ID
	project.Tasks = append(project.Tasks, child)
	project.NextShortID++

	project.BuildTaskTree()

	testutil.AssertEqual(t, *child.ParentID, parent.ID)

	err = project.OutdentTask(child)
	testutil.AssertNoError(t, err)

	if child.ParentID == nil {
		t.Error("expected child to move to grandparent, not root")
	} else {
		testutil.AssertEqual(t, *child.ParentID, grandparent.ID)
	}
}
