package models

import "testing"

func TestMoveTaskToProjectBasic(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	sourceProj.Tasks = []*Task{task1, task2}
	sourceProj.BuildTaskTree()

	// Move task1 to destination
	result, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	// Verify result
	if len(result.MovedTasks) != 1 {
		t.Errorf("expected 1 moved task, got %d", len(result.MovedTasks))
	}
	if result.MovedTasks[0].ID != task1.ID {
		t.Errorf("expected task1 to be moved, got %s", result.MovedTasks[0].ID)
	}

	// Verify source project has 1 task remaining
	if len(sourceProj.Tasks) != 1 {
		t.Errorf("expected 1 task in source, got %d", len(sourceProj.Tasks))
	}
	if sourceProj.Tasks[0].ID != task2.ID {
		t.Errorf("expected task2 to remain in source, got %s", sourceProj.Tasks[0].ID)
	}

	// Verify destination project has 1 task
	if len(destProj.Tasks) != 1 {
		t.Errorf("expected 1 task in dest, got %d", len(destProj.Tasks))
	}
	if destProj.Tasks[0].ID != task1.ID {
		t.Errorf("expected task1 in dest, got %s", destProj.Tasks[0].ID)
	}

	// Verify short ID was regenerated
	if result.OldShortIDs[0] != 1 {
		t.Errorf("expected old short ID 1, got %d", result.OldShortIDs[0])
	}
	if task1.ShortID != 1 {
		t.Errorf("expected new short ID 1 (first in dest), got %d", task1.ShortID)
	}
}

func TestMoveTaskToProjectWithChildren(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	parent, err := NewTask("parent", 1, TaskTypeEpic, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := NewTask("child1", 2, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewTask("child2", 3, TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewTask("other", 4, TaskTypeTask, "Other Task")
	if err != nil {
		t.Fatal(err)
	}

	parentID := parent.ID
	child1.ParentID = &parentID
	child2.ParentID = &parentID

	sourceProj.Tasks = []*Task{parent, child1, child2, other}
	sourceProj.BuildTaskTree()

	// Move parent (should move children too)
	result, err := sourceProj.MoveTaskToProject(parent, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	// Verify all descendants moved
	if len(result.MovedTasks) != 3 {
		t.Errorf("expected 3 moved tasks (parent + 2 children), got %d", len(result.MovedTasks))
	}

	// Verify source has only 1 task left
	if len(sourceProj.Tasks) != 1 {
		t.Errorf("expected 1 task in source, got %d", len(sourceProj.Tasks))
	}
	if sourceProj.Tasks[0].ID != other.ID {
		t.Errorf("expected 'other' task to remain in source")
	}

	// Verify destination has 3 tasks
	if len(destProj.Tasks) != 3 {
		t.Errorf("expected 3 tasks in dest, got %d", len(destProj.Tasks))
	}

	// Verify parent's ParentID was cleared (becomes root in dest)
	if parent.ParentID != nil {
		t.Error("expected parent's ParentID to be cleared")
	}

	// Verify children still reference parent
	if child1.ParentID == nil || *child1.ParentID != parent.ID {
		t.Error("expected child1 to still reference parent")
	}
	if child2.ParentID == nil || *child2.ParentID != parent.ID {
		t.Error("expected child2 to still reference parent")
	}
}

func TestMoveTaskToProjectClearsCrossProjectDependencies(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	parent, err := NewTask("parent", 1, TaskTypeEpic, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	child1, err := NewTask("child1", 2, TaskTypeTask, "Child 1")
	if err != nil {
		t.Fatal(err)
	}
	child2, err := NewTask("child2", 3, TaskTypeTask, "Child 2")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewTask("other", 4, TaskTypeTask, "Other Task")
	if err != nil {
		t.Fatal(err)
	}

	parentID := parent.ID
	child1.ParentID = &parentID
	child2.ParentID = &parentID

	// child1 depends on child2 (both will be moved together with parent)
	_ = child1.AddDependency(child2.ID)
	// child1 also depends on "other" task (which stays in source -> should be cleared)
	_ = child1.AddDependency(other.ID)

	sourceProj.Tasks = []*Task{parent, child1, child2, other}
	sourceProj.BuildTaskTree()

	// Move parent (and children) to destination
	result, err := sourceProj.MoveTaskToProject(parent, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	// Verify child1's dependency on child2 is preserved (both moved)
	if len(child1.DependsOn) != 1 || child1.DependsOn[0] != child2.ID {
		t.Errorf("expected child1 to only depend on child2 (both moved), got %v", child1.DependsOn)
	}

	// Verify cleared dependencies in result (child1's dependency on other)
	if len(result.ClearedDeps) != 1 || result.ClearedDeps[0] != other.ID {
		t.Errorf("expected 1 cleared dependency (other), got %v", result.ClearedDeps)
	}
}

func TestMoveTaskToProjectPreservesSectionsIfExist(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	// Add custom section to both projects
	sourceProj.Sections = []string{SectionNext, "InProgress", "CustomSection"}
	destProj.Sections = []string{SectionNext, "InProgress"}

	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Section = "InProgress"

	task2, err := NewTask("task2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Section = "CustomSection" // Only in source

	sourceProj.Tasks = []*Task{task1, task2}
	sourceProj.BuildTaskTree()

	// Move task1 (section exists in dest)
	result1, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}
	if task1.Section != "InProgress" {
		t.Errorf("expected task1 to keep 'InProgress' section, got %s", task1.Section)
	}

	// Move task2 (section doesn't exist in dest, should reset to Next)
	result2, err := sourceProj.MoveTaskToProject(task2, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}
	if task2.Section != SectionNext {
		t.Errorf("expected task2 section to reset to Next, got %s", task2.Section)
	}

	// Verify both tasks moved
	if len(result1.MovedTasks) != 1 || len(result2.MovedTasks) != 1 {
		t.Error("expected each move to report 1 moved task")
	}
}

func TestMoveTaskToProjectNilInputs(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}

	// Nil task
	result, err := sourceProj.MoveTaskToProject(nil, destProj)
	if err == nil {
		t.Error("expected error for nil task")
	}
	if result != nil {
		t.Error("expected nil result for nil task")
	}

	// Nil destination
	result, err = sourceProj.MoveTaskToProject(task, nil)
	if err == nil {
		t.Error("expected error for nil destination")
	}
	if result != nil {
		t.Error("expected nil result for nil destination")
	}
}

func TestMoveTaskToProjectSameProject(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	sourceProj.Tasks = []*Task{task}

	// Move to same project (should return error)
	result, err := sourceProj.MoveTaskToProject(task, sourceProj)
	if err == nil {
		t.Error("expected error when moving to same project")
	}
	if result != nil {
		t.Error("expected nil result when moving to same project")
	}

	// Verify task still in project
	if len(sourceProj.Tasks) != 1 {
		t.Error("expected task to remain in project")
	}
}

func TestMoveTaskToProjectRegeneratesShortIDs(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	task1, err := NewTask("task1", 5, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 10, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}

	sourceProj.Tasks = []*Task{task1, task2}
	sourceProj.BuildTaskTree()

	// Move task1 (should get short ID 1 in empty dest)
	result, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	// Verify old short ID captured
	if result.OldShortIDs[0] != 5 {
		t.Errorf("expected old short ID 5, got %d", result.OldShortIDs[0])
	}

	// Verify new short ID assigned (GetNextShortID on empty project returns 1)
	if task1.ShortID != 1 {
		t.Errorf("expected new short ID 1, got %d", task1.ShortID)
	}
}

func TestMoveTaskToProjectClearsDependenciesInSource(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewTask("task3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}

	// Task 3 depends on task1 and task2
	_ = task3.AddDependency(task1.ID)
	_ = task3.AddDependency(task2.ID)

	sourceProj.Tasks = []*Task{task1, task2, task3}
	sourceProj.BuildTaskTree()

	// Move task1 to destination
	_, err = sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	// Verify task3's dependency on task1 was cleared in source
	if len(task3.DependsOn) != 1 {
		t.Errorf("expected 1 dependency remaining in task3, got %d", len(task3.DependsOn))
	}
	if task3.DependsOn[0] != task2.ID {
		t.Errorf("expected task3 to still depend on task2, got %s", task3.DependsOn[0])
	}
}

func TestMoveTaskToProjectPreservesCrossProjectRefs(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	task1, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewTask("other", 3, TaskTypeTask, "Other Task")
	if err != nil {
		t.Fatal(err)
	}

	// task1 has a cross-project dependency (qualified ref) and a same-project dep on "other"
	crossRef := "external-proj-id:remote-task-id"
	_ = task1.AddDependency(crossRef)
	_ = task1.AddDependency(other.ID)

	// task1 also has a cross-project RelatedTo
	crossRelRef := "another-proj-id:another-task-id"
	task1.RelatedTo = []string{crossRelRef, task2.ID}

	sourceProj.Tasks = []*Task{task1, task2, other}
	sourceProj.BuildTaskTree()

	// Move task1 only (task2 and other stay in source)
	result, err := sourceProj.MoveTaskToProject(task1, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	// Cross-project dependency should be preserved, same-project dep on "other" should be cleared
	if len(task1.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency preserved (cross-project ref), got %d: %v", len(task1.DependsOn), task1.DependsOn)
	}
	if task1.DependsOn[0] != crossRef {
		t.Errorf("expected cross-project ref %q to be preserved, got %q", crossRef, task1.DependsOn[0])
	}

	// Cross-project RelatedTo should be preserved, same-project ref to task2 should be cleared
	if len(task1.RelatedTo) != 1 {
		t.Fatalf("expected 1 RelatedTo preserved (cross-project ref), got %d: %v", len(task1.RelatedTo), task1.RelatedTo)
	}
	if task1.RelatedTo[0] != crossRelRef {
		t.Errorf("expected cross-project RelatedTo %q to be preserved, got %q", crossRelRef, task1.RelatedTo[0])
	}

	// Cleared deps should only contain the same-project dep
	if len(result.ClearedDeps) != 1 || result.ClearedDeps[0] != other.ID {
		t.Errorf("expected 1 cleared dep (other), got %v", result.ClearedDeps)
	}
}

func TestMoveTaskToProjectUpdatesTimestamps(t *testing.T) {
	sourceProj := NewProject("source", "Source Project", "/tmp/source")
	destProj := NewProject("dest", "Dest Project", "/tmp/dest")

	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	sourceProj.Tasks = []*Task{task}
	sourceProj.BuildTaskTree()

	oldTaskTime := task.UpdatedAt
	oldSourceTime := sourceProj.UpdatedAt
	oldDestTime := destProj.UpdatedAt

	// Move task
	_, err = sourceProj.MoveTaskToProject(task, destProj)
	if err != nil {
		t.Fatalf("MoveTaskToProject failed: %v", err)
	}

	// Verify timestamps updated
	if !task.UpdatedAt.After(oldTaskTime) {
		t.Error("expected task timestamp to be updated")
	}
	if !sourceProj.UpdatedAt.After(oldSourceTime) {
		t.Error("expected source project timestamp to be updated")
	}
	if !destProj.UpdatedAt.After(oldDestTime) {
		t.Error("expected dest project timestamp to be updated")
	}
}

func TestMoveTaskToProjectSelfMoveError(t *testing.T) {
	proj := NewProject("proj1", "Project 1", "/tmp/proj1")
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	proj.Tasks = []*Task{task}

	_, err = proj.MoveTaskToProject(task, proj)
	if err == nil {
		t.Fatal("expected error when moving task to same project")
	}
	if err.Error() != "cannot move task to the same project" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMoveTaskToProjectNilTask(t *testing.T) {
	proj := NewProject("proj1", "Project 1", "/tmp/proj1")
	dest := NewProject("proj2", "Project 2", "/tmp/proj2")

	_, err := proj.MoveTaskToProject(nil, dest)
	if err == nil {
		t.Fatal("expected error when moving nil task")
	}
}

func TestMoveTaskToProjectNilDest(t *testing.T) {
	proj := NewProject("proj1", "Project 1", "/tmp/proj1")
	task, err := NewTask("task1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	proj.Tasks = []*Task{task}

	_, err = proj.MoveTaskToProject(task, nil)
	if err == nil {
		t.Fatal("expected error when destination is nil")
	}
}
