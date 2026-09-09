package entityops

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// checklistRowOperationApplicable admits an operation only while the cursor
// rests on a checklist row. That disjointness is what lets these operations
// share e, J, K and Space with their task-row counterparts, since shared-key
// resolution picks the first binding whose descriptor applies.
func checklistRowOperationApplicable(ctx operation.CommandContext) bool {
	return ctx.View == presentation.ViewCrossProjectOutliner && ctx.Entity.HasChecklistCursor
}

// checklistAddItemApplicable is the one checklist operation that also answers
// from the owner's own row, because `c` appends to a checklist the task may not
// have yet: an empty one has no item row the row grammar could act on. Every
// other operation names an existing item and stays checklist-row only.
//
// A task row qualifies only when the cursor positively names that task, so the
// key refuses on a section header, a project header and an empty projection
// rather than falling back to a nearby task. A visual selection refuses too:
// one item appended to one of N tasks is not a bulk act.
func checklistAddItemApplicable(ctx operation.CommandContext) bool {
	if ctx.View != presentation.ViewCrossProjectOutliner {
		return false
	}
	if ctx.Entity.HasChecklistCursor {
		return true
	}
	if ctx.Selection.Visual || operation.CommandContextNamesNonTaskEntity(ctx) {
		return false
	}
	return operation.CurrentTaskIdentity(ctx).Valid()
}

// ChecklistRowOperationDescriptors is the authoritative catalog of checklist
// row operations. They target the owner task because that is the durable
// identity a checklist item hangs off; the item itself is named by the index
// the command context froze.
//
// Every descriptor is Task category so that a key shared with a task operation
// keeps one category across its bindings, which is what lets a non-task cursor
// still explain the key it refused.
func ChecklistRowOperationDescriptors() []operation.OperationDescriptor {
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
	withApplicability := func(
		descriptor operation.OperationDescriptor,
		applicable func(operation.CommandContext) bool,
	) operation.OperationDescriptor {
		descriptor.Applicability = applicable
		return descriptor
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
			withApplicability(
				item(operation.OperationChecklistAddItem, "Add Item", "Append an item to this task's checklist", operation.OperationRiskMutation),
				checklistAddItemApplicable,
			),
			operation.ValueProviderChecklistItem,
		),
		item(operation.OperationChecklistActions, "Checklist Actions", "Open the checklist item action menu", operation.OperationRiskReadOnly),
		item(operation.OperationChecklistDeleteItem, "Delete Item", "Remove this checklist item", operation.OperationRiskDestructive),
	}
}

// ChecklistItemRejectedMsg carries a refused append back to the shell so the
// prompt can reopen on the item the user already typed. It exists because the
// failure can only be known after the value overlay closed — the commit resolves
// the owner again and may find an unwritable project or a newer revision — and
// the typed item is the user's work, not the failure's. Request is the retry the
// shell resubmits and Owner is the identity the append was frozen onto, so a
// cursor that moved in the meantime cannot redirect the retry.
type ChecklistItemRejectedMsg struct {
	Request operation.OperationExecutionRequest
	Owner   taskref.TaskIdentity
	Text    string
	Err     error
}

// withChecklistAppendOutcome decorates both ends of one append. Success names
// the owner task so the shell opens its checklist and seats the cursor on the
// item that was just written; a refusal is rewritten into the retry message
// above. Neither half touches the fold here, which is what keeps a refused write
// from expanding an outline merely because an item was attempted: the reveal
// rides the success message alone.
func withChecklistAppendOutcome(
	cmd tea.Cmd,
	identity taskref.TaskIdentity,
	request operation.OperationExecutionRequest,
	text string,
) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		switch typed := msg.(type) {
		case coordinator.TransactionResultMsg:
			typed.RevealChecklistFor = identity
			return typed
		case coordinator.TransactionErrorMsg:
			return ChecklistItemRejectedMsg{
				Request: request,
				Owner:   identity,
				Text:    text,
				Err:     typed.Err,
			}
		default:
			return msg
		}
	}
}

func (c *Controller) RegisterChecklistRowOperations(shell Shell) error {
	if c == nil || shell == nil || shell.Registry() == nil || shell.Executor() == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := shell.Executor().RegisterValueProvider(
		operation.ValueProviderChecklistItem,
		operation.PendingTextValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range ChecklistRowOperationDescriptors() {
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler: func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
				return c.executeChecklistRowOperation(shell, invocation)
			},
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
func (c *Controller) executeChecklistRowOperation(
	shell Shell,
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	// Add Item is the one member of this family that names no existing item, so
	// it resolves before the checklist-cursor gate every other member needs.
	if invocation.Descriptor.ID == operation.OperationChecklistAddItem {
		return c.executeChecklistAddItem(shell, invocation)
	}
	entity := invocation.Context.Entity
	if !entity.HasChecklistCursor {
		return operation.OperationHandlerResult{Err: errors.New(operation.ChecklistRowRejectionMessage)}
	}
	identity := operation.CurrentTaskIdentity(invocation.Context)
	task, _ := shell.TaskAndProjectForIdentity(identity)
	if task == nil {
		return operation.OperationHandlerResult{Err: errors.New("task is no longer available")}
	}
	index := entity.ChecklistIndex
	if index < 0 || index >= len(task.Checklist) {
		return operation.OperationHandlerResult{Err: errors.New("checklist item is no longer available")}
	}

	coordinator := shell.SubviewCoordinator()
	switch invocation.Descriptor.ID {
	case operation.OperationChecklistToggleItem:
		return checklistCommandResult(coordinator.ToggleChecklistItem(identity, index))
	case operation.OperationChecklistEditItem:
		text, err := checklistItemText(invocation)
		if err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
		return checklistCommandResult(coordinator.EditChecklistItem(identity, index, text))
	case operation.OperationChecklistDeleteItem:
		return checklistCommandResult(coordinator.RemoveChecklistItem(identity, index))
	case operation.OperationChecklistMoveDown:
		if index == len(task.Checklist)-1 {
			return operation.OperationHandlerResult{Err: errors.New("this item is already last")}
		}
		c.followReorderedChecklistItem(shell, 1)
		return checklistCommandResult(coordinator.ReorderChecklistItem(identity, index, index+1))
	case operation.OperationChecklistMoveUp:
		if index == 0 {
			return operation.OperationHandlerResult{Err: errors.New("this item is already first")}
		}
		c.followReorderedChecklistItem(shell, -1)
		return checklistCommandResult(coordinator.ReorderChecklistItem(identity, index, index-1))
	case operation.OperationChecklistMoveBottom:
		last := len(task.Checklist) - 1
		if index == last {
			return operation.OperationHandlerResult{Err: errors.New("this item is already last")}
		}
		c.followReorderedChecklistItem(shell, last-index)
		return checklistCommandResult(coordinator.MoveChecklistItem(identity, index, last))
	case operation.OperationChecklistMoveTop:
		if index == 0 {
			return operation.OperationHandlerResult{Err: errors.New("this item is already first")}
		}
		c.followReorderedChecklistItem(shell, -index)
		return checklistCommandResult(coordinator.MoveChecklistItem(identity, index, 0))
	case operation.OperationChecklistActions:
		if err := c.openChecklistActionSheet(shell, invocation.Context); err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
		return operation.OperationHandlerResult{}
	default:
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("operation %s has no checklist handler", invocation.Descriptor.ID),
		}
	}
}

// executeChecklistAddItem appends one item to the cursor's owner task, whether
// the cursor rests on that task's row or on one of its existing items. It
// resolves the owner identity rather than an item index, which is what lets the
// same operation answer from a task whose checklist is still empty.
func (c *Controller) executeChecklistAddItem(
	shell Shell,
	invocation operation.OperationInvocation,
) operation.OperationHandlerResult {
	identity := operation.CurrentTaskIdentity(invocation.Context)
	task, _ := shell.TaskAndProjectForIdentity(identity)
	if task == nil {
		return operation.OperationHandlerResult{Err: errors.New("task is no longer available")}
	}
	text, err := checklistItemText(invocation)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	// The retry request is rebuilt from the invocation rather than carried
	// through the mutation: ID and context are all the prompt factory reads, and
	// the value it would carry is exactly the one being retried.
	return checklistCommandResult(withChecklistAppendOutcome(
		shell.SubviewCoordinator().AddChecklistItem(identity, text),
		identity,
		operation.OperationExecutionRequest{
			ID:      invocation.Descriptor.ID,
			Context: invocation.Context,
		},
		text,
	))
}

// followReorderedChecklistItem walks the cursor with the item it just moved.
// Checklist items occupy contiguous rows and the rebuild that follows the
// transaction re-seats a checklist cursor by row index, so a one-row step is
// what keeps the item under the cursor; without it a second J would swap the
// pair straight back.
func (c *Controller) followReorderedChecklistItem(shell Shell, delta int) {
	out, ok := shell.OutlinerPrimary()
	if !ok || out == nil {
		return
	}
	target := out.Cursor + delta
	if target < 0 || target >= len(out.Rows) || !out.Rows[target].IsChecklistItem {
		return
	}
	out.Cursor = target
}

func checklistCommandResult(cmd tea.Cmd) operation.OperationHandlerResult {
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
}

func checklistItemText(invocation operation.OperationInvocation) (string, error) {
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

func (c *Controller) openChecklistActionSheet(shell Shell, context operation.CommandContext) error {
	if !context.Entity.HasChecklistCursor {
		return errors.New(operation.ChecklistRowRejectionMessage)
	}
	sheet, err := listsurface.NewContextActionSheetModel(
		shell.Registry(),
		shell.Executor(),
		context,
		checklistActionSheetOperations,
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostOutliner,
			Completion:   listsurface.ContextActionCompleteCloseSheet,
			Title:        "Checklist Item",
			Subtitle:     c.ChecklistItemTextForContext(shell, context),
			ItemIDPrefix: "checklist-action:",
			SecondaryFor: operation.ActionSecondaryText,
			Targets:      1,
			EmptyState: listsurface.ListEmptyState{
				Title:  "No checklist actions available.",
				Detail: "Move the cursor to a checklist item.",
			},
			NoMatchState: listsurface.ListEmptyState{
				Title:  "No matching actions.",
				Detail: "Clear or change the search.",
			},
		},
	)
	if err != nil {
		return err
	}
	shell.MountActionSheet(sheet)
	return nil
}

func (c *Controller) ChecklistItemTextForContext(shell Shell, context operation.CommandContext) string {
	task, _ := shell.TaskAndProjectForIdentity(operation.CurrentTaskIdentity(context))
	if task == nil {
		return ""
	}
	index := context.Entity.ChecklistIndex
	if index < 0 || index >= len(task.Checklist) {
		return ""
	}
	return strings.TrimSpace(task.Checklist[index].Text)
}
