package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *CustomContextEditorModel) maxVisibleProjects() int {
	// Reserve space for header, name input, help, etc.
	reserved := 15
	if m.IsSearchMode() || m.SearchQuery() != "" {
		reserved++ // Extra line for search input/filter indicator
	}
	return max(m.height-reserved, 5)
}

// HasTextInput returns true when the name input or search input is focused.
func (m *CustomContextEditorModel) HasTextInput() bool {
	return !m.projectListMode || m.IsSearchMode()
}

// ShouldCaptureEsc returns true to let the editor handle esc internally.
func (m *CustomContextEditorModel) ShouldCaptureEsc() bool { return true }

// IsFullHeight returns false; the context editor is a panel overlay, not full-screen.
func (m *CustomContextEditorModel) IsFullHeight() bool { return false }

func (m *CustomContextEditorModel) SupportsHelpShortcut() bool { return false }

func (m *CustomContextEditorModel) View() tea.View {
	var b strings.Builder

	// Title
	if m.editMode {
		b.WriteString(renderFullscreenTitle("Edit Custom Context", m.originalName, m.AccentColor()))
	} else {
		b.WriteString(renderFullscreenTitle("New Custom Context", "", m.AccentColor()))
	}
	b.WriteString("\n\n")

	// Name input
	var nameLabel string
	if !m.projectListMode {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Project selection label
	selectedCount := len(m.selectedIDs)
	var projectsLabel string
	if m.projectListMode {
		projectsLabel = FieldLabelFocusedStyle.Render(fmt.Sprintf("Projects (%d selected):", selectedCount))
	} else {
		projectsLabel = FieldLabelStyle.Render(fmt.Sprintf("Projects (%d selected):", selectedCount))
	}
	b.WriteString(projectsLabel)
	b.WriteString("\n")

	// Search input (shown when search is active or has a query)
	if m.IsSearchMode() {
		b.WriteString("  " + m.SearchInputView())
		b.WriteString("\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  Filter: %s (%d/%d)", m.SearchQuery(), len(m.filteredProjects), len(m.projects))))
		b.WriteString("\n")
	}

	// Project list
	if len(m.filteredProjects) == 0 {
		if m.SearchQuery() != "" {
			b.WriteString(MutedStyle.Render("  No matching projects"))
		} else {
			b.WriteString(MutedStyle.Render("  No projects available"))
		}
		b.WriteString("\n")
	} else {
		maxVisible := m.maxVisibleProjects()
		end := min(m.scrollOffset+maxVisible, len(m.filteredProjects))
		scrollbar := RenderScrollbar(len(m.filteredProjects), maxVisible, m.scrollOffset)
		listWidth := m.width
		if scrollbar != "" {
			listWidth = max(listWidth-1, 1)
		}

		var rowBuilder strings.Builder
		for i := m.scrollOffset; i < end; i++ {
			p := m.filteredProjects[i]
			isCursor := m.projectListMode && i == m.cursor
			isSelected := m.selectedIDs[p.ID]

			// Cursor
			cursor := "  "
			if isCursor {
				cursor = IconCursorRight + " "
			}

			// Checkbox
			checkbox := "[ ]"
			if isSelected {
				checkbox = "[x]"
			}

			// Project info
			info := fmt.Sprintf("%s (%s) - %d tasks", p.Name, p.Context, p.CountTasks())
			infoWidth := max(listWidth-lipgloss.Width(cursor)-lipgloss.Width(checkbox)-1, 1)
			info = truncateToVisualWidth(info, infoWidth)

			line := fmt.Sprintf("%s%s %s", cursor, checkbox, info)
			if isCursor {
				rowBuilder.WriteString(SelectedStyle.Render(line))
			} else if isSelected {
				rowBuilder.WriteString(fmt.Sprintf("%s%s %s", cursor, SelectionCheckStyle.Render(checkbox), info))
			} else {
				rowBuilder.WriteString(fmt.Sprintf("%s%s %s", cursor, MutedStyle.Render(checkbox), MutedStyle.Render(info)))
			}
			rowBuilder.WriteString("\n")
		}

		b.WriteString(attachScrollbarAtWidth(rowBuilder.String(), scrollbar, listWidth))
	}
	b.WriteString("\n")

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Footer
	var footerLeft, footerRight string
	if m.IsSearchMode() {
		footerLeft = HelpStyle.Render("Type to filter · Enter: confirm · Esc: clear")
		footerRight = HelpStyle.Render("")
	} else if m.projectListMode {
		footerLeft = HelpStyle.Render("j/k: move · Space: toggle · /: search · a/A: all/none · Tab: name")
		footerRight = HelpStyle.Render("Ctrl+S: save · Esc: cancel")
	} else {
		footerLeft = HelpStyle.Render("Tab/Enter: project list")
		footerRight = HelpStyle.Render("Ctrl+S: save · Esc: cancel")
	}
	b.WriteString(renderStatusBarFooter(footerLeft, footerRight, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

func (m *CustomContextEditorModel) StatusBarLabel() string { return "Custom Context Editor" }
