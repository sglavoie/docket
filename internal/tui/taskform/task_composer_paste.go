package taskform

import (
	"fmt"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskComposerModel) updateCompactInput(msg tea.Msg) tea.Cmd {
	if m.refuseOversizedPaste(msg) {
		m.syncInputHeight()
		return nil
	}
	return m.UpdateInput(msg)
}

// Ask a fresh textarea to normalize the paste exactly as the live input does
// (including expanded tabs and removed control characters), without its size
// limit. Never trial an insertion on a copy of Input: its buffers are shared.
func (m *TaskComposerModel) refuseOversizedPaste(msg tea.Msg) bool {
	paste, ok := msg.(tea.PasteMsg)
	if !ok {
		return false
	}
	m.pasteRefusal = ""
	if m.Input.CharLimit <= 0 {
		return false
	}
	probe := textarea.New()
	probe.SetValue(paste.Content)
	remaining := max(m.Input.CharLimit-m.Input.Length(), 0)
	if utf8.RuneCountInString(probe.Value()) <= remaining {
		return false
	}
	m.pasteRefusal = fmt.Sprintf("Paste exceeds the %d-character limit; nothing pasted. Paste a smaller batch (%d remaining).", m.Input.CharLimit, remaining)
	return true
}
