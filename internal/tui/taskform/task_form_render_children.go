package taskform

import (
	"fmt"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// renderChildrenTab renders the Children tab: collapsible Checklist and Subtasks sections.
func (m *TaskFormModel) renderChildrenTab(b *formBodyBuilder, bodyWidth int) {
	m.renderChildrenChecklistSection(b, bodyWidth)
	m.renderChildrenSubtasksSection(b, bodyWidth)
}

// renderChildrenChecklistSection renders the Checklist collapsible section.
// focusIndex == 0 is the checklist header; 1..N are individual items (when expanded).
func (m *TaskFormModel) renderChildrenChecklistSection(b *formBodyBuilder, bodyWidth int) {
	b.field(0)
	headerFocused := m.activeTab == formTabChildren && m.focusIndex == 0
	count := len(m.draft.checklist)
	expandIcon := theme.IconCollapsed
	if m.checklistExpanded {
		expandIcon = theme.IconExpanded
	}
	m.renderFormSectionHeader(b, expandIcon, "Checklist", count, headerFocused,
		"c toggle · Ctrl+A add", bodyWidth)

	if m.checklistExpanded {
		for i, item := range m.draft.checklist {
			itemFocused := m.activeTab == formTabChildren && m.focusIndex == 1+i
			b.field(1 + i)
			b.WriteString(renderChecklistItemRow(item, itemFocused, bodyWidth))
		}
		if count == 0 {
			renderFormSectionEmptyState(b, "No checklist items. Press Ctrl+A to add one.", bodyWidth)
		}
	}
}

// renderChecklistItemRow renders a single checklist item row.
func renderChecklistItemRow(item models.ChecklistItem, focused bool, bodyWidth int) string {
	checkIcon := icons.Get("checkbox.unchecked")
	if item.Done {
		checkIcon = icons.Get("checkbox.checked")
	}
	text := item.Text
	if text == "" {
		text = theme.MutedStyle.Render("(empty)")
	}
	return renderFormItemRow(fmt.Sprintf("%s %s", checkIcon, text), focused,
		"Enter edit · Space check · Ctrl+D delete · Ctrl+J/K move", bodyWidth)
}

// renderChildrenSubtasksSection renders the Subtasks collapsible section.
// The subtasks header is at focusIndex == m.childrenSubtasksHeaderIdx().
// Subtask items follow at subsequent indices (when expanded).
func (m *TaskFormModel) renderChildrenSubtasksSection(b *formBodyBuilder, bodyWidth int) {
	headerIdx := m.childrenSubtasksHeaderIdx()
	b.field(headerIdx)
	headerFocused := m.activeTab == formTabChildren && m.focusIndex == headerIdx
	count := len(m.draft.subtasks)
	expandIcon := theme.IconCollapsed
	if m.subtasksExpanded {
		expandIcon = theme.IconExpanded
	}
	m.renderFormSectionHeader(b, expandIcon, "Subtasks", count, headerFocused,
		"S toggle · Ctrl+T add", bodyWidth)

	if m.subtasksExpanded {
		for i, st := range m.draft.subtasks {
			itemIdx := headerIdx + 1 + i
			itemFocused := m.activeTab == formTabChildren && m.focusIndex == itemIdx
			b.field(itemIdx)
			b.WriteString(renderSubtaskItemRow(st, itemFocused, bodyWidth))
		}
		if count == 0 {
			renderFormSectionEmptyState(b, "No subtasks. Press Ctrl+T to add one.", bodyWidth)
		}
	}
}

// renderSubtaskItemRow renders a single subtask draft row.
func renderSubtaskItemRow(st formSubtaskDraft, focused bool, bodyWidth int) string {
	title := st.title
	if title == "" {
		title = theme.MutedStyle.Render("(empty)")
	}
	return renderFormItemRow(title, focused,
		"Enter edit · Ctrl+D delete · Ctrl+J/K move", bodyWidth)
}
