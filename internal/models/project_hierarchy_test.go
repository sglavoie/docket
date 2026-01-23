package models

import "testing"

func TestProjectGetSiblings(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	// Create tasks: two root tasks, one with children
	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	epic := NewTask("epic", 10, TaskTypeEpic, "Epic")
	child1 := NewTask("child1", 11, TaskTypeTask, "Child 1")
	child2 := NewTask("child2", 12, TaskTypeTask, "Child 2")

	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	project.Tasks = []*Task{task1, task2, epic, child1, task3, child2}

	// Root tasks have 4 siblings (task1, task2, task3, epic)
	rootSiblings := project.GetSiblings(task1)
	if len(rootSiblings) != 4 {
		t.Errorf("expected 4 root siblings, got %d", len(rootSiblings))
	}

	// Children have 2 siblings
	childSiblings := project.GetSiblings(child1)
	if len(childSiblings) != 2 {
		t.Errorf("expected 2 child siblings, got %d", len(childSiblings))
	}
}

func TestProjectMoveTaskUp(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	project.Tasks = []*Task{task1, task2, task3}

	// Move task2 up (should swap with task1)
	err := project.MoveTaskUp(task2)
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	// Check order: should be task2, task1, task3
	if project.Tasks[0].ID != "2" {
		t.Errorf("expected task2 at position 0, got %s", project.Tasks[0].ID)
	}
	if project.Tasks[1].ID != "1" {
		t.Errorf("expected task1 at position 1, got %s", project.Tasks[1].ID)
	}

	// Move task2 up again (already at top, should be no-op)
	err = project.MoveTaskUp(task2)
	if err != nil {
		t.Errorf("MoveTaskUp at top returned error: %v", err)
	}
	if project.Tasks[0].ID != "2" {
		t.Error("task2 should still be at top")
	}
}

func TestProjectMoveTaskDown(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	project.Tasks = []*Task{task1, task2, task3}

	// Move task2 down (should swap with task3)
	err := project.MoveTaskDown(task2)
	if err != nil {
		t.Errorf("MoveTaskDown returned error: %v", err)
	}

	// Check order: should be task1, task3, task2
	if project.Tasks[1].ID != "3" {
		t.Errorf("expected task3 at position 1, got %s", project.Tasks[1].ID)
	}
	if project.Tasks[2].ID != "2" {
		t.Errorf("expected task2 at position 2, got %s", project.Tasks[2].ID)
	}

	// Move task2 down again (already at bottom, should be no-op)
	err = project.MoveTaskDown(task2)
	if err != nil {
		t.Errorf("MoveTaskDown at bottom returned error: %v", err)
	}
	if project.Tasks[2].ID != "2" {
		t.Error("task2 should still be at bottom")
	}
}

func TestProjectMoveTaskUpWithChildren(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	child1 := NewTask("child1", 2, TaskTypeTask, "Child 1")
	child2 := NewTask("child2", 3, TaskTypeTask, "Child 2")

	epicID := epic.ID
	child1.ParentID = &epicID
	child2.ParentID = &epicID

	project.Tasks = []*Task{epic, child1, child2}
	project.BuildTaskTree()

	// Move child2 up (swap with child1)
	err := project.MoveTaskUp(child2)
	if err != nil {
		t.Errorf("MoveTaskUp returned error: %v", err)
	}

	// Get children order
	siblings := project.GetSiblings(child2)
	if siblings[0].ID != "child2" {
		t.Errorf("expected child2 first, got %s", siblings[0].ID)
	}
}

func TestProjectIndentTask(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	task := NewTask("task", 2, TaskTypeTask, "Task")

	project.Tasks = []*Task{epic, task}
	project.BuildTaskTree()

	// Indent task under epic
	err := project.IndentTask(task)
	if err != nil {
		t.Errorf("IndentTask returned error: %v", err)
	}

	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Error("task should now be child of epic")
	}
}

func TestProjectOutdentTask(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")

	epicID := epic.ID
	storyID := story.ID
	story.ParentID = &epicID
	task.ParentID = &storyID

	project.Tasks = []*Task{epic, story, task}
	project.BuildTaskTree()

	// Outdent task (should become child of epic)
	err := project.OutdentTask(task)
	if err != nil {
		t.Errorf("OutdentTask returned error: %v", err)
	}

	if task.ParentID == nil || *task.ParentID != epic.ID {
		t.Error("task should now be child of epic")
	}

	// Outdent story (should become root)
	err = project.OutdentTask(story)
	if err != nil {
		t.Errorf("OutdentTask returned error: %v", err)
	}

	if story.ParentID != nil {
		t.Error("story should now be root")
	}
}
