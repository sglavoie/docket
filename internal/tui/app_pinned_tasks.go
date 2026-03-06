package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type pinnedTaskInfo struct {
	Title       string
	ShortID     int
	ProjectName string
	PinnedAt    time.Time
	Task        *models.Task
	Project     *models.Project
}

func (m *AppModel) collectPinnedTasks() []pinnedTaskInfo {
	projects := m.config.GetActiveProjects()
	var pinned []pinnedTaskInfo

	for _, project := range projects {
		if project.Tasks == nil {
			if err := storage.LoadProject(project); err != nil {
				continue
			}
		}
		for _, task := range project.Tasks {
			if task.Pinned && !task.Status.IsComplete() {
				info := pinnedTaskInfo{
					Title:       task.Title,
					ShortID:     task.ShortID,
					ProjectName: project.Name,
					Task:        task,
					Project:     project,
				}
				if task.PinnedAt != nil {
					info.PinnedAt = *task.PinnedAt
				}
				pinned = append(pinned, info)
			}
		}
	}

	sort.Slice(pinned, func(i, j int) bool {
		return pinned[i].PinnedAt.Before(pinned[j].PinnedAt)
	})

	return pinned
}

func (m *AppModel) renderPinnedTasksBar(width int) string {
	pinned := m.collectPinnedTasks()
	if len(pinned) == 0 {
		return ""
	}

	separatorStr := MutedStyle.Render(" · ")
	separatorWidth := lipgloss.Width(separatorStr)

	maxDisplay := m.config.GetMaxPinnedBarItems()
	if maxDisplay == 0 {
		return ""
	}
	overflow := 0
	if len(pinned) > maxDisplay {
		overflow = len(pinned) - maxDisplay
		pinned = pinned[:maxDisplay]
	}

	numEntries := len(pinned)
	totalSeparatorWidth := separatorWidth * (numEntries - 1)
	if numEntries == 1 {
		totalSeparatorWidth = 0
	}
	perEntryWidth := (width - totalSeparatorWidth) / numEntries

	// Fixed overhead per entry: initial(1) + space(1)
	var entries []string
	for _, p := range pinned {
		initial := MutedStyle.Render(string([]rune(p.ProjectName)[0:1]))

		prefixWidth := 1 + 1
		titleWidth := perEntryWidth - prefixWidth
		if titleWidth < 3 {
			titleWidth = 3
		}

		title := p.Title
		titleRunes := []rune(title)
		if len(titleRunes) > titleWidth {
			title = string(titleRunes[:titleWidth-1]) + "…"
		} else if len(titleRunes) < titleWidth {
			title += strings.Repeat(" ", titleWidth-len(titleRunes))
		}

		entries = append(entries, initial+" "+title)
	}

	result := strings.Join(entries, separatorStr)

	if overflow > 0 {
		more := MutedStyle.Render(fmt.Sprintf(" +%d", overflow))
		result += more
	}

	return result
}

// handlePinnedPopupKey handles key events when the pinned tasks popup is open.
func (m *AppModel) handlePinnedPopupKey(msg tea.KeyMsg) (*AppModel, tea.Cmd) {
	// Handle second key of a pending prefix
	if m.pinnedPendingPrefix != "" {
		prefix := m.pinnedPendingPrefix
		m.pinnedPendingPrefix = ""
		key := msg.String()

		if key == "esc" {
			return m, nil
		}

		if prefix == "!" {
			priority, ok := PriorityForPrefixKey(key)
			if !ok {
				return m, nil
			}
			if m.pinnedPopupCursor < 0 || m.pinnedPopupCursor >= len(m.pinnedPopupItems) {
				return m, nil
			}
			entry := m.pinnedPopupItems[m.pinnedPopupCursor]
			result := applyPriorityChange(simplePersist(m.persister), entry.Project, entry.Task.ID, priority)
			m.statusMessage = result.StatusMessage

			m.pinnedPopupItems = m.collectPinnedTasks()
			if len(m.pinnedPopupItems) == 0 {
				m.showPinnedPopup = false
			} else if m.pinnedPopupCursor >= len(m.pinnedPopupItems) {
				m.pinnedPopupCursor = len(m.pinnedPopupItems) - 1
			}
		}

		if prefix == "," {
			status, ok := StatusForPrefixKey(key)
			if !ok {
				return m, nil
			}
			if m.pinnedPopupCursor < 0 || m.pinnedPopupCursor >= len(m.pinnedPopupItems) {
				return m, nil
			}
			entry := m.pinnedPopupItems[m.pinnedPopupCursor]
			result := applyStatusChange(simplePersist(m.persister), entry.Project, entry.Task.ID, status)
			m.statusMessage = result.StatusMessage

			// Rebuild list (task may have been completed and filtered out)
			m.pinnedPopupItems = m.collectPinnedTasks()
			if len(m.pinnedPopupItems) == 0 {
				m.showPinnedPopup = false
			} else if m.pinnedPopupCursor >= len(m.pinnedPopupItems) {
				m.pinnedPopupCursor = len(m.pinnedPopupItems) - 1
			}
		}
		return m, nil
	}

	switch msg.String() {
	case ",":
		m.pinnedPendingPrefix = ","
		return m, nil
	case "!":
		m.pinnedPendingPrefix = "!"
		return m, nil
	case "j", "down":
		if m.pinnedPopupCursor < len(m.pinnedPopupItems)-1 {
			m.pinnedPopupCursor++
		}
		return m, nil
	case "k", "up":
		if m.pinnedPopupCursor > 0 {
			m.pinnedPopupCursor--
		}
		return m, nil
	case "enter":
		return m.pinnedPopupSelect(m.pinnedPopupCursor)
	case "u":
		return m.unpinFromPopup()
	case "r":
		return m.repinFromPopup()
	case "J":
		return m.reorderPinnedTask(1)
	case "K":
		return m.reorderPinnedTask(-1)
	case "ctrl+j":
		if m.pinnedPreview != nil {
			visible := m.pinnedPreview.visibleNodes()
			if m.pinnedPreview.cursor < len(visible)-1 {
				m.pinnedPreview.cursor++
			}
		}
		return m, nil
	case "ctrl+k":
		if m.pinnedPreview != nil {
			if m.pinnedPreview.cursor > 0 {
				m.pinnedPreview.cursor--
			}
		}
		return m, nil
	case "ctrl+h":
		if m.pinnedPreview != nil {
			modes := m.pinnedPreview.cyclableViewModes()
			if idx := viewModeIndex(modes, m.pinnedPreview.viewMode); idx >= 0 {
				m.pinnedPreview.viewMode = modes[(idx+len(modes)-1)%len(modes)]
			} else {
				m.pinnedPreview.viewMode = modes[len(modes)-1]
			}
			m.pinnedPreview.updateVisibility()
			m.pinnedPreview.clampCursor()
		}
		return m, nil
	case "ctrl+l":
		if m.pinnedPreview != nil {
			modes := m.pinnedPreview.cyclableViewModes()
			if idx := viewModeIndex(modes, m.pinnedPreview.viewMode); idx >= 0 {
				m.pinnedPreview.viewMode = modes[(idx+1)%len(modes)]
			} else {
				m.pinnedPreview.viewMode = modes[0]
			}
			m.pinnedPreview.updateVisibility()
			m.pinnedPreview.clampCursor()
		}
		return m, nil
	case "esc", "#":
		m.showPinnedPopup = false
		m.pinnedPendingPrefix = ""
		m.pinnedLastUnpinned = nil
		m.invalidatePinnedPreview()
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		return m.pinnedPopupSelect(idx)
	default:
		m.showPinnedPopup = false
		m.pinnedLastUnpinned = nil
		m.invalidatePinnedPreview()
		return m, nil
	}
}

// pinnedPopupSelect navigates to the entry at the given index and closes the popup.
func (m *AppModel) pinnedPopupSelect(idx int) (*AppModel, tea.Cmd) {
	if idx < 0 || idx >= len(m.pinnedPopupItems) {
		m.showPinnedPopup = false
		m.invalidatePinnedPreview()
		return m, nil
	}
	entry := m.pinnedPopupItems[idx]
	m.showPinnedPopup = false
	m.invalidatePinnedPreview()
	m.navigateToTaskInProject(entry.Task, entry.Project)
	return m, nil
}

// reorderPinnedTask moves the current pinned item by delta (+1 = down, -1 = up)
// by swapping PinnedAt timestamps and persisting the affected projects.
func (m *AppModel) reorderPinnedTask(delta int) (*AppModel, tea.Cmd) {
	target := m.pinnedPopupCursor + delta
	if target < 0 || target >= len(m.pinnedPopupItems) {
		return m, nil
	}

	a := m.pinnedPopupItems[m.pinnedPopupCursor]
	b := m.pinnedPopupItems[target]

	// Swap PinnedAt timestamps. If they are equal (e.g. both pinned in the
	// same second, or truncated on a previous save), nudge one by a second
	// so the swap actually changes the sort order.
	aTime := a.PinnedAt
	bTime := b.PinnedAt
	if aTime.Equal(bTime) {
		bTime = bTime.Add(time.Second)
	}

	a.Project.MutateTask(a.Task.ID, func(t *models.Task) {
		t.PinnedAt = &bTime
	})
	b.Project.MutateTask(b.Task.ID, func(t *models.Task) {
		t.PinnedAt = &aTime
	})

	// Persist affected projects (may be the same project)
	_ = m.persister.SaveWithBackupSync(a.Project)
	if b.Project.Name != a.Project.Name {
		_ = m.persister.SaveWithBackupSync(b.Project)
	}

	// Rebuild the list and follow the cursor to the moved item
	m.pinnedPopupItems = m.collectPinnedTasks()
	m.pinnedPopupCursor = target

	return m, nil
}

// unpinFromPopup unpins the currently selected task and removes it from the popup.
func (m *AppModel) unpinFromPopup() (*AppModel, tea.Cmd) {
	if m.pinnedPopupCursor < 0 || m.pinnedPopupCursor >= len(m.pinnedPopupItems) {
		return m, nil
	}

	entry := m.pinnedPopupItems[m.pinnedPopupCursor]
	saved := entry
	m.pinnedLastUnpinned = &saved
	entry.Project.MutateTask(entry.Task.ID, func(t *models.Task) {
		t.Unpin()
	})
	_ = m.persister.SaveWithBackupSync(entry.Project)

	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.showPinnedPopup = false
	} else if m.pinnedPopupCursor >= len(m.pinnedPopupItems) {
		m.pinnedPopupCursor = len(m.pinnedPopupItems) - 1
	}

	return m, nil
}

// repinFromPopup re-pins the last unpinned task (undo for unpin).
func (m *AppModel) repinFromPopup() (*AppModel, tea.Cmd) {
	if m.pinnedLastUnpinned == nil {
		return m, nil
	}

	entry := m.pinnedLastUnpinned
	entry.Project.MutateTask(entry.Task.ID, func(t *models.Task) {
		t.Pin()
	})
	_ = m.persister.SaveWithBackupSync(entry.Project)

	m.pinnedLastUnpinned = nil
	m.pinnedPopupItems = m.collectPinnedTasks()

	return m, nil
}

// renderPinnedPopup renders the pinned tasks overlay popup.
func (m *AppModel) renderPinnedPopup() string {
	if !m.showPinnedPopup || len(m.pinnedPopupItems) == 0 {
		return ""
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	title := titleStyle.Render("Pinned Tasks")

	popupWidth := m.width - 4
	if popupWidth < 30 {
		popupWidth = 30
	}

	cursorStyle := SelectionRowStyle
	badgeStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	dimBadgeStyle := MutedStyle
	projectStyle := MutedStyle

	maxProjLen := 0
	for _, entry := range m.pinnedPopupItems {
		if len(entry.ProjectName) > maxProjLen {
			maxProjLen = len(entry.ProjectName)
		}
	}

	innerWidth := popupWidth - 4
	projColWidth := maxProjLen
	fixedWidth := 1 + 1 + 1 + 1 + 3 + 1 + 2 + projColWidth // badge, space, icon, space, priority(3), space, gap, project
	titleWidth := innerWidth - fixedWidth
	if titleWidth < 10 {
		titleWidth = 10
	}

	var contentLines []string
	for i, entry := range m.pinnedPopupItems {
		var badge string
		if i < 9 {
			badge = badgeStyle.Render(fmt.Sprintf("%d", i+1))
		} else {
			badge = dimBadgeStyle.Render("·")
		}

		statusIcon := entry.Task.Status.Icon()
		priorityBadge := renderPriorityBadge(entry.Task.Priority)
		projectName := entry.ProjectName

		taskTitle := entry.Title
		if len(taskTitle) > titleWidth {
			taskTitle = taskTitle[:titleWidth-3] + "..."
		}

		if len(taskTitle) < titleWidth {
			taskTitle += strings.Repeat(" ", titleWidth-len(taskTitle))
		}

		paddedProject := fmt.Sprintf("%*s", projColWidth, projectName)

		line := badge + " " + statusIcon + " " + priorityBadge + " " + taskTitle + "  " + projectStyle.Render(paddedProject)

		if i == m.pinnedPopupCursor {
			lineWidth := lipgloss.Width(line)
			if lineWidth < innerWidth {
				line += strings.Repeat(" ", innerWidth-lineWidth)
			}
			line = cursorStyle.Render(ansi.Truncate(line, innerWidth, ""))
		}

		contentLines = append(contentLines, line)
	}

	content := strings.Join(contentLines, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(popupWidth - 2)

	rendered := boxStyle.Render(content)

	lines := strings.Split(rendered, "\n")
	if len(lines) >= 2 {
		actualWidth := lipgloss.Width(lines[0])
		lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
		footer := MutedStyle.Render("j/k navigate  J/K reorder  ,status  !priority  u unpin  r repin  ctrl+j/k scroll  ctrl+h/l section  esc close")
		lines[len(lines)-1] = renderTitledBottomBorder(footer, actualWidth, borderStyle)
	}

	return strings.Join(lines, "\n")
}
