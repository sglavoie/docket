package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesHierarchicalTrigger_ParentStatus(t *testing.T) {
	parent := &models.Task{ID: "parent", WorkflowState: models.StateInProgress, Type: models.TaskTypeEpic}
	child := &models.Task{ID: "child", WorkflowState: models.StateReady, Parent: parent}

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
			trigger:  RuleTrigger{Field: FieldParentState, Operation: OpEquals, Value: "in_progress"},
			expected: true,
		},
		{
			name:     "parent status does not match",
			trigger:  RuleTrigger{Field: FieldParentState, Operation: OpEquals, Value: "done"},
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
		childStatuses []models.WorkflowState
		expectedMatch bool
	}{
		{
			name:          "all children done",
			childStatuses: []models.WorkflowState{models.StateDone, models.StateDone},
			expectedMatch: true,
		},
		{
			name:          "some children incomplete",
			childStatuses: []models.WorkflowState{models.StateDone, models.StateReady},
			expectedMatch: false,
		},
		{
			name:          "no children",
			childStatuses: []models.WorkflowState{},
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := &models.Task{ID: "parent", Type: models.TaskTypeEpic, Children: []*models.Task{}}
			for i, status := range tt.childStatuses {
				child := &models.Task{
					ID:            "child" + string(rune('0'+i)),
					WorkflowState: status,
					Type:          models.TaskTypeTask,
					Parent:        parent,
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
	blocker1 := &models.Task{ID: "blocker1", WorkflowState: models.StateReady}
	blocker2 := &models.Task{ID: "blocker2", WorkflowState: models.StateDone}
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
	result := &EvaluationResult{}
	modified := applyHierarchicalAction(parent, action, evalCtx, result)

	if !modified {
		t.Error("expected cascade_tag to modify at least one child")
	}
	if !child1.HasTag("urgent") {
		t.Error("expected child1 to have 'urgent' tag after cascade")
	}
	if !containsTask(result.ModifiedTasks, child1) {
		t.Error("expected cascaded child1 to be recorded as modified")
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
	result := &EvaluationResult{}
	modified := applyHierarchicalAction(child, action, evalCtx, result)

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
	result := &EvaluationResult{}
	modified := applyHierarchicalAction(child, action, evalCtx, result)

	if !modified {
		t.Error("expected notify_parent to modify parent")
	}
	if len(parent.Comments) != 1 {
		t.Errorf("expected parent to have 1 comment, got %d", len(parent.Comments))
	}
	if parent.Comments[0].Text != "[Workflow] Child completed" {
		t.Errorf("unexpected comment text: %s", parent.Comments[0].Text)
	}
	if !containsTask(result.ModifiedTasks, parent) {
		t.Error("expected parent (notified) to be recorded as modified")
	}
}

func TestEvaluateRulesWithContext_HierarchicalTrigger(t *testing.T) {
	// Epic with two completed children
	child1 := &models.Task{ID: "c1", WorkflowState: models.StateDone, Type: models.TaskTypeTask}
	child2 := &models.Task{ID: "c2", WorkflowState: models.StateDone, Type: models.TaskTypeTask}
	epic := &models.Task{
		ID:            "epic",
		WorkflowState: models.StateInProgress,
		Type:          models.TaskTypeEpic,
		Children:      []*models.Task{child1, child2},
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
			Actions:      []RuleAction{{ActionType: ActionSetState, Value: "done"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task (epic), got %d", len(result.ModifiedTasks))
	}
	if epic.State() != models.StateDone {
		t.Errorf("expected epic status to be done, got %s", epic.State())
	}
}

func TestEvaluateRulesWithContext_AllChildrenDoneDoesNotRecompleteReopenedParent(t *testing.T) {
	tests := []struct {
		name string
		rule WorkflowRule
	}{
		{
			name: "flat triggers",
			rule: WorkflowRule{
				ID:      "rule-flat",
				Name:    "Auto Complete Story",
				Enabled: true,
				Triggers: []RuleTrigger{
					{Field: FieldType, Operation: OpEquals, Value: "story"},
					{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"},
				},
				TriggerLogic: LogicAnd,
				Actions:      []RuleAction{{ActionType: ActionSetState, Value: "done"}},
			},
		},
		{
			name: "nested trigger group",
			rule: WorkflowRule{
				ID:      "rule-group",
				Name:    "Grouped Auto Complete Story",
				Enabled: true,
				TriggerGroup: &TriggerGroup{
					Logic:    LogicAnd,
					Triggers: []RuleTrigger{{Field: FieldType, Operation: OpEquals, Value: "story"}},
					Groups: []TriggerGroup{
						{
							Logic:    LogicAnd,
							Triggers: []RuleTrigger{{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"}},
						},
					},
				},
				Actions: []RuleAction{{ActionType: ActionSetState, Value: "done"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			child1 := &models.Task{ID: "c1", WorkflowState: models.StateDone, Type: models.TaskTypeTask}
			child2 := &models.Task{ID: "c2", WorkflowState: models.StateDone, Type: models.TaskTypeTask}
			story := &models.Task{
				ID:            "story",
				WorkflowState: models.StateReady,
				Type:          models.TaskTypeStory,
				Children:      []*models.Task{child1, child2},
			}
			child1.Parent = story
			child2.Parent = story

			evalCtx := &EvaluationContext{
				Changes:  []TaskChangeContext{{Task: story, Previous: &models.Task{ID: "story", WorkflowState: models.StateDone}}},
				AllTasks: []*models.Task{story, child1, child2},
				TaskMap:  map[string]*models.Task{"story": story, "c1": child1, "c2": child2},
			}

			result := EvaluateRulesWithContext(evalCtx, []WorkflowRule{tt.rule})

			if story.State() != models.StateReady {
				t.Errorf("expected reopened story to remain todo, got %s", story.State())
			}
			if len(result.ModifiedTasks) != 0 {
				t.Errorf("expected no workflow modifications, got %d", len(result.ModifiedTasks))
			}
			if result.RuleTriggerCounts[tt.rule.ID] != 0 {
				t.Errorf("expected rule trigger count 0, got %d", result.RuleTriggerCounts[tt.rule.ID])
			}
		})
	}
}

func TestEvaluateRulesWithContext_ReopenedRuleCanStillSetCompleteStatus(t *testing.T) {
	child := &models.Task{ID: "child", WorkflowState: models.StateDone, Type: models.TaskTypeTask}
	task := &models.Task{
		ID:            "task",
		WorkflowState: models.StateReady,
		Type:          models.TaskTypeStory,
		Children:      []*models.Task{child},
	}
	child.Parent = task
	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: task, Previous: &models.Task{ID: "task", WorkflowState: models.StateDone}}},
		AllTasks: []*models.Task{task, child},
		TaskMap:  map[string]*models.Task{"task": task, "child": child},
	}
	autoCompleteRule := WorkflowRule{
		ID:      "rule-auto-complete",
		Name:    "Auto Complete Story",
		Enabled: true,
		Triggers: []RuleTrigger{
			{Field: FieldType, Operation: OpEquals, Value: "story"},
			{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"},
		},
		TriggerLogic: LogicAnd,
		Actions:      []RuleAction{{ActionType: ActionSetState, Value: "done"}},
	}
	reopenedRule := WorkflowRule{
		ID:       "rule-reopened",
		Name:     "Explicit Reopen Override",
		Enabled:  true,
		Triggers: []RuleTrigger{{Field: FieldReopened, Operation: OpEquals, Value: "true"}},
		Actions:  []RuleAction{{ActionType: ActionSetState, Value: "done"}},
	}

	result := EvaluateRulesWithContext(evalCtx, []WorkflowRule{autoCompleteRule, reopenedRule})

	if task.State() != models.StateDone {
		t.Errorf("expected explicit reopened rule to set status done, got %s", task.State())
	}
	if len(result.ModifiedTasks) != 1 || result.ModifiedTasks[0] != task {
		t.Errorf("expected task to be recorded as modified once, got %d", len(result.ModifiedTasks))
	}
	if result.RuleTriggerCounts[autoCompleteRule.ID] != 0 {
		t.Errorf("expected auto-complete rule trigger count 0, got %d", result.RuleTriggerCounts[autoCompleteRule.ID])
	}
	if result.RuleTriggerCounts[reopenedRule.ID] != 1 {
		t.Errorf("expected reopened rule trigger count 1, got %d", result.RuleTriggerCounts[reopenedRule.ID])
	}
}

func TestMatchesRule_TriggerGroupWithHierarchicalTrigger_NilEvalCtx(t *testing.T) {
	// Regression: matchesRule passes nil evalCtx to matchesTriggerGroup.
	// If TriggerGroup contains hierarchical triggers like FieldIsBlocked or
	// FieldBlockerCount, the code used to dereference evalCtx.TaskMap and panic.
	task := &models.Task{ID: "task", WorkflowState: models.StateReady, DependsOn: []string{"blocker1"}}

	tests := []struct {
		name     string
		rule     WorkflowRule
		expected bool
	}{
		{
			name: "TriggerGroup with FieldIsBlocked does not panic",
			rule: WorkflowRule{
				ID:      "r1",
				Name:    "blocked check",
				Enabled: true,
				TriggerGroup: &TriggerGroup{
					Logic: LogicAnd,
					Triggers: []RuleTrigger{
						{Field: FieldState, Operation: OpEquals, Value: "todo"},
						{Field: FieldIsBlocked, Operation: OpEquals, Value: "true"},
					},
				},
				Actions: []RuleAction{{ActionType: ActionAddTag, Value: "x"}},
			},
			// With nil taskMap, hasIncompleteBlockers returns false because
			// the range over a nil map yields no iterations.
			expected: false,
		},
		{
			name: "TriggerGroup with FieldBlockerCount does not panic",
			rule: WorkflowRule{
				ID:      "r2",
				Name:    "blocker count",
				Enabled: true,
				TriggerGroup: &TriggerGroup{
					Logic: LogicOr,
					Triggers: []RuleTrigger{
						{Field: FieldBlockerCount, Operation: OpGreaterThan, Value: "0"},
					},
				},
				Actions: []RuleAction{{ActionType: ActionAddTag, Value: "x"}},
			},
			// nil taskMap -> countIncompleteBlockers returns 0 -> 0 > 0 is false
			expected: false,
		},
		{
			name: "nested TriggerGroup with hierarchical trigger does not panic",
			rule: WorkflowRule{
				ID:      "r3",
				Name:    "nested blocker",
				Enabled: true,
				TriggerGroup: &TriggerGroup{
					Logic: LogicAnd,
					Groups: []TriggerGroup{
						{
							Logic: LogicOr,
							Triggers: []RuleTrigger{
								{Field: FieldIsBlocked, Operation: OpEquals, Value: "false"},
								{Field: FieldBlockerCount, Operation: OpEquals, Value: "0"},
							},
						},
					},
				},
				Actions: []RuleAction{{ActionType: ActionAddTag, Value: "x"}},
			},
			// nil taskMap -> is_blocked=false (no blockers found) matches "false"=true
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This must not panic.
			got := matchesRule(task, &tt.rule)
			if got != tt.expected {
				t.Errorf("matchesRule() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestMatchesRuleWithContext_HierarchicalTrigger_NilEvalCtx checks that
// matchesRuleWithContext is safe when called with nil evalCtx and a rule that
// uses flat hierarchical triggers (no TriggerGroup wrapper).
func TestMatchesRuleWithContext_HierarchicalTrigger_NilEvalCtx(t *testing.T) {
	task := &models.Task{ID: "task", WorkflowState: models.StateReady, DependsOn: []string{"blocker1"}}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:    "FieldIsBlocked with nil evalCtx does not panic",
			trigger: RuleTrigger{Field: FieldIsBlocked, Operation: OpEquals, Value: "true"},
			// nil evalCtx -> nil taskMap -> no blockers resolved -> blocked=false -> does not match "true"
			expected: false,
		},
		{
			name:    "FieldBlockerCount with nil evalCtx does not panic",
			trigger: RuleTrigger{Field: FieldBlockerCount, Operation: OpGreaterThan, Value: "0"},
			// nil evalCtx -> nil taskMap -> count=0 -> 0 > 0 is false
			expected: false,
		},
		{
			name:    "FieldIsBlocked == false with nil evalCtx matches correctly",
			trigger: RuleTrigger{Field: FieldIsBlocked, Operation: OpEquals, Value: "false"},
			// nil taskMap -> blocked=false -> matches "false"
			expected: true,
		},
		{
			name:    "FieldDependencyCount with nil evalCtx does not need taskMap",
			trigger: RuleTrigger{Field: FieldDependencyCount, Operation: OpEquals, Value: "1"},
			// DependencyCount uses task.DependsOn directly, no taskMap needed.
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := WorkflowRule{
				ID:           "r",
				Name:         "test",
				Enabled:      true,
				Triggers:     []RuleTrigger{tt.trigger},
				TriggerLogic: LogicAnd,
				Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "x"}},
			}
			// nil evalCtx must not panic.
			got := matchesRuleWithContext(task, nil, &rule, nil)
			if got != tt.expected {
				t.Errorf("matchesRuleWithContext() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestMatchesRuleWithContext_HierarchicalTrigger_NilTaskMap checks that
// matchesRuleWithContext is safe when evalCtx is non-nil but TaskMap is nil or
// empty, which can occur when AllTasks is populated but the caller omits the map.
func TestMatchesRuleWithContext_HierarchicalTrigger_NilTaskMap(t *testing.T) {
	task := &models.Task{ID: "task", WorkflowState: models.StateReady, DependsOn: []string{"blocker1"}}

	blockerTask := &models.Task{ID: "blocker1", WorkflowState: models.StateReady}

	scenarios := []struct {
		name    string
		taskMap map[string]*models.Task
	}{
		{name: "nil TaskMap", taskMap: nil},
		{name: "empty TaskMap", taskMap: map[string]*models.Task{}},
		{name: "TaskMap missing the blocker", taskMap: map[string]*models.Task{"other": blockerTask}},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			evalCtx := &EvaluationContext{
				Changes:  []TaskChangeContext{{Task: task}},
				AllTasks: []*models.Task{task},
				TaskMap:  sc.taskMap,
			}

			blockedRule := WorkflowRule{
				ID:           "r-blocked",
				Name:         "is blocked",
				Enabled:      true,
				Triggers:     []RuleTrigger{{Field: FieldIsBlocked, Operation: OpEquals, Value: "true"}},
				TriggerLogic: LogicAnd,
				Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "x"}},
			}
			// Must not panic; blocker is unresolvable so result must be false.
			got := matchesRuleWithContext(task, nil, &blockedRule, evalCtx)
			if got {
				t.Errorf("%s: expected false (blocker not in taskMap), got true", sc.name)
			}

			countRule := WorkflowRule{
				ID:           "r-count",
				Name:         "blocker count",
				Enabled:      true,
				Triggers:     []RuleTrigger{{Field: FieldBlockerCount, Operation: OpGreaterThan, Value: "0"}},
				TriggerLogic: LogicAnd,
				Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "x"}},
			}
			got = matchesRuleWithContext(task, nil, &countRule, evalCtx)
			if got {
				t.Errorf("%s: expected false (blocker count 0 from empty map), got true", sc.name)
			}
		})
	}
}

// TestMatchesRuleWithContext_HierarchicalTrigger_TriggerGroup_NilEvalCtx checks
// that the TriggerGroup path in matchesRuleWithContext is also safe with nil evalCtx.
func TestMatchesRuleWithContext_HierarchicalTrigger_TriggerGroup_NilEvalCtx(t *testing.T) {
	task := &models.Task{ID: "task", WorkflowState: models.StateReady, DependsOn: []string{"blocker1"}}

	rule := WorkflowRule{
		ID:      "r1",
		Name:    "trigger group hierarchical nil ctx",
		Enabled: true,
		TriggerGroup: &TriggerGroup{
			Logic: LogicAnd,
			Triggers: []RuleTrigger{
				{Field: FieldIsBlocked, Operation: OpEquals, Value: "false"},
				{Field: FieldBlockerCount, Operation: OpEquals, Value: "0"},
			},
		},
		Actions: []RuleAction{{ActionType: ActionAddTag, Value: "x"}},
	}

	// Must not panic with nil evalCtx.
	got := matchesRuleWithContext(task, nil, &rule, nil)
	// nil taskMap -> blocked=false, count=0 -> both true -> AND match -> true
	if !got {
		t.Errorf("expected true (nil evalCtx, blocked=false, count=0), got false")
	}
}

func TestEvaluateRulesWithContext_AllChildrenDoneRequiresAggregateTransition(t *testing.T) {
	autoComplete := func(id string, taskType models.TaskType) WorkflowRule {
		return WorkflowRule{
			ID: id, Name: id, Enabled: true,
			Triggers: []RuleTrigger{
				{Field: FieldType, Operation: OpEquals, Value: string(taskType)},
				{Field: FieldAllChildrenDone, Operation: OpEquals, Value: "true"},
			},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionSetState, Value: "done"}},
		}
	}

	t.Run("final child cascades from story to epic", func(t *testing.T) {
		epic := &models.Task{ID: "epic", Type: models.TaskTypeEpic, WorkflowState: models.StateReady}
		story := &models.Task{ID: "story", Type: models.TaskTypeStory, WorkflowState: models.StateReady, ParentID: &epic.ID}
		epicSibling := &models.Task{ID: "epic-sibling", Type: models.TaskTypeStory, WorkflowState: models.StateDone, ParentID: &epic.ID}
		childDone := &models.Task{ID: "child-done", Type: models.TaskTypeTask, WorkflowState: models.StateDone, ParentID: &story.ID}
		finalChild := &models.Task{ID: "final-child", Type: models.TaskTypeTask, WorkflowState: models.StateReady, ParentID: &story.ID}
		project := &models.Project{Tasks: []*models.Task{epic, story, childDone, finalChild, epicSibling}}
		project.BuildTaskTree()
		previous := CaptureTaskStates(project.Tasks)
		finalChild.ApplyWorkflowState(models.StateCanceled)

		result := EvaluateRulesWithContext(
			BuildEvaluationContext(project.Tasks, previous, "Project", "project"),
			[]WorkflowRule{autoComplete("story-done", models.TaskTypeStory), autoComplete("epic-done", models.TaskTypeEpic)},
		)

		if story.State() != models.StateDone || epic.State() != models.StateDone {
			t.Fatalf("cascade states = story:%s epic:%s, want both done", story.State(), epic.State())
		}
		if result.RuleTriggerCounts["story-done"] != 1 || result.RuleTriggerCounts["epic-done"] != 1 {
			t.Fatalf("trigger counts = %v, want one story and one epic completion", result.RuleTriggerCounts)
		}
	})

	t.Run("unrelated parent edit does not complete", func(t *testing.T) {
		story := &models.Task{ID: "story", Type: models.TaskTypeStory, WorkflowState: models.StateReady}
		child := &models.Task{ID: "child", Type: models.TaskTypeTask, WorkflowState: models.StateDone, ParentID: &story.ID}
		project := &models.Project{Tasks: []*models.Task{story, child}}
		project.BuildTaskTree()
		previous := CaptureTaskStates(project.Tasks)
		story.ApplyWorkflowState(models.StateSomeday)

		result := EvaluateRulesWithContext(
			BuildEvaluationContext(project.Tasks, previous, "Project", "project"),
			[]WorkflowRule{autoComplete("story-done", models.TaskTypeStory)},
		)

		if story.State() != models.StateSomeday {
			t.Fatalf("story state = %s, want requested someday state", story.State())
		}
		if len(result.ModifiedTasks) != 0 {
			t.Fatalf("workflow modified %d tasks after unrelated edit", len(result.ModifiedTasks))
		}
	})
}
