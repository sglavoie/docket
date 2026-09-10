package taskdetail

import (
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// BuildKeyMap constructs the declarative KeyMap for CommentsModel. It is the
// panel's help and footer source; CommentsInputStrategy is what the browse keys
// actually resolve through, so the two must keep describing the same grammar.
func (m *CommentsModel) BuildKeyMap() *keymap.KeyMap {
	inInputMode := func() bool { return m.AddMode || m.EditMode || m.ExpandedView }
	inBrowseMode := func() bool { return !inInputMode() }
	hasBrowseSelection := func() bool { return inBrowseMode() && m.HasSelection() }
	addMode := func() bool { return m.AddMode && !m.Confirm.IsActive() }
	editMode := func() bool { return m.EditMode && !m.Confirm.IsActive() }
	inputMode := func() bool { return addMode() || editMode() }
	editableInput := func() bool { return inputMode() && !m.composerReadOnly() }
	expandedMode := func() bool { return m.ExpandedView && !m.Confirm.IsActive() }
	saveLabel := "save"
	if m.Err != nil {
		saveLabel = "retry"
	}

	return &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation", keymap.WithFooter(8, "navigate", false), keymap.WithFooterKey("j/k"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("down", "Move cursor down", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("k", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("up", "Move cursor up", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		// One row per end, spelled as the whole path the panel answers to. gg is a
		// chord this keymap cannot bind and end is the hidden alias beside G, so the
		// help spelling is what names all four presses without inventing a dispatch
		// key for any of them.
		keymap.Bind("home", "Jump to first comment", "Navigation", keymap.WithHelpKey("gg/Home"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("G", "Jump to last comment", "Navigation", keymap.WithHelpKey("G/End"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("end", "Jump to last comment", "Navigation", keymap.AsHidden(), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("a", "Add comment", "Actions", keymap.WithFooter(2, "add", false), keymap.WithGate(inBrowseMode)),
		keymap.Bind("e", "Edit comment", "Actions", keymap.WithFooter(3, "edit", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("E", "Edit in external editor", "Actions", keymap.WithFooter(7, "editor", false), keymap.WithGate(hasBrowseSelection)),
		// Space takes the slot Delete gave up: it is the only route to Delete, so
		// a narrow footer must shed a copy variant and the editor hint before it
		// sheds the menu.
		keymap.Bind("space", "Comment actions", "Actions", keymap.WithFooter(4, "actions", false), keymap.WithFooterKey("Space"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("y", "Copy comment text", "Actions", keymap.WithFooter(5, "copy", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("Y", "Copy all as Markdown", "Actions", keymap.WithFooter(6, "copy all", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("enter", "Expand comment", "Actions", keymap.WithFooter(1, "expand", false), keymap.WithFooterKey("Enter"), keymap.WithGate(hasBrowseSelection)),
		// Deliberately unranked. The core undo descriptors answer only to the
		// Outliner, so this keymap is the only place that can put u and Ctrl+R in
		// the panel's `?` help; the footer stays spent on the acts the panel exists
		// for, and the recovery toast a comment mutation installs already says
		// "u undo" while it is live.
		keymap.Bind("u", "Undo workspace change", "Actions", keymap.WithGate(inBrowseMode)),
		keymap.Bind("ctrl+r", "Redo workspace change", "Actions", keymap.WithGate(inBrowseMode)),
		// Gated to browse mode like its neighbors because Update hands the add,
		// edit, and expanded states their whole key streams, and deliberately
		// unranked: the panel is a HelpProvider now, so the shell strip under this
		// footer already prints "? help" beside the breadcrumb. "System" is the core
		// descriptor's own category, so the row lands in the same help bucket as the
		// Outliner's `?`.
		keymap.Bind("?", "Help", "System", keymap.WithGate(inBrowseMode)),
		keymap.Bind("esc", "Close panel", "Navigation", keymap.WithFooter(9, "close", false), keymap.WithFooterKey("Esc"), keymap.WithGate(inBrowseMode)),
		keymap.Bind("ctrl+s", "Save comment", "Actions", keymap.WithFooter(1, saveLabel, true), keymap.WithFooterKey("Ctrl+S"), keymap.WithGate(inputMode)),
		keymap.Bind("ctrl+e", "Open in external editor", "Actions", keymap.WithFooter(3, "editor", false), keymap.WithFooterKey("Ctrl+E"), keymap.WithGate(inputMode)),
		// Gated off a read-only composer, which is the one state that refuses it: the
		// box is then either a rendering of a draft the panel is holding or a copy of
		// text a commit has already been handed, and a footer offering to type into it
		// would advertise a press that answers with a refusal. Ctrl+S, Ctrl+E, Ctrl+X,
		// and Esc all still mean what they say there — a pending submission answers
		// three of them with its own refusal rather than with silence — which is why
		// this is the only row that narrows.
		keymap.Bind("shift+enter", "Insert newline", "Actions", keymap.WithFooter(4, "newline", false), keymap.WithFooterKey("Shift+Enter"), keymap.WithGate(editableInput)),
		// Ranked last of the five input keys because it is the one a reader only
		// needs when a save is refused — and ranked at all because that is the
		// moment it has to be findable: a draft the panel cannot write and the
		// composer cannot show in full is reachable through this key alone, so it
		// is advertised wherever text is being written rather than only where it
		// has already been rescued.
		keymap.Bind("ctrl+x", "Write draft to a file", "Actions", keymap.WithFooter(5, "to file", false), keymap.WithFooterKey("Ctrl+X"), keymap.WithGate(inputMode)),
		keymap.Bind("esc", "Cancel comment input", "Navigation", keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc"), keymap.WithGate(inputMode)),
		keymap.Bind("enter", "Close expanded comment", "Navigation", keymap.WithFooter(1, "close", true), keymap.WithFooterKey("Enter"), keymap.WithGate(expandedMode)),
		keymap.Bind("esc", "Close expanded comment", "Navigation", keymap.WithFooter(2, "close", true), keymap.WithFooterKey("Esc"), keymap.WithGate(expandedMode)),
	}}
}
