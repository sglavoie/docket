package taskform

import (
	"github.com/sglavoie/docket/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskFormModel.
func (m *TaskFormModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries,
		keymap.Bind("shift+left", "Switch tab left", "Navigation", keymap.AsHidden()),
		keymap.Bind("shift+right", "Switch tab right", "Navigation", keymap.AsHidden()),
		keymap.Bind("tab", "Navigate to next field", "Navigation", keymap.AsHidden()),
		keymap.Bind("shift+tab", "Navigate to previous field", "Navigation", keymap.AsHidden()),
		keymap.Bind("j", "Cycle priority down", "Basics", keymap.AsHidden()),
		keymap.Bind("k", "Cycle priority up", "Basics", keymap.AsHidden()),
		keymap.Bind("up", "Cycle priority up", "Basics", keymap.AsHidden()),
		keymap.Bind("down", "Cycle priority down", "Basics", keymap.AsHidden()),
	)

	// Actions
	if m.discardDraft != nil {
		entries = append(entries, keymap.Bind("ctrl+x", "Discard draft", "Actions"))
	}
	entries = append(entries,
		keymap.Bind("enter", "Open picker / confirm text field", "Actions"),
		keymap.Bind("ctrl+s", "Save task", "Actions"),
		keymap.Bind("ctrl+v", "Paste clipboard text into focused field", "Actions"),
		keymap.Bind("ctrl+e", "Open description in external editor", "Actions"),
	)

	// Basics tab
	entries = append(entries,
		keymap.Bind("0", "Clear priority", "Basics", keymap.AsHidden()),
		keymap.Bind("1", "Set High priority", "Basics", keymap.AsHidden()),
		keymap.Bind("2", "Set Medium priority", "Basics", keymap.AsHidden()),
		keymap.Bind("3", "Set Low priority", "Basics", keymap.AsHidden()),
	)

	// The per-tab blocks below are scoped to the active tab as a whole rather
	// than row by row, following the precedent guidanceSpec() sets for the
	// Children tab. Extras is the first tab whose keys collide with rows that
	// used to be appended unconditionally — Ctrl+D is Schedule's due-date preset
	// picker and Ctrl+J/Ctrl+K are the Children reorder rows — and a block that
	// advertises half of one tab's keys beside half of another's is not honest
	// about either.
	switch m.activeTab {
	case formTabSchedule:
		entries = append(entries,
			keymap.Bind("ctrl+d", "Open due date preset picker", "Schedule",
				keymap.WithGate(func() bool { return m.focusIndex == 0 })),
		)

	case formTabChildren:
		entries = append(entries,
			keymap.Bind("c", "Toggle checklist section", "Children"),
			keymap.Bind("S", "Toggle subtasks section", "Children"),
			keymap.Bind("ctrl+a", "Add checklist item", "Children"),
			keymap.Bind("ctrl+t", "Add subtask", "Children"),
			keymap.Bind("ctrl+j", "Move item down", "Children"),
			keymap.Bind("ctrl+k", "Move item up", "Children"),
			keymap.Bind("space", "Toggle checklist item done", "Children"),
		)

	case formTabExtras:
		entries = append(entries,
			keymap.Bind("a", "Toggle attachments section", "Extras"),
			keymap.Bind("c", "Toggle comments section", "Extras"),
			keymap.Bind("ctrl+l", "Add Attachment", "Extras"),
			keymap.Bind("ctrl+y", "Add Comment", "Extras"),
			keymap.Bind("ctrl+d", "Delete focused draft", "Extras"),
			keymap.Bind("ctrl+j", "Move attachment down", "Extras"),
			keymap.Bind("ctrl+k", "Move attachment up", "Extras"),
		)
	}

	return &keymap.KeyMap{
		Entries: entries,
	}
}
