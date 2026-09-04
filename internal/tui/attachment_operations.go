package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// Attachment operation IDs. They name what the Attachments panel does to the
// item under its cursor, so they stay apart from tui.task.attachments, which
// only opens the panel and belongs to the task row that owns it.
const (
	operationAttachmentAdd                   operation.OperationID = "tui.attachment.add"
	operationAttachmentOpen                  operation.OperationID = "tui.attachment.open"
	operationAttachmentOpenInEditor          operation.OperationID = "tui.attachment.open-in-editor"
	operationAttachmentOpenOriginalDirectory operation.OperationID = "tui.attachment.open-original-directory"
	operationAttachmentRevealOriginal        operation.OperationID = "tui.attachment.reveal-original"
	operationAttachmentOpenFolder            operation.OperationID = "tui.attachment.open-attachments-folder"
	operationAttachmentDelete                operation.OperationID = "tui.attachment.delete"
	operationAttachmentUnlink                operation.OperationID = "tui.attachment.unlink"
	operationAttachmentCopyPath              operation.OperationID = "tui.attachment.copy-path"
	operationAttachmentCopyPathWithLine      operation.OperationID = "tui.attachment.copy-path-with-line"
	operationAttachmentReorderUp             operation.OperationID = "tui.attachment.reorder-up"
	operationAttachmentReorderDown           operation.OperationID = "tui.attachment.reorder-down"
	operationAttachmentActions               operation.OperationID = "tui.attachment.actions"
)

// AttachmentsInputStrategy is the Attachments panel's whole direct key grammar
// and the single declaration of what each of its keys means. Seven keys stay
// direct because they are what the panel is for; the five rare or destructive
// acts reach the user through the Space sheet instead, so the panel's direct
// grammar no longer contradicts the app-wide meanings of d, x, O, Ctrl+E, and
// Ctrl+O. u is the one the panel took back: it no longer unlinks, so it is free
// to mean here what it means everywhere else.
//
// The external editor answers E, the slot it holds everywhere else, and e is
// deliberately bound to nothing at all. e means edit or rename on all four other
// surfaces and this panel has nothing to rename — an attachment is a file it
// hands to another program — so the key stays silent here rather than becoming a
// second meaning for the one key the app spends on editing.
//
// The cursor keys, help, Esc, and the workspace ledger's undo and redo resolve
// to the core operations because they are the same acts everywhere. Those
// descriptors answer only to the Outliner, so they document the panel's grammar
// without dispatching here: the panel moves its own cursor, raises its own help,
// closes itself, and hands u and Ctrl+R to the shell's one ledger. Adding and
// removing an attachment are ledger transactions, so undo has something to
// reverse from inside the panel that made them.
//
// No binding carries footer metadata. The panel's footer has to narrow with the
// add and confirmation sub-states and with whether the selected attachment is a
// URL or a code reference, which only the gated keymap beside this strategy can
// express, so ranking a key here would be a second description of the same
// footer with no way to stay in step.
func AttachmentsInputStrategy() operation.InputStrategy {
	return operation.InputStrategy{
		ID: operation.InputStrategyAttachments,
		Bindings: []operation.OperationKeyBinding{
			{OperationID: operationAttachmentOpen, Keys: []string{"enter"}, DisplayKey: "Enter"},
			{OperationID: operationAttachmentAdd, Keys: []string{"a"}, DisplayKey: "a"},
			{OperationID: operationAttachmentOpenInEditor, Keys: []string{"E"}, DisplayKey: "E"},
			{OperationID: operationAttachmentCopyPath, Keys: []string{"y"}, DisplayKey: "y"},
			{OperationID: operationAttachmentActions, Keys: []string{"space"}, DisplayKey: "Space"},
			{OperationID: operationAttachmentCopyPathWithLine, Keys: []string{"Y"}, DisplayKey: "Y"},
			{OperationID: operationAttachmentReorderDown, Keys: []string{"J"}, DisplayKey: "J"},
			{OperationID: operationAttachmentReorderUp, Keys: []string{"K"}, DisplayKey: "K"},
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

// attachmentOperationDescriptors declares the twelve acts the Attachments panel
// performs on one item, plus the sheet that lists five of them. None of them
// names a target the registry can resolve: an attachment has no entity identity,
// so the cursor inside the panel is the only thing that says which item an
// operation means, and every descriptor is therefore target-free and
// capability-scoped to the mounted panel.
func attachmentOperationDescriptors() []operation.OperationDescriptor {
	attachment := func(
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
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityAttachmentsPanel),
			Risk:                 risk,
		}
	}
	return []operation.OperationDescriptor{
		attachment(operationAttachmentAdd, "Add attachment", "Attach a file, URL, or code reference", operation.OperationRiskMutation),
		attachment(operationAttachmentOpen, "Open attachment", "Open this attachment with the system handler", operation.OperationRiskReadOnly),
		attachment(operationAttachmentOpenInEditor, "Open attachment in editor", "Open this attachment in the external editor", operation.OperationRiskReadOnly),
		attachment(operationAttachmentOpenOriginalDirectory, "Open original directory", "Open the directory the attached file was copied from", operation.OperationRiskReadOnly),
		attachment(operationAttachmentRevealOriginal, "Reveal original file", "Reveal the attached file's source in the file manager", operation.OperationRiskReadOnly),
		attachment(operationAttachmentOpenFolder, "Open attachments directory", "Open the task's attachments directory", operation.OperationRiskReadOnly),
		attachment(operationAttachmentDelete, "Delete attachment", "Remove this attachment and its stored copy", operation.OperationRiskDestructive),
		attachment(operationAttachmentUnlink, "Unlink attachment", "Remove this attachment but keep its stored copy", operation.OperationRiskDestructive),
		attachment(operationAttachmentCopyPath, "Copy attachment path", "Copy this attachment's path to the clipboard", operation.OperationRiskReadOnly),
		attachment(operationAttachmentCopyPathWithLine, "Copy attachment path with line", "Copy this code reference as path:line", operation.OperationRiskReadOnly),
		attachment(operationAttachmentReorderUp, "Reorder attachment up", "Move this attachment one position earlier", operation.OperationRiskMutation),
		attachment(operationAttachmentReorderDown, "Reorder attachment down", "Move this attachment one position later", operation.OperationRiskMutation),
		attachment(operationAttachmentActions, "Attachment actions", "Open the action sheet for the attachment under the cursor", operation.OperationRiskReadOnly),
	}
}

// attachmentOperationManifest is the authoritative ordered Attachments action
// list. Eight entries are DirectOnly: they keep the keys the panel exists for
// and leave the Space sheet to the three rare acts on an attachment's origin
// plus the two destructive ones, which reach the user only there and then only
// behind the panel's own confirmation.
func attachmentOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{ID: operationAttachmentAdd, Label: "Add attachment", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationAttachmentOpen, Label: "Open attachment", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationAttachmentOpenInEditor, Label: "Open attachment in editor", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationAttachmentCopyPath, Label: "Copy attachment path", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationAttachmentCopyPathWithLine, Label: "Copy attachment path with line", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationAttachmentReorderUp, Label: "Reorder attachment up", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationAttachmentReorderDown, Label: "Reorder attachment down", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: operationAttachmentActions, Label: "Attachment actions", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: operationAttachmentRevealOriginal, Label: "Reveal original file", Risk: operation.OperationRiskReadOnly},
		{ID: operationAttachmentOpenOriginalDirectory, Label: "Open original directory", Risk: operation.OperationRiskReadOnly},
		{ID: operationAttachmentOpenFolder, Label: "Open attachments directory", Risk: operation.OperationRiskReadOnly},
		{ID: operationAttachmentUnlink, Label: "Unlink attachment", Risk: operation.OperationRiskDestructive},
		{ID: operationAttachmentDelete, Label: "Delete attachment", Risk: operation.OperationRiskDestructive},
	}
}

// attachmentActionSheetOperations is the Attachments Space sheet, in the order
// it lists them. The three origin acts are rare enough that a direct key spent
// them for nothing, and the two destructive ones are the reason the sheet
// exists: an attachment now leaves the list only through a deliberate menu row.
var attachmentActionSheetOperations = []operation.OperationID{
	operationAttachmentRevealOriginal,
	operationAttachmentOpenOriginalDirectory,
	operationAttachmentOpenFolder,
	operationAttachmentUnlink,
	operationAttachmentDelete,
}

// openAttachmentsActionSheet raises the shell-owned sheet over the panel. Every
// row acts on the panel's cursor rather than on a named entity, so an empty list
// or a cursor past its end names nothing and the sheet refuses to open.
func (m *AppModel) openAttachmentsActionSheet(ctx operation.CommandContext, panel *TaskAttachmentsModel) error {
	if !panel.hasSelection() {
		return errors.New("select an attachment to open actions")
	}
	sheet, err := listsurface.NewContextActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		ctx,
		attachmentActionSheetOperations,
		listsurface.ContextActionSheetConfig{
			Host:            listsurface.ContextActionHostAttachments,
			Completion:      listsurface.ContextActionCompleteCloseSheet,
			Title:           "Attachment Actions",
			Subtitle:        panel.task.Attachments[panel.Cursor].Filename,
			ItemIDPrefix:    "attachment-action:",
			SecondaryFor:    operationActionSecondaryText,
			ImmediateFilter: true,
			Targets:         1,
			EmptyState: listsurface.ListEmptyState{
				Title:  "No attachment actions available.",
				Detail: "Move the cursor to an attachment.",
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

// validateAttachmentOperationOnePath holds Attachments to the shared one-path
// rule: a manifested operation is either bound directly and marked DirectOnly or
// listed in a menu, never both.
func validateAttachmentOperationOnePath(strategy operation.InputStrategy) error {
	return operation.ValidateOperationOnePath(attachmentOperationManifest(), strategy)
}

// registerAttachmentOperations installs the panel's vocabulary. Every binding
// delegates all four executor stages because the panel already owns them: it
// resolves its own item from its cursor, prompts for the added path inline, runs
// its own confirmation before a delete or an unlink, and commits through the
// attachment coordinator's transaction.
func (m *AppModel) registerAttachmentOperations() error {
	for _, descriptor := range attachmentOperationDescriptors() {
		if err := m.registerOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities:     operation.NewCapabilitySet(operation.CapabilityAttachmentsPanel),
			Handler:                  m.executeAttachmentOperation,
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

// executeAttachmentOperation applies one operation to the live panel. Every one
// of them moves the model the user is looking at, so the handler reaches for the
// mounted panel rather than rebuilding one.
//
//nolint:exhaustive // Only the Attachments grammar's operations are registered with this handler.
func (m *AppModel) executeAttachmentOperation(invocation operation.OperationInvocation) operation.OperationHandlerResult {
	panel := m.activeAttachmentsPanel()
	if panel == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Attachments panel is not open")}
	}
	command := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	switch invocation.Descriptor.ID {
	case operationAttachmentAdd:
		return command(panel.beginAddMode())
	case operationAttachmentOpen:
		return command(panel.openSelectedAttachmentDefault())
	case operationAttachmentOpenInEditor:
		return command(panel.openSelectedAttachmentInEditor())
	case operationAttachmentOpenOriginalDirectory:
		return command(panel.openSelectedOriginalPath())
	case operationAttachmentRevealOriginal:
		return command(panel.revealSelectedOriginalFile())
	case operationAttachmentOpenFolder:
		return command(panel.openAttachmentsDir())
	case operationAttachmentCopyPath:
		return command(panel.copySelectedPath())
	case operationAttachmentCopyPathWithLine:
		return command(panel.copySelectedPathWithLine())
	case operationAttachmentReorderUp:
		return command(panel.moveSelectedAttachmentUp())
	case operationAttachmentReorderDown:
		return command(panel.moveSelectedAttachmentDown())
	case operationAttachmentActions:
		if err := m.openAttachmentsActionSheet(invocation.Context, panel); err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
	case operationAttachmentDelete:
		panel.confirmDeleteSelectedAttachment()
	case operationAttachmentUnlink:
		panel.confirmUnlinkSelectedAttachment()
	}
	return operation.OperationHandlerResult{}
}

// activeAttachmentsPanel returns the mounted panel, or nil when the route moved
// on. A dispatched key already knows the panel is what it came from, so the
// lookup keys on the current view rather than searching the whole stack.
func (m *AppModel) activeAttachmentsPanel() *TaskAttachmentsModel {
	if m == nil || m.views == nil || m.currentView() != presentation.ViewTaskAttachments {
		return nil
	}
	panel, _ := m.views.Get(presentation.ViewTaskAttachments).(*TaskAttachmentsModel)
	return panel
}
