package workflow

import (
	"testing"
	"time"

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

// Tests for new action types

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

// Tests for compound triggers

func TestMatchesRule_CompoundAND(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Status:   models.StatusBlocked,
		Priority: 1,
	}

	rule := WorkflowRule{
		ID:      "rule-1",
		Name:    "Compound AND",
		Enabled: true,
		Triggers: []RuleTrigger{
			{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
			{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		},
		TriggerLogic: LogicAnd,
		Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "urgent"}},
	}

	if !matchesRule(task, &rule) {
		t.Error("expected compound AND rule to match when all conditions met")
	}

	// Change one condition to not match
	task.Priority = 3
	if matchesRule(task, &rule) {
		t.Error("expected compound AND rule to not match when one condition fails")
	}
}

func TestMatchesRule_CompoundOR(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Status:   models.StatusTodo,
		Priority: 1,
	}

	rule := WorkflowRule{
		ID:      "rule-1",
		Name:    "Compound OR",
		Enabled: true,
		Triggers: []RuleTrigger{
			{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
			{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		},
		TriggerLogic: LogicOr,
		Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "important"}},
	}

	// Only priority matches, but OR should still succeed
	if !matchesRule(task, &rule) {
		t.Error("expected compound OR rule to match when one condition met")
	}

	// Neither matches
	task.Priority = 3
	if matchesRule(task, &rule) {
		t.Error("expected compound OR rule to not match when no conditions met")
	}
}

func TestMatchesRule_LegacySingleTrigger(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Priority: 1,
	}

	// Legacy rule with single Trigger field (not Triggers slice)
	rule := WorkflowRule{
		ID:      "rule-1",
		Name:    "Legacy",
		Enabled: true,
		Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		Actions: []RuleAction{{ActionType: ActionAddTag, Value: "test"}},
	}

	if !matchesRule(task, &rule) {
		t.Error("expected legacy single trigger rule to match")
	}
}

func TestEvaluateRules_CompoundTriggers(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Title: "Urgent blocked", Priority: 1, Status: models.StatusBlocked, Tags: []string{}},
		{ID: "2", Title: "Normal blocked", Priority: 3, Status: models.StatusBlocked, Tags: []string{}},
		{ID: "3", Title: "Urgent todo", Priority: 1, Status: models.StatusTodo, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Urgent AND Blocked",
			Enabled: true,
			Triggers: []RuleTrigger{
				{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
				{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "critical"}},
		},
	}

	result := EvaluateRules(tasks, rules)

	// Only task 1 should match (both blocked AND priority 1)
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !tasks[0].HasTag("critical") {
		t.Error("expected task 1 to have 'critical' tag")
	}
	if tasks[1].HasTag("critical") {
		t.Error("expected task 2 to NOT have 'critical' tag")
	}
	if tasks[2].HasTag("critical") {
		t.Error("expected task 3 to NOT have 'critical' tag")
	}
}

// Tests for time-based triggers

func TestMatchesTrigger_DueDays(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		dueDate  *time.Time
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "due in 2 days, less than 3",
			dueDate:  timePtr(now.AddDate(0, 0, 2)),
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
			expected: true,
		},
		{
			name:     "due in 5 days, not less than 3",
			dueDate:  timePtr(now.AddDate(0, 0, 5)),
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
			expected: false,
		},
		{
			name:     "overdue by 2 days (negative)",
			dueDate:  timePtr(now.AddDate(0, 0, -2)),
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "0"},
			expected: true,
		},
		{
			name:     "no due date",
			dueDate:  nil,
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:      "task-1",
				DueDate: tt.dueDate,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_CreatedDays(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		trigger   RuleTrigger
		expected  bool
	}{
		{
			name:      "created 10 days ago, greater than 7",
			createdAt: now.AddDate(0, 0, -10),
			trigger:   RuleTrigger{Field: FieldCreatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  true,
		},
		{
			name:      "created 3 days ago, not greater than 7",
			createdAt: now.AddDate(0, 0, -3),
			trigger:   RuleTrigger{Field: FieldCreatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  false,
		},
		{
			name:      "created today, equals 0",
			createdAt: now,
			trigger:   RuleTrigger{Field: FieldCreatedDays, Operation: OpEquals, Value: "0"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:        "task-1",
				CreatedAt: tt.createdAt,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_UpdatedDays(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		updatedAt time.Time
		trigger   RuleTrigger
		expected  bool
	}{
		{
			name:      "updated 10 days ago, greater than 7 (stale)",
			updatedAt: now.AddDate(0, 0, -10),
			trigger:   RuleTrigger{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  true,
		},
		{
			name:      "updated 3 days ago, not greater than 7",
			updatedAt: now.AddDate(0, 0, -3),
			trigger:   RuleTrigger{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:        "task-1",
				UpdatedAt: tt.updatedAt,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEvaluateRules_TimeBased(t *testing.T) {
	now := time.Now()

	tasks := []*models.Task{
		{ID: "1", Title: "Due soon", DueDate: timePtr(now.AddDate(0, 0, 2)), Priority: 3, Tags: []string{}},
		{ID: "2", Title: "Due later", DueDate: timePtr(now.AddDate(0, 0, 10)), Priority: 3, Tags: []string{}},
		{ID: "3", Title: "No due date", DueDate: nil, Priority: 3, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Due Soon Priority",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
			Actions: []RuleAction{{ActionType: ActionSetPriority, Value: "1"}},
		},
	}

	result := EvaluateRules(tasks, rules)

	// Only task 1 should be modified (due in 2 days < 3)
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if tasks[0].Priority != 1 {
		t.Errorf("expected task 1 priority to be 1, got %d", tasks[0].Priority)
	}
	if tasks[1].Priority != 3 {
		t.Errorf("expected task 2 priority to remain 3, got %d", tasks[1].Priority)
	}
	if tasks[2].Priority != 3 {
		t.Errorf("expected task 3 priority to remain 3, got %d", tasks[2].Priority)
	}
}

// Tests for ParseRelativeDuration

func TestParseRelativeDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"+3d", 3, false},
		{"3d", 3, false},
		{"+1w", 7, false},
		{"+2w", 14, false},
		{"+1m", 30, false},
		{"+1y", 365, false},
		{"+10d", 10, false},
		{"", 0, true},
		{"d", 0, true},
		{"+0d", 0, true},
		{"+3x", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRelativeDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRelativeDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseRelativeDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// Tests for rule validation with new features

func TestRuleValidation_NewActions(t *testing.T) {
	tests := []struct {
		name    string
		rule    WorkflowRule
		wantErr bool
	}{
		{
			name: "valid set_due_relative",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetDueRelative, Value: "+3d"}},
			},
			wantErr: false,
		},
		{
			name: "invalid set_due_relative format",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetDueRelative, Value: "invalid"}},
			},
			wantErr: true,
		},
		{
			name: "valid clear_due (no value)",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionClearDue, Value: ""}},
			},
			wantErr: false,
		},
		{
			name: "valid set_active true",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetActive, Value: "true"}},
			},
			wantErr: false,
		},
		{
			name: "invalid set_active value",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetActive, Value: "yes"}},
			},
			wantErr: true,
		},
		{
			name: "valid compound rule",
			rule: WorkflowRule{
				Name: "Test",
				Triggers: []RuleTrigger{
					{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
					{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				},
				TriggerLogic: LogicAnd,
				Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "urgent"}},
			},
			wantErr: false,
		},
		{
			name: "valid time-based trigger",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
				Actions: []RuleAction{{ActionType: ActionSetPriority, Value: "1"}},
			},
			wantErr: false,
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

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}

// Tests for event-based triggers

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
			name:       "in_progress to cancelled fires completed",
			prevStatus: models.StatusInProgress,
			currStatus: models.StatusCancelled,
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
			name:       "cancelled to in_progress fires reopened",
			prevStatus: models.StatusCancelled,
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

// Tests for hierarchical triggers

func TestMatchesHierarchicalTrigger_ParentStatus(t *testing.T) {
	parent := &models.Task{ID: "parent", Status: models.StatusInProgress, Type: models.TaskTypeEpic}
	child := &models.Task{ID: "child", Status: models.StatusTodo, Parent: parent}

	evalCtx := &EvaluationContext{
		AllTasks: []*models.Task{parent, child},
		TaskMap:  map[string]*models.Task{"parent": parent, "child": child},
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "parent status matches",
			trigger:  RuleTrigger{Field: FieldParentStatus, Operation: OpEquals, Value: "in_progress"},
			expected: true,
		},
		{
			name:     "parent status does not match",
			trigger:  RuleTrigger{Field: FieldParentStatus, Operation: OpEquals, Value: "done"},
			expected: false,
		},
		{
			name:     "parent type matches",
			trigger:  RuleTrigger{Field: FieldParentType, Operation: OpEquals, Value: "epic"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesHierarchicalTrigger(child, tt.trigger, evalCtx); got != tt.expected {
				t.Errorf("matchesHierarchicalTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesHierarchicalTrigger_AllChildrenDone(t *testing.T) {
	tests := []struct {
		name          string
		childStatuses []models.TaskStatus
		expectedMatch bool
	}{
		{
			name:          "all children done",
			childStatuses: []models.TaskStatus{models.StatusDone, models.StatusDone},
			expectedMatch: true,
		},
		{
			name:          "some children incomplete",
			childStatuses: []models.TaskStatus{models.StatusDone, models.StatusTodo},
			expectedMatch: false,
		},
		{
			name:          "no children",
			childStatuses: []models.TaskStatus{},
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := &models.Task{ID: "parent", Type: models.TaskTypeEpic, Children: []*models.Task{}}
			for i, status := range tt.childStatuses {
				child := &models.Task{
					ID:     "child" + string(rune('0'+i)),
					Status: status,
					Type:   models.TaskTypeTask,
					Parent: parent,
				}
				parent.Children = append(parent.Children, child)
			}

			evalCtx := &EvaluationContext{
				AllTasks: append([]*models.Task{parent}, parent.Children...),
			}

			trigger := RuleTrigger{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"}
			if got := matchesHierarchicalTrigger(parent, trigger, evalCtx); got != tt.expectedMatch {
				t.Errorf("matchesHierarchicalTrigger() = %v, want %v", got, tt.expectedMatch)
			}
		})
	}
}

func TestMatchesHierarchicalTrigger_IsBlocked(t *testing.T) {
	blocker1 := &models.Task{ID: "blocker1", Status: models.StatusTodo}
	blocker2 := &models.Task{ID: "blocker2", Status: models.StatusDone}
	task := &models.Task{ID: "task", DependsOn: []string{"blocker1", "blocker2"}}

	evalCtx := &EvaluationContext{
		TaskMap: map[string]*models.Task{
			"blocker1": blocker1,
			"blocker2": blocker2,
			"task":     task,
		},
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "is_blocked true (has incomplete blocker)",
			trigger:  RuleTrigger{Field: FieldIsBlocked, Operation: OpEquals, Value: "true"},
			expected: true,
		},
		{
			name:     "blocker_count greater than 0",
			trigger:  RuleTrigger{Field: FieldBlockerCount, Operation: OpGreaterThan, Value: "0"},
			expected: true,
		},
		{
			name:     "blocker_count equals 1",
			trigger:  RuleTrigger{Field: FieldBlockerCount, Operation: OpEquals, Value: "1"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesHierarchicalTrigger(task, tt.trigger, evalCtx); got != tt.expected {
				t.Errorf("matchesHierarchicalTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Tests for hierarchical actions

func TestApplyHierarchicalAction_CascadeTag(t *testing.T) {
	parent := &models.Task{ID: "parent", Tags: []string{"urgent"}, Children: []*models.Task{}}
	child1 := &models.Task{ID: "child1", Tags: []string{}, Parent: parent}
	child2 := &models.Task{ID: "child2", Tags: []string{"urgent"}, Parent: parent}
	parent.Children = []*models.Task{child1, child2}

	evalCtx := &EvaluationContext{
		AllTasks: []*models.Task{parent, child1, child2},
		TaskMap:  map[string]*models.Task{"parent": parent, "child1": child1, "child2": child2},
	}

	action := RuleAction{ActionType: ActionCascadeTag, Value: "urgent"}
	modified := applyHierarchicalAction(parent, action, evalCtx)

	if !modified {
		t.Error("expected cascade_tag to modify at least one child")
	}
	if !child1.HasTag("urgent") {
		t.Error("expected child1 to have 'urgent' tag after cascade")
	}
}

func TestApplyHierarchicalAction_InheritTag(t *testing.T) {
	parent := &models.Task{ID: "parent", Tags: []string{"urgent"}}
	child := &models.Task{ID: "child", Tags: []string{}, Parent: parent}

	evalCtx := &EvaluationContext{
		AllTasks: []*models.Task{parent, child},
		TaskMap:  map[string]*models.Task{"parent": parent, "child": child},
	}

	action := RuleAction{ActionType: ActionInheritTag, Value: "urgent"}
	modified := applyHierarchicalAction(child, action, evalCtx)

	if !modified {
		t.Error("expected inherit_tag to modify child")
	}
	if !child.HasTag("urgent") {
		t.Error("expected child to have 'urgent' tag after inherit")
	}
}

func TestApplyHierarchicalAction_NotifyParent(t *testing.T) {
	parent := &models.Task{ID: "parent", Comments: []*models.Comment{}}
	child := &models.Task{ID: "child", Parent: parent}

	evalCtx := &EvaluationContext{
		AllTasks: []*models.Task{parent, child},
		TaskMap:  map[string]*models.Task{"parent": parent, "child": child},
	}

	action := RuleAction{ActionType: ActionNotifyParent, Value: "Child completed"}
	modified := applyHierarchicalAction(child, action, evalCtx)

	if !modified {
		t.Error("expected notify_parent to modify parent")
	}
	if len(parent.Comments) != 1 {
		t.Errorf("expected parent to have 1 comment, got %d", len(parent.Comments))
	}
	if parent.Comments[0].Text != "[Workflow] Child completed" {
		t.Errorf("unexpected comment text: %s", parent.Comments[0].Text)
	}
}

// Tests for EvaluateRulesWithContext

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

func TestEvaluateRulesWithContext_HierarchicalTrigger(t *testing.T) {
	// Epic with two completed children
	child1 := &models.Task{ID: "c1", Status: models.StatusDone, Type: models.TaskTypeTask}
	child2 := &models.Task{ID: "c2", Status: models.StatusDone, Type: models.TaskTypeTask}
	epic := &models.Task{
		ID:       "epic",
		Status:   models.StatusInProgress,
		Type:     models.TaskTypeEpic,
		Children: []*models.Task{child1, child2},
	}
	child1.Parent = epic
	child2.Parent = epic

	evalCtx := &EvaluationContext{
		Changes: []TaskChangeContext{
			{Task: epic, Previous: nil},
			{Task: child1, Previous: nil},
			{Task: child2, Previous: nil},
		},
		AllTasks: []*models.Task{epic, child1, child2},
		TaskMap:  map[string]*models.Task{"epic": epic, "c1": child1, "c2": child2},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Auto Complete Epic",
			Enabled: true,
			Triggers: []RuleTrigger{
				{Field: FieldType, Operation: OpEquals, Value: "epic"},
				{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"},
			},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionSetStatus, Value: "done"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task (epic), got %d", len(result.ModifiedTasks))
	}
	if epic.Status != models.StatusDone {
		t.Errorf("expected epic status to be done, got %s", epic.Status)
	}
}
