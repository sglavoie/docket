package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// Comment operation IDs. They name what the Comments panel does to the comment
// under its cursor, so they stay apart from tui.task.comments, which only opens
// the panel and belongs to the task row that owns it.
const (
	operationCommentAdd          operation.OperationID = "tui.comment.add"
	operationCommentEdit         operation.OperationID = "tui.comment.edit"
	operationCommentEditExternal operation.OperationID = "tui.comment.edit-in-external-editor"
	operationCommentDelete       operation.OperationID = "tui.comment.delete"
	operationCommentCopy         operation.OperationID = "tui.comment.copy"
	operationCommentCopyAll      operation.OperationID = "tui.comment.copy-all-as-markdown"
	operationCommentExpand       operation.OperationID = "tui.comment.expand"
	operationCommentActions      operation.OperationID = "tui.comment.actions"
)

// CommentsInputStrategy is the Comments panel's whole browse grammar and the
// single declaration of what each of its keys means. Six keys stay direct
// because they are what the panel is for; Delete reaches the user through the
// Space sheet instead, so removing a comment costs a deliberate menu row rather
// than one press of a key the Outliner retired.
//
// The cursor keys, help, Esc, and the workspace ledger's undo and redo resolve
// to the core operations because they are the same acts everywhere. Those
// descriptors answer only to the Outliner, so they document the panel's grammar
// without dispatching here: the panel moves its own cursor, raises its own help,
// closes itself, and hands u and Ctrl+R to the shell's one ledger. Adding,
// editing, and removing a comment are ledger transactions that already install a
// `u undo` recovery toast, so undo has something to reverse from inside the
// panel that made them rather than a promise only the Outliner could keep.
//
// No binding carries footer metadata. The panel's footer has to narrow with the
// add, edit, expanded, and confirmation sub-states, which only the gated keymap
// beside this strategy can express, so ranking a key here would be a second
// description of the same footer with no way to stay in step.
func CommentsInputStrategy() operation.InputStrategy {
	return operation.InputStrategy{
		ID: operation.InputStrategyComments,
		Bindings: []operation.OperationKeyBinding{
			{OperationID: operationCommentExpand, Keys: []string{"enter"}, DisplayKey: "Enter"},
			{OperationID: operationCommentAdd, Keys: []string{"a"}, DisplayKey: "a"},
			{OperationID: operationCommentEdit, Keys: []string{"e"}, DisplayKey: "e"},
			{OperationID: operationCommentActions, Keys: []string{"space"}, DisplayKey: "Space"},
			{OperationID: operationCommentCopy, Keys: []string{"y"}, DisplayKey: "y"},
			{OperationID: operationCommentCopyAll, Keys: []string{"Y"}, DisplayKey: "Y"},
			{OperationID: operationCommentEditExternal, Keys: []string{"E"}, DisplayKey: "E"},
			{OperationID: operation.OperationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{OperationID: operation.OperationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: operation.OperationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: operation.OperationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: operation.OperationCoreCursorTop, Keys: []string{"home"}, Sequence: []string{"g", "g"}, DisplayKey: "gg/Home", HintLabel: "top"},
			{OperationID: operation.OperationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			{OperationID: operation.OperationCoreHelp, Keys: []string{"?"}, DisplayKey: "?"},
			{OperationID: operation.OperationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
		},
		ChordGroups: []operation.OperationChordGroup{
			{Sequence: []string{"g"}, HintLabel: "go to"},
		},
	}
}

// commentOperationDescriptors declares the seven acts the Comments panel
// performs on one comment, plus the sheet that lists the destructive one. None
// of them names a target the registry can resolve: a comment has no entity
// identity, so the cursor inside the panel is the only thing that says which
// comment an operation means, and every descriptor is therefore target-free and
// capability-scoped to the mounted panel.
func commentOperationDescriptors() []operation.OperationDescriptor {
	comment := func(
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
			TargetSource:         operation.OperationTargetGlobal,
			TargetCardinality:    operation.TargetCardinalityNone,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityCommentsPanel),
			Risk:                 risk,
		}
	}
	return []operation.OperationDescriptor{
		comment(operationCommentAdd, "Add comment", "Write a new comment on this task", operation.OperationRiskMutation),
		comment(operationCommentEdit, "Edit comment", "Edit this comment inline", operation.OperationRiskMutation),
		comment(operationCommentEditExternal, "Edit comment in external editor", "Edit this comment in the external editor", operation.OperationRiskMutation),
		comment(operationCommentDelete, "Delete comment", "Remove this comment from the task", operation.OperationRiskDestructive),
		comment(operationCommentCopy, "Copy comment text", "Copy this comment's text to the clipboard", operation.OperationRiskReadOnly),
		comment(operationCommentCopyAll, "Copy all as Markdown", "Copy every comment as a numbered Markdown list", operation.OperationRiskReadOnly),
		comment(operationCommentExpand, "Expand comment", "Show this comment's full text", operation.OperationRiskReadOnly),
		comment(operationCommentActions, "Comment actions", "Open the action sheet for the comment under the cursor", operation.OperationRiskReadOnly),
	}
}

// commentOperationManifest is the authoritative ordered Comments action list.
// Seven entries are DirectOnly: they keep the keys the panel exists for and
// leave the Space sheet to Delete, which is destructive and so carries no direct
// key at all.
func commentOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{ID: operationCommentExpand, Label: "Expand comment", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationCommentAdd, Label: "Add comment", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationCommentEdit, Label: "Edit comment", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationCommentEditExternal, Label: "Edit comment in external editor", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationCommentCopy, Label: "Copy comment text", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationCommentCopyAll, Label: "Copy all as Markdown", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationCommentActions, Label: "Comment actions", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationCommentDelete, Label: "Delete comment", Risk: operation.OperationRiskDestructive},
	}
}

// commentActionSheetOperations is the Comments Space sheet. It holds Delete
// alone: the panel has no multi-select, so this is one comment's action sheet
// rather than a bulk menu, and Delete reaches the user only here behind the
// panel's own confirmation.
var commentActionSheetOperations = []operation.OperationID{operationCommentDelete}

// openCommentsActionSheet raises the shell-owned sheet over the panel. Its row
// acts on the panel's cursor rather than on a named entity, so an empty list or
// a cursor past its end names nothing and the sheet refuses to open.
func (m *AppModel) openCommentsActionSheet(ctx operation.CommandContext, panel *TaskCommentsModel) error {
	if !panel.hasSelection() {
		return errors.New("select a comment to open actions")
	}
	sheet, err := listsurface.NewContextActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		ctx,
		commentActionSheetOperations,
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostComments,
			Completion:   listsurface.ContextActionCompleteCloseSheet,
			Title:        "Comment Actions",
			Subtitle:     fmt.Sprintf("Comment [%d]", panel.Cursor),
			ItemIDPrefix: "comment-action:",
			SecondaryFor: operationActionSecondaryText,
			Targets:      1,
			EmptyState: listsurface.ListEmptyState{
				Title:  "No comment actions available.",
				Detail: "Move the cursor to a comment.",
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
	m.contextActionSheet = sheet
	return nil
}

// validateCommentOperationOnePath holds Comments to the shared one-path rule: a
// manifested operation is either bound directly and marked DirectOnly or listed
// in a menu, never both.
func validateCommentOperationOnePath(strategy operation.InputStrategy) error {
	return operation.ValidateOperationOnePath(commentOperationManifest(), strategy)
}

// registerCommentOperations installs the panel's vocabulary. Every binding
// delegates all four executor stages because the panel already owns them: it
// resolves its own comment from its cursor, collects the text in its own
// textarea, runs its own confirmation before a delete, and commits through the
// subview coordinator's transaction.
func (m *AppModel) registerCommentOperations() error {
	for _, descriptor := range commentOperationDescriptors() {
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilityCommentsPanel),
			Handler:                  m.executeCommentOperation,
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// executeCommentOperation applies one operation to the live panel. Every one of
// them moves the model the user is looking at, so the handler reaches for the
// mounted panel rather than rebuilding one.
//
//nolint:exhaustive // Only the Comments grammar's operations are registered with this handler.
func (m *AppModel) executeCommentOperation(invocation operation.OperationInvocation) operation.OperationHandlerResult {
	panel := m.activeCommentsPanel()
	if panel == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Comments panel is not open")}
	}
	command := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	switch invocation.Descriptor.ID {
	case operationCommentAdd:
		return command(panel.beginAddMode())
	case operationCommentEdit:
		return command(panel.startEditSelectedComment())
	case operationCommentEditExternal:
		return command(panel.editSelectedCommentInExternalEditor())
	case operationCommentCopy:
		panel.copySelectedCommentText()
	case operationCommentCopyAll:
		panel.copyAllCommentsText()
	case operationCommentActions:
		if err := m.openCommentsActionSheet(invocation.Context, panel); err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
	case operationCommentDelete:
		panel.confirmDeleteSelectedComment()
	case operationCommentExpand:
		panel.expandSelectedComment()
	}
	return operation.OperationHandlerResult{}
}

// activeCommentsPanel returns the mounted panel, or nil when the route moved on.
// A dispatched key already knows the panel is what it came from, so the lookup
// keys on the current view rather than searching the whole stack.
func (m *AppModel) activeCommentsPanel() *TaskCommentsModel {
	if m == nil || m.views == nil || m.currentView() != presentation.ViewTaskComments {
		return nil
	}
	panel, _ := m.views.Get(presentation.ViewTaskComments).(*TaskCommentsModel)
	return panel
}
