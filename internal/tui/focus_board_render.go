package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func (m *FocusBoardModel) View() string {
	var b strings.Builder

	// Title
	totalBoard := 0
	totalTasks := 0
	for _, r := range m.rows {
		if r.isBoardItem() {
			totalBoard++
		} else {
			totalTasks++
		}
	}
	subtitle := fmt.Sprintf("%d items, %d tasks", totalBoard, totalTasks)
	b.WriteString(renderFullscreenTitle("Focus Board", subtitle))
	b.WriteString("\n")

	// Tab bar
	b.WriteString(renderTabBarWithOptionalSubTabs(focusBoardTabNames, m.activeTab, nil, false, m.width))
	b.WriteString("\n")

	// Quick-add input
	addBlockHeight := 0
	if m.addMode {
		b.WriteString("  > ")
		b.WriteString(m.addInput.View())
		b.WriteString("\n")
		addBlockHeight++

		if m.isAutocompleteActive() {
			b.WriteString(MutedStyle.Render("    Tab: accept · ↑↓: navigate · Esc: dismiss"))
		} else {
			b.WriteString(MutedStyle.Render("    %project · !priority · #tag · -s source · -u url"))
		}
		b.WriteString("\n")
		addBlockHeight++

		// Autocomplete dropdown
		if m.isAutocompleteActive() {
			if m.acViewportTop > 0 {
				b.WriteString(MutedStyle.Render("    ↑ " + fmt.Sprintf("%d more", m.acViewportTop)))
				b.WriteString("\n")
				addBlockHeight++
			}

			visibleEnd := m.acViewportTop + acVisibleCount
			if visibleEnd > len(m.acSuggestions) {
				visibleEnd = len(m.acSuggestions)
			}

			for i := m.acViewportTop; i < visibleEnd; i++ {
				s := m.acSuggestions[i]
				if i == m.acCursor {
					b.WriteString(SelectedStyle.Render("    > " + s.display))
				} else {
					b.WriteString(MutedStyle.Render("      " + s.display))
				}
				b.WriteString("\n")
				addBlockHeight++
			}

			remaining := len(m.acSuggestions) - visibleEnd
			if remaining > 0 {
				b.WriteString(MutedStyle.Render("    ↓ " + fmt.Sprintf("%d more", remaining)))
				b.WriteString("\n")
				addBlockHeight++
			}
		}

		// Live preview of parsed prefixes/flags
		if value := strings.TrimSpace(m.addInput.Value()); value != "" {
			result := parseBoardAddInput(value)
			hasExtras := result.source != "" || result.url != "" ||
				(result.priority >= 1 && result.priority <= 5) ||
				len(result.tags) > 0 || result.projectSlug != ""
			if hasExtras {
				var previewParts []string
				if result.title != "" {
					previewParts = append(previewParts, "Title: "+result.title)
				}
				if result.projectSlug != "" {
					projectName := result.projectSlug
					for _, p := range m.acProjectCache {
						if p.Slug == result.projectSlug {
							projectName = p.Name
							break
						}
					}
					previewParts = append(previewParts, "Project: "+projectName)
				}
				if result.priority >= 1 && result.priority <= 5 {
					previewParts = append(previewParts, fmt.Sprintf("P%d", result.priority))
				}
				if len(result.tags) > 0 {
					tagStrs := make([]string, len(result.tags))
					for i, t := range result.tags {
						tagStrs[i] = "#" + t
					}
					previewParts = append(previewParts, strings.Join(tagStrs, " "))
				}
				if result.source != "" {
					previewParts = append(previewParts, "Source: "+result.source)
				}
				if result.url != "" {
					previewParts = append(previewParts, "URL: "+result.url)
				}
				b.WriteString(HelpStyle.Render("    " + strings.Join(previewParts, " · ")))
				b.WriteString("\n")
				addBlockHeight++
			}
		}
		b.WriteString("\n")
		addBlockHeight++
	}

	// Calculate available height
	contentHeight := m.height - 7 // title(1) + nl(1) + tabs(2) + footer(2) + status(1)
	if m.addMode {
		contentHeight -= addBlockHeight
	}

	// Empty state
	if len(m.filtered) == 0 {
		b.WriteString("\n")
		if m.activeTab == boardTabClosed {
			b.WriteString(MutedStyle.Render("  No closed items."))
		} else if len(m.rows) == 0 {
			b.WriteString(MutedStyle.Render("  No items on the board."))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("  Press a to add an item, or mark tasks as active in a project."))
		} else {
			b.WriteString(MutedStyle.Render("  No items match this tab."))
		}
		b.WriteString("\n\n")
		b.WriteString(renderStatusMessages(m.statusMessage, m.err))
		left := HelpStyle.Render("a add | tab switch | q close | ? help")
		right := MutedStyle.Render("0 items")
		b.WriteString(renderStatusBarFooter(left, right, m.width))
		return b.String()
	}

	// Render rows
	var lines []string
	maxTitleLen := m.width - 40
	if maxTitleLen < 20 {
		maxTitleLen = 20
	}

	for i, row := range m.filtered {
		isSelected := i == m.cursor

		// Quick-edit mode for the selected row
		if m.editMode && i == m.editIdx {
			line := "  > " + m.editInput.View()
			lines = append(lines, line)
			continue
		}

		line := m.renderRow(row, isSelected, maxTitleLen)
		lines = append(lines, line)
	}

	b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
	b.WriteString("\n\n")

	b.WriteString(renderStatusMessages(m.statusMessage, m.err))

	// Footer
	var left string
	if m.addMode || m.editMode {
		left = HelpStyle.Render("Enter: save · Esc: cancel")
	} else if m.activeTab == boardTabClosed {
		left = HelpStyle.Render("d reopen | D delete | o open URL | ? help")
	} else {
		left = HelpStyle.Render("a add | x done | X dismiss | p priority | e edit | enter open/go")
	}
	var right string
	if m.activeTab == boardTabClosed {
		right = MutedStyle.Render(fmt.Sprintf("%d closed", len(m.closedRows)))
	} else {
		right = MutedStyle.Render(fmt.Sprintf("%d/%d items", len(m.filtered), len(m.rows)))
	}
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// renderRow renders a single board row line.
func (m *FocusBoardModel) renderRow(row boardRow, isSelected bool, maxTitleLen int) string {
	var b strings.Builder

	if row.isBoardItem() {
		item := row.BoardItem
		isClosed := item.IsClosed()

		priorityBadge := boardPriorityBadge(item.Priority)
		sourceBadge := ""
		if item.Source != "" {
			sourceBadge = DependencyStyle.Render(fmt.Sprintf("[%s]", util.Truncate(item.Source, 10)))
		} else {
			sourceBadge = DependencyStyle.Render("[board]")
		}
		projectBadge := ""
		if item.ProjectName != "" {
			projectBadge = " " + MutedStyle.Render(fmt.Sprintf("[%s]", util.Truncate(item.ProjectName, 12)))
		}
		urlIndicator := ""
		if item.URL != "" {
			urlIndicator = " ↗"
		}
		linkIndicator := ""
		if item.LinkedTaskID != "" {
			linkIndicator = " ◆"
		}

		statusMarker := ""
		if isClosed {
			if item.Status == models.BoardItemDone {
				statusMarker = "[x] "
			} else {
				statusMarker = "[-] "
			}
		}

		title := util.Truncate(item.Title, maxTitleLen)
		content := fmt.Sprintf("  %s %s%s%s%s %s%s", priorityBadge, sourceBadge, projectBadge, urlIndicator, linkIndicator, statusMarker, title)

		if len(item.Tags) > 0 {
			content += " " + renderInlineTags(item.Tags)
		}

		if isClosed {
			content = MutedStyle.Render(content)
		}

		line := RenderSelectableRow(isSelected, false, content, m.width-2)
		b.WriteString(line)
	} else if row.isTask() {
		task := row.Task
		projectName := util.Truncate(row.Project.Name, 10)

		priorityBadge := boardPriorityBadge(task.Priority)
		projectBadge := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))
		typeIcon := " " + task.Type.Icon()

		dueStr := ""
		if task.DueDate != nil {
			dueStr = "  " + formatRelativeDue(task.DueDate, true)
		}

		title := util.Truncate(task.Title, maxTitleLen)
		bullet := BoldStyle.Render("●")
		content := fmt.Sprintf("  %s %s %s%s %s%s", bullet, priorityBadge, projectBadge, typeIcon, title, dueStr)

		if len(task.Tags) > 0 {
			content += " " + renderInlineTags(task.Tags)
		}

		line := RenderSelectableRow(isSelected, false, content, m.width-2)
		b.WriteString(line)
	}

	return b.String()
}

// boardPriorityBadge returns a colored priority label for board items (P1, P2, etc.).
func boardPriorityBadge(priority int) string {
	label := fmt.Sprintf("P%d", priority)
	switch priority {
	case 1:
		return OverdueStyle.Render(label)
	case 2:
		return DueSoonStyle.Render(label)
	default:
		return MutedStyle.Render(label)
	}
}
