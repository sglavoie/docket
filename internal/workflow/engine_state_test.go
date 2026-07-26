package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestCanonicalTaskStatePredicates(t *testing.T) {
	task := &models.Task{ID: "task", WorkflowState: models.StatePaused}
	tests := []struct {
		name    string
		trigger RuleTrigger
		want    bool
	}{
		{
			name:    "equals",
			trigger: RuleTrigger{Field: FieldState, Operation: OpEquals, Value: "paused"},
			want:    true,
		},
		{
			name:    "in state set",
			trigger: RuleTrigger{Field: FieldState, Operation: OpIn, Value: "waiting,paused"},
			want:    true,
		},
		{
			name:    "not in state set",
			trigger: RuleTrigger{Field: FieldState, Operation: OpNotIn, Value: "done,canceled"},
			want:    true,
		},
		{
			name:    "not equals",
			trigger: RuleTrigger{Field: FieldState, Operation: OpNotEquals, Value: "paused"},
			want:    false,
		},
		{
			name:    "legacy status is not a canonical state",
			trigger: RuleTrigger{Field: FieldState, Operation: OpEquals, Value: "blocked"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTrigger(task, tt.trigger); got != tt.want {
				t.Fatalf("matchesTrigger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanonicalStateChangedMatchesEndpointSets(t *testing.T) {
	previous := &models.Task{ID: "task", WorkflowState: models.StateWaiting}
	current := &models.Task{ID: "task", WorkflowState: models.StateReady}

	if !matchesEventTrigger(current, previous, RuleTrigger{
		Field: FieldStateChanged, Operation: OpEquals, Value: "waiting,paused->ready",
	}) {
		t.Fatal("state_changed did not match a canonical source set")
	}
	if !matchesEventTrigger(current, previous, RuleTrigger{
		Field: FieldStateChanged, Operation: OpNotEquals, Value: "next->ready",
	}) {
		t.Fatal("state_changed not_equals did not negate the endpoint match")
	}
	if matchesEventTrigger(current, CopyTaskState(current), RuleTrigger{
		Field: FieldStateChanged, Operation: OpNotEquals, Value: "next->ready",
	}) {
		t.Fatal("state_changed matched without an actual state transition")
	}
}

func TestCanonicalParentStateMatchesDirectly(t *testing.T) {
	parent := &models.Task{ID: "parent", WorkflowState: models.StateReview}
	task := &models.Task{ID: "child", WorkflowState: models.StateReady, Parent: parent}
	trigger := RuleTrigger{Field: FieldParentState, Operation: OpIn, Value: "in_progress,review"}

	if !matchesHierarchicalTrigger(task, trigger, nil) {
		t.Fatal("parent_state did not match the parent's canonical state")
	}
}

func TestCanonicalSetStateUsesTransitionSeamAndRecursiveEvent(t *testing.T) {
	task := &models.Task{
		ID:            "task",
		Priority:      1,
		WorkflowState: models.StateReady,
	}
	rules := []WorkflowRule{
		{
			ID: "enter-next", Name: "Enter Next", Enabled: true, Priority: 1,
			Triggers: []RuleTrigger{{Field: FieldState, Operation: OpEquals, Value: "ready"}},
			Actions:  []RuleAction{{ActionType: ActionSetState, Value: "next"}},
		},
		{
			ID: "tag-next", Name: "Tag Next", Enabled: true, Priority: 2,
			Triggers: []RuleTrigger{{Field: FieldStateChanged, Operation: OpEquals, Value: "ready->next"}},
			Actions:  []RuleAction{{ActionType: ActionAddTag, Value: "queued"}},
		},
	}

	now := time.Date(2026, 7, 25, 9, 30, 0, 0, time.UTC)
	transitionCalls := 0
	transitionUsedInjectedTime := false
	evalCtx := BuildEvaluationContext([]*models.Task{task}, nil, "Project", "project")
	evalCtx.TransitionState = func(task *models.Task, target models.WorkflowState) error {
		transitionCalls++
		err := task.ApplyStateTransitionAt(target, now)
		transitionUsedInjectedTime = task.UpdatedAt.Equal(now)
		return err
	}
	result := EvaluateRulesWithContext(evalCtx, rules)

	if task.State() != models.StateNext {
		t.Fatalf("state = %s, want next", task.State())
	}
	if !task.HasTag("queued") {
		t.Fatal("recursive state_changed rule did not fire")
	}
	if transitionCalls != 1 {
		t.Fatalf("transition callback calls = %d, want 1", transitionCalls)
	}
	if !transitionUsedInjectedTime {
		t.Fatal("transition callback did not stamp the injected transition time")
	}
	if len(result.ModifiedTasks) != 1 {
		t.Fatalf("ModifiedTasks = %d, want one deduplicated task", len(result.ModifiedTasks))
	}

	second := EvaluateRules([]*models.Task{task}, rules)
	if len(second.ModifiedTasks) != 0 || len(second.RuleTriggerCounts) != 0 {
		t.Fatalf("idempotent re-evaluation modified task: result=%+v", second)
	}
}

func TestCanonicalGroupedConditionAndPriorityConflict(t *testing.T) {
	task := &models.Task{ID: "task", Priority: 1, WorkflowState: models.StateReady}
	grouped := WorkflowRule{
		ID: "grouped", Name: "Grouped", Enabled: true, Priority: 1,
		TriggerGroup: &TriggerGroup{
			Logic:    LogicAnd,
			Triggers: []RuleTrigger{{Field: FieldState, Operation: OpIn, Value: "inbox,ready"}},
			Groups: []TriggerGroup{{
				Logic: LogicOr,
				Triggers: []RuleTrigger{
					{Field: FieldPriority, Operation: OpEquals, Value: "1"},
					{Field: FieldPriority, Operation: OpEquals, Value: "2"},
				},
			}},
		},
		Actions: []RuleAction{{
			ActionType: ActionSetState,
			Value:      "next",
			Condition:  &RuleTrigger{Field: FieldState, Operation: OpNotIn, Value: "done,canceled"},
		}},
	}
	lowerPriority := WorkflowRule{
		ID: "lower", Name: "Lower", Enabled: true, Priority: 5,
		Triggers: []RuleTrigger{{Field: FieldState, Operation: OpEquals, Value: "ready"}},
		Actions:  []RuleAction{{ActionType: ActionSetState, Value: "waiting"}},
	}

	result := EvaluateRules([]*models.Task{task}, []WorkflowRule{lowerPriority, grouped})
	if task.State() != models.StateNext {
		t.Fatalf("state = %s, want next from the higher-priority grouped rule", task.State())
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("priority-resolved state actions produced conflicts: %+v", result.Conflicts)
	}
}

func TestCanonicalProjectStateAggregates(t *testing.T) {
	tasks := []*models.Task{
		{ID: "ready-1", WorkflowState: models.StateReady},
		{ID: "ready-2", WorkflowState: models.StateReady},
		{ID: "done", WorkflowState: models.StateDone},
	}
	ctx := &ProjectEvaluationContext{
		Project: &models.Project{ID: "project", Name: "Project"},
		Tasks:   tasks,
	}
	tests := []RuleTrigger{
		{Field: FieldAllTasksState, Operation: OpIn, Value: "inbox,ready"},
		{Field: FieldMajorityTaskState, Operation: OpEquals, Value: "ready"},
		{Field: FieldHasTasksWithState, Operation: OpIn, Value: "done,canceled"},
		{Field: FieldHasTasksWithState, Operation: OpNotIn, Value: "waiting,paused"},
	}
	for _, trigger := range tests {
		if !matchProjectTrigger(ctx, trigger) {
			t.Errorf("canonical project trigger did not match: %s", trigger)
		}
	}

	ctx.Tasks = append(ctx.Tasks, &models.Task{ID: "next", WorkflowState: models.StateNext})
	if matchProjectTrigger(ctx, RuleTrigger{
		Field: FieldAllTasksState, Operation: OpEquals, Value: "ready",
	}) {
		t.Fatal("all_tasks_state matched mixed active states")
	}
}

func TestBuiltInCanonicalStateRuleExecutesIdempotently(t *testing.T) {
	blocker := &models.Task{ID: "blocker", WorkflowState: models.StateReady}
	blocked := &models.Task{
		ID:            "blocked",
		WorkflowState: models.StateReady,
		DependsOn:     []string{blocker.ID},
	}
	rules := GetDefaultRules()
	var autoBlock WorkflowRule
	for _, rule := range rules {
		if rule.ID == "def-auto-block" {
			autoBlock = rule
			break
		}
	}
	if autoBlock.ID == "" {
		t.Fatal("def-auto-block default rule not found")
	}
	autoBlock.Enabled = true

	first := EvaluateRules([]*models.Task{blocker, blocked}, []WorkflowRule{autoBlock})
	if blocked.State() != models.StateWaiting {
		t.Fatalf("state = %s, want waiting", blocked.State())
	}
	if first.RuleTriggerCounts[autoBlock.ID] != 1 {
		t.Fatalf("first trigger count = %d, want 1", first.RuleTriggerCounts[autoBlock.ID])
	}

	second := EvaluateRules([]*models.Task{blocker, blocked}, []WorkflowRule{autoBlock})
	if len(second.ModifiedTasks) != 0 || second.RuleTriggerCounts[autoBlock.ID] != 0 {
		t.Fatalf("default rule was not idempotent: %+v", second)
	}
}
