package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplyAction_SetStatusLifecycle(t *testing.T) {
	t.Run("completing a task sets CompletedAt and Section", func(t *testing.T) {
		task := &models.Task{ID: "1", Status: models.StatusInProgress, Active: true}
		modified := applyAction(task, RuleAction{ActionType: ActionSetStatus, Value: "done"})
		if !modified {
			t.Error("expected modification")
		}
		if task.Status != models.StatusDone {
			t.Errorf("expected done, got %s", task.Status)
		}
		if task.CompletedAt == nil {
			t.Error("expected CompletedAt to be set")
		}
		if task.Active {
			t.Error("expected Active to be cleared")
		}
		if task.Section != models.SectionCompleted {
			t.Errorf("expected Section=%q, got %q", models.SectionCompleted, task.Section)
		}
	})

	t.Run("canceling a task sets CompletedAt and Section", func(t *testing.T) {
		task := &models.Task{ID: "2", Status: models.StatusTodo}
		modified := applyAction(task, RuleAction{ActionType: ActionSetStatus, Value: "canceled"})
		if !modified {
			t.Error("expected modification")
		}
		if task.CompletedAt == nil {
			t.Error("expected CompletedAt to be set")
		}
		if task.Section != models.SectionCompleted {
			t.Errorf("expected Section=%q, got %q", models.SectionCompleted, task.Section)
		}
	})

	t.Run("reopening a task clears CompletedAt", func(t *testing.T) {
		now := time.Now()
		task := &models.Task{ID: "3", Status: models.StatusDone, CompletedAt: &now, Section: models.SectionCompleted}
		modified := applyAction(task, RuleAction{ActionType: ActionSetStatus, Value: "todo"})
		if !modified {
			t.Error("expected modification")
		}
		if task.CompletedAt != nil {
			t.Error("expected CompletedAt to be nil after reopen")
		}
		if task.Section != models.SectionInbox {
			t.Errorf("expected Section=%q, got %q", models.SectionInbox, task.Section)
		}
	})

	t.Run("blocking a task sets Section to Waiting", func(t *testing.T) {
		task := &models.Task{ID: "4", Status: models.StatusInProgress}
		modified := applyAction(task, RuleAction{ActionType: ActionSetStatus, Value: "blocked"})
		if !modified {
			t.Error("expected modification")
		}
		if task.Section != models.SectionWaiting {
			t.Errorf("expected Section=%q, got %q", models.SectionWaiting, task.Section)
		}
	})

	t.Run("starting a task clears Inbox section", func(t *testing.T) {
		task := &models.Task{ID: "5", Status: models.StatusTodo, Section: models.SectionInbox}
		modified := applyAction(task, RuleAction{ActionType: ActionSetStatus, Value: "in_progress"})
		if !modified {
			t.Error("expected modification")
		}
		if task.Section != "" {
			t.Errorf("expected Section to be empty, got %q", task.Section)
		}
	})
}

func TestApplyAction_SetSnooze(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		setupTask    func() *models.Task
		shouldModify bool
		check        func(*models.Task) bool
	}{
		{
			name:  "sets SnoozedUntil correctly for +3d",
			value: "+3d",
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Status: models.StatusTodo}
			},
			shouldModify: true,
			check: func(t *models.Task) bool {
				if t.SnoozedUntil == nil {
					return false
				}
				expected := time.Now().AddDate(0, 0, 3)
				diff := t.SnoozedUntil.Sub(expected)
				return diff < time.Hour && diff > -time.Hour
			},
		},
		{
			name:  "overwrites existing snooze",
			value: "+1w",
			setupTask: func() *models.Task {
				old := time.Now().Add(24 * time.Hour)
				return &models.Task{ID: "1", SnoozedUntil: &old}
			},
			shouldModify: true,
			check: func(t *models.Task) bool {
				if t.SnoozedUntil == nil {
					return false
				}
				expected := time.Now().AddDate(0, 0, 7)
				diff := t.SnoozedUntil.Sub(expected)
				return diff < time.Hour && diff > -time.Hour
			},
		},
		{
			name:  "invalid duration returns false",
			value: "invalid",
			setupTask: func() *models.Task {
				return &models.Task{ID: "1"}
			},
			shouldModify: false,
			check: func(t *models.Task) bool {
				return t.SnoozedUntil == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tt.setupTask()
			modified := applyAction(task, RuleAction{ActionType: ActionSetSnooze, Value: tt.value})
			if modified != tt.shouldModify {
				t.Errorf("applyAction() modified = %v, want %v", modified, tt.shouldModify)
			}
			if !tt.check(task) {
				t.Errorf("applyAction() did not produce expected result")
			}
		})
	}
}

func TestApplyAction_SetRecurrence(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		setupTask    func() *models.Task
		shouldModify bool
		check        func(*models.Task) bool
	}{
		{
			name:  "sets daily recurrence",
			value: "daily",
			setupTask: func() *models.Task {
				return &models.Task{ID: "1"}
			},
			shouldModify: true,
			check: func(t *models.Task) bool {
				return t.Recurrence != nil && t.Recurrence.Unit == models.RecurrenceDaily && t.Recurrence.Interval == 1
			},
		},
		{
			name:  "updates existing pattern",
			value: "weekly",
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}}
			},
			shouldModify: true,
			check: func(t *models.Task) bool {
				return t.Recurrence != nil && t.Recurrence.Unit == models.RecurrenceWeekly
			},
		},
		{
			name:  "invalid pattern returns false",
			value: "badpattern",
			setupTask: func() *models.Task {
				return &models.Task{ID: "1"}
			},
			shouldModify: false,
			check: func(t *models.Task) bool {
				return t.Recurrence == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tt.setupTask()
			modified := applyAction(task, RuleAction{ActionType: ActionSetRecurrence, Value: tt.value})
			if modified != tt.shouldModify {
				t.Errorf("applyAction() modified = %v, want %v", modified, tt.shouldModify)
			}
			if !tt.check(task) {
				t.Errorf("applyAction() did not produce expected result")
			}
		})
	}
}
