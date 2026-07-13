package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func makeProject(statusIcon string) *models.Project {
	return &models.Project{
		ID:         "proj-1",
		Name:       "Test Project",
		UpdatedAt:  time.Now(),
		StatusIcon: statusIcon,
	}
}

func makeTask(status models.TaskStatus, taskType models.TaskType) *models.Task {
	return &models.Task{
		ID:            "task-" + string(status) + "-" + string(taskType),
		WorkflowState: models.DeriveWorkflowState(status, "", ""),
		Type:          taskType,
		UpdatedAt:     time.Now(),
	}
}

func projectRule(id, name string, priority int, triggers []RuleTrigger, statusKey string) WorkflowRule {
	return WorkflowRule{
		ID:           id,
		Name:         name,
		Level:        RuleLevelProject,
		Enabled:      true,
		Priority:     priority,
		Triggers:     triggers,
		TriggerLogic: LogicAnd,
		Actions: []RuleAction{
			{ActionType: ActionSetProjectStatus, Value: statusKey},
		},
		Source: "default",
	}
}

func TestMatchProjectTrigger_HasInProgressTasks(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusInProgress, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected has_in_progress_tasks = true to match")
	}

	trigger.Value = "false"
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected has_in_progress_tasks = false to not match")
	}

	// No in-progress tasks
	ctx.Tasks = []*models.Task{makeTask(models.StatusTodo, models.TaskTypeTask)}
	trigger.Value = "false"
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected has_in_progress_tasks = false to match when no tasks are in_progress")
	}
}

func TestMatchProjectTrigger_ActiveTaskCount(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusInProgress, models.TaskTypeBug),
		makeTask(models.StatusDone, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldActiveTaskCount, Operation: OpGreaterThan, Value: "0"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected active_task_count > 0 to match with 2 active tasks")
	}

	trigger = RuleTrigger{Field: FieldActiveTaskCount, Operation: OpEquals, Value: "2"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected active_task_count = 2 to match")
	}
}

func TestMatchProjectTrigger_AllTasksStatus(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusTodo, models.TaskTypeBug),
		makeTask(models.StatusDone, models.TaskTypeTask), // complete, ignored
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldAllTasksStatus, Operation: OpEquals, Value: "todo"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected all_tasks_status = todo to match when all non-complete tasks are todo")
	}

	// Mix of statuses
	tasks = append(tasks, makeTask(models.StatusInProgress, models.TaskTypeTask))
	ctx.Tasks = tasks
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected all_tasks_status = todo to not match when statuses are mixed")
	}
}

func TestMatchProjectTrigger_MajorityTaskType(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeBug),
		makeTask(models.StatusInProgress, models.TaskTypeBug),
		makeTask(models.StatusTodo, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldMajorityTaskType, Operation: OpEquals, Value: "bug"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected majority_task_type = bug to match (2/3 bugs)")
	}

	trigger.Value = "task"
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected majority_task_type = task to not match (only 1/3)")
	}
}

func TestMatchProjectTrigger_MajorityTaskStatus(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusInProgress, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldMajorityTaskStatus, Operation: OpEquals, Value: "todo"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected majority_task_status = todo to match (2/3)")
	}
}

func TestMatchProjectTrigger_CompletionRatio(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusDone, models.TaskTypeTask),
		makeTask(models.StatusDone, models.TaskTypeTask),
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusTodo, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldCompletionRatio, Operation: OpEquals, Value: "50"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected completion_ratio = 50 to match (2/4 done)")
	}

	trigger = RuleTrigger{Field: FieldCompletionRatio, Operation: OpGreaterThan, Value: "25"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected completion_ratio > 25 to match")
	}
}

func TestMatchProjectTrigger_DaysSinceUpdate(t *testing.T) {
	proj := makeProject("")
	proj.UpdatedAt = time.Now().AddDate(0, 0, -10)
	// Task updated 5 days ago (more recent than project)
	task := makeTask(models.StatusTodo, models.TaskTypeTask)
	task.UpdatedAt = time.Now().AddDate(0, 0, -5)
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: []*models.Task{task}}

	trigger := RuleTrigger{Field: FieldDaysSinceUpdate, Operation: OpGreaterThan, Value: "3"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected days_since_update > 3 to match (most recent update was 5 days ago)")
	}

	trigger = RuleTrigger{Field: FieldDaysSinceUpdate, Operation: OpGreaterThan, Value: "7"}
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected days_since_update > 7 to not match (most recent update was 5 days ago)")
	}
}

func TestMatchProjectTrigger_ProjectStatus(t *testing.T) {
	proj := makeProject("active")
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: nil}

	trigger := RuleTrigger{Field: FieldProjectStatus, Operation: OpEquals, Value: "active"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected project_status = active to match")
	}

	trigger.Value = "paused"
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected project_status = paused to not match")
	}
}

func TestMatchProjectTrigger_HasTasksWithStatus(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusBlocked, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldHasTasksWithStatus, Operation: OpEquals, Value: "blocked"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected has_tasks_with_status = blocked to match")
	}

	trigger.Value = "in_progress"
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected has_tasks_with_status = in_progress to not match")
	}
}

func TestMatchProjectTrigger_HasTasksWithStatus_NotEquals(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeTask),
		makeTask(models.StatusBlocked, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	// not_equals "in_progress": no task has in_progress → should match
	trigger := RuleTrigger{Field: FieldHasTasksWithStatus, Operation: OpNotEquals, Value: "in_progress"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected has_tasks_with_status not_equals in_progress to match (no task has in_progress)")
	}

	// not_equals "blocked": a task has blocked → should NOT match
	trigger.Value = "blocked"
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected has_tasks_with_status not_equals blocked to not match (a task has blocked)")
	}
}

func TestEvaluateProjectRules_SingleMatch(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusInProgress, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		projectRule("r1", "Active Project", 50,
			[]RuleTrigger{{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}},
			"active"),
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "active" {
		t.Errorf("expected NewStatusIcon = active, got %s", result.NewStatusIcon)
	}
	if result.AppliedRuleName != "Active Project" {
		t.Errorf("expected AppliedRuleName = Active Project, got %s", result.AppliedRuleName)
	}
	if result.AppliedRuleID != "r1" {
		t.Errorf("expected AppliedRuleID = r1, got %s", result.AppliedRuleID)
	}
}

func TestEvaluateProjectRules_PriorityResolution(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeBug),
		makeTask(models.StatusTodo, models.TaskTypeBug),
		makeTask(models.StatusTodo, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		projectRule("r-bugfix", "Bugfix Mode", 30,
			[]RuleTrigger{{Field: FieldMajorityTaskType, Operation: OpEquals, Value: "bug"}},
			"bugfix"),
		projectRule("r-planning", "Planning", 60,
			[]RuleTrigger{
				{Field: FieldActiveTaskCount, Operation: OpGreaterThan, Value: "0"},
				{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "false"},
			},
			"planning"),
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "bugfix" {
		t.Errorf("expected bugfix (priority 30) to win over planning (priority 60), got %s", result.NewStatusIcon)
	}
}

func TestEvaluateProjectRules_SamePriorityConflict(t *testing.T) {
	proj := makeProject("")
	// Both bugs and in_progress status are majority
	tasks := []*models.Task{
		makeTask(models.StatusInProgress, models.TaskTypeBug),
		makeTask(models.StatusInProgress, models.TaskTypeBug),
		makeTask(models.StatusTodo, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		projectRule("r-bugfix", "Bugfix Mode", 30,
			[]RuleTrigger{{Field: FieldMajorityTaskType, Operation: OpEquals, Value: "bug"}},
			"bugfix"),
		projectRule("r-review", "In Review", 30,
			[]RuleTrigger{{Field: FieldMajorityTaskStatus, Operation: OpEquals, Value: "in_progress"}},
			"review"),
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "" {
		t.Errorf("expected same-priority conflict to cancel out, got %s", result.NewStatusIcon)
	}
	if len(result.TriggeredRuleIDs) != 2 {
		t.Errorf("expected 2 triggered rules, got %d", len(result.TriggeredRuleIDs))
	}
}

func TestEvaluateProjectRules_DisabledRulesSkipped(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{makeTask(models.StatusInProgress, models.TaskTypeTask)}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		{
			ID:       "r-disabled",
			Name:     "Disabled Rule",
			Level:    RuleLevelProject,
			Enabled:  false,
			Priority: 10,
			Triggers: []RuleTrigger{{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}},
			Actions:  []RuleAction{{ActionType: ActionSetProjectStatus, Value: "active"}},
		},
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "" {
		t.Errorf("expected disabled rule to be skipped, got %s", result.NewStatusIcon)
	}
}

func TestEvaluateProjectRules_TaskLevelRulesIgnored(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{makeTask(models.StatusInProgress, models.TaskTypeTask)}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		{
			ID:      "task-rule",
			Name:    "Task Rule",
			Enabled: true,
			// Level not set, defaults to task
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
			Actions: []RuleAction{{ActionType: ActionSetStatus, Value: "done"}},
		},
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "" {
		t.Errorf("expected task-level rule to be ignored, got %s", result.NewStatusIcon)
	}
}

func TestEvaluateProjectRules_NoMatch(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{makeTask(models.StatusTodo, models.TaskTypeTask)}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		projectRule("r1", "Active", 50,
			[]RuleTrigger{{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}},
			"active"),
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "" {
		t.Errorf("expected no match, got %s", result.NewStatusIcon)
	}
}

func TestEvaluateProjectRules_RespectsProjectScope(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{makeTask(models.StatusInProgress, models.TaskTypeTask)}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		{
			ID:           "r-other",
			Name:         "Other Project Only",
			Level:        RuleLevelProject,
			Enabled:      true,
			Priority:     10,
			Triggers:     []RuleTrigger{{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionSetProjectStatus, Value: "active"}},
			Scope:        &RuleScope{Projects: []string{"proj-other"}},
		},
		{
			ID:           "r-this",
			Name:         "This Project Only",
			Level:        RuleLevelProject,
			Enabled:      true,
			Priority:     20,
			Triggers:     []RuleTrigger{{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionSetProjectStatus, Value: "review"}},
			Scope:        &RuleScope{Projects: []string{proj.ID}},
		},
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "review" {
		t.Fatalf("expected only the matching scoped rule to apply, got %q", result.NewStatusIcon)
	}
	if result.AppliedRuleID != "r-this" {
		t.Fatalf("expected scoped rule r-this to apply, got %q", result.AppliedRuleID)
	}
}

func TestEvaluateProjectRules_CompoundTriggers(t *testing.T) {
	proj := makeProject("")
	proj.UpdatedAt = time.Now().AddDate(0, 0, -20)
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeTask),
	}
	tasks[0].UpdatedAt = time.Now().AddDate(0, 0, -20)
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		projectRule("r-paused", "Project Paused", 40,
			[]RuleTrigger{
				{Field: FieldDaysSinceUpdate, Operation: OpGreaterThan, Value: "14"},
				{Field: FieldActiveTaskCount, Operation: OpGreaterThan, Value: "0"},
			},
			"paused"),
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "paused" {
		t.Errorf("expected paused, got %s", result.NewStatusIcon)
	}
}

func TestEvaluateProjectRules_EmptyTasks(t *testing.T) {
	proj := makeProject("")
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: nil}

	rules := []WorkflowRule{
		projectRule("r1", "Active", 50,
			[]RuleTrigger{{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}},
			"active"),
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "" {
		t.Errorf("expected no match with empty tasks, got %s", result.NewStatusIcon)
	}
}

func TestEvaluateProjectRules_CompletionRatioNoTasks(t *testing.T) {
	proj := makeProject("")
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: nil}

	trigger := RuleTrigger{Field: FieldCompletionRatio, Operation: OpEquals, Value: "0"}
	if !matchProjectTrigger(ctx, trigger) {
		t.Error("expected completion_ratio = 0 with no tasks")
	}
}

func TestEvaluateProjectRules_MajorityExactlyHalf(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusTodo, models.TaskTypeBug),
		makeTask(models.StatusTodo, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	trigger := RuleTrigger{Field: FieldMajorityTaskType, Operation: OpEquals, Value: "bug"}
	if matchProjectTrigger(ctx, trigger) {
		t.Error("expected exactly 50% (1/2) to NOT count as majority (needs >50%)")
	}
}

func TestEvaluateProjectRules_ZeroPrioritySortsLast(t *testing.T) {
	proj := makeProject("")
	tasks := []*models.Task{
		makeTask(models.StatusInProgress, models.TaskTypeTask),
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	rules := []WorkflowRule{
		projectRule("r-zero", "Zero Priority", 0,
			[]RuleTrigger{{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "true"}},
			"wip"),
		projectRule("r-fifty", "Priority 50", 50,
			[]RuleTrigger{{Field: FieldActiveTaskCount, Operation: OpGreaterThan, Value: "0"}},
			"active"),
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "active" {
		t.Errorf("expected priority 50 to win over unset priority, got %s", result.NewStatusIcon)
	}
}
