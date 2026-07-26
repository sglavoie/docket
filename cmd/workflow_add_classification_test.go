package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/workflow"
)

func TestWorkflowAdd_IsEventTrigger(t *testing.T) {
	eventTriggers := []workflow.TriggerField{
		workflow.FieldStateChanged,
		workflow.FieldStatusChanged,
		workflow.FieldSectionChanged,
		workflow.FieldPriorityChanged,
		workflow.FieldTagAdded,
		workflow.FieldTagRemoved,
		workflow.FieldCompleted,
		workflow.FieldReopened,
	}

	for _, field := range eventTriggers {
		if !workflow.IsEventTrigger(field) {
			t.Errorf("expected %s to be an event trigger", field)
		}
	}

	stateTriggers := []workflow.TriggerField{
		workflow.FieldState,
		workflow.FieldStatus,
		workflow.FieldPriority,
		workflow.FieldSection,
		workflow.FieldTags,
	}

	for _, field := range stateTriggers {
		if workflow.IsEventTrigger(field) {
			t.Errorf("expected %s to not be an event trigger", field)
		}
	}
}

func TestWorkflowAdd_IsHierarchicalTrigger(t *testing.T) {
	hierarchicalTriggers := []workflow.TriggerField{
		workflow.FieldParentState,
		workflow.FieldParentStatus,
		workflow.FieldParentType,
		workflow.FieldParentTags,
		workflow.FieldLevel,
		workflow.FieldHasChildren,
		workflow.FieldChildrenProgress,
		workflow.FieldAllChildrenDone,
		workflow.FieldIsBlocked,
		workflow.FieldBlockerCount,
	}

	for _, field := range hierarchicalTriggers {
		if !workflow.IsHierarchicalTrigger(field) {
			t.Errorf("expected %s to be a hierarchical trigger", field)
		}
	}

	nonHierarchical := []workflow.TriggerField{
		workflow.FieldState,
		workflow.FieldStatus,
		workflow.FieldPriority,
		workflow.FieldStatusChanged,
	}

	for _, field := range nonHierarchical {
		if workflow.IsHierarchicalTrigger(field) {
			t.Errorf("expected %s to not be a hierarchical trigger", field)
		}
	}
}

func TestWorkflowAdd_IsHierarchicalAction(t *testing.T) {
	hierarchicalActions := []workflow.ActionType{
		workflow.ActionCompleteParent,
		workflow.ActionBlockParent,
		workflow.ActionCascadeTag,
		workflow.ActionInheritTag,
		workflow.ActionNotifyParent,
	}

	for _, action := range hierarchicalActions {
		if !workflow.IsHierarchicalAction(action) {
			t.Errorf("expected %s to be a hierarchical action", action)
		}
	}

	nonHierarchical := []workflow.ActionType{
		workflow.ActionSetState,
		workflow.ActionSetStatus,
		workflow.ActionAddTag,
		workflow.ActionSetSection,
	}

	for _, action := range nonHierarchical {
		if workflow.IsHierarchicalAction(action) {
			t.Errorf("expected %s to not be a hierarchical action", action)
		}
	}
}
