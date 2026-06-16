package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSortRulesByPriority(t *testing.T) {
	rules := []WorkflowRule{
		{ID: "a", Name: "Zero priority", Priority: 0},
		{ID: "b", Name: "Priority 3", Priority: 3},
		{ID: "c", Name: "Priority 1", Priority: 1},
		{ID: "d", Name: "Another zero", Priority: 0},
		{ID: "e", Name: "Priority 2", Priority: 2},
	}

	sorted := sortRulesByPriority(rules)

	expected := []string{"c", "e", "b", "a", "d"}
	for i, id := range expected {
		if sorted[i].ID != id {
			t.Errorf("position %d: expected ID %s, got %s (name=%s, priority=%d)",
				i, id, sorted[i].ID, sorted[i].Name, sorted[i].Priority)
		}
	}

	// Original slice should be unchanged
	if rules[0].ID != "a" {
		t.Error("original slice was mutated")
	}
}

func TestEvaluateRules_PriorityOrdering(t *testing.T) {
	// Two rules that don't conflict (different fields), but we verify execution order
	// by using chaining: rule with priority 1 sets status, rule with priority 5 checks status
	tasks := []*models.Task{
		{ID: "1", Priority: 1, Status: models.StatusTodo, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:       "rule-low",
			Name:     "Low Priority Rule",
			Enabled:  true,
			Priority: 5,
			Trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
			Actions:  []RuleAction{{ActionType: ActionAddTag, Value: "started"}},
		},
		{
			ID:       "rule-high",
			Name:     "High Priority Rule",
			Enabled:  true,
			Priority: 1,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions:  []RuleAction{{ActionType: ActionSetStatus, Value: "in_progress"}},
		},
	}

	result := EvaluateRules(tasks, rules)

	// High priority rule runs first: sets status to in_progress
	if tasks[0].Status != models.StatusInProgress {
		t.Errorf("expected status in_progress, got %s", tasks[0].Status)
	}
	// Low priority rule runs second (via chaining): adds tag "started"
	if !tasks[0].HasTag("started") {
		t.Error("expected 'started' tag from chained low-priority rule")
	}
	if len(result.ModifiedTasks) == 0 {
		t.Error("expected at least one modified task")
	}
}

func TestEvaluateRules_PriorityConflictResolution(t *testing.T) {
	// Two rules setting the same field (section) with different priorities
	// Highest priority (lowest number) should win
	tasks := []*models.Task{
		{ID: "1", Title: "Test Task", Priority: 1, Section: models.SectionNext},
	}

	rules := []WorkflowRule{
		{
			ID:       "rule-low",
			Name:     "Low Priority Move",
			Enabled:  true,
			Priority: 5,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions:  []RuleAction{{ActionType: ActionSetSection, Value: "Inbox"}},
		},
		{
			ID:       "rule-high",
			Name:     "High Priority Move",
			Enabled:  true,
			Priority: 1,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions:  []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
	}

	result := EvaluateRules(tasks, rules)

	// High priority rule wins: section set to "" (Active)
	if tasks[0].Section != "" {
		t.Errorf("expected section to be empty (Active), got %q", tasks[0].Section)
	}
	// No conflicts should be reported (clear winner)
	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts (priority resolved it), got %d", len(result.Conflicts))
	}
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
}

func TestEvaluateRules_SamePriorityConflict(t *testing.T) {
	// Two rules with the same priority setting the same field should still conflict
	tasks := []*models.Task{
		{ID: "1", Title: "Test Task", Priority: 1, Section: models.SectionNext},
	}

	rules := []WorkflowRule{
		{
			ID:       "rule-a",
			Name:     "Move to Active",
			Enabled:  true,
			Priority: 3,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions:  []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
		{
			ID:       "rule-b",
			Name:     "Move to Inbox",
			Enabled:  true,
			Priority: 3,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions:  []RuleAction{{ActionType: ActionSetSection, Value: "Inbox"}},
		},
	}

	result := EvaluateRules(tasks, rules)

	// Same-priority conflict: section should be unchanged
	if tasks[0].Section != models.SectionNext {
		t.Errorf("expected section unchanged due to same-priority conflict, got %q", tasks[0].Section)
	}
	if len(result.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(result.Conflicts))
	}
}

func TestEvaluateRules_ZeroPriorityConflict(t *testing.T) {
	// Two rules with zero priority (unset) should conflict just like the old behavior
	tasks := []*models.Task{
		{ID: "1", Title: "Test Task", Priority: 1, Section: models.SectionNext},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-a",
			Name:    "Move to Active",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
		{
			ID:      "rule-b",
			Name:    "Move to Inbox",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: "Inbox"}},
		},
	}

	result := EvaluateRules(tasks, rules)

	// Zero-priority conflict (backward compat): section unchanged
	if tasks[0].Section != models.SectionNext {
		t.Errorf("expected section unchanged due to conflict, got %q", tasks[0].Section)
	}
	if len(result.Conflicts) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(result.Conflicts))
	}
}

func TestEvaluateRules_PriorityNonConflictingActions(t *testing.T) {
	// A lower-priority rule should still apply its non-conflicting actions
	// even when a higher-priority rule wins for a specific field
	tasks := []*models.Task{
		{ID: "1", Title: "Test Task", Priority: 1, Section: models.SectionNext, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:       "rule-high",
			Name:     "High Priority",
			Enabled:  true,
			Priority: 1,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions:  []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
		{
			ID:       "rule-low",
			Name:     "Low Priority",
			Enabled:  true,
			Priority: 5,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{
				{ActionType: ActionSetSection, Value: "Inbox"},    // loses to high priority
				{ActionType: ActionAddTag, Value: "needs-review"}, // no conflict, should apply
			},
		},
	}

	result := EvaluateRules(tasks, rules)

	// High priority wins for section
	if tasks[0].Section != "" {
		t.Errorf("expected section to be empty (Active), got %q", tasks[0].Section)
	}
	// Low priority's non-conflicting action should still apply
	if !tasks[0].HasTag("needs-review") {
		t.Error("expected 'needs-review' tag from low-priority rule's non-conflicting action")
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts (priority resolved it), got %d", len(result.Conflicts))
	}
}

func TestEvaluateRules_BackwardCompatNoPriority(t *testing.T) {
	// Rules without Priority field should work exactly as before
	tasks := []*models.Task{
		{ID: "1", Title: "High priority task", Priority: 1, Section: models.SectionNext, Status: models.StatusTodo},
		{ID: "2", Title: "Normal task", Priority: 3, Section: models.SectionNext, Status: models.StatusTodo},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "High Priority to Active",
			Enabled: true,
			// No Priority field set (zero value)
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
}

func TestEvaluateRules_PriorityWithMixedZeroAndNonZero(t *testing.T) {
	// Non-zero priority rule should win over zero-priority rule for same field
	tasks := []*models.Task{
		{ID: "1", Title: "Test Task", Priority: 1, Section: models.SectionNext},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-zero",
			Name:    "Zero Priority Move",
			Enabled: true,
			// Priority: 0 (unset, sorts last)
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetSection, Value: "Inbox"}},
		},
		{
			ID:       "rule-set",
			Name:     "Set Priority Move",
			Enabled:  true,
			Priority: 5,
			Trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions:  []RuleAction{{ActionType: ActionSetSection, Value: ""}},
		},
	}

	result := EvaluateRules(tasks, rules)

	// Priority 5 wins over priority 0 (unset)
	if tasks[0].Section != "" {
		t.Errorf("expected section to be empty (Active), got %q", tasks[0].Section)
	}
	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(result.Conflicts))
	}
}

func TestRuleSummary_WithPriority(t *testing.T) {
	rule := WorkflowRule{
		Name:     "Test Rule",
		Priority: 3,
		Trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
		Actions:  []RuleAction{{ActionType: ActionAddTag, Value: "new"}},
	}

	summary := rule.Summary()
	if summary != "[P3] When status = todo: add tag #new" {
		t.Errorf("unexpected summary: %q", summary)
	}

	// Without priority
	rule.Priority = 0
	summary = rule.Summary()
	if summary != "When status = todo: add tag #new" {
		t.Errorf("unexpected summary without priority: %q", summary)
	}
}
