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

// newBulkCaptureEnv wires a BulkCaptureModel over the real transactional stack so
// createTasks commits through the CaptureCoordinator (CaptureMany) and its output
// can be reloaded from disk, mirroring production.
func newBulkCaptureEnv(t *testing.T, seed ...*models.Task) (*BulkCaptureModel, *scenarioEnv) {
	t.Helper()
	env := newScenarioEnv(t, "bulk", seed...)
	m := NewBulkCaptureModel(env.project)
	m.SetCaptureCoordinator(env.features.Capture)
	return m, env
}

// TestBulkCaptureModel_validTaskCreation tests that valid tasks are created and
// committed through the coordinator. It checks the created count and that the
// reloaded project holds the new tasks.
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
			epic, err := models.NewTask("epic-1", 1, models.TaskTypeEpic, "User Authentication")
			if err != nil {
				t.Fatalf("NewTask epic: %v", err)
			}
			story, err := models.NewTask("story-2", 2, models.TaskTypeStory, "Login Flow")
			if err != nil {
				t.Fatalf("NewTask story: %v", err)
			}
			m, env := newBulkCaptureEnv(t, epic, story)
			m.input.SetValue(tt.input)

			cmd := m.createTasks()
			if cmd != nil {
				if _, ok := cmd().(ErrorMsg); ok {
					t.Fatalf("bulk capture commit failed for input %q", tt.input)
				}
			}

			if m.created != tt.wantCreated {
				t.Errorf("created = %d, want %d", m.created, tt.wantCreated)
			}
			// The committed project holds the two seeded tasks plus the new ones.
			fresh := env.reload(t)
			if got := len(fresh.Tasks); got != tt.wantCreated+2 {
				t.Errorf("reloaded task count = %d, want %d", got, tt.wantCreated+2)
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
	m, env := newBulkCaptureEnv(t)
	m.input.SetValue("Task one\nTask two")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	fresh := env.reload(t)
	// Check that Inbox section was created
	if !fresh.HasSection(models.SectionInbox) {
		t.Error("Inbox section should have been created")
	}
	// Check that tasks are in the Inbox section
	for _, task := range fresh.Tasks {
		if models.SectionForState(task.State()) != models.SectionInbox {
			t.Errorf("task %q section = %q, want %q", task.Title, models.SectionForState(task.State()), models.SectionInbox)
		}
	}
}

func TestBulkCaptureModel_taskOrder(t *testing.T) {
	existing, err := models.NewTask("existing", 1, models.TaskTypeTask, "Existing task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	m, env := newBulkCaptureEnv(t, existing)
	m.input.SetValue("New task one\nNew task two")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	fresh := env.reload(t)
	// New tasks should be prepended, so they appear first
	if len(fresh.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(fresh.Tasks))
	}
	if fresh.Tasks[0].Title != "New task one" {
		t.Errorf("first task = %q, want %q", fresh.Tasks[0].Title, "New task one")
	}
	if fresh.Tasks[1].Title != "New task two" {
		t.Errorf("second task = %q, want %q", fresh.Tasks[1].Title, "New task two")
	}
	if fresh.Tasks[2].Title != "Existing task" {
		t.Errorf("third task = %q, want %q", fresh.Tasks[2].Title, "Existing task")
	}
}

func TestBulkCaptureModel_rejectsInvalidTitles(t *testing.T) {
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
	}

	// All lines have storage-corrupting characters, so no tasks should be created
	// and errors should be reported for each line.
	m := NewBulkCaptureModel(project)
	m.input.SetValue("Fix <!-- bug\nFix --> issue")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	if m.created != 0 {
		t.Errorf("created = %d, want 0", m.created)
	}
	if len(m.errors) != 2 {
		t.Errorf("errors = %d, want 2", len(m.errors))
	}
	if len(project.Tasks) != 0 {
		t.Errorf("tasks = %d, want 0", len(project.Tasks))
	}

	// Verify error messages reference the title validation
	for _, err := range m.errors {
		if !strings.Contains(err.message, "must not contain") {
			t.Errorf("expected validation error, got: %s", err.message)
		}
	}
}

func TestBulkCaptureModel_appliesRecurrence(t *testing.T) {
	m, env := newBulkCaptureEnv(t)
	m.input.SetValue("Daily standup ~daily")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	if m.created != 1 {
		t.Fatalf("created = %d, want 1", m.created)
	}

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 {
		t.Fatalf("reloaded tasks = %d, want 1", len(fresh.Tasks))
	}
	task := fresh.Tasks[0]
	if task.Recurrence == nil {
		t.Fatal("recurrence should be set")
	}
	if task.Recurrence.Unit != models.RecurrenceDaily || task.Recurrence.Interval != 1 {
		t.Errorf("recurrence = %v, want daily", task.Recurrence)
	}
}

func TestBulkCaptureModel_appliesSection(t *testing.T) {
	m, env := newBulkCaptureEnv(t)
	m.input.SetValue("Important task /next")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	if m.created != 1 {
		t.Fatalf("created = %d, want 1", m.created)
	}

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 {
		t.Fatalf("reloaded tasks = %d, want 1", len(fresh.Tasks))
	}
	if models.SectionForState(fresh.Tasks[0].State()) != "Next" {
		t.Errorf("section = %q, want %q", models.SectionForState(fresh.Tasks[0].State()), "Next")
	}
}

func TestBulkCaptureModel_appliesStatus(t *testing.T) {
	m, env := newBulkCaptureEnv(t)
	m.input.SetValue("Blocked task $blocked")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	if m.created != 1 {
		t.Fatalf("created = %d, want 1", m.created)
	}

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 {
		t.Fatalf("reloaded tasks = %d, want 1", len(fresh.Tasks))
	}
	// $blocked maps to the waiting state; v2 persists state, not the retired
	// blocked status, so the reloaded task derives to waiting.
	if got := fresh.Tasks[0].State(); got != models.StateWaiting {
		t.Errorf("state = %q, want %q", got, models.StateWaiting)
	}
}

func TestBulkCaptureModel_appliesStandbyStatus(t *testing.T) {
	m, env := newBulkCaptureEnv(t)
	m.input.SetValue("On hold task $standby")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	if m.created != 1 {
		t.Fatalf("created = %d, want 1", m.created)
	}

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 {
		t.Fatalf("reloaded tasks = %d, want 1", len(fresh.Tasks))
	}
	task := fresh.Tasks[0]
	// $standby maps to the paused state (canonical section Standby); v2 persists
	// the derived state, not the retired standby status.
	if got := task.State(); got != models.StatePaused {
		t.Errorf("state = %q, want %q", got, models.StatePaused)
	}
	if models.SectionForState(task.State()) != models.SectionStandby {
		t.Errorf("section = %q, want %q", models.SectionForState(task.State()), models.SectionStandby)
	}
}

func TestBulkCaptureModel_appliesReviewStatus(t *testing.T) {
	m, env := newBulkCaptureEnv(t)
	m.input.SetValue("Review this $review")

	cmd := m.createTasks()
	if cmd != nil {
		cmd()
	}

	if m.created != 1 {
		t.Fatalf("created = %d, want 1", m.created)
	}

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 {
		t.Fatalf("reloaded tasks = %d, want 1", len(fresh.Tasks))
	}
	task := fresh.Tasks[0]
	// $review maps to the review state (canonical section Review); v2 persists
	// the derived state, not the retired review status.
	if got := task.State(); got != models.StateReview {
		t.Errorf("state = %q, want %q", got, models.StateReview)
	}
	if models.SectionForState(task.State()) != models.SectionReview {
		t.Errorf("section = %q, want %q", models.SectionForState(task.State()), models.SectionReview)
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
