package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesTrigger_Status(t *testing.T) {
	task := &models.Task{
		ID:     "task-1",
		Status: models.StatusBlocked,
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "status equals match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
			expected: true,
		},
		{
			name:     "status equals no match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			expected: false,
		},
		{
			name:     "status not equals match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpNotEquals, Value: "todo"},
			expected: true,
		},
		{
			name:     "status not equals no match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpNotEquals, Value: "blocked"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_Priority(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Priority: 1,
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "priority equals match",
			trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			expected: true,
		},
		{
			name:     "priority greater than match",
			trigger:  RuleTrigger{Field: FieldPriority, Operation: OpLessThan, Value: "3"},
			expected: true,
		},
		{
			name:     "priority greater than no match",
			trigger:  RuleTrigger{Field: FieldPriority, Operation: OpGreaterThan, Value: "1"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_Tags(t *testing.T) {
	task := &models.Task{
		ID:   "task-1",
		Tags: []string{"backend", "urgent"},
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "tag contains match",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpContains, Value: "backend"},
			expected: true,
		},
		{
			name:     "tag contains no match",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpContains, Value: "frontend"},
			expected: false,
		},
		{
			name:     "tag not equals match (tag not present)",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpNotEquals, Value: "frontend"},
			expected: true,
		},
		{
			name:     "tag not equals no match (tag present)",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpNotEquals, Value: "backend"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

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

func TestEvaluateRules_Basic(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Title: "High priority task", Priority: 1, Section: "Backlog", Status: models.StatusTodo},
		{ID: "2", Title: "Normal task", Priority: 3, Section: "Backlog", Status: models.StatusTodo},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "High Priority to Active",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
	}

	result := EvaluateRules(tasks, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if tasks[0].Section != "" {
		t.Errorf("expected task section to be empty (Active), got %q", tasks[0].Section)
	}
	if tasks[1].Section != "Backlog" {
		t.Errorf("expected second task section unchanged, got %q", tasks[1].Section)
	}
}

func TestEvaluateRules_DisabledRule(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Priority: 1, Section: "Backlog"},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Disabled Rule",
			Enabled: false,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
	}

	result := EvaluateRules(tasks, rules)

	if len(result.ModifiedTasks) != 0 {
		t.Errorf("expected no modified tasks for disabled rule, got %d", len(result.ModifiedTasks))
	}
	if tasks[0].Section != "Backlog" {
		t.Errorf("expected section unchanged for disabled rule")
	}
}

func TestEvaluateRules_Conflict(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Title: "Test Task", Priority: 1, Section: "Backlog"},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Move to Active",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
		{
			ID:      "rule-2",
			Name:    "Move to Inbox",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: "Inbox"}},
		},
	}

	result := EvaluateRules(tasks, rules)

	if len(result.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(result.Conflicts))
	}
	if len(result.ModifiedTasks) != 0 {
		t.Errorf("expected no modified tasks when conflict exists, got %d", len(result.ModifiedTasks))
	}
	if tasks[0].Section != "Backlog" {
		t.Errorf("expected section unchanged when conflict exists")
	}
}

func TestEvaluateRules_LoopPrevention(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Priority: 1, Status: models.StatusTodo, Section: "Backlog"},
	}

	// These rules would loop if not prevented:
	// Rule 1: priority=1 -> set status to in_progress
	// Rule 2: status=in_progress -> set priority to 1
	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Priority 1 starts",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetStatus, Value: "in_progress"}},
		},
		{
			ID:      "rule-2",
			Name:    "In progress gets priority 1",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
			Actions: []RuleAction{{ActionType: ActionSetPriority, Value: "1"}},
		},
	}

	// This should complete without infinite loop
	result := EvaluateRules(tasks, rules)

	// Rule 1 should apply (priority=1 matched)
	if tasks[0].Status != models.StatusInProgress {
		t.Errorf("expected status to be in_progress, got %s", tasks[0].Status)
	}
	// Rule 2 would match but priority is already 1, so no change
	if tasks[0].Priority != 1 {
		t.Errorf("expected priority to remain 1, got %d", tasks[0].Priority)
	}
	// Should not have gotten stuck in loop
	if len(result.ModifiedTasks) == 0 {
		t.Error("expected at least one modified task")
	}
}

func TestEvaluateRules_ChainedRules(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Priority: 1, Status: models.StatusTodo, Section: "Backlog", Tags: []string{}},
	}

	// Chained rules:
	// Rule 1: priority=1 -> set section to "" (Active)
	// Rule 2: section="" -> add tag "active"
	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Priority 1 to Active",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
		{
			ID:      "rule-2",
			Name:    "Active gets tag",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "active"}},
		},
	}

	result := EvaluateRules(tasks, rules)

	if tasks[0].Section != "" {
		t.Errorf("expected section to be empty, got %q", tasks[0].Section)
	}
	if !tasks[0].HasTag("active") {
		t.Errorf("expected task to have 'active' tag")
	}
	if len(result.AppliedRules["1"]) != 2 {
		t.Errorf("expected 2 rules applied, got %d", len(result.AppliedRules["1"]))
	}
}

func TestRuleValidation(t *testing.T) {
	tests := []struct {
		name    string
		rule    WorkflowRule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test Rule",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
			},
			wantErr: true,
		},
		{
			name: "invalid trigger field",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test",
				Enabled: true,
				Trigger: RuleTrigger{Field: "invalid", Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
			},
			wantErr: true,
		},
		{
			name: "no actions",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{},
			},
			wantErr: true,
		},
		{
			name: "invalid action type",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: "invalid", Value: "x"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTestRule(t *testing.T) {
	task := &models.Task{
		ID:       "1",
		Priority: 1,
		Section:  "Backlog",
	}

	rule := WorkflowRule{
		ID:      "rule-1",
		Name:    "Test",
		Enabled: true,
		Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		Actions: []RuleAction{
			{ActionType: ActionSetSection, Value: ""},
			{ActionType: ActionAddTag, Value: "urgent"},
		},
	}

	matches, actions := TestRule(task, rule)
	if !matches {
		t.Error("expected rule to match")
	}
	if len(actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(actions))
	}

	// Task should not be modified by TestRule
	if task.Section != "Backlog" {
		t.Error("TestRule should not modify task")
	}
}
