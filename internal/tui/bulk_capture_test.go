package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestBulkCaptureModel_parseAllLines tests the parsing behavior without storage dependencies.
// Tests that only contain invalid lines (no valid ones) don't trigger storage operations.
func TestBulkCaptureModel_parseAllLines(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantErrors     int
		wantRemaining  string
		wantErrMsgPart string
	}{
		{
			name:           "empty input",
			input:          "",
			wantErrors:     1,
			wantErrMsgPart: "no tasks",
		},
		{
			name:       "only whitespace and empty lines",
			input:      "   \n\n  \n",
			wantErrors: 1,
		},
		{
			name:           "all invalid - empty titles",
			input:          "!1 #tag\n@tomorrow\n>999",
			wantErrors:     3,
			wantRemaining:  "!1 #tag\n@tomorrow\n>999",
			wantErrMsgPart: "title cannot be empty",
		},
		{
			name:           "invalid date",
			input:          "Task @notadate",
			wantErrors:     1,
			wantErrMsgPart: "invalid date",
		},
		{
			name:           "missing parent",
			input:          "Task >999",
			wantErrors:     1,
			wantErrMsgPart: "parent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testProject := &models.Project{
				ID:   "test-project",
				Name: "Test Project",
			}

			m := NewBulkCaptureModel(testProject)
			m.input.SetValue(tt.input)

			cmd := m.createTasks()
			if cmd != nil {
				cmd()
			}

			if len(m.errors) != tt.wantErrors {
				t.Errorf("errors count = %d, want %d", len(m.errors), tt.wantErrors)
			}

			if tt.wantRemaining != "" {
				remaining := m.input.Value()
				if remaining != tt.wantRemaining {
					t.Errorf("remaining input = %q, want %q", remaining, tt.wantRemaining)
				}
			}

			if tt.wantErrMsgPart != "" && len(m.errors) > 0 {
				foundMatch := false
				for _, err := range m.errors {
					if strings.Contains(err.message, tt.wantErrMsgPart) {
						foundMatch = true
						break
					}
				}
				if !foundMatch {
					t.Errorf("expected error containing %q, got %v", tt.wantErrMsgPart, m.errors)
				}
			}
		})
	}
}

// TestBulkCaptureModel_validTaskCreation tests that valid tasks are created in-memory.
// These tests check created count and task properties before storage is called.
func TestBulkCaptureModel_validTaskCreation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCreated int
	}{
		{
			name:        "single valid line",
			input:       "Fix login bug",
			wantCreated: 1,
		},
		{
			name:        "multiple valid lines",
			input:       "Task one\nTask two\nTask three",
			wantCreated: 3,
		},
		{
			name:        "valid lines with metadata",
			input:       "Fix bug !1 #backend\nAdd feature !2 #frontend @tomorrow",
			wantCreated: 2,
		},
		{
			name:        "valid parent reference",
			input:       "Subtask >1\nAnother subtask >2",
			wantCreated: 2,
		},
		{
			name:        "lines with extra whitespace",
			input:       "  First task  \n  Second task  \n  Third task  ",
			wantCreated: 3,
		},
		{
			name:        "empty lines between valid",
			input:       "First task\n\nSecond task\n\n\nThird task",
			wantCreated: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testProject := &models.Project{
				ID:   "test-project",
				Name: "Test Project",
				Tasks: []*models.Task{
					{ID: "epic-1", ShortID: 1, Type: models.TaskTypeEpic, Title: "User Authentication"},
					{ID: "story-2", ShortID: 2, Type: models.TaskTypeStory, Title: "Login Flow"},
				},
			}
			testProject.BuildTaskTree()

			m := NewBulkCaptureModel(testProject)
			m.input.SetValue(tt.input)

			cmd := m.createTasks()
			if cmd != nil {
				cmd() // Execute - storage will fail but created count is set first
			}

			if m.created != tt.wantCreated {
				t.Errorf("created = %d, want %d", m.created, tt.wantCreated)
			}
		})
	}
}

func TestBulkCaptureModel_lineErrorNumbers(t *testing.T) {
	// Test that line numbers in errors are sequential for the failed lines
	// Note: This test uses only invalid lines to avoid storage dependencies
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
	}

	m := NewBulkCaptureModel(project)
	m.input.SetValue("!1\n!2\n!3")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	// Should have 3 errors (all lines are invalid - no title)
	if len(m.errors) != 3 {
		t.Fatalf("errors count = %d, want 3", len(m.errors))
	}

	// Lines should be numbered 1, 2, 3
	for i, err := range m.errors {
		expectedLine := i + 1
		if err.line != expectedLine {
			t.Errorf("error %d line = %d, want %d", i, err.line, expectedLine)
		}
	}
}

func TestBulkCaptureModel_tasksAddedToInbox(t *testing.T) {
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
	}

	m := NewBulkCaptureModel(project)
	m.input.SetValue("Task one\nTask two")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	// Check that Inbox section was created
	if !project.HasSection(models.SectionInbox) {
		t.Error("Inbox section should have been created")
	}

	// Check that tasks are in the Inbox section
	for _, task := range project.Tasks {
		if task.Section != models.SectionInbox {
			t.Errorf("task %q section = %q, want %q", task.Title, task.Section, models.SectionInbox)
		}
	}
}

func TestBulkCaptureModel_taskOrder(t *testing.T) {
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
		Tasks: []*models.Task{
			{ID: "existing", ShortID: 1, Title: "Existing task"},
		},
	}

	m := NewBulkCaptureModel(project)
	m.input.SetValue("New task one\nNew task two")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	// New tasks should be prepended, so they appear first
	if len(project.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(project.Tasks))
	}

	// First two should be the new ones
	if project.Tasks[0].Title != "New task one" {
		t.Errorf("first task = %q, want %q", project.Tasks[0].Title, "New task one")
	}
	if project.Tasks[1].Title != "New task two" {
		t.Errorf("second task = %q, want %q", project.Tasks[1].Title, "New task two")
	}
	if project.Tasks[2].Title != "Existing task" {
		t.Errorf("third task = %q, want %q", project.Tasks[2].Title, "Existing task")
	}
}

func TestNewBulkCaptureModel(t *testing.T) {
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
	}

	m := NewBulkCaptureModel(project)

	if m.project != project {
		t.Error("project not set correctly")
	}

	if m.created != 0 {
		t.Errorf("created should start at 0, got %d", m.created)
	}

	if len(m.errors) != 0 {
		t.Errorf("errors should start empty, got %d", len(m.errors))
	}
}
