package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// checklistRowOperationApplicable admits an operation only while the cursor
// rests on a checklist row. That disjointness is what lets these operations
// share e, A, J, K and Space with their task-row counterparts, since shared-key
// resolution picks the first binding whose descriptor applies.
func checklistRowOperationApplicable(ctx operation.CommandContext) bool {
	return ctx.View == presentation.ViewCrossProjectOutliner && ctx.Entity.HasChecklistCursor
}

// checklistRowOperationDescriptors is the authoritative catalog of checklist
// row operations. They target the owner task because that is the durable
// identity a checklist item hangs off; the item itself is named by the index
// the command context froze.
//
// Every descriptor is Task category so that a key shared with a task operation
// keeps one category across its bindings, which is what lets a non-task cursor
// still explain the key it refused.
func checklistRowOperationDescriptors() []operation.OperationDescriptor {
	item := func(
		id operation.OperationID,
		label string,
		description string,
		risk operation.OperationRisk,
	) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Category:             operation.OperationCategoryTask,
			TargetSource:         operation.OperationTargetCurrentTask,
			TargetCardinality:    operation.TargetCardinalityOne,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        checklistRowOperationApplicable,
			Risk:                 risk,
		}
	}
	return []operation.OperationDescriptor{
		item(operation.OperationChecklistToggleItem, "Toggle Item", "Tick or untick this checklist item", operation.OperationRiskMutation),
		operation.WithValueProvider(
			item(operation.OperationChecklistEditItem, "Edit Item", "Rewrite this checklist item", operation.OperationRiskMutation),
			operation.ValueProviderChecklistItem,
		),
		item(operation.OperationChecklistMoveDown, "Move Item Down", "Swap this item with the one below it", operation.OperationRiskMutation),
		item(operation.OperationChecklistMoveUp, "Move Item Up", "Swap this item with the one above it", operation.OperationRiskMutation),
		item(operation.OperationChecklistMoveBottom, "Move Item to Bottom", "Send this item past every item below it", operation.OperationRiskMutation),
		item(operation.OperationChecklistMoveTop, "Move Item to Top", "Send this item past every item above it", operation.OperationRiskMutation),
		operation.WithValueProvider(
			item(operation.OperationChecklistAddItem, "Add Item", "Append an item to this checklist", operation.OperationRiskMutation),
			operation.ValueProviderChecklistItem,
		),
		item(operation.OperationChecklistActions, "Checklist Actions", "Open the checklist item action menu", operation.OperationRiskReadOnly),
		item(operation.OperationChecklistDeleteItem, "Delete Item", "Remove this checklist item", operation.OperationRiskDestructive),
	}
}

// checklistStartOperationDescriptor admits the cursor's task only while that
// task holds no checklist at all, which is exactly the case no checklist row can
// cover. The gate is disjoint from OperationChecklistAddItem rather than a
// second route to it, so the row grammar keeps owning every later item.
func (m *AppModel) checklistStartOperationDescriptor() operation.OperationDescriptor {
	return operation.WithValueProvider(operation.OperationDescriptor{
		ID:                   operation.OperationChecklistStart,
		Label:                "Start Checklist",
		Description:          "Add the first checklist item to this task",
		Category:             operation.OperationCategoryTask,
		TargetSource:         operation.OperationTargetCurrentTask,
		TargetCardinality:    operation.TargetCardinalityOne,
		RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
		Applicability:        m.checklistStartApplicable,
		Risk:                 operation.OperationRiskMutation,
	}, operation.ValueProviderChecklistItem)
}

func (m *AppModel) checklistStartApplicable(ctx operation.CommandContext) bool {
	if ctx.View != presentation.ViewCrossProjectOutliner ||
		ctx.Entity.HasChecklistCursor ||
		ctx.Selection.Visual ||
		operation.CommandContextNamesNonTaskEntity(ctx) {
		return false
	}
	identity := operation.CurrentTaskIdentity(ctx)
	if !identity.Valid() {
		return false
	}
	task, _ := m.taskAndProjectForIdentity(identity)
	return task != nil && len(task.Checklist) == 0
}

// executeChecklistStart appends the first item through the same coordinator the
// row grammar uses, then expands the checklist so the item it just created is
// visible and the row keys take over from here.
func (m *AppModel) executeChecklistStart(
	invocation OperationInvocation,
) OperationHandlerResult {
	identity := operation.CurrentTaskIdentity(invocation.Context)
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil {
		return OperationHandlerResult{Err: errors.New("task is no longer available")}
	}
	text, err := checklistItemText(invocation)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	if out, ok := m.outlinerPrimary(); ok && out != nil {
		out.setChecklistExpanded(identity, true)
	}
	return checklistCommandResult(m.secondaryFeatures.Subview.AddChecklistItem(identity, text))
}

func (m *AppModel) registerChecklistRowOperations() error {
	if m == nil || m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		operation.ValueProviderChecklistItem,
		pendingTextValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range checklistRowOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler:              m.executeChecklistRowOperation,
			// Every handler dispatches to the subview coordinator, which owns the
			// application-layer commit and rollback boundary already.
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	start := m.checklistStartOperationDescriptor()
	return m.registerOperation(start, OperationBinding{
		RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
		Applicability:        start.Applicability,
		Handler:              m.executeChecklistStart,
		DelegateTransaction:  true,
	})
}

// executeChecklistRowOperation resolves the frozen owner task and item index,
// then dispatches one application-layer mutation. The cursor is re-read only
// through the command context the invocation carries, so a row rebuild between
// the key press and a nested value step cannot retarget the action.
func (m *AppModel) executeChecklistRowOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	entity := invocation.Context.Entity
	if !entity.HasChecklistCursor {
		return OperationHandlerResult{Err: errors.New(operation.ChecklistRowRejectionMessage)}
	}
	identity := operation.CurrentTaskIdentity(invocation.Context)
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil {
		return OperationHandlerResult{Err: errors.New("task is no longer available")}
	}
	index := entity.ChecklistIndex
	if index < 0 || index >= len(task.Checklist) {
		return OperationHandlerResult{Err: errors.New("checklist item is no longer available")}
	}

	coordinator := m.secondaryFeatures.Subview
	switch invocation.Descriptor.ID {
	case operation.OperationChecklistToggleItem:
		return checklistCommandResult(coordinator.ToggleChecklistItem(identity, index))
	case operation.OperationChecklistEditItem:
		text, err := checklistItemText(invocation)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return checklistCommandResult(coordinator.EditChecklistItem(identity, index, text))
	case operation.OperationChecklistDeleteItem:
		return checklistCommandResult(coordinator.RemoveChecklistItem(identity, index))
	case operation.OperationChecklistMoveDown:
		if index == len(task.Checklist)-1 {
			return OperationHandlerResult{Err: errors.New("this item is already last")}
		}
		m.followReorderedChecklistItem(1)
		return checklistCommandResult(coordinator.ReorderChecklistItem(identity, index, index+1))
	case operation.OperationChecklistMoveUp:
		if index == 0 {
			return OperationHandlerResult{Err: errors.New("this item is already first")}
		}
		m.followReorderedChecklistItem(-1)
		return checklistCommandResult(coordinator.ReorderChecklistItem(identity, index, index-1))
	case operation.OperationChecklistMoveBottom:
		last := len(task.Checklist) - 1
		if index == last {
			return OperationHandlerResult{Err: errors.New("this item is already last")}
		}
		m.followReorderedChecklistItem(last - index)
		return checklistCommandResult(coordinator.MoveChecklistItem(identity, index, last))
	case operation.OperationChecklistMoveTop:
		if index == 0 {
			return OperationHandlerResult{Err: errors.New("this item is already first")}
		}
		m.followReorderedChecklistItem(-index)
		return checklistCommandResult(coordinator.MoveChecklistItem(identity, index, 0))
	case operation.OperationChecklistAddItem:
		text, err := checklistItemText(invocation)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return checklistCommandResult(coordinator.AddChecklistItem(identity, text))
	case operation.OperationChecklistActions:
		if err := m.openChecklistActionSheet(invocation.Context); err != nil {
			return OperationHandlerResult{Err: err}
		}
		return OperationHandlerResult{}
	default:
		return OperationHandlerResult{
			Err: fmt.Errorf("operation %s has no checklist handler", invocation.Descriptor.ID),
		}
	}
}

// followReorderedChecklistItem walks the cursor with the item it just moved.
// Checklist items occupy contiguous rows and the rebuild that follows the
// transaction re-seats a checklist cursor by row index, so a one-row step is
// what keeps the item under the cursor; without it a second J would swap the
// pair straight back.
func (m *AppModel) followReorderedChecklistItem(delta int) {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return
	}
	target := out.cursor + delta
	if target < 0 || target >= len(out.rows) || !out.rows[target].IsChecklistItem {
		return
	}
	out.cursor = target
}

func checklistCommandResult(cmd tea.Cmd) OperationHandlerResult {
	return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
}

func checklistItemText(invocation OperationInvocation) (string, error) {
	text, ok := invocation.Value.(string)
	if !ok {
		return "", fmt.Errorf("operation %s requires item text", invocation.Descriptor.ID)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("item text cannot be empty")
	}
	return text, nil
}

// checklistActionSheetOperations is the checklist row's Space menu. Delete has
// no direct key and reaches the user only here, behind the executor's
// destructive confirmation.
var checklistActionSheetOperations = []operation.OperationID{
	operation.OperationChecklistEditItem,
	operation.OperationChecklistDeleteItem,
}

func (m *AppModel) openChecklistActionSheet(context operation.CommandContext) error {
	if !context.Entity.HasChecklistCursor {
		return errors.New(operation.ChecklistRowRejectionMessage)
	}
	sheet, err := NewContextActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		context,
		checklistActionSheetOperations,
		ContextActionSheetConfig{
			Host:         ContextActionHostOutliner,
			Completion:   ContextActionCompleteCloseSheet,
			Title:        "Checklist Item",
			Subtitle:     m.checklistItemTextForContext(context),
			ItemIDPrefix: "checklist-action:",
			SecondaryFor: operationActionSecondaryText,
			Targets:      1,
			EmptyState: ListEmptyState{
				Title:  "No checklist actions available.",
				Detail: "Move the cursor to a checklist item.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No matching actions.",
				Detail: "Clear or change the search.",
			},
		},
	)
	if err != nil {
		return err
	}
	m.contextActionSheet = sheet
	return nil
}

func (m *AppModel) checklistItemTextForContext(context operation.CommandContext) string {
	task, _ := m.taskAndProjectForIdentity(operation.CurrentTaskIdentity(context))
	if task == nil {
		return ""
	}
	index := context.Entity.ChecklistIndex
	if index < 0 || index >= len(task.Checklist) {
		return ""
	}
	return strings.TrimSpace(task.Checklist[index].Text)
}
