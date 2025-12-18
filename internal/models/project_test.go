package models

import "testing"

func TestNewProject(t *testing.T) {
	project := NewProject("test-id", "My Project", "/tmp/my-project")

	if project.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", project.ID)
	}
	if project.Slug != "my-project" {
		t.Errorf("expected Slug 'my-project', got '%s'", project.Slug)
	}
	if project.Name != "My Project" {
		t.Errorf("expected Name 'My Project', got '%s'", project.Name)
	}
	if project.Path != "/tmp/my-project" {
		t.Errorf("expected Path '/tmp/my-project', got '%s'", project.Path)
	}
	if project.Context != "personal" {
		t.Errorf("expected Context 'personal', got '%s'", project.Context)
	}
	if project.Archived {
		t.Error("expected Archived to be false")
	}
	if project.NextShortID != 1 {
		t.Errorf("expected NextShortID 1, got %d", project.NextShortID)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Project", "my-project"},
		{"UPPERCASE", "uppercase"},
		{"with spaces", "with-spaces"},
		{"special!@#chars", "specialchars"},
		{"multiple   spaces", "multiple-spaces"},
		{"---dashes---", "dashes"},
		{"MixedCase123", "mixedcase123"},
		{"already-slugified", "already-slugified"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := Slugify(tc.input)
			if got != tc.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestProjectArchive(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	if project.Archived {
		t.Error("project should not be archived initially")
	}
	if project.ArchivedAt != nil {
		t.Error("ArchivedAt should be nil initially")
	}

	project.Archive()

	if !project.Archived {
		t.Error("project should be archived after Archive()")
	}
	if project.ArchivedAt == nil {
		t.Error("ArchivedAt should be set after Archive()")
	}

	project.Unarchive()

	if project.Archived {
		t.Error("project should not be archived after Unarchive()")
	}
	if project.ArchivedAt != nil {
		t.Error("ArchivedAt should be nil after Unarchive()")
	}
}

func TestProjectGetRootTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	task := NewTask("task", 2, TaskTypeTask, "Task")
	childTask := NewTask("child", 3, TaskTypeTask, "Child")

	project.Tasks = []*Task{epic, task, childTask}
	epic.AddChild(childTask)

	roots := project.GetRootTasks()

	if len(roots) != 2 {
		t.Errorf("expected 2 root tasks, got %d", len(roots))
	}

	// Check that childTask is not in roots
	for _, root := range roots {
		if root.ID == "child" {
			t.Error("child task should not be in root tasks")
		}
	}
}

func TestProjectGetTaskByID(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("task-2", 2, TaskTypeTask, "Task 2")

	project.Tasks = []*Task{task1, task2}

	// Find existing task
	found := project.GetTaskByID("task-1")
	if found == nil {
		t.Error("expected to find task-1")
	}
	if found.Title != "Task 1" {
		t.Errorf("expected Title 'Task 1', got '%s'", found.Title)
	}

	// Find non-existent task
	notFound := project.GetTaskByID("task-999")
	if notFound != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestProjectGetTaskByShortID(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("task-2", 42, TaskTypeTask, "Task 2")

	project.Tasks = []*Task{task1, task2}

	// Find by short ID
	found := project.GetTaskByShortID(42)
	if found == nil {
		t.Error("expected to find task with short ID 42")
	}
	if found.Title != "Task 2" {
		t.Errorf("expected Title 'Task 2', got '%s'", found.Title)
	}

	// Non-existent short ID
	notFound := project.GetTaskByShortID(999)
	if notFound != nil {
		t.Error("expected nil for non-existent short ID")
	}
}

func TestProjectBuildTaskTree(t *testing.T) {
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

	// Check parent-child relationships
	if len(epic.Children) != 1 {
		t.Errorf("expected epic to have 1 child, got %d", len(epic.Children))
	}
	if story.Parent == nil || story.Parent.ID != "epic" {
		t.Error("story's parent should be epic")
	}
	if task.Parent == nil || task.Parent.ID != "story" {
		t.Error("task's parent should be story")
	}

	// Check levels
	if epic.Level != 0 {
		t.Errorf("epic level should be 0, got %d", epic.Level)
	}
	if story.Level != 1 {
		t.Errorf("story level should be 1, got %d", story.Level)
	}
	if task.Level != 2 {
		t.Errorf("task level should be 2, got %d", task.Level)
	}
}

func TestProjectGetNextShortID(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	id1 := project.GetNextShortID()
	if id1 != 1 {
		t.Errorf("first short ID should be 1, got %d", id1)
	}

	id2 := project.GetNextShortID()
	if id2 != 2 {
		t.Errorf("second short ID should be 2, got %d", id2)
	}

	if project.NextShortID != 3 {
		t.Errorf("NextShortID should be 3, got %d", project.NextShortID)
	}
}

func TestProjectTaskCounts(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1") // todo
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task2.Start() // in_progress
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")
	task3.Complete() // done
	task4 := NewTask("4", 4, TaskTypeTask, "Task 4")
	task4.Block() // blocked
	task5 := NewTask("5", 5, TaskTypeTask, "Task 5")
	task5.Cancel() // cancelled

	project.Tasks = []*Task{task1, task2, task3, task4, task5}

	if project.CountTasks() != 5 {
		t.Errorf("expected 5 total tasks, got %d", project.CountTasks())
	}

	// Active: todo, in_progress, blocked = 3
	if project.CountActiveTasks() != 3 {
		t.Errorf("expected 3 active tasks, got %d", project.CountActiveTasks())
	}

	// Completed: done, cancelled = 2
	if project.CountCompletedTasks() != 2 {
		t.Errorf("expected 2 completed tasks, got %d", project.CountCompletedTasks())
	}
}

func TestProjectGetTasksByStatus(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task2.Start()
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	project.Tasks = []*Task{task1, task2, task3}

	todoTasks := project.GetTasksByStatus(StatusTodo)
	if len(todoTasks) != 2 {
		t.Errorf("expected 2 todo tasks, got %d", len(todoTasks))
	}

	inProgressTasks := project.GetTasksByStatus(StatusInProgress)
	if len(inProgressTasks) != 1 {
		t.Errorf("expected 1 in_progress task, got %d", len(inProgressTasks))
	}
}

func TestProjectGetTasksByType(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("1", 1, TaskTypeEpic, "Epic")
	task1 := NewTask("2", 2, TaskTypeTask, "Task 1")
	task2 := NewTask("3", 3, TaskTypeTask, "Task 2")
	bug := NewTask("4", 4, TaskTypeBug, "Bug")

	project.Tasks = []*Task{epic, task1, task2, bug}

	tasks := project.GetTasksByType(TaskTypeTask)
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks of type Task, got %d", len(tasks))
	}

	epics := project.GetTasksByType(TaskTypeEpic)
	if len(epics) != 1 {
		t.Errorf("expected 1 epic, got %d", len(epics))
	}
}

func TestProjectGetTasksByTag(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task1.AddTag("backend")
	task1.AddTag("urgent")

	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task2.AddTag("backend")

	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")
	task3.AddTag("frontend")

	project.Tasks = []*Task{task1, task2, task3}

	backend := project.GetTasksByTag("backend")
	if len(backend) != 2 {
		t.Errorf("expected 2 backend tasks, got %d", len(backend))
	}

	urgent := project.GetTasksByTag("urgent")
	if len(urgent) != 1 {
		t.Errorf("expected 1 urgent task, got %d", len(urgent))
	}

	nonexistent := project.GetTasksByTag("nonexistent")
	if len(nonexistent) != 0 {
		t.Errorf("expected 0 tasks with nonexistent tag, got %d", len(nonexistent))
	}
}

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

func TestProjectGetTaskAbove(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	epic := NewTask("epic", 1, TaskTypeEpic, "Epic")
	story := NewTask("story", 2, TaskTypeStory, "Story")
	task := NewTask("task", 3, TaskTypeTask, "Task")
	task2 := NewTask("task2", 4, TaskTypeTask, "Task 2")

	epicID := epic.ID
	storyID := story.ID
	story.ParentID = &epicID
	task.ParentID = &storyID

	project.Tasks = []*Task{epic, story, task, task2}
	project.BuildTaskTree()

	// Visual order: epic, story, task, task2
	// Task above epic should be nil
	above := project.GetTaskAbove(epic)
	if above != nil {
		t.Errorf("expected nil above epic, got %s", above.ID)
	}

	// Task above story should be epic
	above = project.GetTaskAbove(story)
	if above == nil || above.ID != "epic" {
		t.Error("expected epic above story")
	}

	// Task above task should be story
	above = project.GetTaskAbove(task)
	if above == nil || above.ID != "story" {
		t.Error("expected story above task")
	}

	// Task above task2 (root) should be task (last in epic's tree)
	above = project.GetTaskAbove(task2)
	if above == nil || above.ID != "task" {
		t.Errorf("expected task above task2, got %v", above)
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

func TestProjectGetBlockingTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	// Task 3 depends on Task 1 and Task 2
	task3.AddDependency(task1.ID)
	task3.AddDependency(task2.ID)

	project.Tasks = []*Task{task1, task2, task3}

	blockers := project.GetBlockingTasks(task3)
	if len(blockers) != 2 {
		t.Errorf("expected 2 blocking tasks, got %d", len(blockers))
	}

	// Task 1 has no dependencies
	blockers = project.GetBlockingTasks(task1)
	if len(blockers) != 0 {
		t.Errorf("expected 0 blocking tasks for task1, got %d", len(blockers))
	}
}

func TestProjectGetBlockedByTasks(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	// Task 2 and Task 3 depend on Task 1
	task2.AddDependency(task1.ID)
	task3.AddDependency(task1.ID)

	project.Tasks = []*Task{task1, task2, task3}

	blocked := project.GetBlockedByTasks(task1)
	if len(blocked) != 2 {
		t.Errorf("expected task1 to block 2 tasks, got %d", len(blocked))
	}

	// Task 3 blocks no one
	blocked = project.GetBlockedByTasks(task3)
	if len(blocked) != 0 {
		t.Errorf("expected task3 to block 0 tasks, got %d", len(blocked))
	}
}

func TestProjectHasCircularDependency(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	task1 := NewTask("1", 1, TaskTypeTask, "Task 1")
	task2 := NewTask("2", 2, TaskTypeTask, "Task 2")
	task3 := NewTask("3", 3, TaskTypeTask, "Task 3")

	project.Tasks = []*Task{task1, task2, task3}

	// No dependencies yet - no circular dependency
	if project.HasCircularDependency(task1.ID, task2.ID) {
		t.Error("should not detect circular dependency when no deps exist")
	}

	// Task 1 depends on Task 2
	task1.AddDependency(task2.ID)

	// Adding Task 2 -> Task 1 would create cycle
	if !project.HasCircularDependency(task2.ID, task1.ID) {
		t.Error("should detect circular dependency: task2 -> task1 -> task2")
	}

	// Adding Task 2 -> Task 3 would not create cycle
	if project.HasCircularDependency(task2.ID, task3.ID) {
		t.Error("should not detect circular dependency for task2 -> task3")
	}

	// Test transitive: Task 1 -> Task 2, Task 2 -> Task 3, adding Task 3 -> Task 1 creates cycle
	task2.AddDependency(task3.ID)
	if !project.HasCircularDependency(task3.ID, task1.ID) {
		t.Error("should detect transitive circular dependency: task3 -> task1 -> task2 -> task3")
	}
}

func TestProjectCheckAndAutoUnblock(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	blocker1 := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	blocker2 := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	blocked := NewTask("blocked", 3, TaskTypeTask, "Blocked Task")

	// Blocked task depends on both blockers
	blocked.AddDependency(blocker1.ID)
	blocked.AddDependency(blocker2.ID)
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocker1, blocker2, blocked}

	// Neither blocker is done yet - blocked should stay blocked
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusBlocked {
		t.Errorf("blocked task should remain blocked, got %s", blocked.Status)
	}

	// Complete blocker1 - still blocked (blocker2 not done)
	blocker1.Complete()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusBlocked {
		t.Errorf("blocked task should remain blocked (one dep incomplete), got %s", blocked.Status)
	}

	// Complete blocker2 - now blocked should auto-unblock to todo
	blocker2.Complete()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusTodo {
		t.Errorf("blocked task should be unblocked to todo, got %s", blocked.Status)
	}
}

func TestProjectCheckAndAutoUnblockWithCancelled(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	blocker := NewTask("blocker", 1, TaskTypeTask, "Blocker")
	blocked := NewTask("blocked", 2, TaskTypeTask, "Blocked Task")

	blocked.AddDependency(blocker.ID)
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocker, blocked}

	// Cancel blocker (considered complete)
	blocker.Cancel()
	project.CheckAndAutoUnblock()
	if blocked.Status != StatusTodo {
		t.Errorf("blocked task should be unblocked when blocker cancelled, got %s", blocked.Status)
	}
}

func TestProjectCheckAndAutoUnblockNoDependencies(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	// Blocked task with no dependencies should not be auto-unblocked
	blocked := NewTask("blocked", 1, TaskTypeTask, "Blocked Task")
	blocked.Status = StatusBlocked

	project.Tasks = []*Task{blocked}

	project.CheckAndAutoUnblock()
	if blocked.Status != StatusBlocked {
		t.Errorf("blocked task without deps should remain blocked, got %s", blocked.Status)
	}
}

func TestHasIncompleteDependencies(t *testing.T) {
	project := NewProject("test-project", "Test Project", "/tmp/test")

	blocker1 := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	blocker2 := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	dependent := NewTask("dependent", 3, TaskTypeTask, "Dependent Task")

	project.Tasks = []*Task{blocker1, blocker2, dependent}

	// No dependencies - should return false
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with no dependencies should return false")
	}

	// Add dependencies - both incomplete
	dependent.AddDependency(blocker1.ID)
	dependent.AddDependency(blocker2.ID)

	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with incomplete dependencies should return true")
	}

	// Complete one blocker - still has incomplete deps
	blocker1.Complete()
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with one incomplete dependency should return true")
	}

	// Complete second blocker - no incomplete deps
	blocker2.Complete()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with all dependencies complete should return false")
	}

	// Test with cancelled dependency
	blocker2.Reopen()
	if !project.HasIncompleteDependencies(dependent) {
		t.Error("task with reopened dependency should return true")
	}
	blocker2.Cancel()
	if project.HasIncompleteDependencies(dependent) {
		t.Error("task with cancelled dependency should return false (cancelled is complete)")
	}
}

func TestGetIncompleteDependencies(t *testing.T) {
	project := NewProject("test-project", "Test Project", "/tmp/test")

	blocker1 := NewTask("blocker1", 1, TaskTypeTask, "Blocker 1")
	blocker2 := NewTask("blocker2", 2, TaskTypeTask, "Blocker 2")
	blocker3 := NewTask("blocker3", 3, TaskTypeTask, "Blocker 3")
	dependent := NewTask("dependent", 4, TaskTypeTask, "Dependent Task")

	project.Tasks = []*Task{blocker1, blocker2, blocker3, dependent}

	// No dependencies
	incomplete := project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps, got %d", len(incomplete))
	}

	// Add three dependencies
	dependent.AddDependency(blocker1.ID)
	dependent.AddDependency(blocker2.ID)
	dependent.AddDependency(blocker3.ID)

	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 3 {
		t.Errorf("expected 3 incomplete deps, got %d", len(incomplete))
	}

	// Complete one
	blocker1.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 2 {
		t.Errorf("expected 2 incomplete deps, got %d", len(incomplete))
	}

	// Cancel another
	blocker2.Cancel()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 1 {
		t.Errorf("expected 1 incomplete dep, got %d", len(incomplete))
	}

	// Verify the remaining incomplete is blocker3
	if incomplete[0].ID != blocker3.ID {
		t.Errorf("expected remaining incomplete to be blocker3, got %s", incomplete[0].ID)
	}

	// Complete the last one
	blocker3.Complete()
	incomplete = project.GetIncompleteDependencies(dependent)
	if len(incomplete) != 0 {
		t.Errorf("expected 0 incomplete deps, got %d", len(incomplete))
	}
}
