package storage

import (
	"strings"
	"testing"
)

func TestParseTasksMarkdown_DuplicateID_UpdatesDependsOn(t *testing.T) {
	// Task A and Task A2 share the same ID "dup-id". A2 will get a new UUID.
	// Task B depends on "dup-id". After dedup, B's DependsOn should point to A2's new ID.
	content := `# Project: Test

## Task: Task A
<!-- id: dup-id, status: todo, priority: 1 -->

## Task: Task A2
<!-- id: dup-id, status: todo, priority: 2 -->

## Task: Task B
<!-- id: unique-b, status: todo, priority: 3, depends: dup-id -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.Tasks))
	}

	taskA := result.Tasks[0]
	taskA2 := result.Tasks[1]
	taskB := result.Tasks[2]

	// Task A keeps original ID
	if taskA.ID != "dup-id" {
		t.Errorf("Task A should keep original ID 'dup-id', got %q", taskA.ID)
	}

	// Task A2 should have a new ID (not "dup-id")
	if taskA2.ID == "dup-id" {
		t.Fatal("Task A2 should have been reassigned a new ID")
	}

	// Task B's DependsOn should reference A2's new ID
	if len(taskB.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(taskB.DependsOn))
	}
	if taskB.DependsOn[0] != taskA2.ID {
		t.Errorf("Task B DependsOn should be updated to %q, got %q", taskA2.ID, taskB.DependsOn[0])
	}

	// Should have a warning about the duplicate
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "duplicate ID") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a warning about duplicate ID")
	}
}

func TestParseTasksMarkdown_DuplicateID_UpdatesParentID(t *testing.T) {
	// Parent and Parent2 share "dup-id". Parent2 gets reassigned.
	// Child has parent: dup-id, which should be updated to Parent2's new ID.
	content := `# Project: Test

## Epic: Parent
<!-- id: dup-id, status: todo, priority: 1 -->

## Epic: Parent2
<!-- id: dup-id, status: todo, priority: 2 -->

### Task: Child
<!-- id: child-1, status: todo, priority: 3, parent: dup-id -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.Tasks))
	}

	parent2 := result.Tasks[1]
	child := result.Tasks[2]

	if parent2.ID == "dup-id" {
		t.Fatal("Parent2 should have been reassigned a new ID")
	}

	if child.ParentID == nil {
		t.Fatal("Child should have a ParentID")
	}
	if *child.ParentID != parent2.ID {
		t.Errorf("Child ParentID should be updated to %q, got %q", parent2.ID, *child.ParentID)
	}
}

func TestParseTasksMarkdown_OrphanedParentRef(t *testing.T) {
	// A task that references a parent ID that doesn't exist in the file should
	// have its ParentID cleared and a DataCorrection warning emitted.
	content := `# Project: Test

## Task: Orphaned Child
<!-- id: child-id, status: todo, priority: 1, parent: nonexistent-parent-id -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(result.Tasks))
	}

	task := result.Tasks[0]

	// ParentID should have been cleared since the referenced parent doesn't exist
	if task.ParentID != nil {
		t.Errorf("expected ParentID nil (orphaned ref cleared), got %q", *task.ParentID)
	}

	// Should have a DataCorrection warning about the orphaned parent reference
	found := false
	for _, w := range result.Warnings {
		if w.Field == "parent" && w.TaskID == "child-id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a DataCorrection warning for orphaned parent reference, got none")
	}
}

func TestParseTasksMarkdown_DuplicateID_UpdatesRelatedTo(t *testing.T) {
	// Task A and Task A2 share "dup-id". A2 gets reassigned.
	// Task C has related: dup-id, which should be updated to A2's new ID.
	content := `# Project: Test

## Task: Task A
<!-- id: dup-id, status: todo, priority: 1 -->

## Task: Task A2
<!-- id: dup-id, status: todo, priority: 2 -->

## Task: Task C
<!-- id: unique-c, status: todo, priority: 3, related: dup-id -->
`
	result, err := ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.Tasks))
	}

	taskA2 := result.Tasks[1]
	taskC := result.Tasks[2]

	if taskA2.ID == "dup-id" {
		t.Fatal("Task A2 should have been reassigned a new ID")
	}

	if len(taskC.RelatedTo) != 1 {
		t.Fatalf("expected 1 related entry, got %d", len(taskC.RelatedTo))
	}
	if taskC.RelatedTo[0] != taskA2.ID {
		t.Errorf("Task C RelatedTo should be updated to %q, got %q", taskA2.ID, taskC.RelatedTo[0])
	}
}
