package tui

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskAttachmentsModel.
func (m *TaskAttachmentsModel) buildKeyMap() *keymap.KeyMap {
	inAddMode := func() bool { return m.addMode && !m.confirm.IsActive() }
	inBrowseMode := func() bool { return !m.addMode && !m.confirm.IsActive() }
	hasBrowseSelection := func() bool { return inBrowseMode() && m.hasSelection() }
	hasMultipleAttachments := func() bool {
		return hasBrowseSelection() && m.task != nil && len(m.task.Attachments) > 1
	}
	selectedIsURL := func() bool {
		return hasBrowseSelection() && m.task.Attachments[m.cursor].IsURL
	}
	selectedIsCodeRef := func() bool {
		return hasBrowseSelection() && m.task.Attachments[m.cursor].IsCodeRef
	}
	addLabel := "add"
	if m.err != nil {
		addLabel = "retry"
	}

	entries := []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation", keymap.WithFooter(9, "navigate", false), keymap.WithFooterKey("j/k"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("down", "Move cursor down", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("k", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("up", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("G", "Jump to last attachment", "Navigation", keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("home", "Jump to first attachment", "Navigation", keymap.WithGate(hasBrowseSelection)),

		keymap.Bind("enter", "Open attachment", "Actions", keymap.WithFooter(1, "open", false), keymap.WithFooterKey("Enter"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("a", "Add attachment", "Actions", keymap.WithFooter(2, "add", false), keymap.WithGate(inBrowseMode)),
		keymap.Bind("e", "Open in editor", "Actions", keymap.WithFooter(3, "edit", false), keymap.WithGate(func() bool {
			return hasBrowseSelection() && !selectedIsURL()
		})),
		keymap.Bind("y", "Copy path to clipboard", "Actions", keymap.WithFooter(4, "copy", false), keymap.WithGate(hasBrowseSelection)),
		// Space outranks the two niche hints below it: it is the only route to
		// five operations, two of them destructive, so a narrow footer must shed
		// a copy variant and a reorder before it sheds the menu.
		keymap.Bind("space", "Attachment actions", "Actions", keymap.WithFooter(5, "actions", false), keymap.WithFooterKey("Space"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("Y", "Copy path with line", "Actions", keymap.WithFooter(6, "copy path:line", false), keymap.WithGate(selectedIsCodeRef)),
		keymap.Bind("J", "Reorder attachment down", "Actions", keymap.WithFooter(7, "reorder", false), keymap.WithFooterKey("J/K"), keymap.WithGate(hasMultipleAttachments)),
		keymap.Bind("K", "Reorder attachment up", "Actions", keymap.AsHidden(), keymap.WithGate(hasMultipleAttachments)),
		keymap.Bind("?", "Toggle help", "Navigation", keymap.WithFooter(10, "help", false), keymap.WithGate(inBrowseMode)),
		keymap.Bind("esc", "Close attachments", "Navigation", keymap.WithFooter(11, "close", false), keymap.WithFooterKey("Esc"), keymap.WithGate(inBrowseMode)),

		keymap.Bind("enter", "Add attachment", "Actions", keymap.WithFooter(1, addLabel, true), keymap.WithFooterKey("Enter"), keymap.WithGate(inAddMode)),
		keymap.Bind("esc", "Cancel attachment input", "Navigation", keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc"), keymap.WithGate(inAddMode)),
	}

	return &keymap.KeyMap{Entries: entries}
}
