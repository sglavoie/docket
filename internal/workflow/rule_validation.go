package workflow

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

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

// Validate checks if the trigger is properly configured.
func (t RuleTrigger) Validate() error {
	validFields := map[TriggerField]bool{
		FieldStatus:              true,
		FieldPriority:            true,
		FieldSection:             true,
		FieldTags:                true,
		FieldType:                true,
		FieldDueDays:             true,
		FieldCreatedDays:         true,
		FieldUpdatedDays:         true,
		FieldStatusChanged:       true,
		FieldSectionChanged:      true,
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
		FieldParentStatus:        true,
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
		FieldAllTasksStatus:      true,
		FieldMajorityTaskType:    true,
		FieldMajorityTaskStatus:  true,
		FieldCompletionRatio:     true,
		FieldDaysSinceUpdate:     true,
		FieldProjectStatus:       true,
		FieldHasTasksWithStatus:  true,
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
	}
	if !validOps[t.Operation] {
		return fmt.Errorf("invalid operation: %s", t.Operation)
	}
	if t.Value == "" && !isValidEmptyTriggerValue(t) {
		return fmt.Errorf("trigger value is required")
	}
	if err := validateSectionTriggerValue(t); err != nil {
		return err
	}

	return nil
}

func isValidEmptyTriggerValue(t RuleTrigger) bool {
	return t.Field == FieldSection && (t.Operation == OpEquals || t.Operation == OpNotEquals)
}

func validateSectionTriggerValue(t RuleTrigger) error {
	switch t.Field {
	case FieldSection:
		if t.Value != "" && !models.IsGTDSection(t.Value) {
			return fmt.Errorf("section trigger value must be a built-in section: %s", t.Value)
		}
	case FieldSectionChanged:
		parts := strings.Split(t.Value, "->")
		if len(parts) != 2 {
			return nil
		}
		for _, part := range parts {
			if err := validateSectionTransitionPattern(strings.TrimSpace(part)); err != nil {
				return err
			}
		}
	default:
	}
	return nil
}

func validateSectionTransitionPattern(pattern string) error {
	if pattern == "*" || pattern == "" || models.IsGTDSection(pattern) {
		return nil
	}
	return fmt.Errorf("section_changed value must target built-in sections: %s", pattern)
}

// Validate checks if the action is properly configured.
func (a RuleAction) Validate() error {
	validActions := map[ActionType]bool{
		ActionSetStatus:        true,
		ActionSetPriority:      true,
		ActionSetSection:       true,
		ActionAddTag:           true,
		ActionRemoveTag:        true,
		ActionSetDueRelative:   true,
		ActionClearDue:         true,
		ActionSetActive:        true,
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
		ActionSetSection:     true,
		ActionClearDue:       true,
		ActionCompleteParent: true,
		ActionBlockParent:    true,
	}
	if !noValueRequired[a.ActionType] && a.Value == "" {
		return fmt.Errorf("action value is required")
	}

	switch a.ActionType { //nolint:exhaustive
	case ActionSetActive, ActionSetPinned:
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
	case ActionSetSection:
		if a.Value != "" && !models.IsGTDSection(a.Value) {
			return fmt.Errorf("set_section value must be a built-in section: %s", a.Value)
		}
	}

	if a.Condition != nil {
		if err := a.Condition.Validate(); err != nil {
			return fmt.Errorf("invalid action condition: %w", err)
		}
	}

	return nil
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
		FieldStatus, FieldPriority, FieldSection, FieldTags, FieldType,
		FieldDueDays, FieldCreatedDays, FieldUpdatedDays,
		FieldStatusChanged, FieldSectionChanged, FieldPriorityChanged,
		FieldTagAdded, FieldTagRemoved, FieldCompleted, FieldReopened,
		FieldDueDateChanged, FieldTypeChanged, FieldRecurrenceCompleted, FieldStreakBroken,
		FieldHasDueDate, FieldHasTags, FieldDescriptionEmpty,
		FieldParentStatus, FieldParentType, FieldParentTags, FieldLevel,
		FieldHasChildren, FieldChildrenProgress, FieldAllChildrenDone,
		FieldIsBlocked, FieldBlockerCount, FieldDependencyCount,
		FieldHasInProgressTasks, FieldActiveTaskCount, FieldAllTasksStatus,
		FieldMajorityTaskType, FieldMajorityTaskStatus, FieldCompletionRatio,
		FieldDaysSinceUpdate, FieldProjectStatus, FieldHasTasksWithStatus,
	}
}

// ValidTriggerOperations returns all valid trigger operation options.
func ValidTriggerOperations() []TriggerOperation {
	return []TriggerOperation{OpEquals, OpNotEquals, OpContains, OpGreaterThan, OpLessThan}
}

// ValidActionTypes returns all valid action type options.
func ValidActionTypes() []ActionType {
	return []ActionType{
		ActionSetStatus, ActionSetPriority, ActionSetSection,
		ActionAddTag, ActionRemoveTag,
		ActionSetDueRelative, ActionClearDue,
		ActionSetActive, ActionSetPinned,
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
	case FieldStatusChanged, FieldSectionChanged, FieldPriorityChanged,
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
	case FieldParentStatus, FieldParentType, FieldParentTags, FieldLevel,
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
	case FieldHasInProgressTasks, FieldActiveTaskCount, FieldAllTasksStatus,
		FieldMajorityTaskType, FieldMajorityTaskStatus, FieldCompletionRatio,
		FieldDaysSinceUpdate, FieldProjectStatus, FieldHasTasksWithStatus:
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

	if len(r.Scope.Sections) > 0 {
		if task == nil {
			return false
		}
		matched := false
		for _, section := range r.Scope.Sections {
			if strings.EqualFold(section, task.Section) {
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
	for _, section := range r.Scope.Sections {
		if section != "" && strings.TrimSpace(section) == "" {
			return fmt.Errorf("empty section name in scope")
		}
		if section != "" && !models.IsGTDSection(section) {
			return fmt.Errorf("invalid section in scope: %s", section)
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
	if len(r.Scope.Sections) > 0 {
		fields = append(fields, "sections")
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
