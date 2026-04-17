package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *TaskEditorModel) View() tea.View {
	cardTitle := fmt.Sprintf("Edit Task #%d", m.task.ShortID)
	if m.hasChanges {
		cardTitle += " *"
	}

	left := HelpStyle.Render("Tab/↔ navigate · Space toggle · Ctrl+S save · Ctrl+E editor · Esc cancel")
	right := MutedStyle.Render(cardTitle)

	var out strings.Builder
	out.WriteString(renderFullscreenTitle(cardTitle, ""))
	out.WriteString("\n\n")
	out.WriteString(m.renderContent())
	out.WriteString("\n")
	out.WriteString(renderStatusBarFooter(left, right, m.width))

	return tea.NewView(out.String())
}

func (m *TaskEditorModel) renderContent() string {
	var content strings.Builder

	content.WriteString(m.renderFieldRows())
	content.WriteString(m.renderAttributesSection())
	content.WriteString(m.renderScoreSummary())
	content.WriteString(m.renderTagsRow())
	content.WriteString(m.renderDescriptionRow())

	if m.err != nil {
		content.WriteString("\n\n")
		content.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	return content.String()
}

func (m *TaskEditorModel) renderFieldRows() string {
	rows := []string{
		m.renderTitleRow(),
		m.renderSimpleRow("Type", 1, m.renderTypeSelector()),
		m.renderParentRow(),
		m.renderSimpleRow("Priority", 3, m.renderPrioritySelector()),
		m.renderSimpleRow("Section", 4, m.renderSectionSelector()),
		m.renderSimpleRow("Due", 5, m.renderDueDateField()),
		m.renderSimpleRow("Flags", 6, m.renderFlagsField()),
		m.renderSimpleRow("Recur", 7, m.renderRecurrenceSelector()),
		m.renderSimpleRow("URL", 8, m.renderURLField()),
		m.renderSimpleRow("Source", 9, m.renderSourceField()),
	}

	return strings.Join(rows, "\n") + "\n"
}

func (m *TaskEditorModel) renderTitleRow() string {
	applyThemedTextarea(&m.titleInput)
	return m.renderFormLabel("Title", 0) + "\n" + m.titleInput.View()
}

func (m *TaskEditorModel) renderSimpleRow(label string, focusIdx int, value string) string {
	return m.renderFormLabel(label, focusIdx) + value
}

func (m *TaskEditorModel) renderParentRow() string {
	var row strings.Builder
	row.WriteString(m.renderFormLabel("Parent", 2))
	row.WriteString(m.renderParentSelector())
	if len(m.validParents) > 0 {
		row.WriteString("  ")
		row.WriteString(HelpStyle.Render("/ to search"))
	}
	return row.String()
}

func (m *TaskEditorModel) renderTagsRow() string {
	var row strings.Builder
	row.WriteString(m.renderFormLabel("Tags", -1))
	if len(m.tags) == 0 {
		row.WriteString(MutedStyle.Render("(none)"))
	} else {
		row.WriteString(strings.Join(m.tags, ", "))
	}
	row.WriteString("  ")
	row.WriteString(HelpStyle.Render("Ctrl+T"))
	row.WriteString("\n")
	return row.String()
}

func (m *TaskEditorModel) renderDescriptionRow() string {
	var row strings.Builder
	row.WriteString(m.renderFormLabel("Description", -1))
	if m.task.Description == "" {
		row.WriteString(MutedStyle.Render("(empty)"))
	} else {
		lines := strings.Count(m.task.Description, "\n") + 1
		chars := len(m.task.Description)
		row.WriteString(MutedStyle.Render(fmt.Sprintf("(%d lines, %d chars)", lines, chars)))
	}
	row.WriteString("  ")
	row.WriteString(HelpStyle.Render("Ctrl+E"))
	return row.String()
}

// HasTextInput returns true because the task editor contains inputs.
func (m *TaskEditorModel) HasTextInput() bool {
	return true
}

// GetConfirmDialog syncs boolean confirm state into the overlay for floating rendering.
func (m *TaskEditorModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmTypeChange {
		m.confirmOverlay.Show("type-change", fmt.Sprintf("Changing type will clear %s attributes. Continue?", m.taskType.HeadingPrefix()))
	} else if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

// ShouldCaptureEsc returns true so the view always handles Esc internally.
func (m *TaskEditorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *TaskEditorModel) IsFullHeight() bool { return false }

func (m *TaskEditorModel) SupportsHelpShortcut() bool { return false }

func (m *TaskEditorModel) StatusBarLabel() string { return "Edit Task" }
