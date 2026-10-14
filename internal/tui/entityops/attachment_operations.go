package entityops

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// Attachment operation IDs. They name what the Attachments panel does to the
// item under its cursor, so they stay apart from tui.task.attachments, which
// only opens the panel and belongs to the task row that owns it.
const (
	OperationAttachmentAdd                   operation.OperationID = "tui.attachment.add"
	OperationAttachmentOpen                  operation.OperationID = "tui.attachment.open"
	OperationAttachmentOpenInEditor          operation.OperationID = "tui.attachment.open-in-editor"
	OperationAttachmentOpenOriginalDirectory operation.OperationID = "tui.attachment.open-original-directory"
	OperationAttachmentRevealOriginal        operation.OperationID = "tui.attachment.reveal-original"
	OperationAttachmentOpenFolder            operation.OperationID = "tui.attachment.open-attachments-folder"
	OperationAttachmentDelete                operation.OperationID = "tui.attachment.delete"
	OperationAttachmentUnlink                operation.OperationID = "tui.attachment.unlink"
	OperationAttachmentCopyPath              operation.OperationID = "tui.attachment.copy-path"
	OperationAttachmentCopyPathWithLine      operation.OperationID = "tui.attachment.copy-path-with-line"
	OperationAttachmentReorderUp             operation.OperationID = "tui.attachment.reorder-up"
	OperationAttachmentReorderDown           operation.OperationID = "tui.attachment.reorder-down"
	OperationAttachmentActions               operation.OperationID = "tui.attachment.actions"
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
			{OperationID: OperationAttachmentOpen, Keys: []string{"enter"}, DisplayKey: "Enter"},
			{OperationID: OperationAttachmentAdd, Keys: []string{"a"}, DisplayKey: "a"},
			{OperationID: OperationAttachmentOpenInEditor, Keys: []string{"E"}, DisplayKey: "E"},
			{OperationID: OperationAttachmentCopyPath, Keys: []string{"y"}, DisplayKey: "y"},
			{OperationID: OperationAttachmentActions, Keys: []string{"space"}, DisplayKey: "Space"},
			{OperationID: OperationAttachmentCopyPathWithLine, Keys: []string{"Y"}, DisplayKey: "Y"},
			{OperationID: OperationAttachmentReorderDown, Keys: []string{"J"}, DisplayKey: "J"},
			{OperationID: OperationAttachmentReorderUp, Keys: []string{"K"}, DisplayKey: "K"},
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
		attachment(OperationAttachmentAdd, "Add attachment", "Attach a file, URL, or code reference", operation.OperationRiskMutation),
		attachment(OperationAttachmentOpen, "Open attachment", "Open this attachment with the system handler", operation.OperationRiskReadOnly),
		attachment(OperationAttachmentOpenInEditor, "Open attachment in editor", "Open this attachment in the external editor", operation.OperationRiskReadOnly),
		attachment(OperationAttachmentOpenOriginalDirectory, "Open original directory", "Open the directory the attached file was copied from", operation.OperationRiskReadOnly),
		attachment(OperationAttachmentRevealOriginal, "Reveal original file", "Reveal the attached file's source in the file manager", operation.OperationRiskReadOnly),
		attachment(OperationAttachmentOpenFolder, "Open attachments directory", "Open the task's attachments directory", operation.OperationRiskReadOnly),
		attachment(OperationAttachmentDelete, "Delete attachment", "Remove this attachment and its stored copy", operation.OperationRiskDestructive),
		attachment(OperationAttachmentUnlink, "Unlink attachment", "Remove this attachment but keep its stored copy", operation.OperationRiskDestructive),
		attachment(OperationAttachmentCopyPath, "Copy attachment path", "Copy this attachment's path to the clipboard", operation.OperationRiskReadOnly),
		attachment(OperationAttachmentCopyPathWithLine, "Copy attachment path with line", "Copy this code reference as path:line", operation.OperationRiskReadOnly),
		attachment(OperationAttachmentReorderUp, "Reorder attachment up", "Move this attachment one position earlier", operation.OperationRiskMutation),
		attachment(OperationAttachmentReorderDown, "Reorder attachment down", "Move this attachment one position later", operation.OperationRiskMutation),
		attachment(OperationAttachmentActions, "Attachment actions", "Open the action sheet for the attachment under the cursor", operation.OperationRiskReadOnly),
	}
}

// AttachmentOperationManifest is the authoritative ordered Attachments action
// list. Eight entries are DirectOnly: they keep the keys the panel exists for
// and leave the Space sheet to the three rare acts on an attachment's origin
// plus the two destructive ones, which reach the user only there and then only
// behind the panel's own confirmation.
func AttachmentOperationManifest() []operation.OperationManifestEntry {
	return []operation.OperationManifestEntry{
		{ID: OperationAttachmentAdd, Label: "Add attachment", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: OperationAttachmentOpen, Label: "Open attachment", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: OperationAttachmentOpenInEditor, Label: "Open attachment in editor", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: OperationAttachmentCopyPath, Label: "Copy attachment path", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: OperationAttachmentCopyPathWithLine, Label: "Copy attachment path with line", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: OperationAttachmentReorderUp, Label: "Reorder attachment up", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: OperationAttachmentReorderDown, Label: "Reorder attachment down", Risk: operation.OperationRiskMutation, DirectOnly: true},
		{ID: OperationAttachmentActions, Label: "Attachment actions", Risk: operation.OperationRiskReadOnly, DirectOnly: true},
		{ID: OperationAttachmentRevealOriginal, Label: "Reveal original file", Risk: operation.OperationRiskReadOnly},
		{ID: OperationAttachmentOpenOriginalDirectory, Label: "Open original directory", Risk: operation.OperationRiskReadOnly},
		{ID: OperationAttachmentOpenFolder, Label: "Open attachments directory", Risk: operation.OperationRiskReadOnly},
		{ID: OperationAttachmentUnlink, Label: "Unlink attachment", Risk: operation.OperationRiskDestructive},
		{ID: OperationAttachmentDelete, Label: "Delete attachment", Risk: operation.OperationRiskDestructive},
	}
}

// attachmentActionSheetOperations is the Attachments Space sheet, in the order
// it lists them. The three origin acts are rare enough that a direct key spent
// them for nothing, and the two destructive ones are the reason the sheet
// exists: an attachment now leaves the list only through a deliberate menu row.
var attachmentActionSheetOperations = []operation.OperationID{
	OperationAttachmentRevealOriginal,
	OperationAttachmentOpenOriginalDirectory,
	OperationAttachmentOpenFolder,
	OperationAttachmentUnlink,
	OperationAttachmentDelete,
}

// openAttachmentsActionSheet raises the shell-owned sheet over the panel. Every
// row acts on the panel's cursor rather than on a named entity, so an empty list
// or a cursor past its end names nothing and the sheet refuses to open.
func (c *Controller) openAttachmentsActionSheet(shell Shell, ctx operation.CommandContext, panel *taskdetail.TaskAttachmentsModel) error {
	if !panel.HasSelection() {
		return errors.New("select an attachment to open actions")
	}
	sheet, err := listsurface.NewContextActionSheetModel(
		shell.Registry(),
		shell.Executor(),
		ctx,
		attachmentActionSheetOperations,
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostAttachments,
			Completion:   listsurface.ContextActionCompleteCloseSheet,
			Title:        "Attachment Actions",
			Subtitle:     panel.Task.Attachments[panel.Cursor].Filename,
			ItemIDPrefix: "attachment-action:",
			SecondaryFor: operation.ActionSecondaryText,
			Targets:      1,
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
	shell.MountActionSheet(sheet)
	return nil
}

// ValidateAttachmentOperationOnePath holds Attachments to the shared one-path
// rule: a manifested operation is either bound directly and marked DirectOnly or
// listed in a menu, never both.
func ValidateAttachmentOperationOnePath(strategy operation.InputStrategy) error {
	return operation.ValidateOperationOnePath(AttachmentOperationManifest(), strategy)
}

// RegisterAttachmentOperations installs the panel's vocabulary. Every binding
// delegates all four executor stages because the panel already owns them: it
// resolves its own item from its cursor, prompts for the added path inline, runs
// its own confirmation before a delete or an unlink, and commits through the
// attachment coordinator's transaction.
func (c *Controller) RegisterAttachmentOperations(shell Shell) error {
	for _, descriptor := range attachmentOperationDescriptors() {
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityAttachmentsPanel),
			Handler: func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
				return c.executeAttachmentOperation(shell, invocation)
			},
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
func (c *Controller) executeAttachmentOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	panel := c.ActiveAttachmentsPanel(shell)
	if panel == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Attachments panel is not open")}
	}
	command := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	switch invocation.Descriptor.ID {
	case OperationAttachmentAdd:
		return command(panel.BeginAddMode())
	case OperationAttachmentOpen:
		return command(panel.OpenSelectedAttachmentDefault())
	case OperationAttachmentOpenInEditor:
		return command(panel.OpenSelectedAttachmentInEditor())
	case OperationAttachmentOpenOriginalDirectory:
		return command(panel.OpenSelectedOriginalPath())
	case OperationAttachmentRevealOriginal:
		return command(panel.RevealSelectedOriginalFile())
	case OperationAttachmentOpenFolder:
		return command(panel.OpenAttachmentsDir())
	case OperationAttachmentCopyPath:
		return command(panel.CopySelectedPath())
	case OperationAttachmentCopyPathWithLine:
		return command(panel.CopySelectedPathWithLine())
	case OperationAttachmentReorderUp:
		return command(panel.MoveSelectedAttachmentUp())
	case OperationAttachmentReorderDown:
		return command(panel.MoveSelectedAttachmentDown())
	case OperationAttachmentActions:
		if err := c.openAttachmentsActionSheet(shell, invocation.Context, panel); err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
	case OperationAttachmentDelete:
		panel.ConfirmDeleteSelectedAttachment()
	case OperationAttachmentUnlink:
		panel.ConfirmUnlinkSelectedAttachment()
	}
	return operation.OperationHandlerResult{}
}

// ActiveAttachmentsPanel returns the mounted panel, or nil when the route moved
// on. A dispatched key already knows the panel is what it came from, so the
// lookup keys on the current view rather than searching the whole stack.
func (c *Controller) ActiveAttachmentsPanel(shell Shell) *taskdetail.TaskAttachmentsModel {
	if shell == nil || shell.CurrentView() != presentation.ViewTaskAttachments {
		return nil
	}
	panel, _ := shell.RenderedView(presentation.ViewTaskAttachments).(*taskdetail.TaskAttachmentsModel)
	return panel
}
