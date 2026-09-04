package taskdetail

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// BuildKeyMap constructs the declarative KeyMap for TaskAttachmentsModel.
func (m *TaskAttachmentsModel) BuildKeyMap() *keymap.KeyMap {
	inAddMode := func() bool { return m.AddMode && !m.Confirm.IsActive() }
	inBrowseMode := func() bool { return !m.AddMode && !m.Confirm.IsActive() }
	hasBrowseSelection := func() bool { return inBrowseMode() && m.HasSelection() }
	hasMultipleAttachments := func() bool {
		return hasBrowseSelection() && m.Task != nil && len(m.Task.Attachments) > 1
	}
	selectedIsURL := func() bool {
		return hasBrowseSelection() && m.Task.Attachments[m.Cursor].IsURL
	}
	selectedIsCodeRef := func() bool {
		return hasBrowseSelection() && m.Task.Attachments[m.Cursor].IsCodeRef
	}
	addLabel := "add"
	if m.Err != nil {
		addLabel = "retry"
	}

	entries := []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation", keymap.WithFooter(9, "navigate", false), keymap.WithFooterKey("j/k"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("down", "Move cursor down", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("k", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("up", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		// One row per end, spelled as the whole path the panel answers to. gg is a
		// chord this keymap cannot bind and end is the hidden alias beside G, so the
		// help spelling is what names all four presses without inventing a dispatch
		// key for any of them.
		keymap.Bind("home", "Jump to first attachment", "Navigation", keymap.WithHelpKey("gg/Home"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("G", "Jump to last attachment", "Navigation", keymap.WithHelpKey("G/End"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("end", "Jump to last attachment", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),

		keymap.Bind("enter", "Open attachment", "Actions", keymap.WithFooter(1, "open", false), keymap.WithFooterKey("Enter"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("a", "Add attachment", "Actions", keymap.WithFooter(2, "add", false), keymap.WithGate(inBrowseMode)),
		// E, not e: the footer says "editor" the way Comments does, because the
		// panel has nothing to rename and e means edit everywhere else.
		keymap.Bind("E", "Open in external editor", "Actions", keymap.WithFooter(3, "editor", false), keymap.WithGate(func() bool {
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
		// Deliberately unranked. The core undo descriptors answer only to the
		// Outliner, so this keymap is the only place that can put u and Ctrl+R in
		// the panel's `?` help; the footer stays spent on the acts the panel exists
		// for, and the undo toast already says "u undo" when one is live.
		keymap.Bind("u", "Undo workspace change", "Actions", keymap.WithGate(inBrowseMode)),
		keymap.Bind("ctrl+r", "Redo workspace change", "Actions", keymap.WithGate(inBrowseMode)),
		keymap.Bind("?", "Toggle help", "Navigation", keymap.WithFooter(10, "help", false), keymap.WithGate(inBrowseMode)),
		keymap.Bind("esc", "Close attachments", "Navigation", keymap.WithFooter(11, "close", false), keymap.WithFooterKey("Esc"), keymap.WithGate(inBrowseMode)),

		keymap.Bind("enter", "Add attachment", "Actions", keymap.WithFooter(1, addLabel, true), keymap.WithFooterKey("Enter"), keymap.WithGate(inAddMode)),
		keymap.Bind("esc", "Cancel attachment input", "Navigation", keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc"), keymap.WithGate(inAddMode)),
	}

	return &keymap.KeyMap{Entries: entries}
}
