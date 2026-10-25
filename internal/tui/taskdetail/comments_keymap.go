package taskdetail

import (
	"github.com/sglavoie/docket/internal/tui/keymap"
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
	// Esc is one key over two acts in input mode, so it is two gated rows rather
	// than one label that would have to be right in both states: text the panel
	// would have to throw away raises the discard question, and an untouched
	// composer closes outright.
	inputDraft := func() bool {
		return (addMode() && m.hasUnsavedAddText()) || (editMode() && m.hasUnsavedEditChanges())
	}
	inputClean := func() bool { return inputMode() && !inputDraft() }
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
		keymap.Bind("a", "Add Comment", "Actions", keymap.WithFooter(2, "add", false), keymap.WithGate(inBrowseMode)),
		keymap.Bind("e", "Edit Comment", "Actions", keymap.WithFooter(3, "edit", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("E", "Edit in external editor", "Actions", keymap.WithFooter(7, "editor", false), keymap.WithGate(hasBrowseSelection)),
		// Space takes the slot Delete gave up: it is the only route to Delete, so
		// a narrow footer must shed a copy variant and the editor hint before it
		// sheds the menu.
		keymap.Bind("space", "Comment Actions", "Actions", keymap.WithFooter(4, "actions", false), keymap.WithFooterKey("Space"), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("y", "Copy Comment Text", "Actions", keymap.WithFooter(5, "copy", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("Y", "Copy All as Markdown", "Actions", keymap.WithFooter(6, "copy all", false), keymap.WithGate(hasBrowseSelection)),
		keymap.Bind("enter", "Expand Comment", "Actions", keymap.WithFooter(1, "expand", false), keymap.WithFooterKey("Enter"), keymap.WithGate(hasBrowseSelection)),
		// Deliberately unranked. The core undo descriptors answer only to the
		// Outliner, so this keymap is the only place that can put u and Ctrl+R in
		// the panel's `?` help; the footer stays spent on the acts the panel exists
		// for, and the recovery toast a comment mutation installs already says
		// "u undo" while it is live.
		keymap.Bind("u", "Undo workspace change", "Actions", keymap.WithGate(inBrowseMode)),
		keymap.Bind("ctrl+r", "Redo workspace change", "Actions", keymap.WithGate(inBrowseMode)),
		// Gated to browse mode like its neighbors because Update hands the add,
		// edit, and expanded states their whole key streams. Ranked and required
		// beside the exit: the panel publishes this keymap as the shell footer, so
		// no breadcrumb row is left to print either, and the way out and the way to
		// every unranked key are the two entries a narrow footer may not shed.
		// "System" is the core descriptor's own category, so the row lands in the
		// same help bucket as the Outliner's `?`.
		keymap.Bind("?", "Help", "System", keymap.WithFooter(10, "help", true), keymap.WithGate(inBrowseMode)),
		keymap.Bind("esc", "Close comments", "Navigation", keymap.WithFooter(9, "close", true), keymap.WithFooterKey("Esc"), keymap.WithGate(inBrowseMode)),
		keymap.Bind("ctrl+s", "Save comment", "Actions", keymap.WithFooter(1, saveLabel, true), keymap.WithFooterKey("Ctrl+S"), keymap.WithGate(inputMode)),
		keymap.Bind("ctrl+e", "Open in external editor", "Actions", keymap.WithFooter(3, "editor", false), keymap.WithFooterKey("Ctrl+E"), keymap.WithGate(inputMode)),
		// Gated off a read-only composer, which is the one state that refuses it: the
		// box is then either a rendering of a draft the panel is holding or a copy of
		// text a commit has already been handed, and a footer offering to type into it
		// would advertise a press that answers with a refusal. Ctrl+S, Ctrl+E, Ctrl+X,
		// and Esc all still mean what they say there — a pending submission answers
		// three of them with its own refusal rather than with silence — which is why
		// this is the only row that narrows.
		keymap.Bind("alt+enter", "Insert newline", "Actions", keymap.WithFooter(4, "newline", false), keymap.WithFooterKey("Alt+Enter"), keymap.WithGate(editableInput)),
		keymap.Bind("shift+enter", "Insert newline", "Actions", keymap.AsHidden(), keymap.WithGate(editableInput)),
		// Ranked last of the five input keys because it is the one a reader only
		// needs when a save is refused — and ranked at all because that is the
		// moment it has to be findable: a draft the panel cannot write and the
		// composer cannot show in full is reachable through this key alone, so it
		// is advertised wherever text is being written rather than only where it
		// has already been rescued.
		keymap.Bind("ctrl+x", "Write draft to a file", "Actions", keymap.WithFooter(5, "to file", false), keymap.WithFooterKey("Ctrl+X"), keymap.WithGate(inputMode)),
		keymap.Bind("esc", "Discard comment input", "Navigation", keymap.WithFooter(2, "discard", true), keymap.WithFooterKey("Esc"), keymap.WithGate(inputDraft)),
		keymap.Bind("esc", "Cancel comment input", "Navigation", keymap.WithFooter(2, "cancel", true), keymap.WithFooterKey("Esc"), keymap.WithGate(inputClean)),
		keymap.Bind("enter", "Close expanded comment", "Navigation", keymap.WithFooter(1, "close", true), keymap.WithFooterKey("Enter"), keymap.WithGate(expandedMode)),
		keymap.Bind("esc", "Close expanded comment", "Navigation", keymap.WithFooter(2, "close", true), keymap.WithFooterKey("Esc"), keymap.WithGate(expandedMode)),
		// The reader's window. These are ranked immediately under the two closes
		// because a comment too long for the terminal is the state this whole
		// sub-view exists for, and a footer that shed them would leave a reader
		// looking at a body that stops mid-sentence with nothing to press. Each pair
		// is one row: the backward half is the forward half's mirror and spelling it
		// twice would spend footer width on nothing.
		keymap.Bind("j", "Scroll the comment", "Navigation", keymap.WithFooter(3, "scroll", false), keymap.WithFooterKey("j/k"), keymap.WithHelpKey("j/k"), keymap.WithGate(expandedMode)),
		keymap.Bind("k", "Scroll the comment up", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
		keymap.Bind("down", "Scroll the comment down", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
		keymap.Bind("up", "Scroll the comment up", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
		keymap.Bind("ctrl+d", "Scroll half a screen", "Navigation", keymap.WithFooter(4, "half page", false), keymap.WithFooterKey("Ctrl+D/U"), keymap.WithHelpKey("Ctrl+D/U"), keymap.WithGate(expandedMode)),
		keymap.Bind("ctrl+u", "Scroll half a screen back", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
		keymap.Bind("pgdown", "Scroll half a screen", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
		keymap.Bind("pgup", "Scroll half a screen back", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
		keymap.Bind("ctrl+f", "Page the comment", "Navigation", keymap.WithFooter(5, "page", false), keymap.WithFooterKey("Ctrl+F/B"), keymap.WithHelpKey("Ctrl+F/B"), keymap.WithGate(expandedMode)),
		keymap.Bind("ctrl+b", "Page the comment back", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
		// The ends of the body, spelled the way the ends of the list are. They take
		// no footer rank: a reader who can scroll can reach either end, so these are
		// the first thing a narrow footer should not have been spending width on.
		keymap.Bind("home", "Jump to the first line", "Navigation", keymap.WithHelpKey("gg/Home"), keymap.WithGate(expandedMode)),
		keymap.Bind("G", "Jump to the last line", "Navigation", keymap.WithHelpKey("G/End"), keymap.WithGate(expandedMode)),
		keymap.Bind("end", "Jump to the last line", "Navigation", keymap.AsHidden(), keymap.WithGate(expandedMode)),
	}}
}
