package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

const maxRecentTaskVisits = 15

// RecentTaskVisitEntry holds a hydrated task+project pair for the quick-switch list.
type RecentTaskVisitEntry struct {
	Task    *models.Task
	Project *models.Project
}

// recordTaskVisit records a task visit for the quick-switch list.
// It deduplicates by task ID (moving revisits to the front), trims to maxRecentTaskVisits,
// and persists the ID-only form to config.
func (m *AppModel) recordTaskVisit(task *models.Task, project *models.Project) {
	if task == nil || project == nil {
		return
	}

	// Remove existing entry with the same task ID (dedup)
	filtered := m.recentTaskVisits[:0:0]
	for _, entry := range m.recentTaskVisits {
		if entry.Task.ID != task.ID {
			filtered = append(filtered, entry)
		}
	}

	// Prepend the new visit
	m.recentTaskVisits = append([]RecentTaskVisitEntry{{Task: task, Project: project}}, filtered...)

	// Trim to max
	if len(m.recentTaskVisits) > maxRecentTaskVisits {
		m.recentTaskVisits = m.recentTaskVisits[:maxRecentTaskVisits]
	}

	// Persist ID-only form to config
	visits := make([]config.RecentTaskVisit, len(m.recentTaskVisits))
	for i, entry := range m.recentTaskVisits {
		visits[i] = config.RecentTaskVisit{
			TaskID:    entry.Task.ID,
			ProjectID: entry.Project.ID,
		}
	}
	m.config.SetRecentTaskVisits(visits)
	_ = m.configManager.Save(m.config)
}

// handleQuickSwitchKey handles key events when the quick-switch popup is open.
func (m *AppModel) handleQuickSwitchKey(msg tea.KeyMsg) (*AppModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.quickSwitchCursor < len(m.recentTaskVisits)-1 {
			m.quickSwitchCursor++
		}
		return m, nil
	case "k", "up":
		if m.quickSwitchCursor > 0 {
			m.quickSwitchCursor--
		}
		return m, nil
	case "enter":
		return m.quickSwitchSelect(m.quickSwitchCursor)
	case "esc", "ctrl+p":
		m.showQuickSwitch = false
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		return m.quickSwitchSelect(idx)
	default:
		// Close popup and let the key pass through
		m.showQuickSwitch = false
		return m, nil
	}
}

// quickSwitchSelect navigates to the entry at the given index and closes the popup.
func (m *AppModel) quickSwitchSelect(idx int) (*AppModel, tea.Cmd) {
	if idx < 0 || idx >= len(m.recentTaskVisits) {
		m.showQuickSwitch = false
		return m, nil
	}
	entry := m.recentTaskVisits[idx]
	m.showQuickSwitch = false
	m.navigateToTaskInProject(entry.Task, entry.Project)
	return m, nil
}

// renderQuickSwitchPopup renders the quick-switch overlay popup.
func (m *AppModel) renderQuickSwitchPopup() string {
	if !m.showQuickSwitch || len(m.recentTaskVisits) == 0 {
		return ""
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	title := titleStyle.Render("Recent Tasks")

	// Determine popup width
	maxTitleLen := 40
	popupWidth := m.width * 60 / 100
	if popupWidth > 70 {
		popupWidth = 70
	}
	if popupWidth < 30 {
		popupWidth = 30
	}

	cursorStyle := lipgloss.NewStyle().Reverse(true)
	badgeStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	dimBadgeStyle := MutedStyle
	projectStyle := MutedStyle

	var contentLines []string
	for i, entry := range m.recentTaskVisits {
		// Number badge: 1-9 in accent color, 10+ dimmed
		var badge string
		if i < 9 {
			badge = badgeStyle.Render(fmt.Sprintf("%d", i+1))
		} else {
			badge = dimBadgeStyle.Render("·")
		}

		statusIcon := entry.Task.Status.Icon()
		projectName := entry.Project.Name

		// Truncate title to fit within popup width, leaving room for badge, icon, project
		// Layout: "N ● Title (Project)"
		projSuffix := " (" + projectName + ")"
		availableForTitle := popupWidth - 4 - 2 - 1 - len(projSuffix) // padding, badge+space, icon+space, project
		taskTitle := entry.Task.Title
		if len(taskTitle) > availableForTitle && availableForTitle > 3 {
			taskTitle = taskTitle[:availableForTitle-3] + "..."
		} else if availableForTitle <= 3 && len(taskTitle) > maxTitleLen {
			taskTitle = taskTitle[:maxTitleLen-3] + "..."
		}

		line := badge + " " + statusIcon + " " + taskTitle + projectStyle.Render(projSuffix)

		if i == m.quickSwitchCursor {
			// Apply cursor highlight: pad line to inner width, then reverse
			lineWidth := lipgloss.Width(line)
			innerWidth := popupWidth - 4 // account for border + padding
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
		Width(popupWidth - 2) // inner width

	rendered := boxStyle.Render(content)

	// Replace top/bottom borders with titled versions
	lines := strings.Split(rendered, "\n")
	if len(lines) >= 2 {
		actualWidth := lipgloss.Width(lines[0])
		lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
		footer := MutedStyle.Render("j/k navigate  enter select  esc close")
		lines[len(lines)-1] = renderTitledBottomBorder(footer, actualWidth, borderStyle)
	}

	return strings.Join(lines, "\n")
}
