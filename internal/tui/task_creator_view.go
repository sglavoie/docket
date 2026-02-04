package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

func (m *TaskCreatorModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle("Create Task", ""))
	b.WriteString("\n")

	// Tab bar
	tabNames := creatorTabNames
	if !m.currentTypeCanHaveChildren() {
		// Hide subtasks tab for types that can't have children
		tabNames = creatorTabNames[:2]
	}
	b.WriteString(RenderTabBar(tabNames, m.activeTab, m.width-4, 1))
	b.WriteString("\n\n")

	// Render content based on active tab
	switch m.activeTab {
	case creatorTabFields:
		b.WriteString(m.renderFieldsTab())
	case creatorTabChecklist:
		b.WriteString(m.renderChecklistTab())
	case creatorTabSubtasks:
		b.WriteString(m.renderSubtasksTab())
	}

	// Discard confirmation
	if m.confirmDiscard {
		b.WriteString("\n")
		b.WriteString(PreviewWarningStyle.Render("Discard unsaved changes? (y/n)"))
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
	}

	// Footer (use " · " separator for renderWrappingHelpLine)
	var helpText string
	switch m.activeTab {
	case creatorTabFields:
		helpText = "[]: tabs · Tab: next · ←/→: select · Ctrl+S: save · Ctrl+T: tags · Ctrl+A: add checklist · /: search parents · Esc: cancel"
	case creatorTabChecklist:
		helpText = "[]: tabs · Tab: next · Ctrl+A: add · Ctrl+K/J: reorder · Ctrl+D: delete · Ctrl+S: save · Ctrl+T: tags · Esc: cancel"
	case creatorTabSubtasks:
		helpText = "[]: tabs · Tab: next · ←/→: type · Ctrl+O: optional · Ctrl+T: add · Ctrl+K/J: reorder · Ctrl+D: delete · Ctrl+S: save · Esc: cancel"
	}

	content := b.String()

	var out strings.Builder
	out.WriteString(content)
	out.WriteString("\n")
	// Use wrapping help line for better display on narrow terminals
	wrappedHelp := renderWrappingHelpLine(helpText, m.width)
	out.WriteString(MutedStyle.Render(strings.Repeat("─", m.width)))
	out.WriteString("\n")
	out.WriteString(HelpStyle.Render(wrappedHelp))

	return out.String()
}

// renderFieldsTab renders the Fields tab content.
func (m *TaskCreatorModel) renderFieldsTab() string {
	var b strings.Builder

	// Title field (inline textarea)
	b.WriteString(renderFormLabelShared("Title", 0, m.focusIndex))
	b.WriteString("\n")
	b.WriteString(m.titleInput.View())
	b.WriteString("\n")

	// Type selector
	b.WriteString(renderFormLabelShared("Type", 1, m.focusIndex))
	b.WriteString(renderTypeSelectorShared(m.types, m.typeCursor, m.focusIndex == 1))
	b.WriteString("\n")

	// Parent selector
	b.WriteString(renderFormLabelShared("Parent", 2, m.focusIndex))
	b.WriteString(renderParentSelectorShared(m.validParents, m.parentCursor, m.focusIndex == 2))
	b.WriteString("\n")

	// Priority selector
	b.WriteString(renderFormLabelShared("Priority", 3, m.focusIndex))
	b.WriteString(renderPrioritySelectorShared(m.priority, m.focusIndex == 3))
	b.WriteString("\n")

	// Section selector
	b.WriteString(renderFormLabelShared("Section", 4, m.focusIndex))
	b.WriteString(renderSectionSelectorShared(m.sections, m.sectionCursor, m.focusIndex == 4))
	b.WriteString("\n")

	// Due date row
	b.WriteString(renderFormLabelShared("Due", 5, m.focusIndex))
	b.WriteString(renderDueDateFieldShared(m.dueDateInput, m.focusIndex == 5))
	b.WriteString("\n")

	// Flags row
	b.WriteString(renderFormLabelShared("Flags", 6, m.focusIndex))
	b.WriteString(renderFlagsSelectorShared(m.pinned, m.active, m.flagFocus, m.focusIndex == 6))
	b.WriteString("\n")

	// Recurrence selector
	b.WriteString(renderFormLabelShared("Recur", 7, m.focusIndex))
	b.WriteString(renderRecurrenceSelectorShared(m.recurrenceOpts, m.recurrenceCursor, m.focusIndex == 7, m.recurrence))
	b.WriteString("\n")

	// Tags row (read-only, modal)
	b.WriteString(renderFormLabelShared("Tags", 8, m.focusIndex))
	if len(m.tags) == 0 {
		b.WriteString(MutedStyle.Render("(none)"))
	} else {
		b.WriteString(strings.Join(m.tags, ", "))
	}
	b.WriteString("  ")
	b.WriteString(HelpStyle.Render("Ctrl+T"))
	b.WriteString("\n")

	// Description field (single line, truncated, Enter to edit)
	b.WriteString(renderFormLabelShared("Desc", 9, m.focusIndex))
	descDisplay := m.descValue
	if descDisplay == "" {
		descDisplay = MutedStyle.Render("(press Enter to edit)")
	} else {
		descDisplay = strings.ReplaceAll(descDisplay, "\n", " ")
		descDisplay = util.Truncate(descDisplay, 50)
	}
	if m.focusIndex == 9 {
		b.WriteString(SelectorOptionSelectedStyle.Render(descDisplay))
	} else {
		b.WriteString(descDisplay)
	}

	return b.String()
}

// renderChecklistTab renders the Checklist tab content.
func (m *TaskCreatorModel) renderChecklistTab() string {
	var b strings.Builder

	b.WriteString(MutedStyle.Render(fmt.Sprintf("Checklist Items (%d)", len(m.checklist))))
	b.WriteString("\n\n")

	if len(m.checklist) == 0 {
		b.WriteString(MutedStyle.Render("No checklist items. Press Ctrl+A to add one."))
		b.WriteString("\n")
	} else {
		for i, item := range m.checklist {
			fieldIndex := m.checklistStartIndex() + i
			isFocused := m.focusIndex == fieldIndex

			if isFocused {
				b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(fmt.Sprintf("▸ %d. ☐ ", i+1)))
			} else {
				b.WriteString(fmt.Sprintf("  %d. ☐ ", i+1))
			}

			b.WriteString(item.View())
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderSubtasksTab renders the Subtasks tab content.
func (m *TaskCreatorModel) renderSubtasksTab() string {
	var b strings.Builder

	if !m.currentTypeCanHaveChildren() {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("%s cannot have subtasks. Switch to Epic or Story to add subtasks.", m.types[m.typeCursor])))
		b.WriteString("\n")
		return b.String()
	}

	allowedTypes := m.getAllowedSubtaskTypes()

	b.WriteString(MutedStyle.Render(fmt.Sprintf("Subtasks (%d)", len(m.subtasks))))
	b.WriteString("\n\n")

	if len(m.subtasks) == 0 {
		b.WriteString(MutedStyle.Render("No subtasks. Press Ctrl+T to add one."))
		b.WriteString("\n")
	} else {
		for i, st := range m.subtasks {
			fieldIndex := m.subtaskStartIndex() + i
			isFocused := m.focusIndex == fieldIndex

			// Optional indicator
			optMarker := "○"
			if st.optional {
				optMarker = "◌"
			}

			// Get the subtask type, clamping to valid range
			typeCursor := st.typeCursor
			if typeCursor >= len(allowedTypes) {
				typeCursor = 0
			}
			subtaskType := allowedTypes[typeCursor]

			// Type badge with color
			typeBadge := GetTypeBadge(string(subtaskType), subtaskType.Icon())

			if isFocused {
				b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(fmt.Sprintf("▸ %d. %s ", i+1, optMarker)))
				b.WriteString(typeBadge)
				b.WriteString(" ")
			} else {
				b.WriteString(fmt.Sprintf("  %d. %s ", i+1, optMarker))
				b.WriteString(typeBadge)
				b.WriteString(" ")
			}

			b.WriteString(st.titleInput.View())
			if st.optional {
				b.WriteString(MutedStyle.Render(" (optional)"))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}
