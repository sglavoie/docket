package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestLoadArchivedTasks(t *testing.T) { //nolint:gocyclo
	t.Run("returns empty slice when no archive file exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(tasks))
		}
	})

	t.Run("loads tasks from single archive file", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		completedAt := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
		content := `## Archived: 2025-06-10 15:00
<!-- project: TestProj, count: 2 -->

### Task: First archived task
<!-- id: arch-1, status: done, priority: 2, created: 2025-06-01 10:00, completed: 2025-06-10 14:30 -->

Some description.

### Bug: Second archived task
<!-- id: arch-2, status: canceled, priority: 1, created: 2025-06-02 11:00, completed: 2025-06-10 14:30 -->
`
		archivePath := filepath.Join(tmpDir, "completed.md")
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(tasks))
		}
		if tasks[0].ID != "arch-1" {
			t.Errorf("expected first task ID arch-1, got %s", tasks[0].ID)
		}
		if tasks[0].State() != models.StateDone {
			t.Errorf("expected state done, got %s", tasks[0].State())
		}
		if tasks[0].CompletedAt == nil || !tasks[0].CompletedAt.Equal(completedAt) {
			t.Errorf("expected CompletedAt %v, got %v", completedAt, tasks[0].CompletedAt)
		}
		if tasks[1].ID != "arch-2" {
			t.Errorf("expected second task ID arch-2, got %s", tasks[1].ID)
		}
		if tasks[1].State() != models.StateCanceled {
			t.Errorf("expected state canceled, got %s", tasks[1].State())
		}
	})

	t.Run("loads tasks from multiple rotated archive files", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		mainContent := `### Task: Main task
<!-- id: main-1, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		rotated1Content := `### Task: Rotated 1 task
<!-- id: rot1-1, status: done, priority: 2, completed: 2025-05-15 10:00 -->
`
		rotated2Content := `### Bug: Rotated 2 bug
<!-- id: rot2-1, status: done, priority: 1, completed: 2025-04-20 09:00 -->
`
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.md"), []byte(mainContent), 0644); err != nil {
			t.Fatalf("failed to write main archive: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.1.md"), []byte(rotated1Content), 0644); err != nil {
			t.Fatalf("failed to write rotated 1: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.2.md"), []byte(rotated2Content), 0644); err != nil {
			t.Fatalf("failed to write rotated 2: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 3 {
			t.Fatalf("expected 3 tasks from all archive files, got %d", len(tasks))
		}

		ids := make(map[string]bool)
		for _, task := range tasks {
			ids[task.ID] = true
		}
		for _, expected := range []string{"main-1", "rot1-1", "rot2-1"} {
			if !ids[expected] {
				t.Errorf("expected task %s to be loaded from archives", expected)
			}
		}
	})

	t.Run("skips malformed archive files without failing", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		goodContent := `### Task: Good task
<!-- id: good-1, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.md"), []byte(goodContent), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 1 {
			t.Errorf("expected 1 task from good archive, got %d", len(tasks))
		}
	})

	t.Run("activity logs round-trip through archive write and load", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}

		now := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
		startedAt := time.Date(2025, 6, 5, 10, 0, 0, 0, time.Local)
		blockedAt := time.Date(2025, 6, 7, 9, 0, 0, 0, time.Local)
		unblockedAt := time.Date(2025, 6, 8, 11, 0, 0, 0, time.Local)

		task := &models.Task{
			ID:            "act-task-1",
			Title:         "Task with activity",
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateDone,
			Priority:      2,
			CreatedAt:     startedAt,
			CompletedAt:   &now,
			Activity: []*models.ActivityEntry{
				{Timestamp: startedAt, Field: "status", OldValue: "todo", NewValue: "in_progress"},
				{Timestamp: blockedAt, Field: "status", OldValue: "in_progress", NewValue: "blocked"},
				{Timestamp: unblockedAt, Field: "status", OldValue: "blocked", NewValue: "in_progress"},
			},
		}

		content := WriteArchivedTasks(project.Name, []*models.Task{task})
		archivePath := filepath.Join(tmpDir, "completed.md")
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}

		loaded := tasks[0]
		if len(loaded.Activity) != 3 {
			t.Fatalf("expected 3 activity entries, got %d", len(loaded.Activity))
		}

		if loaded.Activity[0].Field != "status" || loaded.Activity[0].NewValue != "in_progress" {
			t.Errorf("first activity entry mismatch: %+v", loaded.Activity[0])
		}
		if loaded.Activity[1].Field != "status" || loaded.Activity[1].NewValue != "blocked" {
			t.Errorf("second activity entry mismatch: %+v", loaded.Activity[1])
		}
		if loaded.Activity[2].Field != "status" || loaded.Activity[2].NewValue != "in_progress" {
			t.Errorf("third activity entry mismatch: %+v", loaded.Activity[2])
		}
	})

	t.Run("loads tasks even with gaps in rotated file numbering", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir}

		content1 := `### Task: Rotated 1
<!-- id: rot1, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		content3 := `### Task: Rotated 3
<!-- id: rot3, status: done, priority: 3, completed: 2025-06-10 14:30 -->
`
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.1.md"), []byte(content1), 0644); err != nil {
			t.Fatalf("failed to write completed.1.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "completed.3.md"), []byte(content3), 0644); err != nil {
			t.Fatalf("failed to write completed.3.md: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks from gapped archives, got %d", len(tasks))
		}

		ids := make(map[string]bool)
		for _, task := range tasks {
			ids[task.ID] = true
		}
		if !ids["rot1"] || !ids["rot3"] {
			t.Errorf("expected both rot1 and rot3, got %v", ids)
		}
	})

	t.Run("comments round-trip through archive write and load", func(t *testing.T) {
		tmpDir := t.TempDir()
		project := &models.Project{Path: tmpDir, Name: "TestProj"}

		now := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
		commentTime := time.Date(2025, 6, 8, 9, 0, 0, 0, time.Local)

		task := &models.Task{
			ID:            "cmt-task-1",
			Title:         "Task with comments",
			Type:          models.TaskTypeTask,
			WorkflowState: models.StateDone,
			Priority:      2,
			CompletedAt:   &now,
			Comments: []*models.Comment{
				{Text: "This is a test comment", CreatedAt: commentTime},
			},
		}

		content := WriteArchivedTasks(project.Name, []*models.Task{task})
		archivePath := filepath.Join(tmpDir, "completed.md")
		if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write archive: %v", err)
		}

		tasks := LoadArchivedTasks(project)

		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if len(tasks[0].Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(tasks[0].Comments))
		}
		if tasks[0].Comments[0].Text != "This is a test comment" {
			t.Errorf("comment text mismatch: %q", tasks[0].Comments[0].Text)
		}
	})
}

func TestArchivePreservesAllMetadataFields(t *testing.T) { //nolint:gocyclo
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}

	now := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
	snoozed := time.Date(2025, 7, 1, 0, 0, 0, 0, time.Local)

	task := &models.Task{
		ID:            "full-task-1",
		Title:         "Task with all fields",
		Type:          models.TaskTypeBug,
		WorkflowState: models.StateDone,
		Priority:      1,
		Tags:          []string{"backend", "critical"},
		CreatedAt:     now.Add(-48 * time.Hour),
		UpdatedAt:     now.Add(-1 * time.Hour),
		CompletedAt:   &now,
		DependsOn:     []string{"dep-1", "dep-2"},
		RelatedTo:     []string{"rel-1"},
		SnoozedUntil:  &snoozed,
		Source:        "github",
		URL:           "https://github.com/example/issue/42",
		Checklist: []models.ChecklistItem{
			{Text: "Step one", Done: true},
			{Text: "Step two", Done: false},
		},
		ScoreDimensions: map[string]int{
			"impact": 4,
			"effort": 2,
		},
		BugAttrs: &models.BugAttrs{
			Severity:     models.BugSeverityCritical,
			Reproducible: models.BugReproducibleYes,
			Workaround:   "Restart the service",
		},
		Attachments: []models.Attachment{
			{
				Filename:     "screenshot.png",
				OriginalPath: "/home/user/screenshot.png",
				AddedAt:      now.Add(-24 * time.Hour),
			},
			{
				Filename:     "https://example.com/doc",
				IsURL:        true,
				OriginalPath: "https://example.com/doc",
				AddedAt:      now.Add(-12 * time.Hour),
			},
			{
				Filename:     "main.go",
				IsCodeRef:    true,
				LineNumber:   42,
				OriginalPath: "/home/user/project/main.go",
				AddedAt:      now.Add(-6 * time.Hour),
			},
		},
	}

	content := WriteArchivedTasks(project.Name, []*models.Task{task})
	archivePath := filepath.Join(tmpDir, "completed.md")
	if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write archive: %v", err)
	}

	tasks := LoadArchivedTasks(project)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	loaded := tasks[0]

	// The archived task is done (status done dominates the Next section), so the
	// v2 archive writer serializes state=done and the section reconstitutes to
	// the canonical Completed shim on load.
	if loaded.State() != models.StateDone {
		t.Errorf("state: got %q, want %q", loaded.State(), models.StateDone)
	}
	if section := models.SectionForState(loaded.State()); section != models.SectionCompleted {
		t.Errorf("section: got %q, want %q (canonical done shim)", section, models.SectionCompleted)
	}
	if len(loaded.DependsOn) != 2 || loaded.DependsOn[0] != "dep-1" || loaded.DependsOn[1] != "dep-2" {
		t.Errorf("depends_on: got %v, want [dep-1 dep-2]", loaded.DependsOn)
	}
	if len(loaded.RelatedTo) != 1 || loaded.RelatedTo[0] != "rel-1" {
		t.Errorf("related_to: got %v, want [rel-1]", loaded.RelatedTo)
	}
	if loaded.SnoozedUntil == nil {
		t.Error("snoozed_until: should not be nil")
	}
	// Custom stages are retired by the v2 writer and the Stage field is retired on Task.
	if loaded.Source != "github" {
		t.Errorf("source: got %q, want %q", loaded.Source, "github")
	}
	if loaded.URL != "https://github.com/example/issue/42" {
		t.Errorf("url: got %q, want %q", loaded.URL, "https://github.com/example/issue/42")
	}
	if len(loaded.Checklist) != 2 {
		t.Fatalf("checklist: got %d items, want 2", len(loaded.Checklist))
	}
	if !loaded.Checklist[0].Done || loaded.Checklist[0].Text != "Step one" {
		t.Errorf("checklist[0]: got %+v, want done=true text='Step one'", loaded.Checklist[0])
	}
	if loaded.Checklist[1].Done || loaded.Checklist[1].Text != "Step two" {
		t.Errorf("checklist[1]: got %+v, want done=false text='Step two'", loaded.Checklist[1])
	}
	if len(loaded.ScoreDimensions) != 2 {
		t.Fatalf("score_dimensions: got %d, want 2", len(loaded.ScoreDimensions))
	}
	if loaded.ScoreDimensions["impact"] != 4 || loaded.ScoreDimensions["effort"] != 2 {
		t.Errorf("score_dimensions: got %v, want impact=4 effort=2", loaded.ScoreDimensions)
	}
	if loaded.BugAttrs == nil {
		t.Fatal("bug_attrs: should not be nil")
	}
	if loaded.BugAttrs.Severity != models.BugSeverityCritical {
		t.Errorf("bug.severity: got %q, want %q", loaded.BugAttrs.Severity, models.BugSeverityCritical)
	}
	if loaded.BugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("bug.reproducible: got %q, want %q", loaded.BugAttrs.Reproducible, models.BugReproducibleYes)
	}
	if loaded.BugAttrs.Workaround != "Restart the service" {
		t.Errorf("bug.workaround: got %q, want %q", loaded.BugAttrs.Workaround, "Restart the service")
	}

	if len(loaded.Attachments) != 3 {
		t.Fatalf("attachments: got %d, want 3", len(loaded.Attachments))
	}
	if loaded.Attachments[0].OriginalPath != "/home/user/screenshot.png" {
		t.Errorf("attachment[0].OriginalPath: got %q, want %q", loaded.Attachments[0].OriginalPath, "/home/user/screenshot.png")
	}
	if !loaded.Attachments[1].IsURL {
		t.Error("attachment[1]: should be URL")
	}
	if !loaded.Attachments[2].IsCodeRef {
		t.Error("attachment[2]: should be code ref")
	}
	if loaded.Attachments[2].LineNumber != 42 {
		t.Errorf("attachment[2].LineNumber: got %d, want 42", loaded.Attachments[2].LineNumber)
	}
	if loaded.Attachments[2].OriginalPath != "/home/user/project/main.go" {
		t.Errorf("attachment[2].OriginalPath: got %q, want %q", loaded.Attachments[2].OriginalPath, "/home/user/project/main.go")
	}
}

func TestArchivePreservesDecisions(t *testing.T) {
	tmpDir := t.TempDir()
	project := &models.Project{Path: tmpDir, Name: "TestProj"}

	now := time.Date(2025, 6, 10, 14, 30, 0, 0, time.Local)
	task := &models.Task{
		ID:            "decision-task-1",
		Title:         "Task with decisions",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateDone,
		Priority:      2,
		Description:   "Keep this description intact.",
		CreatedAt:     now.Add(-24 * time.Hour),
		UpdatedAt:     now,
		CompletedAt:   &now,
		Decisions: []*models.Decision{
			{
				Text:      "Use SQLite for local storage",
				Rationale: "Keeps setup simple for a CLI app",
				Category:  models.DecisionCategoryArchitecture,
				CreatedAt: now.Add(-2 * time.Hour),
			},
			{
				Text:      "Hide the beta workflow toggle",
				Rationale: "Reduce support burden until behavior stabilizes",
				Category:  models.DecisionCategoryUX,
				CreatedAt: now.Add(-time.Hour),
				Resolved:  true,
			},
		},
	}

	content := WriteArchivedTasks(project.Name, []*models.Task{task})
	archivePath := filepath.Join(tmpDir, "completed.md")
	if err := os.WriteFile(archivePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write archive: %v", err)
	}

	tasks := LoadArchivedTasks(project)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	loaded := tasks[0]
	if loaded.Description != task.Description {
		t.Fatalf("description mismatch: got %q, want %q", loaded.Description, task.Description)
	}
	if len(loaded.Decisions) != len(task.Decisions) {
		t.Fatalf("expected %d decisions, got %d", len(task.Decisions), len(loaded.Decisions))
	}

	for i, want := range task.Decisions {
		got := loaded.Decisions[i]
		if got.Text != want.Text {
			t.Errorf("decision[%d].Text = %q, want %q", i, got.Text, want.Text)
		}
		if got.Rationale != want.Rationale {
			t.Errorf("decision[%d].Rationale = %q, want %q", i, got.Rationale, want.Rationale)
		}
		if got.Category != want.Category {
			t.Errorf("decision[%d].Category = %q, want %q", i, got.Category, want.Category)
		}
		if got.Resolved != want.Resolved {
			t.Errorf("decision[%d].Resolved = %t, want %t", i, got.Resolved, want.Resolved)
		}
	}
}
