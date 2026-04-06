package tui

import (
	"fmt"
	"sort"
	"strings"
)

// renderTagsField renders the tags field.
func (m *BulkEditModel) renderTagsField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldTags
	label := "Tags:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("\n")

	prefix := "    "

	if len(m.commonTags) > 0 {
		b.WriteString(prefix)
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Common: %s", strings.Join(m.commonTags, ", "))))
		b.WriteString("\n")
	}

	if m.tagMode == 1 {
		b.WriteString(prefix)
		b.WriteString("Add: ")
		b.WriteString(m.tagAddInput.View())
		b.WriteString("\n")
	} else {
		if len(m.addTags) > 0 {
			b.WriteString(prefix)
			b.WriteString(SuccessStyle.Render(fmt.Sprintf("+ %s", strings.Join(m.addTags, ", "))))
			b.WriteString("\n")
		}

		if len(m.removeTags) > 0 {
			tags := []string{}
			for t := range m.removeTags {
				tags = append(tags, t)
			}
			sort.Strings(tags)
			b.WriteString(prefix)
			b.WriteString(ValidationErrorStyle.Render(fmt.Sprintf("- %s", strings.Join(tags, ", "))))
			b.WriteString("\n")
		}
	}

	if m.tagMode == 2 {
		b.WriteString(prefix)
		b.WriteString("Select tags to remove:\n")
		for i, tag := range m.allTags {
			cursor := "[ ]"
			if m.removeTags[tag] {
				cursor = "[x]"
			}
			if m.tagRemoveCursor == i {
				cursor = "[>]"
			}
			line := fmt.Sprintf("%s  %s %s", prefix, cursor, tag)
			if m.tagRemoveCursor == i {
				b.WriteString(TaskJumperSelectedStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	if focused && m.tagMode == 0 {
		b.WriteString(prefix)
		b.WriteString(MutedStyle.Render("[a] add tag  [r] remove tag"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderDueDateField renders the due date field.
func (m *BulkEditModel) renderDueDateField() string {
	var b strings.Builder

	focused := m.focusIndex == fieldDueDate
	label := "Due Date:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.dueDateSummary)))
	b.WriteString("\n")

	prefix := "  "
	if focused {
		prefix = "    "
	}

	keepSelected := m.setDueDate == nil && !m.clearDueDate
	setSelected := m.setDueDate != nil
	clearSelected := m.clearDueDate

	cursor := "[ ]"
	if keepSelected {
		cursor = "[x]"
	}
	if focused && m.dueDateCursor == 0 {
		cursor = "[>]"
	}
	line := fmt.Sprintf("%s%s Keep current", prefix, cursor)
	if focused && m.dueDateCursor == 0 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	if m.dueDateInputMode {
		b.WriteString(prefix)
		b.WriteString("    Set date: ")
		b.WriteString(m.dueDateInput.View())
		b.WriteString("\n")
		if m.dueDateError != nil {
			b.WriteString(prefix)
			b.WriteString("    ")
			b.WriteString(ValidationErrorStyle.Render(m.dueDateError.Error()))
			b.WriteString("\n")
		}
	} else {
		cursor = "[ ]"
		if setSelected {
			cursor = "[x]"
		}
		if focused && m.dueDateCursor == 1 {
			cursor = "[>]"
		}
		var setLabel string
		if m.setDueDate != nil {
			setLabel = fmt.Sprintf("Set: %s (%s)", m.setDueDate.Format("2006-01-02"), m.setDueDate.Format("Monday"))
		} else {
			setLabel = "Set date..."
		}
		line = fmt.Sprintf("%s%s %s", prefix, cursor, setLabel)
		if focused && m.dueDateCursor == 1 {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	cursor = "[ ]"
	if clearSelected {
		cursor = "[x]"
	}
	if focused && m.dueDateCursor == 2 {
		cursor = "[>]"
	}
	line = fmt.Sprintf("%s%s Clear due date", prefix, cursor)
	if focused && m.dueDateCursor == 2 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	return b.String()
}

// renderParentField renders the parent selection field.
func (m *BulkEditModel) renderParentField() string { //nolint:gocyclo
	var b strings.Builder

	focused := m.focusIndex == fieldParent
	label := "Parent:"
	if focused {
		label = "> " + label
	} else {
		label = "  " + label
	}

	b.WriteString(m.fieldLabel(label, focused))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("(%s)", m.parentSummary)))
	b.WriteString("\n")

	prefix := "  "
	if focused {
		prefix = "    "
	}

	keepSelected := m.parentValue == nil && !m.clearParent
	setSelected := m.parentValue != nil
	rootSelected := m.clearParent

	cursor := "[ ]"
	if keepSelected {
		cursor = "[x]"
	}
	if focused && !m.parentListMode && m.parentCursor == 0 {
		cursor = "[>]"
	}
	line := fmt.Sprintf("%s%s Keep current", prefix, cursor)
	if focused && !m.parentListMode && m.parentCursor == 0 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	if m.parentListMode {
		if len(m.parentCandidates) == 0 {
			b.WriteString(prefix)
			b.WriteString(MutedStyle.Render("    No valid parent candidates"))
			b.WriteString("\n")
		} else {
			maxVisible := 8
			end := min(m.parentListOffset+maxVisible, len(m.parentCandidates))
			if m.parentListOffset > 0 {
				b.WriteString(prefix)
				b.WriteString(MutedStyle.Render("    ↑ more"))
				b.WriteString("\n")
			}
			for i := m.parentListOffset; i < end; i++ {
				candidate := m.parentCandidates[i]
				indent := strings.Repeat("  ", candidate.Level)
				icon := candidate.Type.Icon()
				cLine := fmt.Sprintf("%s    %s[%s] %s", prefix, indent, icon, candidate.Title)
				if m.parentListCursor == i {
					b.WriteString(TaskJumperSelectedStyle.Render("> " + cLine[2:]))
				} else {
					b.WriteString(cLine)
				}
				b.WriteString("\n")
			}
			if end < len(m.parentCandidates) {
				b.WriteString(prefix)
				b.WriteString(MutedStyle.Render("    ↓ more"))
				b.WriteString("\n")
			}
		}
	} else {
		cursor = "[ ]"
		if setSelected {
			cursor = "[x]"
		}
		if focused && m.parentCursor == 1 {
			cursor = "[>]"
		}
		var setLabel string
		if m.parentValue != nil {
			setLabel = "Set parent"
			for _, t := range m.project.Tasks {
				if t.ID == *m.parentValue {
					setLabel = fmt.Sprintf("Parent: [%s] %s", t.Type.Icon(), t.Title)
					break
				}
			}
		} else {
			setLabel = "Set parent..."
		}
		line = fmt.Sprintf("%s%s %s", prefix, cursor, setLabel)
		if focused && m.parentCursor == 1 {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	cursor = "[ ]"
	if rootSelected {
		cursor = "[x]"
	}
	if focused && !m.parentListMode && m.parentCursor == 2 {
		cursor = "[>]"
	}
	line = fmt.Sprintf("%s%s Move to root", prefix, cursor)
	if focused && !m.parentListMode && m.parentCursor == 2 {
		b.WriteString(TaskJumperSelectedStyle.Render(line))
	} else {
		b.WriteString(line)
	}
	b.WriteString("\n")

	return b.String()
}
