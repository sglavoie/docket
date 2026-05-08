package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskDecisionsModel) View() tea.View {
	if m.task == nil {
		return tea.NewView("No task selected")
	}

	var b strings.Builder
	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Decisions · #%d: %s", m.task.ShortID, m.task.Title), "", m.AccentColor()))
	b.WriteString("\n\n")

	if m.expandedView && m.hasSelection() {
		return m.renderExpandedView(&b)
	}
	if m.editMode {
		return m.renderEditMode(&b)
	}
	if m.addMode {
		return m.renderAddMode(&b)
	}

	m.renderDecisionsSummary(&b)
	m.renderMessages(&b)
	m.renderDecisionsList(&b)
	m.renderStatusMessage(&b)
	m.renderFooter(&b)

	return tea.NewView(b.String())
}

func (m *TaskDecisionsModel) renderExpandedView(b *strings.Builder) tea.View {
	decision := m.task.Decisions[m.cursor]
	timestamp := FormatTimestampBracket(decision.CreatedAt)
	headerLabel := fmt.Sprintf("Decision [%d] [%s]", m.cursor, decision.Category)
	if decision.Resolved {
		headerLabel += " [resolved]"
	}

	b.WriteString(FieldLabelFocusedStyle.Render(headerLabel))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(timestamp))
	b.WriteString("\n\n")
	b.WriteString(renderMarkdown(decision.Text, max(m.Width()-4, 1)))
	if decision.Rationale != "" {
		b.WriteString("\n")
		b.WriteString(FieldLabelStyle.Render("Rationale:"))
		b.WriteString("\n")
		b.WriteString(renderMarkdown(decision.Rationale, max(m.Width()-4, 1)))
	}
	b.WriteString("\n\n")
	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Enter/Esc: close expanded view"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskDecisionsModel) renderEditMode(b *strings.Builder) tea.View {
	if m.editIndex >= len(m.task.Decisions) {
		b.WriteString(ValidationErrorStyle.Render("Decision was deleted externally. Press Esc to cancel."))
		return tea.NewView(b.String())
	}
	decision := m.task.Decisions[m.editIndex]
	b.WriteString(FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Decision [%d]:", m.editIndex)))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(FormatTimestampLabeled("Created", decision.CreatedAt)))
	b.WriteString("\n\n")

	b.WriteString(m.renderCategorySelector(m.editCategoryIdx))
	b.WriteString("\n\n")

	applyThemedTextarea(&m.editTextInput)
	applyThemedTextarea(&m.editRationale)

	textLabel := "Text:"
	rationaleLabel := "Rationale:"
	if m.editActiveField == 0 {
		b.WriteString(FieldLabelFocusedStyle.Render(textLabel))
	} else {
		b.WriteString(FieldLabelStyle.Render(textLabel))
	}
	b.WriteString("\n")
	b.WriteString(m.editTextInput.View())
	b.WriteString("\n\n")

	if m.editActiveField == 1 {
		b.WriteString(FieldLabelFocusedStyle.Render(rationaleLabel))
	} else {
		b.WriteString(FieldLabelStyle.Render(rationaleLabel))
	}
	b.WriteString("\n")
	b.WriteString(m.editRationale.View())
	b.WriteString("\n\n")

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Tab: switch field · Ctrl+Y: cycle category · Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskDecisionsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(FieldLabelFocusedStyle.Render("Add Decision:"))
	b.WriteString("\n\n")

	b.WriteString(m.renderCategorySelector(m.categoryIndex))
	b.WriteString("\n\n")

	applyThemedTextarea(&m.textInput)
	applyThemedTextarea(&m.rationaleInput)

	textLabel := "Text:"
	rationaleLabel := "Rationale:"
	if m.activeField == 0 {
		b.WriteString(FieldLabelFocusedStyle.Render(textLabel))
	} else {
		b.WriteString(FieldLabelStyle.Render(textLabel))
	}
	b.WriteString("\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	if m.activeField == 1 {
		b.WriteString(FieldLabelFocusedStyle.Render(rationaleLabel))
	} else {
		b.WriteString(FieldLabelStyle.Render(rationaleLabel))
	}
	b.WriteString("\n")
	b.WriteString(m.rationaleInput.View())
	b.WriteString("\n\n")

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("Tab: switch field · Ctrl+Y: cycle category · Ctrl+S: save · Ctrl+E: external editor · Esc: cancel"), "", m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *TaskDecisionsModel) renderCategorySelector(selected int) string {
	cats := allDecisionCategories()
	var parts []string
	for i, cat := range cats {
		label := string(cat)
		if i == selected {
			parts = append(parts, SelectorOptionSelectedStyle.Render(label))
		} else {
			parts = append(parts, MutedStyle.Render(label))
		}
	}
	return FieldLabelStyle.Render("Category: ") + strings.Join(parts, " ")
}

func (m *TaskDecisionsModel) renderDecisionsSummary(b *strings.Builder) {
	resolvedCount := len(m.task.Decisions) - m.task.UnresolvedDecisionCount()
	countStr := fmt.Sprintf("%d decision(s)", len(m.task.Decisions))
	if resolvedCount > 0 {
		countStr += fmt.Sprintf(" (%d resolved)", resolvedCount)
	}
	b.WriteString(MutedStyle.Render(countStr))
	b.WriteString("\n\n")
}

func (m *TaskDecisionsModel) renderMessages(b *strings.Builder) {
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}
}

func (m *TaskDecisionsModel) renderDecisionsList(b *strings.Builder) {
	if len(m.task.Decisions) == 0 {
		b.WriteString(MutedStyle.Render("No decisions yet. Press 'a' to add one."))
		b.WriteString("\n")
		return
	}

	headerLines := 6
	availableHeight := m.Height() - headerLines
	maxVisible := max(availableHeight/5, 3)
	scrollOffset := calcScrollOffset(m.cursor, maxVisible)
	maxLen := max(m.Width()-12, 1)

	for i := scrollOffset; i < len(m.task.Decisions) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderDecisionEntry(i, m.task.Decisions[i], maxLen))
	}

	if len(m.task.Decisions) > maxVisible {
		remaining := len(m.task.Decisions) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}
}

func (m *TaskDecisionsModel) renderStatusMessage(b *strings.Builder) {
	if m.statusMessage != "" {
		b.WriteString(MutedStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}
}

func (m *TaskDecisionsModel) renderDecisionEntry(index int, decision *models.Decision, maxLen int) string {
	var entry strings.Builder
	entry.WriteString("  ")

	categoryLabel := fmt.Sprintf("[%s]", decision.Category)
	timestamp := FormatTimestampBracket(decision.CreatedAt)
	resolvedSuffix := ""
	if decision.Resolved {
		resolvedSuffix = " [resolved]"
	}

	if index == m.cursor {
		entry.WriteString(BoldStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(FieldLabelFocusedStyle.Render(categoryLabel))
		entry.WriteString(" ")
		entry.WriteString(FieldLabelFocusedStyle.Render(timestamp + resolvedSuffix))
	} else {
		entry.WriteString(MutedStyle.Render(fmt.Sprintf("[%d] ", index)))
		entry.WriteString(MutedStyle.Render(categoryLabel))
		entry.WriteString(" ")
		entry.WriteString(MutedStyle.Render(timestamp + resolvedSuffix))
	}
	entry.WriteString("\n")

	rendered := renderMarkdown(decision.Text, maxLen)
	renderedLines := strings.Split(rendered, "\n")
	for j, line := range renderedLines {
		if j > 1 {
			fmt.Fprintf(&entry, "      ... (%d more lines, press Enter to expand)\n", len(renderedLines)-2)
			break
		}

		entry.WriteString("      ")
		line = ansi.Truncate(line, maxLen, "")
		if decision.Resolved {
			entry.WriteString(MutedStyle.Render(line))
		} else {
			entry.WriteString(line)
		}
		entry.WriteString("\n")
	}

	return entry.String()
}

func (m *TaskDecisionsModel) renderFooter(b *strings.Builder) {
	b.WriteString("\n")
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine("a/n: add · e: edit · E: external editor · r: resolve · d/x: delete · y: copy · Enter: expand · Esc: close", m.Width())))
}
