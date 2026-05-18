package storage

import (
	"os"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestPerformTargetedUndo(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: todo, priority: 3 -->\n\n" +
		"## Task: Task B\n<!-- id: task-b, status: todo, priority: 2 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Complete Task", "#1 Task A", 10, []string{"task-a"}); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	mutatedContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: done, priority: 3 -->\n\n" +
		"## Task: Task B\n<!-- id: task-b, status: done, priority: 2 -->\n"
	writeUndoTasks(t, tasksPath, mutatedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		t.Fatalf("failed to parse restored content: %v", err)
	}

	taskMap := make(map[string]*models.Task)
	for _, task := range result.Tasks {
		taskMap[task.ID] = task
	}

	if taskMap["task-a"].Status != models.StatusTodo {
		t.Errorf("expected task-a status 'todo', got %q", taskMap["task-a"].Status)
	}

	if taskMap["task-b"].Status != models.StatusDone {
		t.Errorf("expected task-b status 'done' (side effect preserved), got %q", taskMap["task-b"].Status)
	}
}

func TestPerformTargetedRedo(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: todo, priority: 3 -->\n\n" +
		"## Task: Task B\n<!-- id: task-b, status: todo, priority: 2 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Complete Task", "#1 Task A", 10, []string{"task-a"}); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	mutatedContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: done, priority: 3 -->\n\n" +
		"## Task: Task B\n<!-- id: task-b, status: done, priority: 2 -->\n"
	writeUndoTasks(t, tasksPath, mutatedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	if _, err := PerformRedo(project); err != nil {
		t.Fatalf("PerformRedo failed: %v", err)
	}

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	result, parseErr := ParseTasksMarkdownWithSections(string(content))
	if parseErr != nil {
		t.Fatalf("failed to parse restored content: %v", parseErr)
	}

	taskMap := make(map[string]*models.Task)
	for _, task := range result.Tasks {
		taskMap[task.ID] = task
	}

	if taskMap["task-a"].Status != models.StatusDone {
		t.Errorf("expected task-a status 'done' after redo, got %q", taskMap["task-a"].Status)
	}

	if taskMap["task-b"].Status != models.StatusDone {
		t.Errorf("expected task-b status 'done' after redo, got %q", taskMap["task-b"].Status)
	}
}

func TestPerformTargetedUndoRemovesNewlyCreatedTask(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Add Task", "#2 Task New", 10, []string{"task-new"}); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	mutatedContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: todo, priority: 3 -->\n\n" +
		"## Task: Task New\n<!-- id: task-new, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, mutatedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		t.Fatalf("failed to parse restored content: %v", err)
	}

	found := false
	for _, task := range result.Tasks {
		if task.ID == "task-a" {
			found = true
		}
		if task.ID == "task-new" {
			t.Error("task-new should have been removed by targeted undo")
		}
	}
	if !found {
		t.Error("task-a should still exist after undo")
	}
}

func TestPerformTargetedUndoFallsBackOnNilTaskIDs(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n## Task: Task A\n<!-- id: task-a, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Delete Task", "#1 Task A", 10, nil); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	mutatedContent := "# Project: Test Project\n\n## Task: Task A\n<!-- id: task-a, status: done, priority: 1 -->\n"
	writeUndoTasks(t, tasksPath, mutatedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	if string(content) != initialContent {
		t.Error("full-file undo did not restore original content")
	}
}

func TestPerformTargetedUndoRestoresDeletedTask(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	initialContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: todo, priority: 3 -->\n\n" +
		"## Task: Task B\n<!-- id: task-b, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, initialContent)

	if err := RecordUndoSnapshot(project, "Delete Task", "#2 Task B", 10, []string{"task-b"}); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	mutatedContent := "# Project: Test Project\n\n" +
		"## Task: Task A\n<!-- id: task-a, status: todo, priority: 3 -->\n"
	writeUndoTasks(t, tasksPath, mutatedContent)

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		t.Fatalf("failed to parse restored content: %v", err)
	}

	foundA, foundB := false, false
	for _, task := range result.Tasks {
		if task.ID == "task-a" {
			foundA = true
		}
		if task.ID == "task-b" {
			foundB = true
		}
	}
	if !foundA {
		t.Error("task-a should still exist after undo")
	}
	if !foundB {
		t.Error("task-b should have been restored by targeted undo")
	}
}

func TestPerformTargetedUndoRestoresDecisionsForTargetedTasks(t *testing.T) {
	project, tasksPath := newUndoTestProject(t)

	now := time.Date(2025, 3, 20, 9, 0, 0, 0, time.UTC)
	initialTasks := []*models.Task{
		{
			ID:        "task-a",
			ShortID:   1,
			Type:      models.TaskTypeTask,
			Title:     "Task A",
			Status:    models.StatusTodo,
			Priority:  3,
			CreatedAt: now,
			UpdatedAt: now,
			Decisions: []*models.Decision{
				{
					Text:      "Use SQLite",
					Rationale: "Simple local setup",
					Category:  models.DecisionCategoryArchitecture,
					CreatedAt: now,
				},
			},
		},
		{
			ID:        "task-b",
			ShortID:   2,
			Type:      models.TaskTypeTask,
			Title:     "Task B",
			Status:    models.StatusTodo,
			Priority:  2,
			CreatedAt: now,
			UpdatedAt: now,
			Decisions: []*models.Decision{
				{
					Text:      "Stay CLI-only",
					Rationale: "Avoid splitting focus",
					Category:  models.DecisionCategoryScope,
					CreatedAt: now.Add(time.Hour),
				},
			},
		},
	}
	writeUndoTasks(t, tasksPath, WriteTasksMarkdown("Test Project", initialTasks))

	if err := RecordUndoSnapshot(project, "Update decision", "#1 Task A", 10, []string{"task-a"}); err != nil {
		t.Fatalf("RecordUndoSnapshot failed: %v", err)
	}

	mutatedTasks := []*models.Task{
		{
			ID:        "task-a",
			ShortID:   1,
			Type:      models.TaskTypeTask,
			Title:     "Task A",
			Status:    models.StatusTodo,
			Priority:  3,
			CreatedAt: now,
			UpdatedAt: now.Add(2 * time.Hour),
			Decisions: []*models.Decision{
				{
					Text:      "Use Postgres",
					Rationale: "Prepare for sync features",
					Category:  models.DecisionCategoryArchitecture,
					CreatedAt: now.Add(2 * time.Hour),
					Resolved:  true,
				},
			},
		},
		{
			ID:        "task-b",
			ShortID:   2,
			Type:      models.TaskTypeTask,
			Title:     "Task B",
			Status:    models.StatusDone,
			Priority:  2,
			CreatedAt: now,
			UpdatedAt: now.Add(2 * time.Hour),
			Decisions: []*models.Decision{
				{
					Text:      "Build a web UI",
					Rationale: "Broaden adoption",
					Category:  models.DecisionCategoryUX,
					CreatedAt: now.Add(3 * time.Hour),
				},
			},
		},
	}
	writeUndoTasks(t, tasksPath, WriteTasksMarkdown("Test Project", mutatedTasks))

	if _, err := PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo failed: %v", err)
	}

	content, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		t.Fatalf("failed to parse restored content: %v", err)
	}

	taskMap := make(map[string]*models.Task)
	for _, task := range result.Tasks {
		taskMap[task.ID] = task
	}

	taskA := taskMap["task-a"]
	if len(taskA.Decisions) != 1 {
		t.Fatalf("task-a should have 1 decision after undo, got %d", len(taskA.Decisions))
	}
	if taskA.Decisions[0].Text != "Use SQLite" {
		t.Errorf("task-a decision text = %q, want %q", taskA.Decisions[0].Text, "Use SQLite")
	}
	if taskA.Decisions[0].Resolved {
		t.Error("task-a decision should not be resolved after undo")
	}

	taskB := taskMap["task-b"]
	if len(taskB.Decisions) != 1 {
		t.Fatalf("task-b should have 1 decision after undo, got %d", len(taskB.Decisions))
	}
	if taskB.Decisions[0].Text != "Build a web UI" {
		t.Errorf("task-b decision text = %q, want %q", taskB.Decisions[0].Text, "Build a web UI")
	}
	if taskB.Status != models.StatusDone {
		t.Errorf("task-b status = %q, want %q", taskB.Status, models.StatusDone)
	}
}

func TestRestoreTaskFieldsPreservesStartDateAndEstimatedDays(t *testing.T) {
	startDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	src := &models.Task{
		ID:            "test-id",
		StartDate:     &startDate,
		EstimatedDays: 5.5,
		Status:        models.StatusTodo,
		Title:         "Task with scheduling",
	}

	dst := &models.Task{ID: "test-id"}
	restoreTaskFields(dst, src)

	if dst.StartDate == nil || !dst.StartDate.Equal(startDate) {
		t.Errorf("restoreTaskFields should preserve StartDate, got %v", dst.StartDate)
	}
	if dst.EstimatedDays != 5.5 {
		t.Errorf("restoreTaskFields should preserve EstimatedDays, got %v", dst.EstimatedDays)
	}

	// Verify clearing works too
	src2 := &models.Task{ID: "test-id-2", StartDate: nil, EstimatedDays: 0}
	dst2 := &models.Task{ID: "test-id-2", StartDate: &startDate, EstimatedDays: 3.0}
	restoreTaskFields(dst2, src2)

	if dst2.StartDate != nil {
		t.Error("restoreTaskFields should clear StartDate when src has nil")
	}
	if dst2.EstimatedDays != 0 {
		t.Errorf("restoreTaskFields should clear EstimatedDays, got %v", dst2.EstimatedDays)
	}
}

func TestRestoreTaskFieldsPreservesHasTime(t *testing.T) {
	dst := &models.Task{ID: "test-id"}
	src := &models.Task{
		ID:      "test-id",
		HasTime: true,
		Status:  models.StatusTodo,
		Title:   "Task with time",
	}
	restoreTaskFields(dst, src)
	if !dst.HasTime {
		t.Error("restoreTaskFields should preserve HasTime=true")
	}

	dst2 := &models.Task{ID: "test-id-2", HasTime: true}
	src2 := &models.Task{ID: "test-id-2", HasTime: false}
	restoreTaskFields(dst2, src2)
	if dst2.HasTime {
		t.Error("restoreTaskFields should restore HasTime=false")
	}
}
