package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestEvaluateRules_Basic(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Title: "High priority task", Priority: 1, WorkflowState: models.StateNext},
		{ID: "2", Title: "Normal task", Priority: 3, WorkflowState: models.StateNext},
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
	if models.SectionForState(tasks[0].State()) != "" {
		t.Errorf("expected task section to be empty (Active), got %q", models.SectionForState(tasks[0].State()))
	}
	if models.SectionForState(tasks[1].State()) != models.SectionNext {
		t.Errorf("expected second task section unchanged, got %q", models.SectionForState(tasks[1].State()))
	}
}

func TestEvaluateRules_DisabledRule(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Priority: 1, WorkflowState: models.StateNext},
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
	if models.SectionForState(tasks[0].State()) != models.SectionNext {
		t.Errorf("expected section unchanged for disabled rule")
	}
}

func TestEvaluateRules_Conflict(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Title: "Test Task", Priority: 1, WorkflowState: models.StateNext},
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
	if models.SectionForState(tasks[0].State()) != models.SectionNext {
		t.Errorf("expected section unchanged when conflict exists")
	}
}

func TestEvaluateRules_LoopPrevention(t *testing.T) {
	tasks := []*models.Task{
		{ID: "1", Priority: 1, WorkflowState: models.StateNext},
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
	if tasks[0].State() != models.StateInProgress {
		t.Errorf("expected status to be in_progress, got %s", tasks[0].State())
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
		{ID: "1", Priority: 1, WorkflowState: models.StateNext, Tags: []string{}},
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

	if models.SectionForState(tasks[0].State()) != "" {
		t.Errorf("expected section to be empty, got %q", models.SectionForState(tasks[0].State()))
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
		{ID: "1", Title: "Urgent blocked", Priority: 1, WorkflowState: models.StateWaiting, Tags: []string{}},
		{ID: "2", Title: "Normal blocked", Priority: 3, WorkflowState: models.StateWaiting, Tags: []string{}},
		{ID: "3", Title: "Urgent todo", Priority: 1, WorkflowState: models.StateReady, Tags: []string{}},
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
		ID:            "1",
		Priority:      1,
		WorkflowState: models.StateNext,
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
	if models.SectionForState(task.State()) != models.SectionNext {
		t.Error("TestRule should not modify task")
	}
}

func TestTestRule_TriggerGroup(t *testing.T) {
	task := &models.Task{
		ID:            "1",
		Priority:      1,
		WorkflowState: models.StateInProgress,
		Tags:          []string{},
		Type:          models.TaskTypeTask,
	}

	rule := WorkflowRule{
		ID:      "rule-group",
		Name:    "Group Rule",
		Enabled: true,
		TriggerGroup: &TriggerGroup{
			Logic: LogicAnd,
			Triggers: []RuleTrigger{
				{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
			},
		},
		Actions: []RuleAction{
			{ActionType: ActionAddTag, Value: "urgent"},
		},
	}

	matches, actions := TestRule(task, rule)
	if !matches {
		t.Error("expected TestRule to match rule with TriggerGroup")
	}
	if len(actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(actions))
	}

	// Task should not be modified
	if task.State() != models.StateInProgress {
		t.Error("TestRule should not modify task")
	}
}

func TestEvaluation_ScopedRule(t *testing.T) {
	task := &models.Task{ID: "1", Priority: 1, Tags: []string{}, Type: models.TaskTypeTask}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Scoped to ProjectA",
			Enabled: true,
			Scope:   &RuleScope{Projects: []string{"ProjectA"}},
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "scoped"}},
		},
	}

	// With matching project
	evalCtx := &EvaluationContext{
		Changes:     []TaskChangeContext{{Task: task, Previous: nil}},
		AllTasks:    []*models.Task{task},
		TaskMap:     map[string]*models.Task{"1": task},
		ProjectName: "ProjectA",
		ProjectID:   "pa-1",
	}
	result := EvaluateRulesWithContext(evalCtx, rules)
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task for matching project, got %d", len(result.ModifiedTasks))
	}
	if !task.HasTag("scoped") {
		t.Error("expected 'scoped' tag for matching project")
	}

	// Reset and test with non-matching project
	task2 := &models.Task{ID: "2", Priority: 1, Tags: []string{}, Type: models.TaskTypeTask}
	evalCtx2 := &EvaluationContext{
		Changes:     []TaskChangeContext{{Task: task2, Previous: nil}},
		AllTasks:    []*models.Task{task2},
		TaskMap:     map[string]*models.Task{"2": task2},
		ProjectName: "ProjectB",
		ProjectID:   "pb-1",
	}
	result2 := EvaluateRulesWithContext(evalCtx2, rules)
	if len(result2.ModifiedTasks) != 0 {
		t.Errorf("expected 0 modified tasks for non-matching project, got %d", len(result2.ModifiedTasks))
	}
	if task2.HasTag("scoped") {
		t.Error("expected no 'scoped' tag for non-matching project")
	}
}

func TestEvaluation_BooleanTriggersEndToEnd(t *testing.T) {
	// Task without due date should get tagged via has_due_date=false trigger
	task := &models.Task{ID: "1", DueDate: nil, Tags: []string{}, Type: models.TaskTypeTask}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Tag tasks without due date",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldHasDueDate, Operation: OpEquals, Value: "false"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "needs-date"}},
		},
	}

	result := EvaluateRules([]*models.Task{task}, rules)
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !task.HasTag("needs-date") {
		t.Error("expected 'needs-date' tag on task without due date")
	}

	// Task with due date should NOT be tagged
	due := time.Now().Add(48 * time.Hour)
	task2 := &models.Task{ID: "2", DueDate: &due, Tags: []string{}, Type: models.TaskTypeTask}
	result2 := EvaluateRules([]*models.Task{task2}, rules)
	if len(result2.ModifiedTasks) != 0 {
		t.Errorf("expected 0 modified tasks for task with due date, got %d", len(result2.ModifiedTasks))
	}
}

func TestEvaluation_DueDateEventEndToEnd(t *testing.T) {
	// When a due date is added, snooze the task
	now := time.Now()
	prevTask := &models.Task{ID: "1", DueDate: nil, Tags: []string{}}
	currTask := &models.Task{ID: "1", DueDate: &now, Tags: []string{}}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Snooze when due date added",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldDueDateChanged, Operation: OpEquals, Value: "added"},
			Actions: []RuleAction{{ActionType: ActionSetSnooze, Value: "+1d"}},
		},
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: currTask, Previous: prevTask}},
		AllTasks: []*models.Task{currTask},
		TaskMap:  map[string]*models.Task{"1": currTask},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if currTask.SnoozedUntil == nil {
		t.Error("expected SnoozedUntil to be set after due_date_changed trigger")
	}
}

func TestEvaluation_RecursiveChainedEventTriggers(t *testing.T) {
	// Rule 1: when priority == 1, set status to in_progress.
	// Rule 2: when status changes from todo -> in_progress, add tag "started".
	// Both rules should fire: rule 1 mutates the task, and the recursive
	// re-evaluation should detect the status_changed event via the snapshot.
	task := &models.Task{
		ID:            "1",
		Priority:      1,
		WorkflowState: models.StateReady,
		Tags:          []string{},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-priority",
			Name:    "High prio starts",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			Actions: []RuleAction{{ActionType: ActionSetStatus, Value: string(models.StatusInProgress)}},
		},
		{
			ID:      "rule-status-tag",
			Name:    "Tag on start",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatusChanged, Operation: OpEquals, Value: "todo->in_progress"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "started"}},
		},
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: task, Previous: nil}},
		AllTasks: []*models.Task{task},
		TaskMap:  map[string]*models.Task{"1": task},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if task.State() != models.StateInProgress {
		t.Errorf("expected status in_progress, got %s", task.State())
	}
	if !task.HasTag("started") {
		t.Errorf("expected tag 'started' from chained event trigger, got tags: %v", task.Tags)
	}
	// Both rules should have been applied.
	if len(result.AppliedRules["1"]) != 2 {
		t.Errorf("expected 2 applied rules, got %d: %v", len(result.AppliedRules["1"]), result.AppliedRules["1"])
	}
}
