package tui

import (
	"fmt"
	"strings"
)

// renderPreview renders the preview of pending changes.
func (m *BulkEditModel) renderPreview() string {
	changes := m.buildChanges()
	if !changes.HasChanges() {
		return MutedStyle.Render("  Preview: No changes pending")
	}

	var parts []string

	if changes.SetStatus != nil {
		parts = append(parts, fmt.Sprintf("Status → %s", *changes.SetStatus))
	}
	if changes.SetType != nil {
		parts = append(parts, fmt.Sprintf("Type → %s", changes.SetType.HeadingPrefix()))
	}
	if changes.SetPriority != nil {
		parts = append(parts, fmt.Sprintf("Priority → P%d", *changes.SetPriority))
	}
	for _, tag := range changes.AddTags {
		parts = append(parts, fmt.Sprintf("+%s", tag))
	}
	for _, tag := range changes.RemoveTags {
		parts = append(parts, fmt.Sprintf("-%s", tag))
	}
	if changes.SetSection != nil {
		if *changes.SetSection == "" {
			parts = append(parts, "Section → Active")
		} else {
			parts = append(parts, fmt.Sprintf("Section → %s", *changes.SetSection))
		}
	}
	if changes.SetDueDate != nil {
		parts = append(parts, fmt.Sprintf("Due → %s", changes.SetDueDate.Format("2006-01-02")))
	} else if changes.ClearDueDate {
		parts = append(parts, "Clear due dates")
	}
	if changes.SetRecurrence != nil {
		parts = append(parts, fmt.Sprintf("Recurrence → %s", changes.SetRecurrence.String()))
	} else if changes.ClearRecurrence {
		parts = append(parts, "Clear recurrence")
	}
	if changes.SetParentID != nil {
		parentName := *changes.SetParentID
		for _, t := range m.project.Tasks {
			if t.ID == *changes.SetParentID {
				parentName = t.Title
				break
			}
		}
		parts = append(parts, fmt.Sprintf("Parent → %s", parentName))
	} else if changes.ClearParent {
		parts = append(parts, "Move to root")
	}

	previewStyle := WarningIconStyle
	return previewStyle.Render(fmt.Sprintf("  Preview: %s", strings.Join(parts, ", ")))
}

// renderActions renders the action buttons.
func (m *BulkEditModel) renderActions() string {
	var b strings.Builder

	focused := m.focusIndex == fieldActions

	applyStyle := NormalStyle
	cancelStyle := NormalStyle

	if focused {
		if m.actionCursor == 0 {
			applyStyle = TaskJumperSelectedStyle
		} else {
			cancelStyle = TaskJumperSelectedStyle
		}
	}

	changes := m.buildChanges()
	applyLabel := "Apply Changes"
	if !changes.HasChanges() {
		applyLabel = "(No changes)"
	}

	b.WriteString("  ")
	b.WriteString(applyStyle.Render(fmt.Sprintf("[%s]", applyLabel)))
	b.WriteString("   ")
	b.WriteString(cancelStyle.Render("[Cancel]"))

	return b.String()
}

// renderFooter renders the help footer.
func (m *BulkEditModel) renderFooter() string {
	if m.tagMode == 1 {
		return MutedStyle.Render("  Enter: add tag · Esc: cancel")
	}
	if m.tagMode == 2 {
		return MutedStyle.Render("  Space: toggle · Tab: next · Esc: done")
	}
	if m.dueDateInputMode {
		return MutedStyle.Render("  Enter: set date · Esc: cancel · Formats: tomorrow, +3d, fri, 2024-01-15")
	}
	if m.parentListMode {
		return MutedStyle.Render("  j/k: browse · Enter: select parent · Esc: cancel")
	}
	return MutedStyle.Render("  Tab/j/k: navigate · Enter: select · h/l: options · Esc: cancel")
}

// fieldLabel formats a field label.
func (m *BulkEditModel) fieldLabel(label string, focused bool) string {
	if focused {
		return BoldStyle.Render(label)
	}
	return label
}
