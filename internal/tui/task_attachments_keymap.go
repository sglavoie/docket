package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskAttachmentsModel.
func (m *TaskAttachmentsModel) buildKeyMap() *keymap.KeyMap {
	inInputMode := func() bool { return m.addMode }

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
		keymap.Bind("G", "Jump to last attachment", "Navigation", func() (tea.Model, tea.Cmd) {
			m.moveCursorBottom()
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	entries = append(entries, keymap.Bind("home", "Jump to first attachment", "Navigation", func() (tea.Model, tea.Cmd) {
		m.cursor = 0
		return m, nil
	}, keymap.WithGate(func() bool { return !inInputMode() })))

	// Open/view actions
	entries = append(entries,
		keymap.Bind("a", "Add attachment", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.beginAddMode()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("o", "Open (system default)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.openSelectedAttachmentDefault()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("enter", "Open (system default)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.openSelectedAttachmentDefault()
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("e", "Open in editor", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.openSelectedAttachmentInEditor()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("ctrl+e", "Open original path (files)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.openSelectedOriginalPath()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("ctrl+o", "Reveal original file in file manager", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.revealSelectedOriginalFile()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("O", "Open attachments directory", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.openAttachmentsDir()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	// Mutation actions
	entries = append(entries,
		keymap.Bind("d", "Delete (removes file)", "Actions", func() (tea.Model, tea.Cmd) {
			m.confirmDeleteSelectedAttachment()
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("x", "Delete (removes file)", "Actions", func() (tea.Model, tea.Cmd) {
			m.confirmDeleteSelectedAttachment()
			return m, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("u", "Unlink (keep file)", "Actions", func() (tea.Model, tea.Cmd) {
			m.confirmUnlinkSelectedAttachment()
			return m, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("y", "Copy path to clipboard", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.copySelectedPath()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("Y", "Copy path with line (code refs)", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.copySelectedPathWithLine()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("J", "Reorder attachment down", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.moveSelectedAttachmentDown()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("K", "Reorder attachment up", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.moveSelectedAttachmentUp()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	// Help
	entries = append(entries,
		keymap.Bind("?", "Toggle help", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, helpCmd(ViewTaskAttachments, "Attachments", m.GetHelpItems())
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	return &keymap.KeyMap{
		ViewName: "Attachments",
		Entries:  entries,
	}
}
