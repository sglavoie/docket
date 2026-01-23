package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

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
