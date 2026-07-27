package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

const (
	legacyFieldStatus             TriggerField = "status"
	legacyFieldSection            TriggerField = "section"
	legacyFieldStatusChanged      TriggerField = "status_changed"
	legacyFieldSectionChanged     TriggerField = "section_changed"
	legacyFieldParentStatus       TriggerField = "parent_status"
	legacyFieldAllTasksStatus     TriggerField = "all_tasks_status"
	legacyFieldMajorityTaskStatus TriggerField = "majority_task_status"
	legacyFieldHasTasksWithStatus TriggerField = "has_tasks_with_status"
	legacyActionSetStatus         ActionType   = "set_status"
	legacyActionSetSection        ActionType   = "set_section"
	legacyStatusTodo                           = "todo"
	legacyStatusInProgress                     = "in_progress"
	legacyStatusBlocked                        = "blocked"
	legacyStatusStandby                        = "standby"
	legacyStatusReview                         = "review"
	legacyStatusMaybe                          = "maybe"
	legacyStatusDone                           = "done"
	legacyStatusCanceled                       = "canceled"
	legacySectionInbox                         = "Inbox"
	legacySectionNext                          = "Next"
	legacySectionWaiting                       = "Waiting"
	legacySectionStandby                       = "Standby"
	legacySectionReview                        = "Review"
	legacySectionSomeday                       = "Someday"
	legacySectionCompleted                     = "Completed"
	legacySectionArchive                       = "Archive"
	legacyDefaultArchiveRuleID                 = "def-archive"
)

var legacyStatusValues = []string{
	legacyStatusTodo,
	legacyStatusInProgress,
	legacyStatusBlocked,
	legacyStatusStandby,
	legacyStatusReview,
	legacyStatusMaybe,
	legacyStatusDone,
	legacyStatusCanceled,
}

// MigrateLegacyLifecycleRules returns an independently mutable canonical copy
// of rules. It rewrites every active and nested legacy lifecycle predicate,
// action condition, action, and scope while preserving rule identity, metadata,
// ordering, and trigger-group shape.
func MigrateLegacyLifecycleRules(rules []WorkflowRule) ([]WorkflowRule, bool, error) {
	migrated := make([]WorkflowRule, len(rules))
	changed := false
	for i, rule := range rules {
		canonical, ruleChanged, err := migrateLegacyLifecycleRule(rule)
		if err != nil {
			return nil, false, fmt.Errorf("workflow rule %d %q: %w", i, rule.Name, err)
		}
		migrated[i] = canonical
		changed = ruleChanged || changed
	}
	return migrated, changed, nil
}

func migrateLegacyLifecycleRule(rule WorkflowRule) (WorkflowRule, bool, error) {
	cloned, err := cloneWorkflowRule(rule)
	if err != nil {
		return WorkflowRule{}, false, err
	}
	changed := false

	if cloned.legacyTrigger.Field != "" {
		if len(cloned.Triggers) == 0 {
			cloned.Triggers = []RuleTrigger{cloned.legacyTrigger}
		}
		cloned.legacyTrigger = RuleTrigger{}
		changed = true
	}

	// TriggerGroup is authoritative when present. Retired flat clauses beside
	// it are inactive compatibility residue, so dropping them preserves
	// behavior and prevents an unrepresentable inactive aggregate from blocking
	// an otherwise valid migration.
	if cloned.TriggerGroup != nil {
		if len(cloned.Triggers) > 0 {
			cloned.Triggers = nil
			changed = true
		}
		groupChanged, migrateErr := migrateLegacyTriggerGroup(cloned.TriggerGroup)
		if migrateErr != nil {
			return WorkflowRule{}, false, migrateErr
		}
		changed = groupChanged || changed
	} else {
		triggerGroup, flatTriggers, flatChanged, migrateErr := migrateLegacyFlatTriggers(
			cloned.Triggers,
			cloned.GetTriggerLogic(),
		)
		if migrateErr != nil {
			return WorkflowRule{}, false, fmt.Errorf("flat triggers: %w", migrateErr)
		}
		if triggerGroup != nil {
			cloned.TriggerGroup = triggerGroup
			cloned.Triggers = nil
		} else {
			cloned.Triggers = flatTriggers
		}
		changed = flatChanged || changed
	}
	for i := range cloned.Actions {
		legacyAction := cloned.Actions[i]
		// The shipped v3 def-archive rule routed completed tasks into a custom
		// "Archive" section. Canonical v4 has no custom workflow sections; the
		// current rule with this stable ID keeps completed tasks in Done.
		// Restrict the rewrite to the exact built-in rule so a user-authored
		// custom-section action is never reinterpreted as task completion.
		if cloned.ID == legacyDefaultArchiveRuleID &&
			cloned.Source == "default" &&
			legacyAction.ActionType == legacyActionSetSection &&
			strings.EqualFold(strings.TrimSpace(legacyAction.Value), legacySectionArchive) {
			legacyAction.ActionType = legacyActionSetStatus
			legacyAction.Value = legacyStatusDone
		}
		action, actionChanged, migrateErr := migrateLegacyAction(legacyAction)
		if migrateErr != nil {
			return WorkflowRule{}, false, fmt.Errorf("action %d: %w", i, migrateErr)
		}
		cloned.Actions[i] = action
		changed = actionChanged || changed
	}
	if cloned.Scope != nil && len(cloned.Scope.legacySections) > 0 {
		sectionStates, migrateErr := legacySectionsToStates(cloned.Scope.legacySections)
		if migrateErr != nil {
			return WorkflowRule{}, false, fmt.Errorf("scope: %w", migrateErr)
		}
		if len(cloned.Scope.States) > 0 {
			existing, setErr := models.NewWorkflowStateSet(cloned.Scope.States...)
			if setErr != nil {
				return WorkflowRule{}, false, fmt.Errorf("scope states: %w", setErr)
			}
			sectionStates = existing.Intersection(sectionStates)
			if sectionStates.IsEmpty() {
				return WorkflowRule{}, false, fmt.Errorf("canonical state and legacy section scopes have an empty intersection")
			}
		}
		cloned.Scope.States = sectionStates.Values()
		cloned.Scope.legacySections = nil
		changed = true
	}

	if err := cloned.Validate(); err != nil {
		return WorkflowRule{}, false, fmt.Errorf("migrated rule is invalid: %w", err)
	}
	if err := validateCanonicalLifecycleRule(cloned); err != nil {
		return WorkflowRule{}, false, err
	}
	return cloned, changed, nil
}

func cloneWorkflowRule(rule WorkflowRule) (WorkflowRule, error) {
	data, err := json.Marshal(rule)
	if err != nil {
		return WorkflowRule{}, fmt.Errorf("clone rule: %w", err)
	}
	var cloned WorkflowRule
	if err := json.Unmarshal(data, &cloned); err != nil {
		return WorkflowRule{}, fmt.Errorf("clone rule: %w", err)
	}
	cloned.legacyTrigger = rule.legacyTrigger
	if cloned.Scope != nil && rule.Scope != nil {
		cloned.Scope.legacySections = append([]string(nil), rule.Scope.legacySections...)
	}
	return cloned, nil
}

func migrateLegacyTriggerGroup(group *TriggerGroup) (bool, error) {
	changed := false
	migratedTriggers := make([]RuleTrigger, 0, len(group.Triggers))
	expandedGroups := make([]TriggerGroup, 0)
	for i := range group.Triggers {
		clause, err := migrateLegacyTriggerClause(group.Triggers[i])
		if err != nil {
			return false, fmt.Errorf("trigger group leaf %d: %w", i, err)
		}
		if clause.group != nil {
			expandedGroups = append(expandedGroups, *clause.group)
		} else {
			migratedTriggers = append(migratedTriggers, clause.trigger)
		}
		changed = clause.changed || changed
	}
	group.Triggers = migratedTriggers

	for i := range group.Groups {
		groupChanged, err := migrateLegacyTriggerGroup(&group.Groups[i])
		if err != nil {
			return false, fmt.Errorf("trigger subgroup %d: %w", i, err)
		}
		changed = groupChanged || changed
	}
	if len(expandedGroups) > 0 {
		group.Groups = append(expandedGroups, group.Groups...)
		changed = true
	}
	return changed, nil
}

type migratedTriggerClause struct {
	trigger RuleTrigger
	group   *TriggerGroup
	changed bool
}

func migrateLegacyFlatTriggers(
	triggers []RuleTrigger,
	logic TriggerLogic,
) (*TriggerGroup, []RuleTrigger, bool, error) {
	migratedTriggers := make([]RuleTrigger, 0, len(triggers))
	expandedGroups := make([]TriggerGroup, 0)
	changed := false
	for i, trigger := range triggers {
		clause, err := migrateLegacyTriggerClause(trigger)
		if err != nil {
			return nil, nil, false, fmt.Errorf("trigger %d: %w", i, err)
		}
		if clause.group != nil {
			expandedGroups = append(expandedGroups, *clause.group)
		} else {
			migratedTriggers = append(migratedTriggers, clause.trigger)
		}
		changed = clause.changed || changed
	}
	if len(expandedGroups) == 0 {
		return nil, migratedTriggers, changed, nil
	}
	return &TriggerGroup{
		Logic:    logic,
		Triggers: migratedTriggers,
		Groups:   expandedGroups,
	}, nil, true, nil
}

func migrateLegacyTriggerClause(trigger RuleTrigger) (migratedTriggerClause, error) {
	if trigger.Field == legacyFieldAllTasksStatus || trigger.Field == legacyFieldMajorityTaskStatus {
		alternatives, err := migrateLegacyAggregateStatusTrigger(trigger)
		if err != nil {
			return migratedTriggerClause{}, err
		}
		if len(alternatives) == 1 {
			return migratedTriggerClause{trigger: alternatives[0], changed: true}, nil
		}
		return migratedTriggerClause{
			group:   &TriggerGroup{Logic: LogicOr, Triggers: alternatives},
			changed: true,
		}, nil
	}
	if trigger.Field == legacyFieldStatusChanged || trigger.Field == legacyFieldSectionChanged {
		alternatives, err := migrateLegacyTransitionTrigger(trigger)
		if err != nil {
			return migratedTriggerClause{}, err
		}
		if len(alternatives) == 1 {
			return migratedTriggerClause{trigger: alternatives[0], changed: true}, nil
		}
		return migratedTriggerClause{
			group:   &TriggerGroup{Logic: LogicOr, Triggers: alternatives},
			changed: true,
		}, nil
	}

	migrated, changed, err := migrateLegacyTrigger(trigger)
	if err != nil {
		return migratedTriggerClause{}, err
	}
	return migratedTriggerClause{trigger: migrated, changed: changed}, nil
}

func migrateLegacyTrigger(trigger RuleTrigger) (RuleTrigger, bool, error) {
	switch trigger.Field {
	case legacyFieldAllTasksStatus, legacyFieldMajorityTaskStatus:
		alternatives, err := migrateLegacyAggregateStatusTrigger(trigger)
		if err != nil {
			return RuleTrigger{}, false, err
		}
		if len(alternatives) != 1 {
			return RuleTrigger{}, false, fmt.Errorf(
				"legacy aggregate predicate %s %s %q requires grouped rule transformation and is unsupported in an action condition",
				trigger.Field,
				trigger.Operation,
				trigger.Value,
			)
		}
		return alternatives[0], true, nil

	case legacyFieldStatus, legacyFieldParentStatus, legacyFieldHasTasksWithStatus:
		field := map[TriggerField]TriggerField{
			legacyFieldStatus:             FieldState,
			legacyFieldParentStatus:       FieldParentState,
			legacyFieldHasTasksWithStatus: FieldHasTasksWithState,
		}[trigger.Field]
		states, operation, err := migrateLegacyProjectionPredicate(
			trigger.Operation,
			trigger.Value,
			legacyStatusStates,
			legacyStatusForState,
		)
		if err != nil {
			return RuleTrigger{}, false, err
		}
		return RuleTrigger{Field: field, Operation: operation, Value: workflowStateSetString(states)}, true, nil

	case legacyFieldSection:
		states, operation, err := migrateLegacyProjectionPredicate(
			trigger.Operation,
			trigger.Value,
			legacySectionStates,
			legacySectionForState,
		)
		if err != nil {
			return RuleTrigger{}, false, err
		}
		return RuleTrigger{Field: FieldState, Operation: operation, Value: workflowStateSetString(states)}, true, nil

	case legacyFieldStatusChanged:
		alternatives, err := migrateLegacyTransitionTrigger(trigger)
		if err != nil {
			return RuleTrigger{}, false, err
		}
		if len(alternatives) != 1 {
			return RuleTrigger{}, false, fmt.Errorf(
				"legacy transition predicate %s %q requires grouped rule transformation and is unsupported in an action condition",
				trigger.Field,
				trigger.Value,
			)
		}
		return alternatives[0], true, nil

	case legacyFieldSectionChanged:
		alternatives, err := migrateLegacyTransitionTrigger(trigger)
		if err != nil {
			return RuleTrigger{}, false, err
		}
		if len(alternatives) != 1 {
			return RuleTrigger{}, false, fmt.Errorf(
				"legacy transition predicate %s %q requires grouped rule transformation and is unsupported in an action condition",
				trigger.Field,
				trigger.Value,
			)
		}
		return alternatives[0], true, nil

	default:
		return trigger, false, nil
	}
}

func migrateLegacyAggregateStatusTrigger(trigger RuleTrigger) ([]RuleTrigger, error) {
	var canonicalField TriggerField
	switch trigger.Field {
	case legacyFieldAllTasksStatus:
		canonicalField = FieldAllTasksState
	case legacyFieldMajorityTaskStatus:
		canonicalField = FieldMajorityTaskState
	default:
		return nil, fmt.Errorf("field %q is not a legacy aggregate status predicate", trigger.Field)
	}

	var alternatives []RuleTrigger
	for _, status := range legacyStatusValues {
		if !legacyStatusPredicateMatches(status, trigger.Operation, trigger.Value) {
			continue
		}
		states, err := legacyStatusStates(status)
		if err != nil {
			return nil, err
		}
		operation := OpEquals
		if states.Len() > 1 {
			operation = OpIn
		}
		alternatives = append(alternatives, RuleTrigger{
			Field:     canonicalField,
			Operation: operation,
			Value:     workflowStateSetString(states),
		})
	}
	if len(alternatives) == 0 {
		return nil, fmt.Errorf(
			"legacy aggregate predicate %s %s %q matches no representable status bucket",
			trigger.Field,
			trigger.Operation,
			trigger.Value,
		)
	}
	return alternatives, nil
}

func legacyStatusPredicateMatches(actual string, operation TriggerOperation, target string) bool {
	switch operation {
	case OpEquals:
		return strings.EqualFold(actual, target)
	case OpNotEquals:
		return !strings.EqualFold(actual, target)
	case OpContains:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(target))
	case OpGreaterThan:
		return actual > target
	case OpLessThan:
		return actual < target
	default:
		return false
	}
}

type legacyTransitionBucket struct {
	label  string
	states models.WorkflowStateSet
}

func migrateLegacyTransitionTrigger(trigger RuleTrigger) ([]RuleTrigger, error) {
	var buckets []legacyTransitionBucket
	switch trigger.Field {
	case legacyFieldStatusChanged:
		for _, status := range legacyStatusValues {
			states, err := legacyStatusStates(status)
			if err != nil {
				return nil, err
			}
			buckets = append(buckets, legacyTransitionBucket{
				label:  status,
				states: states,
			})
		}
	case legacyFieldSectionChanged:
		for _, section := range []string{
			"",
			legacySectionInbox,
			legacySectionNext,
			legacySectionWaiting,
			legacySectionStandby,
			legacySectionReview,
			legacySectionSomeday,
			legacySectionCompleted,
		} {
			states, err := legacySectionStates(section)
			if err != nil {
				return nil, err
			}
			buckets = append(buckets, legacyTransitionBucket{
				label:  section,
				states: states,
			})
		}
	default:
		return nil, fmt.Errorf("field %q is not a legacy lifecycle transition", trigger.Field)
	}

	if strings.Count(trigger.Value, "->") != 1 {
		return nil, fmt.Errorf("legacy transition must use from->to")
	}
	parts := strings.SplitN(trigger.Value, "->", 2)
	fromPattern := strings.TrimSpace(parts[0])
	toPattern := strings.TrimSpace(parts[1])
	if trigger.Field == legacyFieldSectionChanged {
		if strings.EqualFold(fromPattern, "Active") {
			fromPattern = ""
		}
		if strings.EqualFold(toPattern, "Active") {
			toPattern = ""
		}
	}

	var alternatives []RuleTrigger
	for _, from := range buckets {
		if !legacyTransitionPatternMatches(from.label, fromPattern) {
			continue
		}
		for _, to := range buckets {
			if strings.EqualFold(from.label, to.label) ||
				!legacyTransitionPatternMatches(to.label, toPattern) {
				continue
			}
			alternatives = append(alternatives, RuleTrigger{
				Field:     FieldStateChanged,
				Operation: OpEquals,
				Value: workflowStateSetString(from.states) +
					"->" + workflowStateSetString(to.states),
			})
		}
	}
	if len(alternatives) == 0 {
		return nil, fmt.Errorf(
			"legacy transition predicate %s %q matches no representable cross-bucket transition",
			trigger.Field,
			trigger.Value,
		)
	}
	return alternatives, nil
}

func legacyTransitionPatternMatches(label, pattern string) bool {
	return pattern == "*" || strings.EqualFold(label, pattern)
}

type legacyStateProjection func(string) (models.WorkflowStateSet, error)
type legacyProjectionRenderer func(models.WorkflowState) string

func migrateLegacyProjectionPredicate(
	operation TriggerOperation,
	value string,
	project legacyStateProjection,
	render legacyProjectionRenderer,
) (models.WorkflowStateSet, TriggerOperation, error) {
	switch operation {
	case OpEquals, OpNotEquals:
		states, err := project(value)
		if err != nil {
			return models.WorkflowStateSet{}, "", err
		}
		canonicalOperation := OpIn
		if operation == OpNotEquals {
			canonicalOperation = OpNotIn
		}
		if states.Len() == 1 {
			if operation == OpEquals {
				canonicalOperation = OpEquals
			} else {
				canonicalOperation = OpNotEquals
			}
		}
		return states, canonicalOperation, nil

	case OpContains, OpGreaterThan, OpLessThan:
		var matches []models.WorkflowState
		for _, state := range models.AllWorkflowStates() {
			actual := render(state)
			if legacyProjectionMatches(actual, operation, value) {
				matches = append(matches, state)
			}
		}
		states, err := models.NewWorkflowStateSet(matches...)
		if err != nil {
			return models.WorkflowStateSet{}, "", err
		}
		if states.IsEmpty() {
			return models.WorkflowStateSet{}, "", fmt.Errorf("%s %q matches no canonical workflow state", operation, value)
		}
		return states, OpIn, nil

	default:
		return models.WorkflowStateSet{}, "", fmt.Errorf("unsupported legacy lifecycle operation %q", operation)
	}
}

func legacyProjectionMatches(actual string, operation TriggerOperation, target string) bool {
	switch operation {
	case OpContains:
		return strings.Contains(strings.ToLower(actual), strings.ToLower(target))
	case OpGreaterThan:
		return actual > target
	case OpLessThan:
		return actual < target
	default:
		return false
	}
}

func migrateLegacyAction(action RuleAction) (RuleAction, bool, error) {
	changed := false
	switch action.ActionType {
	case legacyActionSetStatus:
		state, err := legacyStatusActionState(action.Value)
		if err != nil {
			return RuleAction{}, false, err
		}
		action.ActionType = ActionSetState
		action.Value = string(state)
		changed = true
	case legacyActionSetSection:
		state, err := legacySectionActionState(action.Value)
		if err != nil {
			return RuleAction{}, false, err
		}
		action.ActionType = ActionSetState
		action.Value = string(state)
		changed = true
	default:
	}
	if action.Condition != nil {
		condition, conditionChanged, err := migrateLegacyTrigger(*action.Condition)
		if err != nil {
			return RuleAction{}, false, fmt.Errorf("condition: %w", err)
		}
		action.Condition = &condition
		changed = conditionChanged || changed
	}
	return action, changed, nil
}

func legacyStatusForState(state models.WorkflowState) string {
	switch state {
	case models.StateInbox, models.StateReady, models.StateNext:
		return legacyStatusTodo
	case models.StateInProgress:
		return legacyStatusInProgress
	case models.StateWaiting:
		return legacyStatusBlocked
	case models.StatePaused:
		return legacyStatusStandby
	case models.StateReview:
		return legacyStatusReview
	case models.StateSomeday:
		return legacyStatusMaybe
	case models.StateDone:
		return legacyStatusDone
	case models.StateCanceled:
		return legacyStatusCanceled
	default:
		return ""
	}
}

func legacySectionForState(state models.WorkflowState) string {
	switch state {
	case models.StateInbox:
		return legacySectionInbox
	case models.StateNext:
		return legacySectionNext
	case models.StateWaiting:
		return legacySectionWaiting
	case models.StatePaused:
		return legacySectionStandby
	case models.StateReview:
		return legacySectionReview
	case models.StateSomeday:
		return legacySectionSomeday
	case models.StateDone, models.StateCanceled:
		return legacySectionCompleted
	default:
		return ""
	}
}

func legacyStatusStates(value string) (models.WorkflowStateSet, error) {
	var states []models.WorkflowState
	switch strings.ToLower(strings.TrimSpace(value)) {
	case legacyStatusTodo:
		states = []models.WorkflowState{models.StateInbox, models.StateReady, models.StateNext}
	case legacyStatusInProgress:
		states = []models.WorkflowState{models.StateInProgress}
	case legacyStatusBlocked:
		states = []models.WorkflowState{models.StateWaiting}
	case legacyStatusStandby:
		states = []models.WorkflowState{models.StatePaused}
	case legacyStatusReview:
		states = []models.WorkflowState{models.StateReview}
	case legacyStatusMaybe:
		states = []models.WorkflowState{models.StateSomeday}
	case legacyStatusDone:
		states = []models.WorkflowState{models.StateDone}
	case legacyStatusCanceled:
		states = []models.WorkflowState{models.StateCanceled}
	default:
		return models.WorkflowStateSet{}, fmt.Errorf("unknown legacy task status %q", value)
	}
	return models.NewWorkflowStateSet(states...)
}

func legacySectionStates(value string) (models.WorkflowStateSet, error) {
	var states []models.WorkflowState
	switch {
	case value == "" || strings.EqualFold(strings.TrimSpace(value), "Active"):
		states = []models.WorkflowState{models.StateReady, models.StateInProgress}
	case strings.EqualFold(strings.TrimSpace(value), legacySectionInbox):
		states = []models.WorkflowState{models.StateInbox}
	case strings.EqualFold(strings.TrimSpace(value), legacySectionNext):
		states = []models.WorkflowState{models.StateNext}
	case strings.EqualFold(strings.TrimSpace(value), legacySectionWaiting):
		states = []models.WorkflowState{models.StateWaiting}
	case strings.EqualFold(strings.TrimSpace(value), legacySectionStandby):
		states = []models.WorkflowState{models.StatePaused}
	case strings.EqualFold(strings.TrimSpace(value), legacySectionReview):
		states = []models.WorkflowState{models.StateReview}
	case strings.EqualFold(strings.TrimSpace(value), legacySectionSomeday):
		states = []models.WorkflowState{models.StateSomeday}
	case strings.EqualFold(strings.TrimSpace(value), legacySectionCompleted):
		states = []models.WorkflowState{models.StateDone, models.StateCanceled}
	default:
		return models.WorkflowStateSet{}, fmt.Errorf("unknown legacy task section %q", value)
	}
	return models.NewWorkflowStateSet(states...)
}

func legacySectionsToStates(sections []string) (models.WorkflowStateSet, error) {
	var result models.WorkflowStateSet
	for _, section := range sections {
		states, err := legacySectionStates(section)
		if err != nil {
			return models.WorkflowStateSet{}, err
		}
		result = result.Union(states)
	}
	if result.IsEmpty() {
		return models.WorkflowStateSet{}, fmt.Errorf("legacy section scope maps to no canonical states")
	}
	return result, nil
}

func legacyStatusActionState(value string) (models.WorkflowState, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case legacyStatusTodo:
		return models.StateInbox, nil
	case legacyStatusInProgress:
		return models.StateInProgress, nil
	case legacyStatusBlocked:
		return models.StateWaiting, nil
	case legacyStatusStandby:
		return models.StatePaused, nil
	case legacyStatusReview:
		return models.StateReview, nil
	case legacyStatusMaybe:
		return models.StateSomeday, nil
	case legacyStatusDone:
		return models.StateDone, nil
	case legacyStatusCanceled:
		return models.StateCanceled, nil
	default:
		return "", fmt.Errorf("unknown set_status value %q", value)
	}
}

func legacySectionActionState(value string) (models.WorkflowState, error) {
	states, err := legacySectionStates(value)
	if err != nil {
		return "", err
	}
	switch {
	case value == "" || strings.EqualFold(strings.TrimSpace(value), "Active"):
		return models.StateReady, nil
	case strings.EqualFold(strings.TrimSpace(value), legacySectionCompleted):
		return models.StateDone, nil
	default:
		values := states.Values()
		if len(values) != 1 {
			return "", fmt.Errorf("section action %q has no single canonical state", value)
		}
		return values[0], nil
	}
}

func workflowStateSetString(states models.WorkflowStateSet) string {
	values := states.Values()
	parts := make([]string, 0, len(values))
	for _, state := range values {
		parts = append(parts, string(state))
	}
	return strings.Join(parts, ",")
}

func validateCanonicalLifecycleRule(rule WorkflowRule) error {
	checkTrigger := func(trigger RuleTrigger) error {
		switch trigger.Field {
		case legacyFieldStatus, legacyFieldSection, legacyFieldStatusChanged, legacyFieldSectionChanged,
			legacyFieldParentStatus, legacyFieldAllTasksStatus, legacyFieldMajorityTaskStatus, legacyFieldHasTasksWithStatus:
			return fmt.Errorf("migrated rule retains legacy trigger field %q", trigger.Field)
		default:
			return nil
		}
	}
	for _, trigger := range rule.Triggers {
		if err := checkTrigger(trigger); err != nil {
			return err
		}
	}
	if rule.TriggerGroup != nil {
		var walk func(*TriggerGroup) error
		walk = func(group *TriggerGroup) error {
			for _, trigger := range group.Triggers {
				if err := checkTrigger(trigger); err != nil {
					return err
				}
			}
			for i := range group.Groups {
				if err := walk(&group.Groups[i]); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(rule.TriggerGroup); err != nil {
			return err
		}
	}
	for _, action := range rule.Actions {
		if action.ActionType == legacyActionSetStatus || action.ActionType == legacyActionSetSection {
			return fmt.Errorf("migrated rule retains legacy action %q", action.ActionType)
		}
		if action.Condition != nil {
			if err := checkTrigger(*action.Condition); err != nil {
				return err
			}
		}
	}
	if rule.Scope != nil && len(rule.Scope.legacySections) > 0 {
		return fmt.Errorf("migrated rule retains legacy section scope")
	}
	return nil
}
