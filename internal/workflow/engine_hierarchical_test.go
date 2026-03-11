package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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
		{
			name:     "dependency_count > 0 with deps",
			trigger:  RuleTrigger{Field: FieldDependencyCount, Operation: OpGreaterThan, Value: "0"},
			expected: true,
		},
		{
			name:     "dependency_count = 2 with two deps",
			trigger:  RuleTrigger{Field: FieldDependencyCount, Operation: OpEquals, Value: "2"},
			expected: true,
		},
	}

	// Test dependency_count with a task that has no dependencies
	noDepsTask := &models.Task{ID: "nodeps"}
	noDepsTests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "dependency_count = 0 with no deps",
			trigger:  RuleTrigger{Field: FieldDependencyCount, Operation: OpEquals, Value: "0"},
			expected: true,
		},
		{
			name:     "dependency_count > 0 with no deps",
			trigger:  RuleTrigger{Field: FieldDependencyCount, Operation: OpGreaterThan, Value: "0"},
			expected: false,
		},
	}
	for _, tt := range noDepsTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesHierarchicalTrigger(noDepsTask, tt.trigger, evalCtx); got != tt.expected {
				t.Errorf("matchesHierarchicalTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesHierarchicalTrigger(task, tt.trigger, evalCtx); got != tt.expected {
				t.Errorf("matchesHierarchicalTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

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
