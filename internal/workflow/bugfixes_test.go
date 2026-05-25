package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// TestApplyHierarchicalAction_RecordsParentInModifiedTasks ensures that a
// rule mutating a parent via ActionCompleteParent surfaces the parent in
// result.ModifiedTasks. Without this, the caller would silently lose the
// parent's status change at persist time.
func TestApplyHierarchicalAction_RecordsParentInModifiedTasks(t *testing.T) {
	parent := &models.Task{ID: "p", Status: models.StatusTodo}
	child := &models.Task{ID: "c", Status: models.StatusDone, Parent: parent}
	parent.Children = []*models.Task{child}

	evalCtx := &EvaluationContext{
		Changes: []TaskChangeContext{
			{Task: child, Previous: &models.Task{ID: "c", Status: models.StatusInProgress}},
		},
		AllTasks: []*models.Task{parent, child},
		TaskMap:  map[string]*models.Task{"p": parent, "c": child},
	}

	rules := []WorkflowRule{
		{
			ID:      "r1",
			Name:    "Complete parent when child completes",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldCompleted, Operation: OpEquals, Value: "true"},
			Actions: []RuleAction{{ActionType: ActionCompleteParent}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)
	if !parent.IsComplete() {
		t.Fatalf("expected parent to be completed, got status %q", parent.Status)
	}
	if !containsTask(result.ModifiedTasks, parent) {
		t.Errorf("expected parent in ModifiedTasks, got %d entries", len(result.ModifiedTasks))
	}
}

// TestApplyHierarchicalAction_BlockParentRecorded verifies the parent is
// surfaced as modified when ActionBlockParent fires.
func TestApplyHierarchicalAction_BlockParentRecorded(t *testing.T) {
	parent := &models.Task{ID: "p", Status: models.StatusTodo}
	child := &models.Task{ID: "c", Status: models.StatusBlocked, Parent: parent}
	parent.Children = []*models.Task{child}

	evalCtx := &EvaluationContext{
		AllTasks: []*models.Task{parent, child},
		TaskMap:  map[string]*models.Task{"p": parent, "c": child},
	}

	result := &EvaluationResult{}
	modified := applyHierarchicalAction(child, RuleAction{ActionType: ActionBlockParent}, evalCtx, result)
	if !modified {
		t.Fatal("expected modified")
	}
	if parent.Status != models.StatusBlocked {
		t.Fatalf("expected parent blocked, got %q", parent.Status)
	}
	if !containsTask(result.ModifiedTasks, parent) {
		t.Error("expected blocked parent in ModifiedTasks")
	}
}

// TestApplyAction_SetRecurrenceUnchangedNoOp verifies that setting the same
// recurrence pattern that is already on the task does not report the task as
// modified. Reporting spurious modifications causes recursive re-evaluation
// and may re-trigger downstream rules with no actual state change.
func TestApplyAction_SetRecurrenceUnchangedNoOp(t *testing.T) {
	existing := models.ParseRecurrence("daily")
	if existing == nil {
		t.Fatal("ParseRecurrence(daily) returned nil")
	}
	task := &models.Task{ID: "1", Recurrence: existing}

	modified := applyAction(task, RuleAction{ActionType: ActionSetRecurrence, Value: "daily"})
	if modified {
		t.Error("expected applyAction to return false when recurrence pattern is unchanged")
	}
}

// TestApplyAction_SetSnoozeUnchangedNoOp verifies that re-asserting the same
// snooze duration on a task whose SnoozedUntil already points at that target
// time does not re-mark the task as modified.
func TestApplyAction_SetSnoozeUnchangedNoOp(t *testing.T) {
	pre := time.Now().AddDate(0, 0, 3)
	task := &models.Task{ID: "1", SnoozedUntil: &pre}

	modified := applyAction(task, RuleAction{ActionType: ActionSetSnooze, Value: "+3d"})
	if modified {
		t.Error("expected applyAction to return false when snooze target is essentially unchanged")
	}
}

// TestEvaluation_ConditionalRuleReFiresAfterRecursiveRound verifies that a
// rule whose action conditions are not satisfied initially is NOT marked as
// permanently applied. If a later rule's mutation triggers a recursive
// re-evaluation that makes the condition true, the deferred rule must be
// able to fire on the recursive pass.
func TestEvaluation_ConditionalRuleReFiresAfterRecursiveRound(t *testing.T) {
	// Task starts as status=todo, no tag "ready".
	// Rule A: priority=1. When status=todo, set status=done IF tag has "ready".
	// Rule B: priority=2. When status=todo, add tag "ready".
	//
	// Without the fix, A is marked applied on pass 1 (even though the
	// condition was unmet) so it cannot fire on the recursive pass kicked
	// off by B adding the tag — status remains todo.
	task := &models.Task{
		ID:     "task-1",
		Status: models.StatusTodo,
		Tags:   []string{},
	}

	rules := []WorkflowRule{
		{
			ID:       "rule-A",
			Name:     "Complete when ready",
			Enabled:  true,
			Priority: 1,
			Trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{
					ActionType: ActionSetStatus,
					Value:      string(models.StatusDone),
					Condition:  &RuleTrigger{Field: FieldTags, Operation: OpContains, Value: "ready"},
				},
			},
		},
		{
			ID:       "rule-B",
			Name:     "Add ready tag",
			Enabled:  true,
			Priority: 2,
			Trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions:  []RuleAction{{ActionType: ActionAddTag, Value: "ready"}},
		},
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: task, Previous: nil}},
		AllTasks: []*models.Task{task},
		TaskMap:  map[string]*models.Task{"task-1": task},
	}

	_ = EvaluateRulesWithContext(evalCtx, rules)

	if !task.HasTag("ready") {
		t.Fatalf("expected tag 'ready' to be added by rule B, got tags: %v", task.Tags)
	}
	if task.Status != models.StatusDone {
		t.Errorf("expected status=done after rule A's condition became true via recursive re-eval, got %s", task.Status)
	}
}

// TestApplyHierarchicalAction_CascadeTagSilentValidationFailure verifies that
// ActionCascadeTag does not report children as modified when task.AddTag
// silently rejects the value due to validation (empty, too long, forbidden
// characters). Without re-checking presence, an invalid cascade tag would mark
// every child as modified, churning their UpdatedAt at the caller and forcing
// unnecessary persistence work.
func TestApplyHierarchicalAction_CascadeTagSilentValidationFailure(t *testing.T) {
	parent := &models.Task{ID: "p", Status: models.StatusTodo}
	childA := &models.Task{ID: "a", Status: models.StatusTodo, Parent: parent}
	childB := &models.Task{ID: "b", Status: models.StatusTodo, Parent: parent}
	parent.Children = []*models.Task{childA, childB}

	evalCtx := &EvaluationContext{
		AllTasks: []*models.Task{parent, childA, childB},
		TaskMap:  map[string]*models.Task{"p": parent, "a": childA, "b": childB},
	}

	result := &EvaluationResult{}
	// Empty string is silently rejected by task.AddTag.
	modified := applyHierarchicalAction(parent, RuleAction{ActionType: ActionCascadeTag, Value: ""}, evalCtx, result)
	if modified {
		t.Error("expected modified=false when cascade tag is invalid and no child actually changed")
	}
	if len(result.ModifiedTasks) != 0 {
		t.Errorf("expected no ModifiedTasks for invalid cascade tag, got %d", len(result.ModifiedTasks))
	}
	if len(childA.Tags) != 0 || len(childB.Tags) != 0 {
		t.Errorf("expected no tags on children for invalid value, got A=%v B=%v", childA.Tags, childB.Tags)
	}
}

// TestRecurrencePatternsEqual_WeekdayOrderIndependent verifies that
// recurrencePatternsEqual treats weekday slices as sets, not ordered lists.
// A task stored with weekdays [1, 5] (Mon, Fri) and a rule re-asserting
// "weekly:fri,mon" (parsed as [5, 1]) represent the same schedule but differ
// only in element order. Without this fix, ActionSetRecurrence returns true
// (modified) for every evaluation, spuriously churning UpdatedAt and triggering
// recursive rule passes.
func TestRecurrencePatternsEqual_WeekdayOrderIndependent(t *testing.T) {
	a := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 5}}
	b := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{5, 1}}
	if !recurrencePatternsEqual(a, b) {
		t.Error("expected recurrencePatternsEqual to return true for same weekdays in different order")
	}
}

// TestApplyAction_SetRecurrenceWeekdayOrderNoOp verifies that ActionSetRecurrence
// is a no-op when the task already has an equivalent weekly pattern with the same
// weekdays in a different order.
func TestApplyAction_SetRecurrenceWeekdayOrderNoOp(t *testing.T) {
	// Task stored as weekly Mon+Fri in sorted order
	existing := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 5}}
	task := &models.Task{ID: "1", Recurrence: existing}

	// Rule value lists the same days in reverse order
	modified := applyAction(task, RuleAction{ActionType: ActionSetRecurrence, Value: "weekly:fri,mon"})
	if modified {
		t.Error("expected applyAction to return false when recurrence weekdays are the same set in different order")
	}
}

// TestApplyAction_MoveToProjectSelfMoveNoOp verifies that ActionMoveToProject
// is a no-op when the destination matches the source project (by name or ID).
// Without this guard, a rule that moves a task to its own project would inflate
// trigger counts, mark the task modified, kick off a recursive re-evaluation
// pass, AND queue a destructive self-move that callers like processDeferredMoves
// would execute — dropping cross-project deps/relations and reassigning ShortIDs
// for no real change.
func TestApplyAction_MoveToProjectSelfMoveNoOp(t *testing.T) {
	task := &models.Task{ID: "task-1", Title: "Test"}
	rules := []WorkflowRule{
		{
			ID:      "r1",
			Name:    "Self-move",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{{ActionType: ActionMoveToProject, Value: "ProjectA"}},
		},
	}
	task.Status = models.StatusTodo

	evalCtx := &EvaluationContext{
		Changes:     []TaskChangeContext{{Task: task, Previous: nil}},
		AllTasks:    []*models.Task{task},
		TaskMap:     map[string]*models.Task{"task-1": task},
		ProjectName: "ProjectA",
		ProjectID:   "proj-a",
	}

	result := EvaluateRulesWithContext(evalCtx, rules)
	if len(result.DeferredMoves) != 0 {
		t.Errorf("expected no deferred moves for self-move, got %d", len(result.DeferredMoves))
	}
	if result.RuleTriggerCounts["r1"] != 0 {
		t.Errorf("expected trigger count 0 for no-op self-move, got %d", result.RuleTriggerCounts["r1"])
	}

	// Also verify match-by-ID works the same way.
	task2 := &models.Task{ID: "task-2", Title: "Test", Status: models.StatusTodo}
	rules[0].Actions[0].Value = "proj-a"
	evalCtx2 := &EvaluationContext{
		Changes:     []TaskChangeContext{{Task: task2, Previous: nil}},
		AllTasks:    []*models.Task{task2},
		TaskMap:     map[string]*models.Task{"task-2": task2},
		ProjectName: "ProjectA",
		ProjectID:   "proj-a",
	}
	result2 := EvaluateRulesWithContext(evalCtx2, rules)
	if len(result2.DeferredMoves) != 0 {
		t.Errorf("expected no deferred moves when destination matches ProjectID, got %d", len(result2.DeferredMoves))
	}
}

// TestApplyAction_SetSectionSilentValidationFailure is a regression test for a
// bug where ActionSetSection returned true (claiming the task was modified) even
// when task.SetSection silently rejected the value due to validation (whitespace,
// semicolons/colons in the name, over-length, or case-variant collision with a
// GTD section name). applyAction must only report success and bump UpdatedAt
// when the assignment actually took effect.
func TestApplyAction_SetSectionSilentValidationFailure(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"space in name", "has space"},
		{"tab in name", "has\ttab"},
		{"newline in name", "has\nnewline"},
		{"semicolon in name", "has;semi"},
		{"colon in name", "has:colon"},
		{"whitespace-only", "   "},
		{"case-variant GTD collision", "inbox"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			task := &models.Task{ID: "t1", Status: models.StatusTodo, UpdatedAt: stale}
			action := RuleAction{ActionType: ActionSetSection, Value: tc.value}

			modified := applyAction(task, action)

			if modified {
				t.Errorf("expected modified=false for invalid section %q (rejected by validation), got true", tc.value)
			}
			if task.CustomSection != "" {
				t.Errorf("expected CustomSection empty for invalid value %q, got %q", tc.value, task.CustomSection)
			}
			if !task.UpdatedAt.Equal(stale) {
				t.Errorf("expected UpdatedAt unchanged for invalid value %q, got %v", tc.value, task.UpdatedAt)
			}
		})
	}

	t.Run("valid custom section still reports modification", func(t *testing.T) {
		task := &models.Task{ID: "t1", Status: models.StatusTodo}
		action := RuleAction{ActionType: ActionSetSection, Value: "Sprint-1"}

		if !applyAction(task, action) {
			t.Fatal("expected modified=true for a valid custom section")
		}
		if task.CustomSection != "Sprint-1" {
			t.Errorf("expected CustomSection=Sprint-1, got %q", task.CustomSection)
		}
	})

	t.Run("invalid value does not overwrite prior custom section", func(t *testing.T) {
		// Precondition: task already has a valid custom section. An invalid
		// new value must leave CustomSection untouched and report modified=false.
		task := &models.Task{ID: "t1", CustomSection: "Sprint-1"}
		action := RuleAction{ActionType: ActionSetSection, Value: "bad name"}

		modified := applyAction(task, action)
		if modified {
			t.Error("expected modified=false when invalid value would no-op SetSection")
		}
		if task.CustomSection != "Sprint-1" {
			t.Errorf("expected CustomSection unchanged, got %q", task.CustomSection)
		}
	})
}

// TestEvaluateProjectRules_TriggeredIDsOnlyWinner verifies that when priority
// resolution selects a single winner, TriggeredRuleIDs contains only that
// winner's ID. Previously all matched rule IDs were included regardless of
// priority, causing callers to inflate trigger counts for overridden rules.
func TestEvaluateProjectRules_TriggeredIDsOnlyWinner(t *testing.T) {
	proj := &models.Project{ID: "proj-1", Name: "Test"}
	tasks := []*models.Task{
		{ID: "t1", Status: models.StatusTodo, Type: models.TaskTypeBug, UpdatedAt: time.Now()},
		{ID: "t2", Status: models.StatusTodo, Type: models.TaskTypeBug, UpdatedAt: time.Now()},
		{ID: "t3", Status: models.StatusTodo, Type: models.TaskTypeTask, UpdatedAt: time.Now()},
	}
	ctx := &ProjectEvaluationContext{Project: proj, Tasks: tasks}

	// r-bugfix (priority 30) should win; r-planning (priority 60) also matches
	// but loses. Only r-bugfix should appear in TriggeredRuleIDs.
	rules := []WorkflowRule{
		{
			ID: "r-bugfix", Name: "Bugfix Mode", Level: RuleLevelProject, Enabled: true, Priority: 30,
			Triggers:     []RuleTrigger{{Field: FieldMajorityTaskType, Operation: OpEquals, Value: "bug"}},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionSetProjectStatus, Value: "bugfix"}},
		},
		{
			ID: "r-planning", Name: "Planning", Level: RuleLevelProject, Enabled: true, Priority: 60,
			Triggers: []RuleTrigger{
				{Field: FieldActiveTaskCount, Operation: OpGreaterThan, Value: "0"},
				{Field: FieldHasInProgressTasks, Operation: OpEquals, Value: "false"},
			},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionSetProjectStatus, Value: "planning"}},
		},
	}

	result := EvaluateProjectRules(ctx, rules)
	if result.NewStatusIcon != "bugfix" {
		t.Fatalf("expected bugfix to win, got %q", result.NewStatusIcon)
	}
	if len(result.TriggeredRuleIDs) != 1 {
		t.Errorf("expected TriggeredRuleIDs to contain only the winner (1), got %d: %v",
			len(result.TriggeredRuleIDs), result.TriggeredRuleIDs)
	}
	if len(result.TriggeredRuleIDs) == 1 && result.TriggeredRuleIDs[0] != "r-bugfix" {
		t.Errorf("expected TriggeredRuleIDs[0] = r-bugfix, got %q", result.TriggeredRuleIDs[0])
	}
}

// TestApplyAction_AddCommentDedupScansAllComments verifies that
// ActionAddComment de-duplicates against ALL existing comments, not just the
// most recent one. Previously the dedup only inspected the last comment, so a
// matching workflow comment buried under unrelated user comments would be
// re-appended on every evaluation, inflating the comment list indefinitely.
func TestApplyAction_AddCommentDedupScansAllComments(t *testing.T) {
	workflowComment := "[Workflow] Auto-tagged"
	task := &models.Task{
		ID: "t1",
		Comments: []*models.Comment{
			{Text: workflowComment, CreatedAt: time.Now()},
			{Text: "User reply 1", CreatedAt: time.Now()},
			{Text: "User reply 2", CreatedAt: time.Now()},
		},
	}

	modified := applyAction(task, RuleAction{ActionType: ActionAddComment, Value: "Auto-tagged"})
	if modified {
		t.Errorf("expected modified=false because workflow comment already exists earlier in the list, got true")
	}
	if len(task.Comments) != 3 {
		t.Errorf("expected comments length unchanged (3), got %d", len(task.Comments))
	}
}

// TestMatchesTagTrigger_ContainsIsSubstringMatch verifies that the OpContains
// operator on tags performs substring matching (any tag contains the value),
// rather than exact membership (which is OpEquals semantics). Previously
// OpContains fell through to the OpEquals branch and therefore could not match
// partial tag names.
func TestMatchesTagTrigger_ContainsIsSubstringMatch(t *testing.T) {
	cases := []struct {
		name  string
		tags  []string
		value string
		op    TriggerOperation
		want  bool
	}{
		{"contains finds substring of a tag", []string{"backend-api"}, "backend", OpContains, true},
		{"contains finds tag as exact substring", []string{"urgent"}, "urgent", OpContains, true},
		{"contains finds partial substring", []string{"high-priority"}, "prio", OpContains, true},
		{"contains misses when no tag has substring", []string{"backend", "api"}, "frontend", OpContains, false},
		{"contains is case-insensitive", []string{"Backend"}, "BACK", OpContains, true},
		{"empty value never matches contains", []string{"any"}, "", OpContains, false},
		// OpEquals must remain exact membership: substring is NOT enough.
		{"equals does not match substring", []string{"backend-api"}, "backend", OpEquals, false},
		{"equals matches exact tag", []string{"backend"}, "backend", OpEquals, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trigger := RuleTrigger{Field: FieldTags, Operation: tc.op, Value: tc.value}
			got := matchesTagTrigger(tc.tags, trigger)
			if got != tc.want {
				t.Errorf("matchesTagTrigger(tags=%v, value=%q, op=%s) = %v, want %v",
					tc.tags, tc.value, tc.op, got, tc.want)
			}
		})
	}
}

// TestTaskStateChanged_RecurrenceChanges verifies that TaskStateChanged
// recognizes changes to Recurrence (added, removed, pattern modified, streak
// reset). Without this, recurring tasks whose ONLY change is recurrence state
// are excluded from EvaluationContext.Changes, so event-based triggers like
// FieldRecurrenceCompleted and FieldStreakBroken never see them.
func TestTaskStateChanged_RecurrenceChanges(t *testing.T) {
	weeklyA := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Streak: 5}
	weeklyB := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Streak: 5}
	weeklyStreakZero := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Streak: 0}
	daily := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	weeklyMon := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1}}
	weeklyMonFri := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 5}}
	weeklyMonFriReverse := &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{5, 1}}

	cases := []struct {
		name     string
		prev     *models.RecurrencePattern
		current  *models.RecurrencePattern
		expected bool
	}{
		{"identical pattern reports no change", weeklyA, weeklyB, false},
		{"recurrence cleared (completed) reports change", weeklyA, nil, true},
		{"recurrence added reports change", nil, weeklyA, true},
		{"streak reset reports change", weeklyA, weeklyStreakZero, true},
		{"unit change reports change", weeklyA, daily, true},
		{"weekdays added reports change", weeklyMon, weeklyMonFri, true},
		{"weekday order ignored", weeklyMonFri, weeklyMonFriReverse, false},
		{"both nil reports no change", nil, nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prev := &models.Task{ID: "t", Status: models.StatusTodo, Recurrence: tc.prev}
			cur := &models.Task{ID: "t", Status: models.StatusTodo, Recurrence: tc.current}
			got := TaskStateChanged(cur, prev)
			if got != tc.expected {
				t.Errorf("TaskStateChanged() = %v, want %v", got, tc.expected)
			}
		})
	}
}
