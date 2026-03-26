package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesEventTrigger_StatusChanged(t *testing.T) {
	tests := []struct {
		name       string
		prevStatus models.TaskStatus
		currStatus models.TaskStatus
		pattern    string
		expected   bool
	}{
		{
			name:       "blocked to in_progress matches blocked->*",
			prevStatus: models.StatusBlocked,
			currStatus: models.StatusInProgress,
			pattern:    "blocked->*",
			expected:   true,
		},
		{
			name:       "todo to done matches *->done",
			prevStatus: models.StatusTodo,
			currStatus: models.StatusDone,
			pattern:    "*->done",
			expected:   true,
		},
		{
			name:       "blocked to done matches blocked->done",
			prevStatus: models.StatusBlocked,
			currStatus: models.StatusDone,
			pattern:    "blocked->done",
			expected:   true,
		},
		{
			name:       "todo to blocked does not match blocked->*",
			prevStatus: models.StatusTodo,
			currStatus: models.StatusBlocked,
			pattern:    "blocked->*",
			expected:   false,
		},
		{
			name:       "same status does not trigger",
			prevStatus: models.StatusBlocked,
			currStatus: models.StatusBlocked,
			pattern:    "blocked->*",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := &models.Task{ID: "1", Status: tt.prevStatus}
			curr := &models.Task{ID: "1", Status: tt.currStatus}
			trigger := RuleTrigger{Field: FieldStatusChanged, Operation: OpEquals, Value: tt.pattern}

			if got := matchesEventTrigger(curr, prev, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_Completed(t *testing.T) {
	tests := []struct {
		name       string
		prevStatus models.TaskStatus
		currStatus models.TaskStatus
		expected   bool
	}{
		{
			name:       "todo to done fires completed",
			prevStatus: models.StatusTodo,
			currStatus: models.StatusDone,
			expected:   true,
		},
		{
			name:       "in_progress to canceled fires completed",
			prevStatus: models.StatusInProgress,
			currStatus: models.StatusCanceled,
			expected:   true,
		},
		{
			name:       "done to done does not fire",
			prevStatus: models.StatusDone,
			currStatus: models.StatusDone,
			expected:   false,
		},
		{
			name:       "todo to blocked does not fire",
			prevStatus: models.StatusTodo,
			currStatus: models.StatusBlocked,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := &models.Task{ID: "1", Status: tt.prevStatus}
			curr := &models.Task{ID: "1", Status: tt.currStatus}
			trigger := RuleTrigger{Field: FieldCompleted, Operation: OpEquals, Value: "true"}

			if got := matchesEventTrigger(curr, prev, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_TagAdded(t *testing.T) {
	tests := []struct {
		name     string
		prevTags []string
		currTags []string
		tagName  string
		expected bool
	}{
		{
			name:     "tag added",
			prevTags: []string{},
			currTags: []string{"urgent"},
			tagName:  "urgent",
			expected: true,
		},
		{
			name:     "tag already existed",
			prevTags: []string{"urgent"},
			currTags: []string{"urgent"},
			tagName:  "urgent",
			expected: false,
		},
		{
			name:     "different tag added",
			prevTags: []string{},
			currTags: []string{"backend"},
			tagName:  "urgent",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := &models.Task{ID: "1", Tags: tt.prevTags}
			curr := &models.Task{ID: "1", Tags: tt.currTags}
			trigger := RuleTrigger{Field: FieldTagAdded, Operation: OpEquals, Value: tt.tagName}

			if got := matchesEventTrigger(curr, prev, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_Reopened(t *testing.T) {
	tests := []struct {
		name       string
		prevStatus models.TaskStatus
		currStatus models.TaskStatus
		expected   bool
	}{
		{
			name:       "done to todo fires reopened",
			prevStatus: models.StatusDone,
			currStatus: models.StatusTodo,
			expected:   true,
		},
		{
			name:       "canceled to in_progress fires reopened",
			prevStatus: models.StatusCanceled,
			currStatus: models.StatusInProgress,
			expected:   true,
		},
		{
			name:       "todo to in_progress does not fire",
			prevStatus: models.StatusTodo,
			currStatus: models.StatusInProgress,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := &models.Task{ID: "1", Status: tt.prevStatus}
			curr := &models.Task{ID: "1", Status: tt.currStatus}
			trigger := RuleTrigger{Field: FieldReopened, Operation: OpEquals, Value: "true"}

			if got := matchesEventTrigger(curr, prev, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_DueDateChanged(t *testing.T) {
	now := time.Now()
	later := now.Add(48 * time.Hour)

	tests := []struct {
		name     string
		prevDue  *time.Time
		currDue  *time.Time
		value    string
		expected bool
	}{
		{"nil to set = added", nil, &now, "added", true},
		{"nil to set matches wildcard", nil, &now, "*", true},
		{"nil to set does not match removed", nil, &now, "removed", false},
		{"set to nil = removed", &now, nil, "removed", true},
		{"set to nil matches wildcard", &now, nil, "*", true},
		{"set to nil does not match added", &now, nil, "added", false},
		{"changed date = changed", &now, &later, "changed", true},
		{"changed date matches wildcard", &now, &later, "*", true},
		{"changed date does not match added", &now, &later, "added", false},
		{"same date = no change", &now, &now, "*", false},
		{"both nil = no change", nil, nil, "*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := &models.Task{ID: "1", DueDate: tt.prevDue}
			curr := &models.Task{ID: "1", DueDate: tt.currDue}
			trigger := RuleTrigger{Field: FieldDueDateChanged, Operation: OpEquals, Value: tt.value}

			if got := matchesEventTrigger(curr, prev, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_TypeChanged(t *testing.T) {
	tests := []struct {
		name     string
		prevType models.TaskType
		currType models.TaskType
		pattern  string
		expected bool
	}{
		{"task to bug matches task->bug", models.TaskTypeTask, models.TaskTypeBug, "task->bug", true},
		{"task to bug matches task->*", models.TaskTypeTask, models.TaskTypeBug, "task->*", true},
		{"task to bug matches *->bug", models.TaskTypeTask, models.TaskTypeBug, "*->bug", true},
		{"task to bug does not match bug->task", models.TaskTypeTask, models.TaskTypeBug, "bug->task", false},
		{"same type does not match", models.TaskTypeTask, models.TaskTypeTask, "task->*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := &models.Task{ID: "1", Type: tt.prevType}
			curr := &models.Task{ID: "1", Type: tt.currType}
			trigger := RuleTrigger{Field: FieldTypeChanged, Operation: OpEquals, Value: tt.pattern}

			if got := matchesEventTrigger(curr, prev, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_RecurrenceCompleted(t *testing.T) {
	tests := []struct {
		name     string
		prevTask *models.Task
		currTask *models.Task
		expected bool
	}{
		{
			"fires on done + recurrence cleared",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusDone},
			true,
		},
		{
			"fires on canceled + recurrence cleared",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusCanceled},
			true,
		},
		{
			"does not fire when still recurring",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}, Status: models.StatusDone},
			false,
		},
		{
			"does not fire when not complete",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusInProgress},
			false,
		},
		{
			"does not fire when no previous recurrence",
			&models.Task{ID: "1", Recurrence: nil},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusDone},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := RuleTrigger{Field: FieldRecurrenceCompleted, Operation: OpEquals, Value: "true"}
			if got := matchesEventTrigger(tt.currTask, tt.prevTask, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_StreakBroken(t *testing.T) {
	tests := []struct {
		name     string
		prevTask *models.Task
		currTask *models.Task
		expected bool
	}{
		{
			"streak >0 to 0 fires",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			true,
		},
		{
			"streak >0 to recurrence removed fires",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 3}},
			&models.Task{ID: "1", Recurrence: nil},
			true,
		},
		{
			"streak unchanged does not fire",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}},
			false,
		},
		{
			"no previous recurrence does not fire",
			&models.Task{ID: "1", Recurrence: nil},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			false,
		},
		{
			"streak was already 0 does not fire",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := RuleTrigger{Field: FieldStreakBroken, Operation: OpEquals, Value: "true"}
			if got := matchesEventTrigger(tt.currTask, tt.prevTask, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEvaluateRulesWithContext_EventTrigger(t *testing.T) {
	// Create previous state with blocked task
	prevTask := &models.Task{ID: "1", Status: models.StatusBlocked, Tags: []string{}}
	// Current state with unblocked task
	currTask := &models.Task{ID: "1", Status: models.StatusInProgress, Tags: []string{}}

	evalCtx := &EvaluationContext{
		Changes: []TaskChangeContext{
			{Task: currTask, Previous: prevTask},
		},
		AllTasks: []*models.Task{currTask},
		TaskMap:  map[string]*models.Task{"1": currTask},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Unblock Tag",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatusChanged, Operation: OpEquals, Value: "blocked->*"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "ready"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !currTask.HasTag("ready") {
		t.Error("expected task to have 'ready' tag from event trigger")
	}
}

func TestEventTrigger_CustomSectionChanged(t *testing.T) {
	prevTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusInProgress,
		CustomSection: "Sprint-1",
	}
	currTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusInProgress,
		CustomSection: "Sprint-2",
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: currTask, Previous: prevTask}},
		AllTasks: []*models.Task{currTask},
		TaskMap:  map[string]*models.Task{"task-1": currTask},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Custom section change",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldSectionChanged, Operation: OpEquals, Value: "Sprint-1->Sprint-2"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "moved"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Fatalf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !currTask.HasTag("moved") {
		t.Error("expected task to have 'moved' tag from custom section change trigger")
	}
}

func TestEventTrigger_CustomSectionChanged_WildcardPattern(t *testing.T) {
	prevTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusTodo,
		CustomSection: "Sprint-1",
	}
	currTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusTodo,
		CustomSection: "Sprint-2",
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: currTask, Previous: prevTask}},
		AllTasks: []*models.Task{currTask},
		TaskMap:  map[string]*models.Task{"task-1": currTask},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Any custom section change",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldSectionChanged, Operation: OpEquals, Value: "*->*"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "section-changed"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Fatalf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !currTask.HasTag("section-changed") {
		t.Error("expected task to have 'section-changed' tag from wildcard section change")
	}
}
