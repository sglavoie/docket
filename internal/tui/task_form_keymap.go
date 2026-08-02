package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskFormModel.
func (m *TaskFormModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries,
		keymap.Bind("shift+left", "Switch tab left", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("shift+right", "Switch tab right", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("tab", "Navigate to next field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Navigate to previous field", "Navigation", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("j", "Cycle priority down", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("k", "Cycle priority up", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("up", "Cycle priority up", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("down", "Cycle priority down", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "Open picker / confirm text field", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+s", "Save task", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+v", "Paste clipboard text into focused field", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+e", "Open description in external editor", "Actions", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	// Basics tab
	entries = append(entries,
		keymap.Bind("0", "Clear priority", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("1", "Set priority 1", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("2", "Set priority 2", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
		keymap.Bind("3", "Set priority 3", "Basics", func() (tea.Model, tea.Cmd) { return m, nil }, keymap.AsHidden()),
	)

	// Schedule tab
	entries = append(entries,
		keymap.Bind("ctrl+d", "Open due date preset picker", "Schedule", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	// Children tab
	entries = append(entries,
		keymap.Bind("c", "Toggle checklist section", "Children", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("S", "Toggle subtasks section", "Children", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+a", "Add checklist item", "Children", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+t", "Add subtask", "Children", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+j", "Move item down", "Children", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("ctrl+k", "Move item up", "Children", func() (tea.Model, tea.Cmd) { return m, nil }),
		keymap.Bind("space", "Toggle checklist item done", "Children", func() (tea.Model, tea.Cmd) { return m, nil }),
	)

	return &keymap.KeyMap{
		ViewName: "TaskForm",
		Entries:  entries,
	}
}
