package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskChecklistModel.
func (m *TaskChecklistModel) buildKeyMap() *keymap.KeyMap {
	inInputMode := func() bool { return m.addMode || m.editMode }

	var entries []keymap.Entry

	// Normal mode navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorDown()
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorDown()
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorUp()
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorUp()
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("G", "Jump to last item", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorBottom()
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	// Goto group: g prefix (gg = top)
	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) {
			m.cursor = 0
			return m, nil
		},
		GoToProject: nil,
	})...)

	// Normal mode actions
	entries = append(entries,
		keymap.Bind("a", "Add item", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginAddMode()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("n", "Add item", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginAddMode()
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("e", "Edit item", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.startEditItem()
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("E", "Edit in external editor", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.editInExternalEditor()
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("d", "Delete item", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				m.confirm.Show("delete", "Delete item?")
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("x", "Delete item", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				m.confirm.Show("delete", "Delete item?")
			}
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("space", "Toggle done/undone", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.toggleItem()
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("enter", "Toggle done/undone", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.toggleItem()
			}
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("J", "Reorder item down", "Actions", func() (tea.Model, tea.Cmd) {
			if m.cursor < len(m.task.Checklist)-1 {
				return m, m.reorderItem(m.cursor, m.cursor+1)
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("K", "Reorder item up", "Actions", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				return m, m.reorderItem(m.cursor, m.cursor-1)
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	// Add/edit mode keys (always included in HandledKeys for esc handling)
	entries = append(entries,
		keymap.Bind("ctrl+s", "Save (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.saveEditedItem()
			}
			if m.addMode {
				return m, m.saveItem()
			}
			return m, nil
		}),
		keymap.Bind("ctrl+e", "Open in external editor (add mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.editItemInExternalEditorFromEditMode()
			}
			if m.addMode {
				return m, m.addItemInExternalEditor()
			}
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Checklist",
		Entries:  entries,
	}
}
