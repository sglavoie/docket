package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestEventTrigger_CustomSectionChanged(t *testing.T) {
	prevTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusInProgress,
		CustomSection: "Sprint-1",
	}
	currTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusInProgress,
		CustomSection: "Sprint-2",
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: currTask, Previous: prevTask}},
		AllTasks: []*models.Task{currTask},
		TaskMap:  map[string]*models.Task{"task-1": currTask},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Custom section change",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldSectionChanged, Operation: OpEquals, Value: "Sprint-1->Sprint-2"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "moved"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Fatalf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !currTask.HasTag("moved") {
		t.Error("expected task to have 'moved' tag from custom section change trigger")
	}
}

func TestEventTrigger_CustomSectionChanged_WildcardPattern(t *testing.T) {
	prevTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusTodo,
		CustomSection: "Sprint-1",
	}
	currTask := &models.Task{
		ID:            "task-1",
		Status:        models.StatusTodo,
		CustomSection: "Sprint-2",
	}

	evalCtx := &EvaluationContext{
		Changes:  []TaskChangeContext{{Task: currTask, Previous: prevTask}},
		AllTasks: []*models.Task{currTask},
		TaskMap:  map[string]*models.Task{"task-1": currTask},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Any custom section change",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldSectionChanged, Operation: OpEquals, Value: "*->*"},
			Actions: []RuleAction{{ActionType: ActionAddTag, Value: "section-changed"}},
		},
	}

	result := EvaluateRulesWithContext(evalCtx, rules)

	if len(result.ModifiedTasks) != 1 {
		t.Fatalf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
	if !currTask.HasTag("section-changed") {
		t.Error("expected task to have 'section-changed' tag from wildcard section change")
	}
}

func TestMatchesEventTrigger_SectionChanged_CustomSectionDoesNotFalseMatch(t *testing.T) {
	t.Run("GTD to custom does not trigger activate-on-active pattern", func(t *testing.T) {
		prev := &models.Task{
			ID:      "t1",
			Section: "Next", CustomSection: "",
		}
		curr := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		trigger := RuleTrigger{
			Field: FieldSectionChanged, Operation: OpEquals,
			Value: "*->",
		}
		if matchesEventTrigger(curr, prev, trigger) {
			t.Error("GTD-to-custom should NOT trigger *-> (activate) pattern")
		}
	})

	t.Run("GTD to custom triggers wildcard pattern", func(t *testing.T) {
		prev := &models.Task{
			ID:      "t1",
			Section: "Next", CustomSection: "",
		}
		curr := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		trigger := RuleTrigger{
			Field: FieldSectionChanged, Operation: OpEquals,
			Value: "*->*",
		}
		if !matchesEventTrigger(curr, prev, trigger) {
			t.Error("GTD-to-custom should trigger *->* (any change) pattern")
		}
	})

	t.Run("GTD to custom matches specific transition", func(t *testing.T) {
		prev := &models.Task{
			ID:      "t1",
			Section: "Next", CustomSection: "",
		}
		curr := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		trigger := RuleTrigger{
			Field: FieldSectionChanged, Operation: OpEquals,
			Value: "Next->Sprint-1",
		}
		if !matchesEventTrigger(curr, prev, trigger) {
			t.Error("should match exact transition Next->Sprint-1")
		}
	})

	t.Run("custom to custom detects change", func(t *testing.T) {
		prev := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		curr := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-2",
		}
		trigger := RuleTrigger{
			Field: FieldSectionChanged, Operation: OpEquals,
			Value: "*->*",
		}
		if !matchesEventTrigger(curr, prev, trigger) {
			t.Error("custom-to-custom should trigger *->* pattern")
		}
	})

	t.Run("Active to custom triggers deactivate pattern", func(t *testing.T) {
		prev := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "",
		}
		curr := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		trigger := RuleTrigger{
			Field: FieldSectionChanged, Operation: OpEquals,
			Value: "->*",
		}
		if !matchesEventTrigger(curr, prev, trigger) {
			t.Error("Active-to-custom should trigger ->* (deactivate) pattern")
		}
	})

	t.Run("custom to Active triggers activate pattern", func(t *testing.T) {
		prev := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		curr := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "",
		}
		trigger := RuleTrigger{
			Field: FieldSectionChanged, Operation: OpEquals,
			Value: "*->",
		}
		if !matchesEventTrigger(curr, prev, trigger) {
			t.Error("custom-to-Active should trigger *-> (activate) pattern")
		}
	})

	t.Run("no change does not trigger", func(t *testing.T) {
		prev := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		curr := &models.Task{
			ID:      "t1",
			Section: "", CustomSection: "Sprint-1",
		}
		trigger := RuleTrigger{
			Field: FieldSectionChanged, Operation: OpEquals,
			Value: "*->*",
		}
		if matchesEventTrigger(curr, prev, trigger) {
			t.Error("same section should NOT trigger change event")
		}
	})
}

func TestMatchesTrigger_SectionUsesEffectiveSection(t *testing.T) {
	t.Run("contains matches GTD section", func(t *testing.T) {
		task := &models.Task{ID: "t1", Section: "Next", CustomSection: ""}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpContains, Value: "ext"}
		if !matchesTrigger(task, trigger) {
			t.Error("'Next' contains 'ext', should match")
		}
	})

	t.Run("contains matches custom section", func(t *testing.T) {
		task := &models.Task{ID: "t1", Section: "", CustomSection: "Sprint-1"}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpContains, Value: "Sprint"}
		if !matchesTrigger(task, trigger) {
			t.Error("'Sprint-1' contains 'Sprint', should match")
		}
	})

	t.Run("not_equals GTD section for task in custom section", func(t *testing.T) {
		task := &models.Task{ID: "t1", Section: "", CustomSection: "Sprint-1"}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpNotEquals, Value: "Inbox"}
		if !matchesTrigger(task, trigger) {
			t.Error("task in Sprint-1 should not_equal Inbox")
		}
	})

	t.Run("equals empty does not match custom section task", func(t *testing.T) {
		task := &models.Task{ID: "t1", Section: "", CustomSection: "Sprint-1"}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""}
		if matchesTrigger(task, trigger) {
			t.Error("task in Sprint-1 should NOT match section equals ''")
		}
	})

	t.Run("equals empty matches Active view task", func(t *testing.T) {
		task := &models.Task{ID: "t1", Section: "", CustomSection: ""}
		trigger := RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""}
		if !matchesTrigger(task, trigger) {
			t.Error("task in Active view should match section equals ''")
		}
	})
}
