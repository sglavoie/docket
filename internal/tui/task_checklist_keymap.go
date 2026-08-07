package tui

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskChecklistModel.
func (m *TaskChecklistModel) buildKeyMap() *keymap.KeyMap {
	inInputMode := func() bool { return (m.addMode || m.editMode) && !m.confirm.IsActive() }
	inBrowseMode := func() bool { return !m.addMode && !m.editMode && !m.confirm.IsActive() }
	hasBrowseSelection := func() bool { return inBrowseMode() && m.hasSelection() }
	hasMultipleItems := func() bool {
		return hasBrowseSelection() && m.task != nil && len(m.task.Checklist) > 1
	}
	saveLabel := "save"
	if m.err != nil {
		saveLabel = "retry"
	}

	entries := []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation", keymap.WithFooter(8, "navigate", false), keymap.WithFooterKey("j/k"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("down", "Move cursor down", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("k", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("up", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("G", "Jump to last item", "Navigation", keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("home", "Jump to first item", "Navigation", keymap.WithGate(hasBrowseSelection)),

		keymap.Bind("a", "Add item", "Actions", keymap.WithFooter(2, "add", false), keymap.WithGate(inBrowseMode)),
		keymap.Bind("n", "Add item", "Actions", keymap.AsHidden(), keymap.WithGate(inBrowseMode)),
		keymap.Bind("e", "Edit item", "Actions", keymap.WithFooter(3, "edit", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("E", "Edit in external editor", "Actions", keymap.WithFooter(7, "editor", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("d", "Delete item", "Actions", keymap.WithFooter(4, "delete", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("x", "Delete item", "Actions", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("space", "Toggle done/undone", "Actions", keymap.WithFooter(1, "toggle", false), keymap.WithFooterKey("Space/Enter"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("enter", "Toggle done/undone", "Actions", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("y", "Copy item text", "Actions", keymap.WithFooter(5, "copy", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("Y", "Copy all as Markdown", "Actions", keymap.WithFooter(6, "copy all", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("J", "Reorder item down", "Actions", keymap.WithFooter(7, "reorder", false), keymap.WithFooterKey("J/K"), keymap.WithGate(hasMultipleItems)),
		keymap.Bind("K", "Reorder item up", "Actions", keymap.AsHidden(), keymap.WithGate(hasMultipleItems)),

		keymap.Bind("enter", "Save item", "Actions", keymap.WithFooter(1, saveLabel, true), keymap.WithFooterKey("Enter"), keymap.WithGate(inInputMode)),
		keymap.Bind("ctrl+e", "Open in external editor", "Actions", keymap.WithFooter(3, "editor", false), keymap.WithFooterKey("Ctrl+E"), keymap.WithGate(inInputMode)),
		keymap.Bind("esc", "Cancel item input", "Navigation", keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc"), keymap.WithGate(inInputMode)),
		keymap.Bind("esc", "Close checklist", "Navigation", keymap.WithFooter(9, "close", false), keymap.WithFooterKey("Esc"), keymap.WithGate(inBrowseMode)),
	}

	return &keymap.KeyMap{Entries: entries}
}
