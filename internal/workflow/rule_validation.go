package workflow

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// StateTransitionEndpoint is one side of a state_changed transition pattern.
// Any represents "*"; otherwise States contains one or more canonical states.
type StateTransitionEndpoint struct {
	Any    bool
	States models.WorkflowStateSet
}

// String returns the endpoint in deterministic canonical state order.
func (e StateTransitionEndpoint) String() string {
	if e.Any {
		return "*"
	}
	values := e.States.Values()
	parts := make([]string, 0, len(values))
	for _, state := range values {
		parts = append(parts, string(state))
	}
	return strings.Join(parts, ",")
}

// StateTransitionPattern is the parsed representation of a state_changed
// trigger value.
type StateTransitionPattern struct {
	From StateTransitionEndpoint
	To   StateTransitionEndpoint
}

// String returns the transition in deterministic canonical state order.
func (p StateTransitionPattern) String() string {
	return p.From.String() + "->" + p.To.String()
}

// ParseWorkflowStateSet parses a non-empty comma-separated set of canonical
// workflow states. Whitespace and duplicates are normalized.
func ParseWorkflowStateSet(value string) (models.WorkflowStateSet, error) {
	parts := strings.Split(value, ",")
	states := make([]models.WorkflowState, 0, len(parts))
	for _, part := range parts {
		raw := strings.TrimSpace(part)
		if raw == "" {
			return models.WorkflowStateSet{}, fmt.Errorf("state set contains an empty value")
		}
		state, ok := models.ParseWorkflowState(raw)
		if !ok {
			return models.WorkflowStateSet{}, fmt.Errorf("invalid workflow state %q", raw)
		}
		states = append(states, state)
	}
	if len(states) == 0 {
		return models.WorkflowStateSet{}, fmt.Errorf("state set must contain at least one state")
	}
	return models.NewWorkflowStateSet(states...)
}

// ParseStateTransitionEndpoint parses "*", one canonical state, or a
// comma-separated canonical state set.
func ParseStateTransitionEndpoint(value string) (StateTransitionEndpoint, error) {
	value = strings.TrimSpace(value)
	if value == "*" {
		return StateTransitionEndpoint{Any: true}, nil
	}
	states, err := ParseWorkflowStateSet(value)
	if err != nil {
		return StateTransitionEndpoint{}, err
	}
	return StateTransitionEndpoint{States: states}, nil
}

// ParseStateTransitionPattern parses a state_changed value with exactly one
// arrow and validates both endpoints.
func ParseStateTransitionPattern(value string) (StateTransitionPattern, error) {
	if strings.Count(value, "->") != 1 {
		return StateTransitionPattern{}, fmt.Errorf("state_changed value must use from->to")
	}
	parts := strings.SplitN(value, "->", 2)
	from, err := ParseStateTransitionEndpoint(parts[0])
	if err != nil {
		return StateTransitionPattern{}, fmt.Errorf("invalid state_changed source: %w", err)
	}
	to, err := ParseStateTransitionEndpoint(parts[1])
	if err != nil {
		return StateTransitionPattern{}, fmt.Errorf("invalid state_changed target: %w", err)
	}
	return StateTransitionPattern{From: from, To: to}, nil
}

// Validate checks that a TriggerGroup is properly configured.
func (g *TriggerGroup) Validate() error {
	return g.validateAtDepth(0)
}

func (g *TriggerGroup) validateAtDepth(depth int) error {
	if depth >= maxTriggerGroupDepth {
		return fmt.Errorf("trigger group nesting exceeds maximum depth of %d", maxTriggerGroupDepth)
	}
	if g.Logic != LogicAnd && g.Logic != LogicOr && g.Logic != "" {
		return fmt.Errorf("invalid trigger group logic: %s (valid: and, or)", g.Logic)
	}
	if len(g.Triggers) == 0 && len(g.Groups) == 0 {
		return fmt.Errorf("trigger group must contain at least one trigger or sub-group")
	}

	for i, trigger := range g.Triggers {
		if err := trigger.Validate(); err != nil {
			return fmt.Errorf("trigger %d in group: %w", i+1, err)
		}
	}
	for i, sub := range g.Groups {
		if err := sub.validateAtDepth(depth + 1); err != nil {
			return fmt.Errorf("sub-group %d: %w", i+1, err)
		}
	}

	return nil
}

// Validate checks if the rule is properly configured.
func (r *WorkflowRule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}

	if r.TriggerGroup != nil {
		if err := r.TriggerGroup.Validate(); err != nil {
			return fmt.Errorf("invalid trigger group: %w", err)
		}
	} else {
		triggers := r.GetTriggers()
		if len(triggers) == 0 {
			return fmt.Errorf("at least one trigger is required")
		}

		for i, trigger := range triggers {
			if err := trigger.Validate(); err != nil {
				if len(triggers) > 1 {
					return fmt.Errorf("invalid trigger %d: %w", i+1, err)
				}
				return fmt.Errorf("invalid trigger: %w", err)
			}
		}

		if r.TriggerLogic != "" && r.TriggerLogic != LogicAnd && r.TriggerLogic != LogicOr {
			return fmt.Errorf("invalid trigger logic: %s (valid: and, or)", r.TriggerLogic)
		}
	}

	if len(r.Actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	for i, action := range r.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("invalid action %d: %w", i+1, err)
		}
	}

	if err := r.validateScope(); err != nil {
		return fmt.Errorf("invalid scope: %w", err)
	}
	if err := r.validateLevel(); err != nil {
		return err
	}

	return nil
}

// ValidateForAuthoring rejects decode-only shapes used by schema migration.
func (r *WorkflowRule) ValidateForAuthoring() error {
	if err := r.Validate(); err != nil {
		return err
	}
	if r.legacyTrigger.Field != "" {
		return fmt.Errorf("singular trigger input was removed; use triggers")
	}
	if r.TriggerGroup != nil {
		if err := validateTriggerGroupForAuthoring(r.TriggerGroup); err != nil {
			return fmt.Errorf("invalid trigger group: %w", err)
		}
	} else {
		for i, trigger := range r.Triggers {
			if err := trigger.ValidateForAuthoring(); err != nil {
				return fmt.Errorf("invalid trigger %d: %w", i+1, err)
			}
		}
	}
	for i, action := range r.Actions {
		if err := action.ValidateForAuthoring(); err != nil {
			return fmt.Errorf("invalid action %d: %w", i+1, err)
		}
	}
	if r.Scope != nil && len(r.Scope.legacySections) > 0 {
		return fmt.Errorf("invalid scope: sections input was removed; use states")
	}
	return nil
}

func validateTriggerGroupForAuthoring(group *TriggerGroup) error {
	for i, trigger := range group.Triggers {
		if err := trigger.ValidateForAuthoring(); err != nil {
			return fmt.Errorf("trigger %d in group: %w", i+1, err)
		}
	}
	for i := range group.Groups {
		if err := validateTriggerGroupForAuthoring(&group.Groups[i]); err != nil {
			return fmt.Errorf("sub-group %d: %w", i+1, err)
		}
	}
	return nil
}

// Validate checks if the trigger is properly configured.
func (t RuleTrigger) Validate() error {
	validFields := map[TriggerField]bool{
		FieldState:               true,
		FieldPriority:            true,
		FieldTags:                true,
		FieldType:                true,
		FieldDueDays:             true,
		FieldCreatedDays:         true,
		FieldUpdatedDays:         true,
		FieldStateChanged:        true,
		FieldPriorityChanged:     true,
		FieldTagAdded:            true,
		FieldTagRemoved:          true,
		FieldCompleted:           true,
		FieldReopened:            true,
		FieldDueDateChanged:      true,
		FieldTypeChanged:         true,
		FieldRecurrenceCompleted: true,
		FieldStreakBroken:        true,
		FieldHasDueDate:          true,
		FieldHasTags:             true,
		FieldDescriptionEmpty:    true,
		FieldParentState:         true,
		FieldParentType:          true,
		FieldParentTags:          true,
		FieldLevel:               true,
		FieldHasChildren:         true,
		FieldChildrenProgress:    true,
		FieldAllChildrenDone:     true,
		FieldIsBlocked:           true,
		FieldBlockerCount:        true,
		FieldDependencyCount:     true,
		FieldHasInProgressTasks:  true,
		FieldActiveTaskCount:     true,
		FieldAllTasksState:       true,
		FieldMajorityTaskType:    true,
		FieldMajorityTaskState:   true,
		FieldCompletionRatio:     true,
		FieldDaysSinceUpdate:     true,
		FieldProjectStatus:       true,
		FieldHasTasksWithState:   true,
	}
	if !validFields[t.Field] {
		return fmt.Errorf("invalid field: %s", t.Field)
	}

	validOps := map[TriggerOperation]bool{
		OpEquals:      true,
		OpNotEquals:   true,
		OpContains:    true,
		OpGreaterThan: true,
		OpLessThan:    true,
		OpIn:          true,
		OpNotIn:       true,
	}
	if !validOps[t.Operation] {
		return fmt.Errorf("invalid operation: %s", t.Operation)
	}
	if t.Value == "" {
		return fmt.Errorf("trigger value is required")
	}
	if err := validateCanonicalStateTrigger(t); err != nil {
		return err
	}

	return nil
}

// ValidateForAuthoring validates a trigger against the canonical schema.
func (t RuleTrigger) ValidateForAuthoring() error {
	if !containsTriggerField(ValidTriggerFields(), t.Field) {
		return fmt.Errorf("trigger field is not available for canonical authoring: %s", t.Field)
	}
	return t.Validate()
}

func containsTriggerField(fields []TriggerField, target TriggerField) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}

func validateCanonicalStateTrigger(t RuleTrigger) error {
	switch t.Field {
	case FieldState, FieldParentState, FieldAllTasksState, FieldMajorityTaskState, FieldHasTasksWithState:
		switch t.Operation {
		case OpEquals, OpNotEquals, OpIn, OpNotIn:
		default:
			return fmt.Errorf("%s only supports equals, not_equals, in, and not_in", t.Field)
		}
		states, err := ParseWorkflowStateSet(t.Value)
		if err != nil {
			return fmt.Errorf("%s value must contain canonical states: %w", t.Field, err)
		}
		if (t.Operation == OpEquals || t.Operation == OpNotEquals) && states.Len() != 1 {
			return fmt.Errorf("%s with %s requires exactly one canonical state", t.Field, t.Operation)
		}
	case FieldStateChanged:
		if t.Operation != OpEquals && t.Operation != OpNotEquals {
			return fmt.Errorf("state_changed only supports equals and not_equals")
		}
		if _, err := ParseStateTransitionPattern(t.Value); err != nil {
			return err
		}
	default:
		if t.Operation == OpIn || t.Operation == OpNotIn {
			return fmt.Errorf("%s is only valid for canonical state fields", t.Operation)
		}
	}
	return nil
}

// Validate checks if the action is properly configured.
func (a RuleAction) Validate() error {
	validActions := map[ActionType]bool{
		ActionSetState:         true,
		ActionSetPriority:      true,
		ActionAddTag:           true,
		ActionRemoveTag:        true,
		ActionSetDueRelative:   true,
		ActionClearDue:         true,
		ActionSetPinned:        true,
		ActionAddComment:       true,
		ActionSetType:          true,
		ActionCompleteParent:   true,
		ActionBlockParent:      true,
		ActionCascadeTag:       true,
		ActionInheritTag:       true,
		ActionNotifyParent:     true,
		ActionSetSnooze:        true,
		ActionCreateFollowup:   true,
		ActionMoveToProject:    true,
		ActionSetRecurrence:    true,
		ActionSetProjectStatus: true,
	}
	if !validActions[a.ActionType] {
		return fmt.Errorf("invalid action type: %s", a.ActionType)
	}

	noValueRequired := map[ActionType]bool{
		ActionClearDue:       true,
		ActionCompleteParent: true,
		ActionBlockParent:    true,
	}
	if !noValueRequired[a.ActionType] && a.Value == "" {
		return fmt.Errorf("action value is required")
	}

	switch a.ActionType { //nolint:exhaustive
	case ActionSetState:
		if state, ok := models.ParseWorkflowState(strings.TrimSpace(a.Value)); !ok || string(state) != strings.TrimSpace(a.Value) {
			return fmt.Errorf("set_state value must be one canonical state: %s", a.Value)
		}
	case ActionSetPinned:
		if a.Value != "true" && a.Value != "false" {
			return fmt.Errorf("%s value must be 'true' or 'false'", a.ActionType)
		}
	case ActionSetDueRelative:
		if _, err := ParseRelativeDuration(a.Value); err != nil {
			return fmt.Errorf("invalid relative duration: %s (use format like +3d, +1w, +2m)", a.Value)
		}
	case ActionSetSnooze:
		if _, err := ParseRelativeDuration(a.Value); err != nil {
			return fmt.Errorf("invalid snooze duration: %s (use format like +3d, +1w, +2m)", a.Value)
		}
	case ActionSetRecurrence:
		if models.ParseRecurrence(a.Value) == nil {
			return fmt.Errorf("invalid recurrence pattern: %s (use format like daily, weekly, 2d, 3w)", a.Value)
		}
	case ActionSetProjectStatus:
		if models.ProjectStatusIconByKey(a.Value) == nil {
			return fmt.Errorf("invalid project status icon key: %s", a.Value)
		}
	}

	if a.Condition != nil {
		if err := a.Condition.Validate(); err != nil {
			return fmt.Errorf("invalid action condition: %w", err)
		}
	}

	return nil
}

// ValidateForAuthoring validates an action against the canonical schema.
func (a RuleAction) ValidateForAuthoring() error {
	if !containsActionType(ValidActionTypes(), a.ActionType) {
		return fmt.Errorf("action type is not available for canonical authoring: %s", a.ActionType)
	}
	if err := a.Validate(); err != nil {
		return err
	}
	if a.Condition != nil {
		if err := a.Condition.ValidateForAuthoring(); err != nil {
			return fmt.Errorf("invalid action condition: %w", err)
		}
	}
	return nil
}

func containsActionType(actions []ActionType, target ActionType) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

// ParseRelativeDuration parses a relative duration string like "+3d", "+1w", "+2m".
// Returns the duration in hours. Supported suffixes: d (days), w (weeks), m (months), y (years).
func ParseRelativeDuration(s string) (int, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short")
	}
	if s[0] == '+' {
		s = s[1:]
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("duration too short")
	}

	numStr := s[:len(s)-1]
	unit := s[len(s)-1]

	num := 0
	for _, char := range numStr {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("invalid number: %s", numStr)
		}
		num = num*10 + int(char-'0')
		if num > 9999 {
			return 0, fmt.Errorf("duration too large: %s", numStr)
		}
	}
	if num <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}

	switch unit {
	case 'd':
		return num, nil
	case 'w':
		return num * 7, nil
	case 'm':
		return num * 30, nil
	case 'y':
		return num * 365, nil
	default:
		return 0, fmt.Errorf("invalid unit: %c (use d, w, m, or y)", unit)
	}
}

// ValidTriggerFields returns all valid trigger field options.
func ValidTriggerFields() []TriggerField {
	return []TriggerField{
		FieldState, FieldPriority, FieldTags, FieldType,
		FieldDueDays, FieldCreatedDays, FieldUpdatedDays,
		FieldStateChanged, FieldPriorityChanged,
		FieldTagAdded, FieldTagRemoved, FieldCompleted, FieldReopened,
		FieldDueDateChanged, FieldTypeChanged, FieldRecurrenceCompleted, FieldStreakBroken,
		FieldHasDueDate, FieldHasTags, FieldDescriptionEmpty,
		FieldParentState, FieldParentType, FieldParentTags, FieldLevel,
		FieldHasChildren, FieldChildrenProgress, FieldAllChildrenDone,
		FieldIsBlocked, FieldBlockerCount, FieldDependencyCount,
		FieldHasInProgressTasks, FieldActiveTaskCount, FieldAllTasksState,
		FieldMajorityTaskType, FieldMajorityTaskState, FieldCompletionRatio,
		FieldDaysSinceUpdate, FieldProjectStatus, FieldHasTasksWithState,
	}
}

// ValidTriggerOperations returns all valid trigger operation options.
func ValidTriggerOperations() []TriggerOperation {
	return []TriggerOperation{OpEquals, OpNotEquals, OpContains, OpGreaterThan, OpLessThan, OpIn, OpNotIn}
}

// ValidActionTypes returns all valid action type options.
func ValidActionTypes() []ActionType {
	return []ActionType{
		ActionSetState, ActionSetPriority,
		ActionAddTag, ActionRemoveTag,
		ActionSetDueRelative, ActionClearDue,
		ActionSetPinned,
		ActionAddComment, ActionSetType,
		ActionCompleteParent, ActionBlockParent, ActionCascadeTag,
		ActionInheritTag, ActionNotifyParent,
		ActionSetSnooze, ActionCreateFollowup, ActionMoveToProject, ActionSetRecurrence,
		ActionSetProjectStatus,
	}
}

// IsEventTrigger returns true if the trigger field requires change detection.
func IsEventTrigger(field TriggerField) bool {
	switch field {
	case FieldStateChanged, FieldPriorityChanged,
		FieldTagAdded, FieldTagRemoved, FieldCompleted, FieldReopened,
		FieldDueDateChanged, FieldTypeChanged, FieldRecurrenceCompleted, FieldStreakBroken:
		return true
	default:
	}
	return false
}

// IsHierarchicalTrigger returns true if the trigger requires parent/child data.
func IsHierarchicalTrigger(field TriggerField) bool {
	switch field {
	case FieldParentState, FieldParentType, FieldParentTags, FieldLevel,
		FieldHasChildren, FieldChildrenProgress, FieldAllChildrenDone,
		FieldIsBlocked, FieldBlockerCount, FieldDependencyCount:
		return true
	default:
	}
	return false
}

// IsHierarchicalAction returns true if the action operates on parent/children.
func IsHierarchicalAction(actionType ActionType) bool {
	switch actionType {
	case ActionCompleteParent, ActionBlockParent, ActionCascadeTag,
		ActionInheritTag, ActionNotifyParent:
		return true
	default:
	}
	return false
}

// IsProjectTrigger returns true if the trigger field is a project-level aggregate field.
func IsProjectTrigger(field TriggerField) bool {
	switch field {
	case FieldHasInProgressTasks, FieldActiveTaskCount, FieldAllTasksState,
		FieldMajorityTaskType, FieldMajorityTaskState, FieldCompletionRatio,
		FieldDaysSinceUpdate, FieldProjectStatus, FieldHasTasksWithState:
		return true
	default:
	}
	return false
}

// MatchesScope checks whether a task falls within the rule's scope.
// A nil scope matches everything. Non-empty fields use AND logic.
func (r *WorkflowRule) MatchesScope(task *models.Task, projectName, projectID string) bool {
	if r.Scope == nil {
		return true
	}

	if len(r.Scope.Projects) > 0 {
		matched := false
		for _, project := range r.Scope.Projects {
			if strings.EqualFold(project, projectName) || project == projectID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(r.Scope.Types) > 0 {
		if task == nil {
			return false
		}
		matched := false
		taskType := string(task.Type)
		for _, ruleType := range r.Scope.Types {
			if strings.EqualFold(ruleType, taskType) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(r.Scope.States) > 0 {
		if task == nil {
			return false
		}
		matched := false
		for _, state := range r.Scope.States {
			if task.State() == state {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// validateScope checks that the rule scope contains valid values.
func (r *WorkflowRule) validateScope() error {
	if r.Scope == nil {
		return nil
	}

	for _, ruleType := range r.Scope.Types {
		if !models.TaskType(strings.ToLower(ruleType)).IsValid() {
			return fmt.Errorf("invalid task type in scope: %s", ruleType)
		}
	}
	for _, project := range r.Scope.Projects {
		if strings.TrimSpace(project) == "" {
			return fmt.Errorf("empty project name in scope")
		}
	}
	for _, state := range r.Scope.States {
		if !state.IsValid() {
			return fmt.Errorf("invalid state in scope: %s", state)
		}
	}
	return nil
}

// validateLevel checks that triggers and actions match the rule's level.
func (r *WorkflowRule) validateLevel() error {
	level := r.GetLevel()

	var triggers []RuleTrigger
	if r.TriggerGroup != nil {
		triggers = collectTriggers(r.TriggerGroup)
	} else {
		triggers = r.GetTriggers()
	}

	if level == RuleLevelProject {
		if invalidScopeFields := r.scopeTaskFields(); len(invalidScopeFields) > 0 {
			return fmt.Errorf(
				"project-level rule %q has task-level scope fields %q; only project scope is supported",
				r.Name,
				strings.Join(invalidScopeFields, ", "),
			)
		}
		for _, trigger := range triggers {
			if !IsProjectTrigger(trigger.Field) {
				return fmt.Errorf("project-level rule %q has task-level trigger field %q", r.Name, trigger.Field)
			}
		}
		for _, action := range r.Actions {
			if action.ActionType != ActionSetProjectStatus {
				return fmt.Errorf("project-level rule %q has non-project action %q", r.Name, action.ActionType)
			}
		}
	} else {
		for _, trigger := range triggers {
			if IsProjectTrigger(trigger.Field) {
				return fmt.Errorf("task-level rule %q has project-level trigger field %q", r.Name, trigger.Field)
			}
		}
		for _, action := range r.Actions {
			if action.ActionType == ActionSetProjectStatus {
				return fmt.Errorf("task-level rule %q has project-level action %q", r.Name, action.ActionType)
			}
		}
	}

	return nil
}

func (r *WorkflowRule) scopeTaskFields() []string {
	if r.Scope == nil {
		return nil
	}

	var fields []string
	if len(r.Scope.Types) > 0 {
		fields = append(fields, "types")
	}
	if len(r.Scope.States) > 0 {
		fields = append(fields, "states")
	}
	return fields
}

// collectTriggers recursively collects all leaf triggers from a trigger group.
func collectTriggers(group *TriggerGroup) []RuleTrigger {
	var result []RuleTrigger
	result = append(result, group.Triggers...)
	for i := range group.Groups {
		result = append(result, collectTriggers(&group.Groups[i])...)
	}
	return result
}
