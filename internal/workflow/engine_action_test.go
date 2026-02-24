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
			name:   "add comment idempotent when last comment matches",
			action: RuleAction{ActionType: ActionAddComment, Value: "Test comment"},
			setupTask: func() *models.Task {
				return &models.Task{ID: "1", Comments: []*models.Comment{
					{Text: "[Workflow] Test comment", CreatedAt: time.Now()},
				}}
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
				return &models.Task{ID: "1", Comments: []*models.Comment{
					{Text: "[Workflow] Old comment", CreatedAt: time.Now()},
				}}
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

func TestApplyAction_CreateFollowup(t *testing.T) {
	task := &models.Task{ID: "original-123", Title: "Original task", RelatedTo: []string{}}
	result := &EvaluationResult{
		AppliedRules:      make(map[string][]string),
		RuleTriggerCounts: make(map[string]int),
	}
	action := RuleAction{ActionType: ActionCreateFollowup, Value: "Follow up on original"}

	modified := applyCreateFollowup(task, action, result)

	if !modified {
		t.Error("expected applyCreateFollowup to return true")
	}
	if len(result.NewFollowupTasks) != 1 {
		t.Fatalf("expected 1 follow-up task, got %d", len(result.NewFollowupTasks))
	}

	followup := result.NewFollowupTasks[0]
	if followup.Title != "Follow up on original" {
		t.Errorf("expected title %q, got %q", "Follow up on original", followup.Title)
	}
	if followup.Status != models.StatusTodo {
		t.Errorf("expected status todo, got %s", followup.Status)
	}
	if followup.Priority != models.PriorityDefault {
		t.Errorf("expected priority %d, got %d", models.PriorityDefault, followup.Priority)
	}
	if !followup.HasTag("followup") {
		t.Error("expected follow-up task to have 'followup' tag")
	}

	// Check bidirectional RelatedTo
	foundOriginalInFollowup := false
	for _, id := range followup.RelatedTo {
		if id == task.ID {
			foundOriginalInFollowup = true
		}
	}
	if !foundOriginalInFollowup {
		t.Error("expected follow-up to have RelatedTo pointing to original task")
	}

	foundFollowupInOriginal := false
	for _, id := range task.RelatedTo {
		if id == followup.ID {
			foundFollowupInOriginal = true
		}
	}
	if !foundFollowupInOriginal {
		t.Error("expected original task to have RelatedTo pointing to follow-up")
	}
}

func TestApplyAction_MoveToProject(t *testing.T) {
	task := &models.Task{ID: "task-1", Title: "Test task", Section: "Backlog"}
	result := &EvaluationResult{
		AppliedRules:      make(map[string][]string),
		RuleTriggerCounts: make(map[string]int),
	}
	action := RuleAction{ActionType: ActionMoveToProject, Value: "DestProject"}

	modified := applyMoveToProject(task, action, result)

	if !modified {
		t.Error("expected applyMoveToProject to return true")
	}
	if len(result.DeferredMoves) != 1 {
		t.Fatalf("expected 1 deferred move, got %d", len(result.DeferredMoves))
	}
	move := result.DeferredMoves[0]
	if move.TaskID != "task-1" {
		t.Errorf("expected TaskID %q, got %q", "task-1", move.TaskID)
	}
	if move.Destination != "DestProject" {
		t.Errorf("expected Destination %q, got %q", "DestProject", move.Destination)
	}
	// Task fields should not be modified
	if task.Section != "Backlog" {
		t.Error("expected task section to remain unchanged")
	}
}
