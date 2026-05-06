package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for WorkflowWizardModel.
// The wizard manages its own step-based state so the Dispatcher is not used
// for dispatch; this keymap is used only for HandledKeys().
func (m *WorkflowWizardModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("enter", "Select / confirm", "Actions", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
		keymap.Bind("esc", "Back one step / cancel", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Workflow Wizard",
		Entries:  entries,
	}
}
