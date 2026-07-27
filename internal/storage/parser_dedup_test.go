package storage

import (
	"strings"
	"testing"
)

func TestParseTasksMarkdown_DuplicateID_PreservesDependsOn(t *testing.T) {
	// Task A and Task A2 share the same ID "dup-id". A2 will get a new UUID.
	// Task B depends on "dup-id". After dedup, B's DependsOn should still point
	// to "dup-id" because Task A (the first) retains the original ID.
	content := `# Project: Test

## Task: Task A
<!-- id: dup-id, status: todo, priority: 1 -->

## Task: Task A2
<!-- id: dup-id, status: todo, priority: 2 -->

## Task: Task B
<!-- id: unique-b, status: todo, priority: 3, depends: dup-id -->
`
	result, err := ParseTasksMarkdownDocument(content)
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

	// Task B's DependsOn should still reference "dup-id" (Task A, the first
	// occurrence that kept the original ID)
	if len(taskB.DependsOn) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(taskB.DependsOn))
	}
	if taskB.DependsOn[0] != "dup-id" {
		t.Errorf("Task B DependsOn should still be %q (first task keeps original ID), got %q", "dup-id", taskB.DependsOn[0])
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

func TestParseTasksMarkdown_DuplicateID_PreservesParentID(t *testing.T) {
	// Parent and Parent2 share "dup-id". Parent2 gets reassigned.
	// Child has parent: dup-id, which should still point to "dup-id" because
	// the first Parent retains the original ID.
	content := `# Project: Test

## Epic: Parent
<!-- id: dup-id, status: todo, priority: 1 -->

## Epic: Parent2
<!-- id: dup-id, status: todo, priority: 2 -->

### Task: Child
<!-- id: child-1, status: todo, priority: 3, parent: dup-id -->
`
	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(result.Tasks))
	}

	parent := result.Tasks[0]
	parent2 := result.Tasks[1]
	child := result.Tasks[2]

	if parent.ID != "dup-id" {
		t.Errorf("Parent should keep original ID 'dup-id', got %q", parent.ID)
	}

	if parent2.ID == "dup-id" {
		t.Fatal("Parent2 should have been reassigned a new ID")
	}

	if child.ParentID == nil {
		t.Fatal("Child should have a ParentID")
	}
	// Child's parent reference points to the first Parent (which kept "dup-id")
	if *child.ParentID != "dup-id" {
		t.Errorf("Child ParentID should still be %q (first parent keeps original ID), got %q", "dup-id", *child.ParentID)
	}
}

func TestParseTasksMarkdown_OrphanedParentRef(t *testing.T) {
	// A task that references a parent ID that doesn't exist in the file should
	// have its ParentID cleared and a DataCorrection warning emitted.
	content := `# Project: Test

## Task: Orphaned Child
<!-- id: child-id, status: todo, priority: 1, parent: nonexistent-parent-id -->
`
	result, err := ParseTasksMarkdownDocument(content)
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

func TestParseTasksMarkdown_DuplicateID_PreservesRelatedTo(t *testing.T) {
	// Task A and Task A2 share "dup-id". A2 gets reassigned.
	// Task C has related: dup-id, which should still point to "dup-id" because
	// Task A (the first) retains the original ID.
	content := `# Project: Test

## Task: Task A
<!-- id: dup-id, status: todo, priority: 1 -->

## Task: Task A2
<!-- id: dup-id, status: todo, priority: 2 -->

## Task: Task C
<!-- id: unique-c, status: todo, priority: 3, related: dup-id -->
`
	result, err := ParseTasksMarkdownDocument(content)
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
	// RelatedTo should still reference "dup-id" (Task A, the first occurrence)
	if taskC.RelatedTo[0] != "dup-id" {
		t.Errorf("Task C RelatedTo should still be %q (first task keeps original ID), got %q", "dup-id", taskC.RelatedTo[0])
	}
}

func TestParseTasksMarkdown_TripleDuplicateID(t *testing.T) {
	// Three tasks share the same ID. Only the first keeps it; the other two
	// get new UUIDs. References to the original ID stay valid.
	content := `# Project: Test

## Task: Task A
<!-- id: dup-id, status: todo, priority: 1 -->

## Task: Task B
<!-- id: dup-id, status: todo, priority: 2 -->

## Task: Task C
<!-- id: dup-id, status: todo, priority: 3 -->

## Task: Task D
<!-- id: unique-d, status: todo, priority: 1, depends: dup-id -->
`
	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(result.Tasks))
	}

	taskA := result.Tasks[0]
	taskB := result.Tasks[1]
	taskC := result.Tasks[2]
	taskD := result.Tasks[3]

	// First task keeps original ID
	if taskA.ID != "dup-id" {
		t.Errorf("Task A should keep original ID, got %q", taskA.ID)
	}

	// Second and third tasks get new IDs
	if taskB.ID == "dup-id" {
		t.Error("Task B should have been reassigned a new ID")
	}
	if taskC.ID == "dup-id" {
		t.Error("Task C should have been reassigned a new ID")
	}

	// All three IDs should be unique
	ids := map[string]bool{taskA.ID: true, taskB.ID: true, taskC.ID: true}
	if len(ids) != 3 {
		t.Errorf("expected 3 unique IDs, got %d (%q, %q, %q)", len(ids), taskA.ID, taskB.ID, taskC.ID)
	}

	// Task D's dependency should still point to "dup-id" (Task A)
	if len(taskD.DependsOn) != 1 || taskD.DependsOn[0] != "dup-id" {
		t.Errorf("Task D DependsOn should still be [\"dup-id\"], got %v", taskD.DependsOn)
	}

	// Should have 2 warnings about duplicate IDs
	dupWarnings := 0
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "duplicate ID") {
			dupWarnings++
		}
	}
	if dupWarnings != 2 {
		t.Errorf("expected 2 duplicate ID warnings, got %d", dupWarnings)
	}
}
