// Package actionplan defines the provider-neutral wire contract for structured
// AI action plans. It deliberately has no dependency on application or storage
// packages so provider adapters can parse untrusted output without introducing
// dependency cycles.
package actionplan

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/sglavoie/tt/internal/models"
)

// Version is the only structured AI action-plan version this build accepts.
const Version = 1

// ActionType is the allowlisted tagged-union discriminator. There is no shell
// command or arbitrary executable action.
type ActionType string

const (
	ActionCreateTask       ActionType = "create_task"
	ActionEditTask         ActionType = "edit_task"
	ActionSetState         ActionType = "set_state"
	ActionSetPriority      ActionType = "set_priority"
	ActionSetDueDate       ActionType = "set_due_date"
	ActionClearDueDate     ActionType = "clear_due_date"
	ActionSetTags          ActionType = "set_tags"
	ActionAddDependency    ActionType = "add_dependency"
	ActionRemoveDependency ActionType = "remove_dependency"
	ActionDeleteTask       ActionType = "delete_task"
)

// Action is one member of the allowlisted action union. Pointer fields retain
// the distinction between an omitted property and an explicitly empty value.
type Action struct {
	Type          ActionType            `json:"type"`
	ProjectID     string                `json:"project_id,omitempty"`
	TaskID        string                `json:"task_id,omitempty"`
	Title         *string               `json:"title,omitempty"`
	Description   *string               `json:"description,omitempty"`
	TaskType      *models.TaskType      `json:"task_type,omitempty"`
	State         *models.WorkflowState `json:"state,omitempty"`
	Priority      *int                  `json:"priority,omitempty"`
	Due           *string               `json:"due,omitempty"`
	HasTime       bool                  `json:"has_time,omitempty"`
	Tags          []string              `json:"tags,omitempty"`
	BlockerTaskID string                `json:"blocker_task_id,omitempty"`
	present       map[string]struct{}
}

// UnmarshalJSON records field presence so union validation rejects disallowed
// fields even when their JSON values are false, empty, or null. It also rejects
// unknown action properties independently of the outer decoder.
func (a *Action) UnmarshalJSON(data []byte) error {
	type wireAction Action
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	allowed := map[string]bool{
		"type": true, "project_id": true, "task_id": true, "title": true,
		"description": true, "task_type": true, "state": true, "priority": true,
		"due": true, "has_time": true, "tags": true, "blocker_task_id": true,
	}
	for field := range fields {
		if !allowed[field] {
			return fmt.Errorf("unknown field %q", field)
		}
	}
	var decoded wireAction
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*a = Action(decoded)
	a.present = make(map[string]struct{}, len(fields))
	for field := range fields {
		a.present[field] = struct{}{}
	}
	return nil
}

// ActionPlan is the complete provider response. Summary is display-only; all
// executable meaning comes from the validated Actions union.
type ActionPlan struct {
	Version int      `json:"version"`
	Summary string   `json:"summary"`
	Actions []Action `json:"actions"`
}

// Validate checks the version, tagged union, required properties, and value
// syntax without consulting workspace state.
func Validate(plan ActionPlan) error {
	if plan.Version != Version {
		return fmt.Errorf("unsupported version %d (want %d)", plan.Version, Version)
	}
	for _, r := range plan.Summary {
		if unicode.IsControl(r) {
			return fmt.Errorf("summary contains unsafe terminal control character U+%04X", r)
		}
	}
	if plan.Actions == nil {
		return fmt.Errorf("actions must be an array")
	}
	if len(plan.Actions) > 100 {
		return fmt.Errorf("contains more than 100 actions")
	}
	for i, action := range plan.Actions {
		if err := validateAction(action); err != nil {
			return fmt.Errorf("action %d: %w", i+1, err)
		}
	}
	return nil
}

// Each allowlisted action has a deliberately explicit field and value contract.
// Keeping those cases together makes the untrusted wire boundary auditable.
//
//nolint:gocyclo
func validateAction(action Action) error {
	requireTask := func() error {
		if strings.TrimSpace(action.TaskID) == "" {
			return fmt.Errorf("task_id is required")
		}
		return nil
	}
	noExtras := func(allowed ...string) error {
		set := make(map[string]bool, len(allowed))
		for _, field := range allowed {
			set[field] = true
		}
		var extras []string
		provided := func(field string, nonzero bool) bool {
			if action.present != nil {
				_, ok := action.present[field]
				return ok
			}
			return nonzero
		}
		if provided("project_id", action.ProjectID != "") && !set["project_id"] {
			extras = append(extras, "project_id")
		}
		if provided("task_id", action.TaskID != "") && !set["task_id"] {
			extras = append(extras, "task_id")
		}
		if provided("title", action.Title != nil) && !set["title"] {
			extras = append(extras, "title")
		}
		if provided("description", action.Description != nil) && !set["description"] {
			extras = append(extras, "description")
		}
		if provided("task_type", action.TaskType != nil) && !set["task_type"] {
			extras = append(extras, "task_type")
		}
		if provided("state", action.State != nil) && !set["state"] {
			extras = append(extras, "state")
		}
		if provided("priority", action.Priority != nil) && !set["priority"] {
			extras = append(extras, "priority")
		}
		if provided("due", action.Due != nil) && !set["due"] {
			extras = append(extras, "due")
		}
		if provided("has_time", action.HasTime) && !set["has_time"] {
			extras = append(extras, "has_time")
		}
		if provided("tags", action.Tags != nil) && !set["tags"] {
			extras = append(extras, "tags")
		}
		if provided("blocker_task_id", action.BlockerTaskID != "") && !set["blocker_task_id"] {
			extras = append(extras, "blocker_task_id")
		}
		if len(extras) > 0 {
			sort.Strings(extras)
			return fmt.Errorf("fields not allowed for %s: %s", action.Type, strings.Join(extras, ", "))
		}
		return nil
	}
	validateTags := func(tags []string) error {
		for _, raw := range tags {
			tag := models.NormalizeTag(raw)
			if err := models.ValidateTag(tag); err != nil {
				return fmt.Errorf("invalid tag: %w", err)
			}
		}
		return nil
	}

	switch action.Type {
	case ActionCreateTask:
		if strings.TrimSpace(action.ProjectID) == "" {
			return fmt.Errorf("project_id is required")
		}
		if action.Title == nil {
			return fmt.Errorf("title is required")
		}
		if err := models.ValidateTaskTitle(strings.TrimSpace(*action.Title)); err != nil {
			return fmt.Errorf("invalid title: %w", err)
		}
		if action.TaskType != nil && !action.TaskType.IsValid() {
			return fmt.Errorf("invalid task_type %q", *action.TaskType)
		}
		if action.Priority != nil {
			if err := models.ValidatePriority(*action.Priority, false); err != nil {
				return fmt.Errorf("invalid priority: %w", err)
			}
		}
		if err := validateTags(action.Tags); err != nil {
			return err
		}
		return noExtras("project_id", "title", "description", "task_type", "priority", "tags")

	case ActionEditTask:
		if err := requireTask(); err != nil {
			return err
		}
		if action.Title == nil && action.Description == nil && action.TaskType == nil {
			return fmt.Errorf("edit_task requires at least one editable field")
		}
		if action.Title != nil {
			if err := models.ValidateTaskTitle(strings.TrimSpace(*action.Title)); err != nil {
				return fmt.Errorf("invalid title: %w", err)
			}
		}
		if action.TaskType != nil && !action.TaskType.IsValid() {
			return fmt.Errorf("invalid task_type %q", *action.TaskType)
		}
		return noExtras("task_id", "title", "description", "task_type")

	case ActionSetState:
		if err := requireTask(); err != nil {
			return err
		}
		if action.State == nil || !action.State.IsValid() {
			return fmt.Errorf("a valid state is required")
		}
		return noExtras("task_id", "state")

	case ActionSetPriority:
		if err := requireTask(); err != nil {
			return err
		}
		if action.Priority == nil {
			return fmt.Errorf("priority is required")
		}
		if err := models.ValidatePriority(*action.Priority, false); err != nil {
			return fmt.Errorf("invalid priority: %w", err)
		}
		return noExtras("task_id", "priority")

	case ActionSetDueDate:
		if err := requireTask(); err != nil {
			return err
		}
		if action.Due == nil {
			return fmt.Errorf("due is required")
		}
		if !validDue(*action.Due) {
			return fmt.Errorf("due must be YYYY-MM-DD or RFC3339")
		}
		return noExtras("task_id", "due", "has_time")

	case ActionClearDueDate:
		if err := requireTask(); err != nil {
			return err
		}
		return noExtras("task_id")

	case ActionSetTags:
		if err := requireTask(); err != nil {
			return err
		}
		if action.Tags == nil {
			return fmt.Errorf("tags is required (use [] to clear)")
		}
		if err := validateTags(action.Tags); err != nil {
			return err
		}
		return noExtras("task_id", "tags")

	case ActionAddDependency, ActionRemoveDependency:
		if err := requireTask(); err != nil {
			return err
		}
		if strings.TrimSpace(action.BlockerTaskID) == "" {
			return fmt.Errorf("blocker_task_id is required")
		}
		return noExtras("task_id", "blocker_task_id")

	case ActionDeleteTask:
		if err := requireTask(); err != nil {
			return err
		}
		return noExtras("task_id")

	default:
		return fmt.Errorf("action type %q is not allowlisted", action.Type)
	}
}

func validDue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}
