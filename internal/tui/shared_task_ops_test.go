package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestStatusForPrefixKey(t *testing.T) {
	tests := []struct {
		key    string
		want   models.TaskStatus
		wantOK bool
	}{
		{"s", models.StatusInProgress, true},
		{"d", models.StatusDone, true},
		{"b", models.StatusBlocked, true},
		{"o", models.StatusTodo, true},
		{"x", models.StatusCanceled, true},
		{"r", models.StatusReview, true},
		{"z", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := StatusForPrefixKey(tt.key)
		if ok != tt.wantOK || got != tt.want {
			t.Errorf("StatusForPrefixKey(%q) = (%v, %v), want (%v, %v)", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestStatusVerb(t *testing.T) {
	tests := []struct {
		status models.TaskStatus
		want   string
	}{
		{models.StatusInProgress, "started"},
		{models.StatusDone, "completed"},
		{models.StatusBlocked, "blocked"},
		{models.StatusTodo, "reopened"},
		{models.StatusCanceled, "canceled"},
		{models.StatusReview, "sent to review"},
	}
	for _, tt := range tests {
		got := StatusVerb(tt.status)
		if got != tt.want {
			t.Errorf("StatusVerb(%v) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestDueDateDaysForPrefixKey(t *testing.T) {
	tests := []struct {
		key       string
		wantDays  int
		wantClear bool
		wantOK    bool
	}{
		{"t", 0, false, true},
		{"m", 1, false, true},
		{"w", 7, false, true},
		{"n", 30, false, true},
		{"0", 0, true, true},
		{"z", 0, false, false},
		{"", 0, false, false},
	}
	for _, tt := range tests {
		days, isClear, ok := DueDateDaysForPrefixKey(tt.key)
		if days != tt.wantDays || isClear != tt.wantClear || ok != tt.wantOK {
			t.Errorf("DueDateDaysForPrefixKey(%q) = (%d, %v, %v), want (%d, %v, %v)",
				tt.key, days, isClear, ok, tt.wantDays, tt.wantClear, tt.wantOK)
		}
	}
}

func TestPriorityForPrefixKey(t *testing.T) {
	tests := []struct {
		key    string
		want   int
		wantOK bool
	}{
		{"1", 1, true},
		{"2", 2, true},
		{"3", 3, true},
		{"4", 4, true},
		{"5", 5, true},
		{"0", 0, false},
		{"6", 0, false},
		{"a", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := PriorityForPrefixKey(tt.key)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("PriorityForPrefixKey(%q) = (%d, %v), want (%d, %v)", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestDueDateFromDays(t *testing.T) {
	t.Run("returns nil for clear", func(t *testing.T) {
		d, desc := dueDateFromDays(0, true)
		if d != nil {
			t.Error("expected nil for isClear=true")
		}
		if desc != "cleared" {
			t.Errorf("got desc %q, want %q", desc, "cleared")
		}
	})

	t.Run("returns start of day", func(t *testing.T) {
		d, _ := dueDateFromDays(1, false)
		if d == nil {
			t.Fatal("expected non-nil date")
		}
		// The time component should be midnight (start of day)
		if d.Hour() != 0 || d.Minute() != 0 || d.Second() != 0 {
			t.Errorf("expected start-of-day (00:00:00), got %02d:%02d:%02d",
				d.Hour(), d.Minute(), d.Second())
		}
	})

	t.Run("correct day offset", func(t *testing.T) {
		d, _ := dueDateFromDays(7, false)
		if d == nil {
			t.Fatal("expected non-nil date")
		}
		expected := time.Now().AddDate(0, 0, 7)
		if d.Year() != expected.Year() || d.Month() != expected.Month() || d.Day() != expected.Day() {
			t.Errorf("expected date %v, got %v", expected.Format("2006-01-02"), d.Format("2006-01-02"))
		}
	})
}

func TestSectionForMoveKey(t *testing.T) {
	tests := []struct {
		key    string
		want   string
		wantOK bool
	}{
		{"a", "", true},
		{"n", models.SectionNext, true},
		{"i", models.SectionInbox, true},
		{"w", models.SectionWaiting, true},
		{"s", models.SectionSomeday, true},
		{"r", models.SectionReview, true},
		{"z", "", false},
	}
	for _, tt := range tests {
		got, ok := sectionForMoveKey(tt.key)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("sectionForMoveKey(%q) = (%q, %v), want (%q, %v)", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
}

// TestPersistWithWorkflow_GeneratesRecurringAfterRules verifies that
// persistWithWorkflow generates recurring instances after workflow rule
// evaluation, not just before. This catches the case where a workflow rule
// sets a recurring task to "done".
func TestPersistWithWorkflow_GeneratesRecurringAfterRules(t *testing.T) {
	cfg := testutil.NewTestConfig()
	configPath := filepath.Join(t.TempDir(), "tt.json")
	manager := config.NewManager(configPath)
	mockPersister := NewMockPersister()

	project := testutil.NewTestProject("Recur")

	// Create a recurring task that is already done (simulating a workflow rule
	// having just set it to done during evaluation).
	task, err := models.NewTask("recur-1", project.GetNextShortID(), models.TaskTypeTask, "Daily standup")
	if err != nil {
		t.Fatal(err)
	}
	task.Status = models.StatusDone
	task.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	opts := WorkflowSaveOptions{
		Persister:     mockPersister,
		Project:       project,
		Config:        cfg,
		ConfigManager: manager,
	}

	if err := persistWithWorkflow(opts); err != nil {
		t.Fatalf("persistWithWorkflow failed: %v", err)
	}

	// The post-rule hooks should have generated a new recurring instance
	if len(project.Tasks) < 2 {
		t.Fatalf("expected at least 2 tasks (original + recurring instance), got %d", len(project.Tasks))
	}

	var newInstance *models.Task
	for _, tsk := range project.Tasks {
		if tsk.ID != task.ID && tsk.Status == models.StatusTodo && tsk.Recurrence != nil {
			newInstance = tsk
			break
		}
	}
	if newInstance == nil {
		t.Fatal("recurring instance was not generated by persistWithWorkflow post-rule hooks")
	}
	if task.Recurrence != nil {
		t.Error("original completed task should have its recurrence cleared")
	}
}
