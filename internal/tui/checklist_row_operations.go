package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Checklist row operation IDs. Each one acts on the item the cursor names,
// addressed as an index inside the owner task the checklist row carries.
const (
	operationChecklistToggleItem OperationID = "tui.checklist.toggle-item"
	operationChecklistEditItem   OperationID = "tui.checklist.edit-item"
	operationChecklistMoveDown   OperationID = "tui.checklist.move-down"
	operationChecklistMoveUp     OperationID = "tui.checklist.move-up"
	operationChecklistAddItem    OperationID = "tui.checklist.add-item"
	operationChecklistActions    OperationID = "tui.checklist.actions"
	operationChecklistDeleteItem OperationID = "tui.checklist.delete-item"
)

// checklistRowGrammar is the set of operations a checklist cursor owns. It is
// what operationRefusedOnChecklistRow exempts, so the refusal rule stays one
// question — does this operation belong to the checklist row? — instead of a
// per-key branch.
var checklistRowGrammar = map[OperationID]bool{
	operationChecklistToggleItem: true,
	operationChecklistEditItem:   true,
	operationChecklistMoveDown:   true,
	operationChecklistMoveUp:     true,
	operationChecklistAddItem:    true,
	operationChecklistActions:    true,
	operationChecklistDeleteItem: true,
}

// checklistRowRejectionMessage is the one sentence every refused operation
// shows from a checklist cursor. It names the row kind the action needs rather
// than the target it failed to find, because the owner task was found: it is
// simply not what the user is pointing at.
const checklistRowRejectionMessage = "move the cursor to a task row to use this action"

// coreOperationsRefusedOnChecklistRow lists the core operations a checklist
// cursor turns off. Category alone cannot name them: the core family mixes
// navigation, which is how the cursor leaves the row and therefore stays, with
// operations that reach through the row to the owner task whose pointer it
// carries. Selection is refused because a selection names task rows, and Peek
// because the item has no detail panel of its own.
var coreOperationsRefusedOnChecklistRow = map[OperationID]bool{
	operationCoreCapture:         true,
	operationCoreEdit:            true,
	operationCoreEditDescription: true,
	operationCoreExternalEditor:  true,
	operationCoreBacklog:         true,
	operationCoreToday:           true,
	operationCoreNext:            true,
	operationCoreActions:         true,
	operationCorePeek:            true,
	operationCoreReorderDown:     true,
	operationCoreReorderUp:       true,
	operationCoreMultiSelect:     true,
	operationCoreRangeSelect:     true,
	operationCoreSelectAll:       true,
}

// operationRefusedOnChecklistRow decides, by category rather than by key, which
// operations a checklist cursor turns off. Every task and tag operation names
// the owner task and is refused, as are the core operations listed above.
func operationRefusedOnChecklistRow(descriptor OperationDescriptor) bool {
	if checklistRowGrammar[descriptor.ID] {
		return false
	}
	switch descriptor.Category {
	case OperationCategoryTask, OperationCategoryTag:
		return true
	default:
		return coreOperationsRefusedOnChecklistRow[descriptor.ID]
	}
}

// checklistRowOperationApplicable admits an operation only while the cursor
// rests on a checklist row. That disjointness is what lets these operations
// share e, A, J, K and Space with their task-row counterparts, since shared-key
// resolution picks the first binding whose descriptor applies.
func checklistRowOperationApplicable(ctx CommandContext) bool {
	return ctx.View == ViewCrossProjectOutliner && ctx.Entity.HasChecklistCursor
}

// checklistRowOperationDescriptors is the authoritative catalog of checklist
// row operations. They target the owner task because that is the durable
// identity a checklist item hangs off; the item itself is named by the index
// the command context froze.
//
// Every descriptor is Task category so that a key shared with a task operation
// keeps one category across its bindings, which is what lets a non-task cursor
// still explain the key it refused.
func checklistRowOperationDescriptors() []OperationDescriptor {
	item := func(
		id OperationID,
		label string,
		description string,
		risk OperationRisk,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Category:             OperationCategoryTask,
			TargetSource:         OperationTargetCurrentTask,
			TargetCardinality:    TargetCardinalityOne,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        checklistRowOperationApplicable,
			Risk:                 risk,
		}
	}
	return []OperationDescriptor{
		item(operationChecklistToggleItem, "Toggle Item", "Tick or untick this checklist item", OperationRiskMutation),
		withValueProvider(
			item(operationChecklistEditItem, "Edit Item", "Rewrite this checklist item", OperationRiskMutation),
			ValueProviderChecklistItem,
		),
		item(operationChecklistMoveDown, "Move Item Down", "Swap this item with the one below it", OperationRiskMutation),
		item(operationChecklistMoveUp, "Move Item Up", "Swap this item with the one above it", OperationRiskMutation),
		withValueProvider(
			item(operationChecklistAddItem, "Add Item", "Append an item to this checklist", OperationRiskMutation),
			ValueProviderChecklistItem,
		),
		item(operationChecklistActions, "Checklist Actions", "Open the checklist item action menu", OperationRiskReadOnly),
		item(operationChecklistDeleteItem, "Delete Item", "Remove this checklist item", OperationRiskDestructive),
	}
}

func (m *AppModel) registerChecklistRowOperations() error {
	if m == nil || m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		ValueProviderChecklistItem,
		pendingTextValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range checklistRowOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler:              m.executeChecklistRowOperation,
			// Every handler dispatches to the subview coordinator, which owns the
			// application-layer commit and rollback boundary already.
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	return nil
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
		return OperationHandlerResult{Err: errors.New(checklistRowRejectionMessage)}
	}
	identity := currentTaskIdentity(invocation.Context)
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
	case operationChecklistToggleItem:
		return checklistCommandResult(coordinator.ToggleChecklistItem(identity, index))
	case operationChecklistEditItem:
		text, err := checklistItemText(invocation)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return checklistCommandResult(coordinator.EditChecklistItem(identity, index, text))
	case operationChecklistDeleteItem:
		return checklistCommandResult(coordinator.RemoveChecklistItem(identity, index))
	case operationChecklistMoveDown:
		if index == len(task.Checklist)-1 {
			return OperationHandlerResult{Err: errors.New("this item is already last")}
		}
		m.followReorderedChecklistItem(1)
		return checklistCommandResult(coordinator.ReorderChecklistItem(identity, index, index+1))
	case operationChecklistMoveUp:
		if index == 0 {
			return OperationHandlerResult{Err: errors.New("this item is already first")}
		}
		m.followReorderedChecklistItem(-1)
		return checklistCommandResult(coordinator.ReorderChecklistItem(identity, index, index-1))
	case operationChecklistAddItem:
		text, err := checklistItemText(invocation)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return checklistCommandResult(coordinator.AddChecklistItem(identity, text))
	case operationChecklistActions:
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
var checklistActionSheetOperations = []OperationID{
	operationChecklistEditItem,
	operationChecklistDeleteItem,
}

func (m *AppModel) openChecklistActionSheet(context CommandContext) error {
	if !context.Entity.HasChecklistCursor {
		return errors.New(checklistRowRejectionMessage)
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

func (m *AppModel) checklistItemTextForContext(context CommandContext) string {
	task, _ := m.taskAndProjectForIdentity(currentTaskIdentity(context))
	if task == nil {
		return ""
	}
	index := context.Entity.ChecklistIndex
	if index < 0 || index >= len(task.Checklist) {
		return ""
	}
	return strings.TrimSpace(task.Checklist[index].Text)
}
