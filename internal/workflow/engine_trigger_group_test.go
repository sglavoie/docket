package workflow

import (
	"encoding/json"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestEvaluateRules_FlatTriggersBackwardCompat(t *testing.T) {
	// Rules using flat triggers (no TriggerGroup) should work unchanged
	tasks := []*models.Task{
		{ID: "1", Title: "Task", Priority: 1, Status: models.StatusTodo, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Flat triggers",
			Enabled: true,
			Triggers: []RuleTrigger{
				{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
				{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			},
			TriggerLogic: LogicAnd,
			Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "matched"}},
		},
	}

	EvaluateRules(tasks, rules)

	if !tasks[0].HasTag("matched") {
		t.Error("expected 'matched' tag with flat AND triggers")
	}
}

func TestEvaluateRules_TriggerGroupANDofORs(t *testing.T) {
	// (status=todo OR status=in_progress) AND (priority=1 OR priority=2)
	tasks := []*models.Task{
		{ID: "1", Title: "Todo P1", Priority: 1, Status: models.StatusTodo, Tags: []string{}},
		{ID: "2", Title: "IP P2", Priority: 2, Status: models.StatusInProgress, Tags: []string{}},
		{ID: "3", Title: "Todo P3", Priority: 3, Status: models.StatusTodo, Tags: []string{}},
		{ID: "4", Title: "Done P1", Priority: 1, Status: models.StatusDone, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "AND of ORs",
			Enabled: true,
			TriggerGroup: &TriggerGroup{
				Logic: LogicAnd,
				Groups: []TriggerGroup{
					{
						Logic: LogicOr,
						Triggers: []RuleTrigger{
							{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
							{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
						},
					},
					{
						Logic: LogicOr,
						Triggers: []RuleTrigger{
							{Field: FieldPriority, Operation: OpEquals, Value: "1"},
							{Field: FieldPriority, Operation: OpEquals, Value: "2"},
						},
					},
				},
			},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "matched"}},
		},
	}

	EvaluateRules(tasks, rules)

	// Task 1: todo AND p1 -> matches
	if !tasks[0].HasTag("matched") {
		t.Error("task 1 (todo, p1) should match")
	}
	// Task 2: in_progress AND p2 -> matches
	if !tasks[1].HasTag("matched") {
		t.Error("task 2 (in_progress, p2) should match")
	}
	// Task 3: todo AND p3 -> no match (p3 not in {1,2})
	if tasks[2].HasTag("matched") {
		t.Error("task 3 (todo, p3) should NOT match")
	}
	// Task 4: done AND p1 -> no match (done not in {todo, in_progress})
	if tasks[3].HasTag("matched") {
		t.Error("task 4 (done, p1) should NOT match")
	}
}

func TestEvaluateRules_TriggerGroupORofANDs(t *testing.T) {
	// (status=todo AND priority=1) OR (status=in_progress AND tags contains urgent)
	tasks := []*models.Task{
		{ID: "1", Title: "Todo P1", Priority: 1, Status: models.StatusTodo, Tags: []string{}},
		{ID: "2", Title: "IP urgent", Priority: 3, Status: models.StatusInProgress, Tags: []string{"urgent"}},
		{ID: "3", Title: "Todo P3", Priority: 3, Status: models.StatusTodo, Tags: []string{}},
		{ID: "4", Title: "IP not urgent", Priority: 3, Status: models.StatusInProgress, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "OR of ANDs",
			Enabled: true,
			TriggerGroup: &TriggerGroup{
				Logic: LogicOr,
				Groups: []TriggerGroup{
					{
						Logic: LogicAnd,
						Triggers: []RuleTrigger{
							{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
							{Field: FieldPriority, Operation: OpEquals, Value: "1"},
						},
					},
					{
						Logic: LogicAnd,
						Triggers: []RuleTrigger{
							{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
							{Field: FieldTags, Operation: OpContains, Value: "urgent"},
						},
					},
				},
			},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "matched"}},
		},
	}

	EvaluateRules(tasks, rules)

	if !tasks[0].HasTag("matched") {
		t.Error("task 1 (todo, p1) should match first group")
	}
	if !tasks[1].HasTag("matched") {
		t.Error("task 2 (in_progress, urgent) should match second group")
	}
	if tasks[2].HasTag("matched") {
		t.Error("task 3 (todo, p3) should NOT match")
	}
	if tasks[3].HasTag("matched") {
		t.Error("task 4 (in_progress, no urgent) should NOT match")
	}
}

func TestEvaluateRules_TriggerGroupMixedTriggersAndSubGroups(t *testing.T) {
	// Root AND: section=Inbox AND (status=todo OR status=blocked)
	tasks := []*models.Task{
		{ID: "1", Title: "Inbox todo", Status: models.StatusTodo, Section: "Inbox", Tags: []string{}},
		{ID: "2", Title: "Inbox blocked", Status: models.StatusBlocked, Section: "Inbox", Tags: []string{}},
		{ID: "3", Title: "Active todo", Status: models.StatusTodo, Section: "", Tags: []string{}},
		{ID: "4", Title: "Inbox done", Status: models.StatusDone, Section: "Inbox", Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Mixed",
			Enabled: true,
			TriggerGroup: &TriggerGroup{
				Logic: LogicAnd,
				Triggers: []RuleTrigger{
					{Field: FieldSection, Operation: OpEquals, Value: "Inbox"},
				},
				Groups: []TriggerGroup{
					{
						Logic: LogicOr,
						Triggers: []RuleTrigger{
							{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
							{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
						},
					},
				},
			},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "matched"}},
		},
	}

	EvaluateRules(tasks, rules)

	if !tasks[0].HasTag("matched") {
		t.Error("task 1 (Inbox, todo) should match")
	}
	if !tasks[1].HasTag("matched") {
		t.Error("task 2 (Inbox, blocked) should match")
	}
	if tasks[2].HasTag("matched") {
		t.Error("task 3 (Active, todo) should NOT match (wrong section)")
	}
	if tasks[3].HasTag("matched") {
		t.Error("task 4 (Inbox, done) should NOT match (wrong status)")
	}
}

func TestTriggerGroup_JSONRoundTrip(t *testing.T) {
	rule := WorkflowRule{
		ID:      "test-id",
		Name:    "JSON Test",
		Enabled: true,
		TriggerGroup: &TriggerGroup{
			Logic: LogicAnd,
			Triggers: []RuleTrigger{
				{Field: FieldSection, Operation: OpEquals, Value: "Inbox"},
			},
			Groups: []TriggerGroup{
				{
					Logic: LogicOr,
					Triggers: []RuleTrigger{
						{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
						{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
					},
				},
			},
		},
		Actions: []RuleAction{{ActionType: ActionAddTag, Value: "test"}},
	}

	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded WorkflowRule
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.TriggerGroup == nil {
		t.Fatal("TriggerGroup should not be nil after round-trip")
	}
	if decoded.TriggerGroup.Logic != LogicAnd {
		t.Errorf("expected root logic AND, got %s", decoded.TriggerGroup.Logic)
	}
	if len(decoded.TriggerGroup.Triggers) != 1 {
		t.Errorf("expected 1 root trigger, got %d", len(decoded.TriggerGroup.Triggers))
	}
	if len(decoded.TriggerGroup.Groups) != 1 {
		t.Errorf("expected 1 sub-group, got %d", len(decoded.TriggerGroup.Groups))
	}
	subGroup := decoded.TriggerGroup.Groups[0]
	if subGroup.Logic != LogicOr {
		t.Errorf("expected sub-group logic OR, got %s", subGroup.Logic)
	}
	if len(subGroup.Triggers) != 2 {
		t.Errorf("expected 2 triggers in sub-group, got %d", len(subGroup.Triggers))
	}
}

func TestTriggerGroup_ValidateRejectsDeepNesting(t *testing.T) {
	// Build nesting deeper than maxTriggerGroupDepth (5)
	deepest := TriggerGroup{
		Logic:    LogicAnd,
		Triggers: []RuleTrigger{{Field: FieldStatus, Operation: OpEquals, Value: "todo"}},
	}
	// Wrap it 5 times to reach depth 6 (exceeds limit of 5)
	for range 5 {
		deepest = TriggerGroup{
			Logic:  LogicAnd,
			Groups: []TriggerGroup{deepest},
		}
	}

	err := deepest.Validate()
	if err == nil {
		t.Error("expected validation error for nesting depth > 5")
	}
}

func TestTriggerGroup_ValidateAcceptsMaxDepth(t *testing.T) {
	// Build nesting at exactly maxTriggerGroupDepth - 1 (4 levels of wrapping = depth 4, valid)
	deepest := TriggerGroup{
		Logic:    LogicAnd,
		Triggers: []RuleTrigger{{Field: FieldStatus, Operation: OpEquals, Value: "todo"}},
	}
	for range 4 {
		deepest = TriggerGroup{
			Logic:  LogicAnd,
			Groups: []TriggerGroup{deepest},
		}
	}

	err := deepest.Validate()
	if err != nil {
		t.Errorf("expected no error at depth 4, got: %v", err)
	}
}

func TestTriggerGroup_ValidateRejectsEmpty(t *testing.T) {
	g := TriggerGroup{Logic: LogicAnd}
	err := g.Validate()
	if err == nil {
		t.Error("expected validation error for empty trigger group")
	}
}

func TestTriggerGroup_Summary(t *testing.T) {
	g := TriggerGroup{
		Logic: LogicAnd,
		Triggers: []RuleTrigger{
			{Field: FieldSection, Operation: OpEquals, Value: "Inbox"},
		},
		Groups: []TriggerGroup{
			{
				Logic: LogicOr,
				Triggers: []RuleTrigger{
					{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
					{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
				},
			},
		},
	}

	summary := g.Summary()
	expected := "section = Inbox AND (status = todo OR status = blocked)"
	if summary != expected {
		t.Errorf("expected %q, got %q", expected, summary)
	}
}

func TestRuleSummary_WithTriggerGroup(t *testing.T) {
	rule := WorkflowRule{
		Name: "Grouped Rule",
		TriggerGroup: &TriggerGroup{
			Logic: LogicOr,
			Groups: []TriggerGroup{
				{
					Logic: LogicAnd,
					Triggers: []RuleTrigger{
						{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
						{Field: FieldPriority, Operation: OpEquals, Value: "1"},
					},
				},
				{
					Logic: LogicAnd,
					Triggers: []RuleTrigger{
						{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
						{Field: FieldTags, Operation: OpContains, Value: "urgent"},
					},
				},
			},
		},
		Actions: []RuleAction{{ActionType: ActionAddTag, Value: "flagged"}},
	}

	summary := rule.Summary()
	expected := "When ((status = todo AND priority = 1) OR (status = blocked AND tags contains urgent)): add tag #flagged"
	if summary != expected {
		t.Errorf("expected %q, got %q", expected, summary)
	}
}

func TestWorkflowRule_ValidateWithTriggerGroup(t *testing.T) {
	rule := WorkflowRule{
		ID:   "test",
		Name: "Test",
		TriggerGroup: &TriggerGroup{
			Logic: LogicAnd,
			Triggers: []RuleTrigger{
				{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			},
		},
		Actions: []RuleAction{{ActionType: ActionAddTag, Value: "test"}},
	}

	err := rule.Validate()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestWorkflowRule_ValidateWithInvalidTriggerGroup(t *testing.T) {
	rule := WorkflowRule{
		ID:           "test",
		Name:         "Test",
		TriggerGroup: &TriggerGroup{Logic: LogicAnd}, // empty group
		Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "test"}},
	}

	err := rule.Validate()
	if err == nil {
		t.Error("expected validation error for empty trigger group")
	}
}

func TestGetTriggerGroup_SynthesizesFromFlat(t *testing.T) {
	rule := WorkflowRule{
		Triggers: []RuleTrigger{
			{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		},
		TriggerLogic: LogicOr,
	}

	tg := rule.GetTriggerGroup()
	if tg == nil {
		t.Fatal("GetTriggerGroup should not return nil for flat triggers")
	}
	if tg.Logic != LogicOr {
		t.Errorf("expected logic OR, got %s", tg.Logic)
	}
	if len(tg.Triggers) != 2 {
		t.Errorf("expected 2 triggers, got %d", len(tg.Triggers))
	}
	if len(tg.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(tg.Groups))
	}
}

func TestGetTriggerGroup_ReturnsTriggerGroupDirectly(t *testing.T) {
	expected := &TriggerGroup{
		Logic: LogicAnd,
		Groups: []TriggerGroup{
			{Logic: LogicOr, Triggers: []RuleTrigger{{Field: FieldStatus, Operation: OpEquals, Value: "todo"}}},
		},
	}
	rule := WorkflowRule{
		TriggerGroup: expected,
		// These should be ignored when TriggerGroup is set
		Triggers:     []RuleTrigger{{Field: FieldPriority, Operation: OpEquals, Value: "1"}},
		TriggerLogic: LogicOr,
	}

	tg := rule.GetTriggerGroup()
	if tg != expected {
		t.Error("GetTriggerGroup should return TriggerGroup directly when set")
	}
}
