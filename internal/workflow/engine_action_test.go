package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestApplyAction(t *testing.T) {
	tests := []struct {
		name           string
		action         RuleAction
		setupTask      func() *models.Task
		expectedChange func(*models.Task) bool
		shouldModify   bool
	}{
		{
			name:   "set status",
			action: RuleAction{ActionType: ActionSetStatus, Value: "in_progress"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Status: models.StatusTodo}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Status == models.StatusInProgress
			},
			shouldModify: true,
		},
		{
			name:   "set status no change",
			action: RuleAction{ActionType: ActionSetStatus, Value: "todo"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Status: models.StatusTodo}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Status == models.StatusTodo
			},
			shouldModify: false,
		},
		{
			name:   "set priority",
			action: RuleAction{ActionType: ActionSetPriority, Value: "1"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Priority: 3}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Priority == 1
			},
			shouldModify: true,
		},
		{
			name:   "set section",
			action: RuleAction{ActionType: ActionSetSection, Value: ""},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Section: "Backlog"}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Section == ""
			},
			shouldModify: true,
		},
		{
			name:   "add tag",
			action: RuleAction{ActionType: ActionAddTag, Value: "urgent"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Tags: []string{}}
			},
			expectedChange: func(t *models.Task) bool {
				return len(t.Tags) == 1 && t.Tags[0] == "urgent"
			},
			shouldModify: true,
		},
		{
			name:   "add tag already exists",
			action: RuleAction{ActionType: ActionAddTag, Value: "urgent"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Tags: []string{"urgent"}}
			},
			expectedChange: func(t *models.Task) bool {
				return len(t.Tags) == 1
			},
			shouldModify: false,
		},
		{
			name:   "remove tag",
			action: RuleAction{ActionType: ActionRemoveTag, Value: "urgent"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Tags: []string{"urgent", "backend"}}
			},
			expectedChange: func(t *models.Task) bool {
				return len(t.Tags) == 1 && t.Tags[0] == "backend"
			},
			shouldModify: true,
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
				return &models.Task{ID: "1", Status: models.StatusTodo}
			},
			expectedChange: func(t *models.Task) bool {
				if t.DueDate == nil {
					return false
				}
				// Should be approximately 3 days from now
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
			name:   "set active true",
			action: RuleAction{ActionType: ActionSetActive, Value: "true"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Active: false}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Active == true
			},
			shouldModify: true,
		},
		{
			name:   "set active false",
			action: RuleAction{ActionType: ActionSetActive, Value: "false"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Active: true}
			},
			expectedChange: func(t *models.Task) bool {
				return t.Active == false
			},
			shouldModify: true,
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
