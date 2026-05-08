package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskDecisionsModel.
func (m *TaskDecisionsModel) buildKeyMap() *keymap.KeyMap {
	inInputMode := func() bool { return m.addMode || m.editMode || m.expandedView }

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
		keymap.Bind("G", "Jump to last decision", "Navigation", func() (tea.Model, tea.Cmd) {
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
		keymap.Bind("a", "Add decision", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginAddMode()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("n", "Add decision", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginAddMode()
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("e", "Edit decision", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.startEditDecision()
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("E", "Edit in external editor", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.editInExternalEditor()
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("d", "Delete decision", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				m.confirm.Show("delete", "Delete decision?")
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("x", "Delete decision", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				m.confirm.Show("delete", "Delete decision?")
			}
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("r", "Toggle resolved", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.toggleResolved()
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("y", "Copy decision text", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				return m, m.copyDecisionText()
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("enter", "Expand decision", "Actions", func() (tea.Model, tea.Cmd) {
			if m.hasSelection() {
				m.expandedView = true
			}
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	// Add/edit mode keys
	entries = append(entries,
		keymap.Bind("tab", "Cycle field (text/rationale)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				m.editActiveField = 1 - m.editActiveField
				return m, m.focusEditField()
			}
			if m.addMode {
				m.activeField = 1 - m.activeField
				return m, m.focusAddField()
			}
			return m, nil
		}),
		keymap.Bind("ctrl+y", "Cycle category", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				cats := allDecisionCategories()
				m.editCategoryIdx = (m.editCategoryIdx + 1) % len(cats)
			} else if m.addMode {
				cats := allDecisionCategories()
				m.categoryIndex = (m.categoryIndex + 1) % len(cats)
			}
			return m, nil
		}),
		keymap.Bind("ctrl+s", "Save (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.saveEditedDecision()
			}
			if m.addMode {
				return m, m.saveDecision()
			}
			return m, nil
		}),
		keymap.Bind("ctrl+e", "Open in external editor (add/edit mode)", "Actions", func() (tea.Model, tea.Cmd) {
			if m.editMode {
				return m, m.editDecisionInExternalEditorFromEditMode()
			}
			if m.addMode {
				return m, m.addDecisionInExternalEditor()
			}
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Decisions",
		Entries:  entries,
	}
}
