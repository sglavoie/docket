package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// renderChildrenTab renders the Children tab: collapsible Checklist and Subtasks sections.
func (m *TaskFormModel) renderChildrenTab() string {
	var b strings.Builder

	b.WriteString(m.renderChildrenChecklistSection())

	switch m.mode {
	case FormModeCreate:
		b.WriteString(m.renderChildrenSubtasksSection())
	case FormModeEdit:
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Subtasks exist as independent tasks — edit via Parent field."))
		b.WriteString("\n")
	}

	return b.String()
}

// renderChildrenChecklistSection renders the Checklist collapsible section.
// focusIndex == 0 is the checklist header; 1..N are individual items (when expanded).
func (m *TaskFormModel) renderChildrenChecklistSection() string {
	var b strings.Builder

	headerFocused := m.activeTab == formTabChildren && m.focusIndex == 0
	count := len(m.draft.checklist)
	expandIcon := IconCollapsed
	if m.checklistExpanded {
		expandIcon = IconExpanded
	}
	headerLabel := fmt.Sprintf("%s Checklist (%d)", expandIcon, count)
	sep := MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(FieldLabelFocusedStyle.Render(IconCursorRight + " "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
		b.WriteString(MutedStyle.Render("  c: toggle · Ctrl+A: add"))
	} else {
		b.WriteString(FieldLabelStyle.Render("  "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
	}
	b.WriteString("\n")

	if m.checklistExpanded {
		for i, item := range m.draft.checklist {
			itemFocused := m.activeTab == formTabChildren && m.focusIndex == 1+i
			b.WriteString(m.renderChecklistItemRow(item, itemFocused))
		}
		if count == 0 {
			b.WriteString(MutedStyle.Render("    No checklist items. Press Ctrl+A to add one."))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderChecklistItemRow renders a single checklist item row.
func (m *TaskFormModel) renderChecklistItemRow(item models.ChecklistItem, focused bool) string {
	checkIcon := icons.Get("checkbox.unchecked")
	if item.Done {
		checkIcon = icons.Get("checkbox.checked")
	}
	text := item.Text
	if text == "" {
		text = MutedStyle.Render("(empty)")
	}
	line := fmt.Sprintf("%s %s", checkIcon, text)
	if focused {
		hint := MutedStyle.Render("  Enter: edit · Space: check · Ctrl+D: del · Ctrl+J/K: move")
		return FieldLabelFocusedStyle.Render("  "+IconCursorRight+" ") + SelectorOptionSelectedStyle.Render(line) + hint + "\n"
	}
	return "    " + line + "\n"
}

// renderChildrenSubtasksSection renders the Subtasks collapsible section.
// The subtasks header is at focusIndex == m.childrenSubtasksHeaderIdx().
// Subtask items follow at subsequent indices (when expanded).
func (m *TaskFormModel) renderChildrenSubtasksSection() string {
	var b strings.Builder

	headerIdx := m.childrenSubtasksHeaderIdx()
	headerFocused := m.activeTab == formTabChildren && m.focusIndex == headerIdx
	count := len(m.draft.subtasks)
	expandIcon := IconCollapsed
	if m.subtasksExpanded {
		expandIcon = IconExpanded
	}
	headerLabel := fmt.Sprintf("%s Subtasks (%d)", expandIcon, count)
	sep := MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(FieldLabelFocusedStyle.Render(IconCursorRight + " "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
		b.WriteString(MutedStyle.Render("  S: toggle · Ctrl+T: add"))
	} else {
		b.WriteString(FieldLabelStyle.Render("  "))
		b.WriteString(sep)
		b.WriteString(BoldStyle.Render(" " + headerLabel + " "))
		b.WriteString(MutedStyle.Render(strings.Repeat("─", 16)))
	}
	b.WriteString("\n")

	if m.subtasksExpanded {
		for i, st := range m.draft.subtasks {
			itemIdx := headerIdx + 1 + i
			itemFocused := m.activeTab == formTabChildren && m.focusIndex == itemIdx
			b.WriteString(renderSubtaskItemRow(st, itemFocused))
		}
		if count == 0 {
			b.WriteString(MutedStyle.Render("    No subtasks. Press Ctrl+T to add one."))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderSubtaskItemRow renders a single subtask draft row.
func renderSubtaskItemRow(st formSubtaskDraft, focused bool) string {
	title := st.title
	if title == "" {
		title = MutedStyle.Render("(empty)")
	}
	line := title
	if focused {
		hint := MutedStyle.Render("  Enter: edit · Ctrl+D: del · Ctrl+J/K: move")
		return FieldLabelFocusedStyle.Render("  "+IconCursorRight+" ") + SelectorOptionSelectedStyle.Render(line) + hint + "\n"
	}
	return "    " + line + "\n"
}
