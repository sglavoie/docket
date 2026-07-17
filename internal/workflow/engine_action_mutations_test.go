package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplyAction_NewActions(t *testing.T) {
	tests := []struct {
		name           string
		action         RuleAction
		setupTask      func() *models.Task
		expectedChange func(*models.Task) bool
		shouldModify   bool
	}{
		{
			name:   "set due relative +3d",
			action: RuleAction{ActionType: ActionSetDueRelative, Value: "+3d"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", WorkflowState: models.StateReady}
			},
			expectedChange: func(t *models.Task) bool {
				if t.DueDate == nil {
					return false
				}
				expected := time.Now().AddDate(0, 0, 3)
				diff := t.DueDate.Sub(expected)
				return diff < time.Hour && diff > -time.Hour
			},
			shouldModify: true,
		},
		{
			name:   "clear due date",
			action: RuleAction{ActionType: ActionClearDue, Value: ""},
			setupTask: func() *models.Task {
				due := time.Now().Add(24 * time.Hour)
				return &models.Task{ID: "1", DueDate: &due}
			},
			expectedChange: func(t *models.Task) bool {
				return t.DueDate == nil
			},
			shouldModify: true,
		},
		{
			name:   "clear due date no-op when nil",
			action: RuleAction{ActionType: ActionClearDue, Value: ""},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", DueDate: nil}
			},
			expectedChange: func(t *models.Task) bool {
				return t.DueDate == nil
			},
			shouldModify: false,
		},
		{
			name:   "set pinned true",
			action: RuleAction{ActionType: ActionSetPinned, Value: "true"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Pinned: false}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Pinned == true && t.PinnedAt != nil
			},
			shouldModify: true,
		},
		{
			name:   "set pinned false",
			action: RuleAction{ActionType: ActionSetPinned, Value: "false"},
			setupTask: func() *models.Task {
				now := time.Now()
				return &models.Task{ID: "1", Pinned: true, PinnedAt: &now}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Pinned == false && t.PinnedAt == nil
			},
			shouldModify: true,
		},
		{
			name:   "add comment",
			action: RuleAction{ActionType: ActionAddComment, Value: "Test comment"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Comments: []*models.Comment{}}
			},
			expectedChange: func(t *models.Task) bool {
				return len(t.Comments) == 1 && t.Comments[0].Text == "[Workflow] Test comment"
			},
			shouldModify: true,
		},
		{
			name:   "add comment idempotent when last comment matches",
			action: RuleAction{ActionType: ActionAddComment, Value: "Test comment"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Comments: []*models.Comment{{Text: "[Workflow] Test comment", CreatedAt: time.Now()}}}
			},
			expectedChange: func(t *models.Task) bool {
				return len(t.Comments) == 1
			},
			shouldModify: false,
		},
		{
			name:   "add comment when last comment differs",
			action: RuleAction{ActionType: ActionAddComment, Value: "New comment"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Comments: []*models.Comment{{Text: "[Workflow] Old comment", CreatedAt: time.Now()}}}
			},
			expectedChange: func(t *models.Task) bool {
				return len(t.Comments) == 2 && t.Comments[1].Text == "[Workflow] New comment"
			},
			shouldModify: true,
		},
		{
			name:   "set due relative idempotent when due date exists",
			action: RuleAction{ActionType: ActionSetDueRelative, Value: "+3d"},
			setupTask: func() *models.Task {
				due := time.Now().Add(48 * time.Hour)
				return &models.Task{ID: "1", DueDate: &due}
			},
			expectedChange: func(t *models.Task) bool {
				return t.DueDate != nil
			},
			shouldModify: false,
		},
		{
			name:   "set type to bug",
			action: RuleAction{ActionType: ActionSetType, Value: "bug"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Type: models.TaskTypeTask}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Type == models.TaskTypeBug
			},
			shouldModify: true,
		},
		{
			name:   "set type no change",
			action: RuleAction{ActionType: ActionSetType, Value: "task"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Type: models.TaskTypeTask}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Type == models.TaskTypeTask
			},
			shouldModify: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tt.setupTask()
			modified := applyAction(task, tt.action)
			if modified != tt.shouldModify {
				t.Errorf("applyAction() modified = %v, want %v", modified, tt.shouldModify)
			}
			if !tt.expectedChange(task) {
				t.Errorf("applyAction() did not produce expected change")
			}
		})
	}
}

func TestApplyAction_UpdatesTimestamp(t *testing.T) {
	tests := []struct {
		name   string
		action RuleAction
		setup  func() *models.Task
	}{
		{
			name:   "set priority updates UpdatedAt",
			action: RuleAction{ActionType: ActionSetPriority, Value: "1"},
			setup: func() *models.Task {
				stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
				return &models.Task{ID: "1", Priority: 3, UpdatedAt: stale}
			},
		},
		{
			name:   "set section updates UpdatedAt",
			action: RuleAction{ActionType: ActionSetSection, Value: "Inbox"},
			setup: func() *models.Task {
				stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
				return &models.Task{ID: "1", WorkflowState: models.StateReady, UpdatedAt: stale}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tt.setup()
			before := task.UpdatedAt
			modified := applyAction(task, tt.action)
			if !modified {
				t.Fatal("expected action to modify the task")
			}
			if !task.UpdatedAt.After(before) {
				t.Errorf("UpdatedAt was not advanced: before=%v, after=%v", before, task.UpdatedAt)
			}
		})
	}
}
