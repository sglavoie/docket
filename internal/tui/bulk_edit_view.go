package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// Update handles messages.
func (m *BulkEditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.ensureFocusedFieldVisible()
		return m, nil

	case tea.KeyPressMsg:
		if m.tagMode == 1 {
			return m.handleTagAddInput(msg)
		}

		if m.dueDateInputMode {
			return m.handleDueDateInput(msg)
		}

		if m.parentListMode {
			return m.handleParentListInput(msg)
		}

		switch msg.String() {
		case "esc":
			if m.tagMode == 2 {
				m.tagMode = 0
				return m, nil
			}
			return m, m.cancel()

		case "tab", "down", "j":
			if m.focusIndex == fieldTags && m.tagMode == 2 {
				if m.tagRemoveCursor < len(m.allTags)-1 {
					m.tagRemoveCursor++
				} else {
					m.tagMode = 0
					m.focusIndex = (m.focusIndex + 1) % fieldCount
				}
			} else {
				m.focusIndex = (m.focusIndex + 1) % fieldCount
			}
			m.ensureFocusedFieldVisible()
			return m, nil

		case "shift+tab", "up", "k":
			if m.focusIndex == fieldTags && m.tagMode == 2 {
				if m.tagRemoveCursor > 0 {
					m.tagRemoveCursor--
				} else {
					m.tagMode = 0
					m.focusIndex = (m.focusIndex - 1 + fieldCount) % fieldCount
				}
			} else {
				m.focusIndex = (m.focusIndex - 1 + fieldCount) % fieldCount
			}
			m.ensureFocusedFieldVisible()
			return m, nil

		case "enter":
			return m.handleEnter()

		case "space":
			return m.handleSpace()

		case "a":
			if m.focusIndex == fieldTags {
				m.tagMode = 1
				m.tagAddInput.Reset()
				m.tagAddInput.Focus()
				return m, textinput.Blink
			}

		case "r":
			if m.focusIndex == fieldTags && len(m.allTags) > 0 {
				m.tagMode = 2
				m.tagRemoveCursor = 0
			}

		case "l", "right":
			m.handleRight()

		case "h", "left":
			m.handleLeft()
		}
	}

	return m, nil
}

// View renders the bulk edit panel.
func (m *BulkEditModel) View() tea.View {
	var content strings.Builder

	content.WriteString(m.renderStatusField())
	content.WriteString("\n")

	content.WriteString(m.renderTypeField())
	content.WriteString("\n")

	content.WriteString(m.renderPriorityField())
	content.WriteString("\n")

	content.WriteString(m.renderTagsField())
	content.WriteString("\n")

	content.WriteString(m.renderSectionField())
	content.WriteString("\n")

	content.WriteString(m.renderDueDateField())
	content.WriteString("\n")

	content.WriteString(m.renderRecurrenceField())
	content.WriteString("\n")

	content.WriteString(m.renderParentField())
	content.WriteString("\n")

	content.WriteString(m.renderPreview())
	content.WriteString("\n")

	content.WriteString(m.renderActions())
	content.WriteString("\n\n")

	content.WriteString(m.renderFooter())

	subtitle := fmt.Sprintf("%d tasks", len(m.tasks))
	fullContent := content.String()
	lines := strings.Split(fullContent, "\n")
	viewportHeight := m.viewportHeight()

	var out strings.Builder
	out.WriteString(renderFullscreenTitle("Bulk Edit", subtitle, m.AccentColor()))
	out.WriteString("\n\n")

	if len(lines) > viewportHeight {
		start := m.scrollOffset
		end := min(start+viewportHeight, len(lines))

		if start > 0 {
			out.WriteString(MutedStyle.Render("  ↑ scroll up for more"))
			out.WriteString("\n")
		}

		for i := start; i < end; i++ {
			out.WriteString(lines[i])
			if i < end-1 {
				out.WriteString("\n")
			}
		}

		if end < len(lines) {
			out.WriteString("\n")
			out.WriteString(MutedStyle.Render("  ↓ scroll down for more"))
		}
	} else {
		out.WriteString(fullContent)
	}

	return tea.NewView(out.String())
}
