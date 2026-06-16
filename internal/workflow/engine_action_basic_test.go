package workflow

import (
	"testing"

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
				return &models.Task{ID: "1", Section: models.SectionNext}
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
