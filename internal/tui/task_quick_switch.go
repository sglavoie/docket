package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// RecentTaskVisitEntry holds a hydrated task+project pair for the quick-switch list.
type RecentTaskVisitEntry struct {
	Task    *models.Task
	Project *models.Project
	Pinned  bool
}

const maxPinnedTaskVisits = 9

// recordTaskVisit records a task visit for the quick-switch list.
// It deduplicates by task ID (moving revisits to the front), trims to m.config.GetMaxRecentTaskVisits(),
// and persists the ID-only form to config.
func (m *AppModel) recordTaskVisit(task *models.Task, project *models.Project) {
	if task == nil || project == nil {
		return
	}

	// Remove existing entry with the same task ID (dedup), preserve Pinned
	var wasPinned bool
	filtered := m.recentTaskVisits[:0:0]
	for _, entry := range m.recentTaskVisits {
		if entry.Task.ID == task.ID {
			wasPinned = entry.Pinned
		} else {
			filtered = append(filtered, entry)
		}
	}

	// Prepend the new visit, preserving pin status
	m.recentTaskVisits = append([]RecentTaskVisitEntry{{Task: task, Project: project, Pinned: wasPinned}}, filtered...)

	// Trim: pinned entries are exempt from eviction
	if len(m.recentTaskVisits) > m.config.GetMaxRecentTaskVisits() {
		var pinned, unpinned []RecentTaskVisitEntry
		for _, entry := range m.recentTaskVisits {
			if entry.Pinned {
				pinned = append(pinned, entry)
			} else {
				unpinned = append(unpinned, entry)
			}
		}
		remaining := max(m.config.GetMaxRecentTaskVisits()-len(pinned), 0)
		if len(unpinned) > remaining {
			unpinned = unpinned[:remaining]
		}
		m.recentTaskVisits = append(pinned, unpinned...)
	}

	m.persistRecentTaskVisits()
}

// persistRecentTaskVisits saves the current recentTaskVisits to config.
func (m *AppModel) persistRecentTaskVisits() {
	visits := make([]config.RecentTaskVisit, len(m.recentTaskVisits))
	for i, entry := range m.recentTaskVisits {
		visits[i] = config.RecentTaskVisit{
			TaskID:    entry.Task.ID,
			ProjectID: entry.Project.ID,
			Pinned:    entry.Pinned,
		}
	}
	m.config.SetRecentTaskVisits(visits)
	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}

// refreshRecentTaskVisits re-hydrates each entry's Task pointer from current project data
// so that status changes (completions, etc.) are reflected when the popup opens.
func (m *AppModel) refreshRecentTaskVisits() {
	valid := m.recentTaskVisits[:0]
	for _, entry := range m.recentTaskVisits {
		_ = m.persister.Load(entry.Project)
		if fresh := entry.Project.GetTaskByID(entry.Task.ID); fresh != nil {
			entry.Task = fresh
			valid = append(valid, entry)
		}
	}
	m.recentTaskVisits = valid
}

// sortedQuickSwitchEntries returns entries sorted: pinned first (by recency), then unpinned (by recency).
func (m *AppModel) sortedQuickSwitchEntries() []RecentTaskVisitEntry {
	var pinned, unpinned []RecentTaskVisitEntry
	for _, entry := range m.recentTaskVisits {
		if entry.Pinned {
			pinned = append(pinned, entry)
		} else {
			unpinned = append(unpinned, entry)
		}
	}
	return append(pinned, unpinned...)
}

// openQuickSwitch resets quick-switch state and refreshes the task list.
func (m *AppModel) openQuickSwitch() {
	m.showQuickSwitch = true
	m.quickSwitchCursor = 0
	m.quickSwitchFilter = ""
	m.quickSwitchFiltering = false
	m.quickSwitchFiltered = nil
	m.quickSwitchPendingPrefix = ""
	m.quickSwitchShowCompleted = false
	m.refreshRecentTaskVisits()
	// Re-sort so pinned entries come first
	m.recentTaskVisits = m.sortedQuickSwitchEntries()
	m.rebuildQuickSwitchFilter()
}

// rebuildQuickSwitchFilter rebuilds the filtered indices based on current filter text.
func (m *AppModel) rebuildQuickSwitchFilter() {
	entries := m.recentTaskVisits
	if m.quickSwitchFilter == "" {
		// No filter: show entries matching completion toggle
		m.quickSwitchFiltered = make([]int, 0, len(entries))
		m.quickSwitchTitlePositions = make([][]int, 0, len(entries))
		for i, entry := range entries {
			if entry.Task.Status.IsComplete() != m.quickSwitchShowCompleted {
				continue
			}
			m.quickSwitchFiltered = append(m.quickSwitchFiltered, i)
			m.quickSwitchTitlePositions = append(m.quickSwitchTitlePositions, nil)
		}
		return
	}

	// Score each entry against the filter
	type scored struct {
		idx            int
		score          int
		titlePositions []int
	}
	var pinned, unpinned []scored
	for i, entry := range entries {
		if entry.Task.Status.IsComplete() != m.quickSwitchShowCompleted {
			continue
		}
		text := entry.Task.Title + " " + entry.Project.Name
		score := fuzzy.ScoreTerms(text, m.quickSwitchFilter)
		if score <= 0 {
			continue
		}
		// Get match positions for just the title (not the combined text)
		_, titlePos := fuzzy.ScoreTermsWithPositions(entry.Task.Title, m.quickSwitchFilter)
		s := scored{idx: i, score: score, titlePositions: titlePos}
		if entry.Pinned {
			pinned = append(pinned, s)
		} else {
			unpinned = append(unpinned, s)
		}
	}

	// Sort by score within each group (higher first)
	sort.Slice(pinned, func(i, j int) bool { return pinned[i].score > pinned[j].score })
	sort.Slice(unpinned, func(i, j int) bool { return unpinned[i].score > unpinned[j].score })

	all := append(pinned, unpinned...)
	m.quickSwitchFiltered = make([]int, 0, len(all))
	m.quickSwitchTitlePositions = make([][]int, 0, len(all))
	for _, s := range all {
		m.quickSwitchFiltered = append(m.quickSwitchFiltered, s.idx)
		m.quickSwitchTitlePositions = append(m.quickSwitchTitlePositions, s.titlePositions)
	}
}

// quickSwitchFilteredLen returns the number of visible (filtered) entries.
func (m *AppModel) quickSwitchFilteredLen() int {
	return len(m.quickSwitchFiltered)
}

// quickSwitchClampCursor clamps the cursor to the filtered list bounds.
func (m *AppModel) quickSwitchClampCursor() {
	n := m.quickSwitchFilteredLen()
	if n == 0 {
		m.quickSwitchCursor = 0
	} else if m.quickSwitchCursor >= n {
		m.quickSwitchCursor = n - 1
	}
}

// handleQuickSwitchKey handles key events when the quick-switch popup is open.
func (m *AppModel) handleQuickSwitchKey(msg tea.KeyMsg) (*AppModel, tea.Cmd) {
	// Handle second key of a pending prefix
	if m.quickSwitchPendingPrefix != "" {
		return m.handleQuickSwitchPrefixKey(msg)
	}
	if m.quickSwitchFiltering {
		return m.handleQuickSwitchFilterKey(msg)
	}
	return m.handleQuickSwitchNormalKey(msg)
}

// handleQuickSwitchPrefixKey dispatches the second key of a `,` or `!` prefix.
func (m *AppModel) handleQuickSwitchPrefixKey(msg tea.KeyMsg) (*AppModel, tea.Cmd) {
	prefix := m.quickSwitchPendingPrefix
	m.quickSwitchPendingPrefix = ""
	key := msg.String()

	if key == "esc" {
		return m, nil
	}

	if m.quickSwitchCursor < 0 || m.quickSwitchCursor >= m.quickSwitchFilteredLen() {
		return m, nil
	}
	realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
	entry := m.recentTaskVisits[realIdx]

	if prefix == "," {
		status, ok := StatusForPrefixKey(key)
		if !ok {
			return m, nil
		}
		result := applyStatusChange(simplePersist(m.persister), entry.Project, entry.Task.ID, status)
		m.setSuccessStatus(result.StatusMessage)
	}

	if prefix == "!" {
		priority, ok := PriorityForPrefixKey(key)
		if !ok {
			return m, nil
		}
		result := applyPriorityChange(simplePersist(m.persister), entry.Project, entry.Task.ID, priority)
		m.setSuccessStatus(result.StatusMessage)
	}

	// Re-hydrate and rebuild after mutation
	m.refreshRecentTaskVisits()
	m.recentTaskVisits = m.sortedQuickSwitchEntries()
	m.rebuildQuickSwitchFilter()
	m.quickSwitchClampCursor()

	return m, nil
}

// handleQuickSwitchNormalKey handles keys in normal (non-filtering) mode.
func (m *AppModel) handleQuickSwitchNormalKey(msg tea.KeyMsg) (*AppModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		if m.quickSwitchCursor < m.quickSwitchFilteredLen()-1 {
			m.quickSwitchCursor++
		}
		return m, nil
	case "k", "up":
		if m.quickSwitchCursor > 0 {
			m.quickSwitchCursor--
		}
		return m, nil
	case "enter":
		return m.quickSwitchSelectFiltered(m.quickSwitchCursor)
	case "esc":
		if m.quickSwitchFilter != "" {
			// Filter active: clear it, stay in popup
			m.quickSwitchFilter = ""
			m.quickSwitchCursor = 0
			m.rebuildQuickSwitchFilter()
			return m, nil
		}
		m.showQuickSwitch = false
		return m, nil
	case "|":
		m.showQuickSwitch = false
		return m, nil
	case "p":
		m.quickSwitchTogglePin()
		return m, nil
	case ",":
		m.quickSwitchPendingPrefix = ","
		return m, nil
	case "!":
		m.quickSwitchPendingPrefix = "!"
		return m, nil
	case "c":
		m.quickSwitchShowCompleted = !m.quickSwitchShowCompleted
		m.quickSwitchCursor = 0
		m.rebuildQuickSwitchFilter()
		m.quickSwitchClampCursor()
		return m, nil
	case "x":
		m.quickSwitchRemoveEntry()
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(key[0]-'0') - 1
		return m.quickSwitchSelectFiltered(idx)
	case "/":
		m.quickSwitchFiltering = true
		m.quickSwitchFilter = ""
		m.quickSwitchCursor = 0
		m.rebuildQuickSwitchFilter()
		return m, nil
	default:
		// Ignore unbound keys (don't close popup)
		return m, nil
	}
}

// handleQuickSwitchFilterKey handles keys in filter (type-to-search) mode.
func (m *AppModel) handleQuickSwitchFilterKey(msg tea.KeyMsg) (*AppModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "up":
		if m.quickSwitchCursor > 0 {
			m.quickSwitchCursor--
		}
		return m, nil
	case "down":
		if m.quickSwitchCursor < m.quickSwitchFilteredLen()-1 {
			m.quickSwitchCursor++
		}
		return m, nil
	case "enter":
		// Exit filter mode but keep filter text for navigation
		m.quickSwitchFiltering = false
		return m, nil
	case "esc":
		// Clear filter and exit filter mode
		m.quickSwitchFilter = ""
		m.quickSwitchFiltering = false
		m.quickSwitchCursor = 0
		m.rebuildQuickSwitchFilter()
		return m, nil
	case "backspace":
		if len(m.quickSwitchFilter) > 0 {
			m.quickSwitchFilter = m.quickSwitchFilter[:len(m.quickSwitchFilter)-1]
			if m.quickSwitchFilter == "" {
				m.quickSwitchFiltering = false
			}
			m.quickSwitchCursor = 0
			m.rebuildQuickSwitchFilter()
		}
		return m, nil
	default:
		runes := msg.Runes
		if len(runes) == 1 && unicode.IsPrint(runes[0]) {
			m.quickSwitchFilter += string(runes[0])
			m.quickSwitchCursor = 0
			m.rebuildQuickSwitchFilter()
			return m, nil
		}
		return m, nil
	}
}

// quickSwitchSelectFiltered selects the entry at the given filtered index.
func (m *AppModel) quickSwitchSelectFiltered(filteredIdx int) (*AppModel, tea.Cmd) {
	if filteredIdx < 0 || filteredIdx >= m.quickSwitchFilteredLen() {
		m.showQuickSwitch = false
		return m, nil
	}
	realIdx := m.quickSwitchFiltered[filteredIdx]
	return m.quickSwitchSelect(realIdx)
}

// quickSwitchTogglePin toggles the pin state of the entry at the cursor.
func (m *AppModel) quickSwitchTogglePin() {
	if m.quickSwitchCursor < 0 || m.quickSwitchCursor >= m.quickSwitchFilteredLen() {
		return
	}

	realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
	entry := &m.recentTaskVisits[realIdx]

	// Enforce max pinned cap
	if !entry.Pinned {
		pinnedCount := 0
		for _, e := range m.recentTaskVisits {
			if e.Pinned {
				pinnedCount++
			}
		}
		if pinnedCount >= maxPinnedTaskVisits {
			m.setInfoStatus(fmt.Sprintf("Maximum %d pinned tasks reached", maxPinnedTaskVisits))
			return
		}
	}

	entry.Pinned = !entry.Pinned

	// Re-sort and fix cursor position: find the same task after sort
	taskID := entry.Task.ID
	m.recentTaskVisits = m.sortedQuickSwitchEntries()
	m.rebuildQuickSwitchFilter()
	for i, filteredIdx := range m.quickSwitchFiltered {
		if m.recentTaskVisits[filteredIdx].Task.ID == taskID {
			m.quickSwitchCursor = i
			break
		}
	}

	m.persistRecentTaskVisits()
}

// quickSwitchRemoveEntry removes the entry at the cursor from the list.
func (m *AppModel) quickSwitchRemoveEntry() {
	if m.quickSwitchCursor < 0 || m.quickSwitchCursor >= m.quickSwitchFilteredLen() {
		return
	}

	realIdx := m.quickSwitchFiltered[m.quickSwitchCursor]
	m.recentTaskVisits = append(m.recentTaskVisits[:realIdx], m.recentTaskVisits[realIdx+1:]...)

	if len(m.recentTaskVisits) == 0 {
		m.showQuickSwitch = false
		return
	}

	// Rebuild filter and clamp cursor
	m.rebuildQuickSwitchFilter()
	if m.quickSwitchFilteredLen() == 0 {
		// Filter matches nothing after removal
		m.quickSwitchFilter = ""
		m.quickSwitchFiltering = false
		m.rebuildQuickSwitchFilter()
	}
	m.quickSwitchClampCursor()

	m.persistRecentTaskVisits()
}

// quickSwitchSelect navigates to the entry at the given index and closes the popup.
func (m *AppModel) quickSwitchSelect(idx int) (*AppModel, tea.Cmd) {
	if idx < 0 || idx >= len(m.recentTaskVisits) {
		m.showQuickSwitch = false
		return m, nil
	}
	entry := m.recentTaskVisits[idx]
	m.showQuickSwitch = false
	return m, m.navigateToTaskInProject(entry.Task, entry.Project)
}

// renderQuickSwitchPopup renders the quick-switch overlay popup.
func (m *AppModel) renderQuickSwitchPopup() string {
	if !m.showQuickSwitch || len(m.recentTaskVisits) == 0 {
		return ""
	}

	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	titleStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	titleText := "Recent Tasks"
	if m.quickSwitchShowCompleted {
		titleText += " (Completed)"
	}
	title := titleStyle.Render(titleText)

	// Determine popup width: full terminal width minus small margin
	popupWidth := max(m.width-4, 30)

	cursorStyle := SelectionRowStyle
	badgeStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	dimBadgeStyle := MutedStyle
	projectStyle := MutedStyle
	pinStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	tagStyle := MutedStyle
	dividerStyle := MutedStyle
	filterStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	filtered := m.quickSwitchFiltered
	innerWidth := popupWidth - 4 // border + padding

	var contentLines []string

	// Filter input line
	if m.quickSwitchFiltering {
		filterLine := filterStyle.Render("> "+m.quickSwitchFilter) + MutedStyle.Render("_")
		contentLines = append(contentLines, filterLine)
	} else if m.quickSwitchFilter != "" {
		// Show locked-in filter (no cursor)
		filterLine := filterStyle.Render("> " + m.quickSwitchFilter)
		contentLines = append(contentLines, filterLine)
	}

	if len(filtered) == 0 {
		noMatch := MutedStyle.Render("No matching tasks")
		contentLines = append(contentLines, noMatch)
	} else {
		// Find the longest project name among visible entries for right-aligned column
		maxProjLen := 0
		for _, realIdx := range filtered {
			entry := m.recentTaskVisits[realIdx]
			if len(entry.Project.Name) > maxProjLen {
				maxProjLen = len(entry.Project.Name)
			}
		}

		// Layout per line: badge(1) + space(1) + icon(1) + space(1) + priority(3) + space(1) + [pin(2) +] title + gap(2) + tags + due + project
		projColWidth := maxProjLen
		fixedWidth := 1 + 1 + 1 + 1 + 3 + 1 + 2 + projColWidth // badge, space, icon, space, priority, space, gap, project
		titleWidth := max(innerWidth-fixedWidth, 10)

		// Check if there's a divider needed between pinned and unpinned in filtered list
		hasPinned := false
		firstUnpinnedFilteredIdx := -1
		for i, realIdx := range filtered {
			entry := m.recentTaskVisits[realIdx]
			if entry.Pinned {
				hasPinned = true
			} else if firstUnpinnedFilteredIdx == -1 {
				firstUnpinnedFilteredIdx = i
			}
		}

		for i, realIdx := range filtered {
			entry := m.recentTaskVisits[realIdx]

			// Insert visual divider between pinned and unpinned sections
			if hasPinned && i == firstUnpinnedFilteredIdx {
				divider := dividerStyle.Render(strings.Repeat("─", innerWidth))
				contentLines = append(contentLines, divider)
			}

			var badge string
			if !m.quickSwitchFiltering && i < 9 {
				badge = badgeStyle.Render(fmt.Sprintf("%d", i+1))
			} else {
				badge = dimBadgeStyle.Render("·")
			}

			statusIcon := GetStatusStyle(string(entry.Task.Status)).Render(entry.Task.Status.Icon())
			priorityBadge := renderPriorityBadge(entry.Task.Priority)
			projectName := entry.Project.Name

			// Pin indicator
			pinIndicator := "  "
			if entry.Pinned {
				pinIndicator = pinStyle.Render("* ")
			}

			// Build metadata suffix: tags + due date
			var metaParts []string
			if tags := entry.Task.Tags; len(tags) > 0 {
				displayTags := tags
				if len(displayTags) > 2 {
					displayTags = displayTags[:2]
				}
				for _, tag := range displayTags {
					metaParts = append(metaParts, tagStyle.Render("#"+tag))
				}
			}
			if dueStr := FormatRelativeDue(entry.Task.DueDate, true); dueStr != "" {
				metaParts = append(metaParts, "@"+dueStr)
			}
			metaSuffix := ""
			if len(metaParts) > 0 {
				metaSuffix = " " + strings.Join(metaParts, " ")
			}

			// Reduce title width to account for pin indicator and meta
			metaVisualWidth := lipgloss.Width(metaSuffix)
			pinVisualWidth := lipgloss.Width(pinIndicator)
			adjustedTitleWidth := max(titleWidth-metaVisualWidth-pinVisualWidth, 10)

			// Truncate title if needed
			taskTitle := entry.Task.Title
			titleRunes := []rune(taskTitle)
			truncated := false
			if len(titleRunes) > adjustedTitleWidth {
				titleRunes = titleRunes[:adjustedTitleWidth-3]
				truncated = true
			}

			// Apply fuzzy match highlighting to title
			if i < len(m.quickSwitchTitlePositions) && len(m.quickSwitchTitlePositions[i]) > 0 {
				// Filter positions to only those within the visible (possibly truncated) range
				maxPos := len(titleRunes)
				var validPositions []int
				for _, p := range m.quickSwitchTitlePositions[i] {
					if p < maxPos {
						validPositions = append(validPositions, p)
					}
				}
				taskTitle = HighlightText(string(titleRunes), validPositions, lipgloss.NewStyle())
				if truncated {
					taskTitle += "..."
				}
				// Pad to fixed width using visual width (ANSI codes don't count)
				visualWidth := lipgloss.Width(taskTitle)
				if visualWidth < adjustedTitleWidth {
					taskTitle += strings.Repeat(" ", adjustedTitleWidth-visualWidth)
				}
			} else {
				taskTitle = string(titleRunes)
				if truncated {
					taskTitle += "..."
				}
				// Pad title to fixed width for alignment
				if len([]rune(taskTitle)) < adjustedTitleWidth {
					taskTitle += strings.Repeat(" ", adjustedTitleWidth-len([]rune(taskTitle)))
				}
			}

			// Right-align project name within its column
			paddedProject := fmt.Sprintf("%*s", projColWidth, projectName)

			line := badge + " " + statusIcon + " " + priorityBadge + " " + pinIndicator + taskTitle + metaSuffix + "  " + projectStyle.Render(paddedProject)

			if i == m.quickSwitchCursor {
				lineWidth := lipgloss.Width(line)
				if lineWidth < innerWidth {
					line += strings.Repeat(" ", innerWidth-lineWidth)
				}
				line = cursorStyle.Render(ansi.Truncate(line, innerWidth, ""))
			}

			contentLines = append(contentLines, line)
		}
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
		var footer string
		if m.quickSwitchFiltering {
			footer = MutedStyle.Render("↑↓ nav  enter confirm  esc clear")
		} else if m.quickSwitchFilter != "" {
			footer = MutedStyle.Render("j/k nav  enter select  ,status  !pri  / filter  esc clear")
		} else {
			footer = MutedStyle.Render("j/k nav  / filter  1-9 jump  p pin  c toggle  ,status  !pri  x remove  esc close")
		}
		lines[len(lines)-1] = renderTitledBottomBorder(footer, actualWidth, borderStyle)
	}

	return strings.Join(lines, "\n")
}
