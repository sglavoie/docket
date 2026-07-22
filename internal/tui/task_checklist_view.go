package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *TaskChecklistModel) View() tea.View {
	if m.task == nil {
		return tea.NewView("No task selected")
	}

	var b strings.Builder
	b.WriteString(m.renderChecklistHeader())
	b.WriteString("\n\n")

	if m.editMode {
		return m.renderEditMode(&b)
	}
	if m.addMode {
		return m.renderAddMode(&b)
	}

	m.renderChecklistMessages(&b)
	m.renderChecklistItems(&b)
	m.renderChecklistHelp(&b)

	return tea.NewView(b.String())
}

func (m *TaskChecklistModel) renderChecklistHeader() string {
	completed := 0
	for _, item := range m.task.Checklist {
		if item.Done {
			completed++
		}
	}
	total := len(m.task.Checklist)

	checklistTitle := fmt.Sprintf("Checklist for: #%d: %s",
		m.task.ShortID,
		TaskTitleStyleFor(m.task, TaskTitleStyleOptions{}).Render(m.task.Title))
	progressStr := ""
	if total > 0 {
		progressStr = fmt.Sprintf("%d/%d completed", completed, total)
	}

	return renderStatusBarFooter(TitleStyle.Render(checklistTitle), MutedStyle.Render(progressStr), m.Width(), m.AccentColor())
}

func (m *TaskChecklistModel) renderEditMode(b *strings.Builder) tea.View {
	b.WriteString(FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Item [%d]:", m.editIndex)))
	b.WriteString("\n\n")
	applyThemedTextarea(&m.editTextInput)
	b.WriteString(m.editTextInput.View())
	b.WriteString("\n\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskChecklistModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(FieldLabelFocusedStyle.Render("Add Item:"))
	b.WriteString("\n")
	applyThemedTextarea(&m.textInput)
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskChecklistModel) renderChecklistMessages(b *strings.Builder) {
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}
}

func (m *TaskChecklistModel) renderChecklistItems(b *strings.Builder) {
	if len(m.task.Checklist) == 0 {
		b.WriteString(MutedStyle.Render("No checklist items yet. Press 'a' to add one."))
		b.WriteString("\n")
		return
	}

	contentWidth := m.Width() - 4
	headerLines := 6
	availableHeight := m.Height() - headerLines
	maxVisible := max(availableHeight/2, 5)
	scrollOffset := calcScrollOffset(m.cursor, maxVisible)
	maxLen := max(contentWidth-8, 1)

	for i := scrollOffset; i < len(m.task.Checklist) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderChecklistItemEntry(i, maxLen))
	}

	if len(m.task.Checklist) > maxVisible {
		remaining := len(m.task.Checklist) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}
}

func (m *TaskChecklistModel) renderChecklistItemEntry(index, maxLen int) string {
	item := m.task.Checklist[index]
	checkbox := "[ ]"
	if item.Done {
		checkbox = "[x]"
	}

	text := strings.ReplaceAll(item.Text, "\n", " ")
	text = truncateToVisualWidth(text, maxLen)

	var entry strings.Builder
	entry.WriteString("  ")

	if index == m.cursor {
		entry.WriteString(BoldStyle.Render(checkbox + " "))
		if item.Done {
			entry.WriteString(MutedStyle.Render(text))
		} else {
			entry.WriteString(FieldLabelFocusedStyle.Render(text))
		}
	} else {
		entry.WriteString(MutedStyle.Render(checkbox + " "))
		if item.Done {
			entry.WriteString(MutedStyle.Render(text))
		} else {
			entry.WriteString(text)
		}
	}

	entry.WriteString("\n")
	return entry.String()
}

func (m *TaskChecklistModel) renderChecklistHelp(b *strings.Builder) {
	b.WriteString("\n")
	helpText := "j/k: nav · J/K: reorder · Space/Enter: toggle · e: edit · a: add · E: external editor · d: delete · y: copy · Y: copy all · Esc: close"
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(helpText, m.Width())))
}
