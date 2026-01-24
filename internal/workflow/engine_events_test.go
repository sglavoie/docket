package workflow

import (
	"testing"

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
