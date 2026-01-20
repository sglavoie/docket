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
