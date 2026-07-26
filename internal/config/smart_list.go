package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// SmartListCriterion represents a single filter criterion.
// Used by both the legacy smartList type and the unified SavedFilter.
type SmartListCriterion struct {
	Field     string                  `json:"field"`     // type, state, priority, tags, due_date, score_min, actionable, parked, created_days, updated_days, completed_days
	Operation string                  `json:"operation"` // equals, not_equals, contains, greater_than, less_than, in, not_in
	Value     string                  `json:"value"`
	States    models.WorkflowStateSet `json:"-"`

	legacyLifecycleDecoded bool
}

type smartListCriterionWire struct {
	Field     string                  `json:"field"`
	Operation string                  `json:"operation"`
	Value     string                  `json:"value,omitempty"`
	States    *[]models.WorkflowState `json:"states,omitempty"`
}

// MarshalJSON writes lifecycle criteria only in canonical state-set form.
func (c SmartListCriterion) MarshalJSON() ([]byte, error) {
	canonical, _ := canonicalizeSmartListCriterion(c)
	if err := canonical.validate(); err != nil {
		return nil, err
	}

	wire := smartListCriterionWire{
		Field:     canonical.Field,
		Operation: canonical.Operation,
		Value:     canonical.Value,
	}
	if canonical.Field == "state" {
		states := canonical.States.Values()
		wire.States = &states
	}
	return json.Marshal(wire)
}

// UnmarshalJSON accepts legacy status/section lifecycle predicates and
// immediately converts them to canonical state-set criteria.
func (c *SmartListCriterion) UnmarshalJSON(data []byte) error {
	var wire smartListCriterionWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	decoded := SmartListCriterion{
		Field:     wire.Field,
		Operation: wire.Operation,
		Value:     wire.Value,
	}
	if wire.Field == "state" && wire.States != nil {
		states, err := models.NewWorkflowStateSet((*wire.States)...)
		if err != nil {
			return fmt.Errorf("state criterion: %w", err)
		}
		decoded.States = states
	}

	canonical, changed := canonicalizeSmartListCriterion(decoded)
	if err := canonical.validate(); err != nil {
		return err
	}
	canonical.legacyLifecycleDecoded = changed
	*c = canonical
	return nil
}

func (c SmartListCriterion) validate() error {
	if c.Field != "state" {
		return nil
	}
	if c.Operation != "in" && c.Operation != "not_in" {
		return fmt.Errorf("state criterion has invalid operation %q", c.Operation)
	}
	if c.Value != "" {
		return fmt.Errorf("state criterion must use states instead of value")
	}
	return nil
}

func canonicalizeSmartListCriterion(c SmartListCriterion) (SmartListCriterion, bool) {
	switch c.Field {
	case "status", "section":
		states := legacyCriterionStates(c.Field, c.Operation, c.Value)
		operation := "in"
		if c.Operation == "not_equals" {
			operation = "not_in"
			states = legacyCriterionBaseStates(c.Field, c.Value)
		}
		return SmartListCriterion{
			Field:     "state",
			Operation: operation,
			States:    states,
		}, true
	case "state":
		return c, false
	default:
		return c, false
	}
}

func legacyCriterionBaseStates(field, value string) models.WorkflowStateSet {
	return legacyCriterionStates(field, "equals", value)
}

func legacyCriterionStates(field, operation, value string) models.WorkflowStateSet {
	var matches []models.WorkflowState
	for _, state := range models.AllWorkflowStates() {
		actual := ""
		switch field {
		case "status":
			actual = string(models.LegacyStatusForState(state))
		case "section":
			actual = models.SectionForState(state)
		}
		if legacyCriterionStringMatch(actual, operation, value) {
			matches = append(matches, state)
		}
	}
	states, err := models.NewWorkflowStateSet(matches...)
	if err != nil {
		panic(err)
	}
	return states
}

func legacyCriterionStringMatch(actual, operation, target string) bool {
	switch operation {
	case "equals":
		return strings.EqualFold(actual, target)
	case "not_equals":
		return !strings.EqualFold(actual, target)
	case "contains":
		return strings.Contains(strings.ToLower(actual), strings.ToLower(target))
	default:
		return false
	}
}

// smartList is a legacy type kept for JSON deserialization during migration.
// New code should use SavedFilter instead.
type smartList struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Criteria         []SmartListCriterion `json:"criteria"`
	Logic            string               `json:"logic,omitempty"` // "and" (default) or "or"
	SortField        int                  `json:"sort_field,omitempty"`
	SortAscending    bool                 `json:"sort_ascending"`
	IncludeCompleted bool                 `json:"include_completed,omitempty"`
}

// EffectiveLogic returns the logic operator, defaulting to "and".
func (s *smartList) EffectiveLogic() string {
	if s.Logic == "or" {
		return "or"
	}
	return "and"
}
