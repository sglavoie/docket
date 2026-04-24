package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// renderChildrenTab renders the Children tab: collapsible Checklist and Subtasks sections.
func (m *TaskFormModel) renderChildrenTab() string {
	var b strings.Builder

	b.WriteString(m.renderChildrenChecklistSection())

	if m.mode == FormModeCreate && formTypeCanHaveSubtasks(m.draft.taskType) {
		b.WriteString(m.renderChildrenSubtasksSection())
	} else if m.mode == FormModeEdit {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Subtasks exist as independent tasks — edit via Parent field."))
		b.WriteString("\n")
	} else {
		// Create mode but type doesn't support subtasks.
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Switch to Epic or Story to add subtasks."))
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
	expandIcon := "▶"
	if m.checklistExpanded {
		expandIcon = "▼"
	}
	headerLabel := fmt.Sprintf("%s Checklist (%d)", expandIcon, count)
	sep := MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(FieldLabelFocusedStyle.Render("▸ "))
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
			b.WriteString(m.renderChecklistItemRow(i, item, itemFocused))
		}
		if count == 0 {
			b.WriteString(MutedStyle.Render("    No checklist items. Press Ctrl+A to add one."))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderChecklistItemRow renders a single checklist item row.
func (m *TaskFormModel) renderChecklistItemRow(i int, item models.ChecklistItem, focused bool) string {
	checkIcon := "☐"
	if item.Done {
		checkIcon = "☑"
	}
	text := item.Text
	if text == "" {
		text = MutedStyle.Render("(empty)")
	}
	line := fmt.Sprintf("%s %s", checkIcon, text)
	if focused {
		hint := MutedStyle.Render("  Enter: edit · Space: check · Ctrl+D: del · Ctrl+J/K: move")
		return FieldLabelFocusedStyle.Render("  ▸ ") + SelectorOptionSelectedStyle.Render(line) + hint + "\n"
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
	expandIcon := "▶"
	if m.subtasksExpanded {
		expandIcon = "▼"
	}
	headerLabel := fmt.Sprintf("%s Subtasks (%d)", expandIcon, count)
	sep := MutedStyle.Render(strings.Repeat("─", 3))

	if headerFocused {
		b.WriteString(FieldLabelFocusedStyle.Render("▸ "))
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
			b.WriteString(renderSubtaskItemRow(i, st, itemFocused))
		}
		if count == 0 {
			b.WriteString(MutedStyle.Render("    No subtasks. Press Ctrl+T to add one."))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderSubtaskItemRow renders a single subtask draft row.
func renderSubtaskItemRow(i int, st formSubtaskDraft, focused bool) string {
	badge := GetTypeBadge(string(st.taskType), st.taskType.Icon())
	title := st.title
	if title == "" {
		title = MutedStyle.Render("(empty)")
	}
	optional := ""
	if st.optional {
		optional = MutedStyle.Render(" (optional)")
	}
	line := fmt.Sprintf("%s  %s%s", badge, title, optional)
	if focused {
		hint := MutedStyle.Render("  Enter: edit · Ctrl+O: optional · Ctrl+D: del · Ctrl+J/K: move")
		return FieldLabelFocusedStyle.Render("  ▸ ") + SelectorOptionSelectedStyle.Render(line) + hint + "\n"
	}
	return "    " + line + "\n"
}
