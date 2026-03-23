package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/fuzzy"
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
			if task == nil {
				continue
			}
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

// rebuildPinnedPopupFilter rebuilds the filtered indices based on current filter text.
func (m *AppModel) rebuildPinnedPopupFilter() {
	items := m.pinnedPopupItems
	if m.pinnedPopupFilter == "" {
		m.pinnedPopupFiltered = make([]int, len(items))
		m.pinnedPopupTitlePos = make([][]int, len(items))
		for i := range items {
			m.pinnedPopupFiltered[i] = i
		}
		return
	}

	type scored struct {
		idx            int
		score          int
		titlePositions []int
	}
	var results []scored
	for i, entry := range items {
		text := entry.Title + " " + entry.ProjectName
		score := fuzzy.ScoreTerms(text, m.pinnedPopupFilter)
		if score <= 0 {
			continue
		}
		_, titlePos := fuzzy.ScoreTermsWithPositions(entry.Title, m.pinnedPopupFilter)
		results = append(results, scored{idx: i, score: score, titlePositions: titlePos})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	m.pinnedPopupFiltered = make([]int, 0, len(results))
	m.pinnedPopupTitlePos = make([][]int, 0, len(results))
	for _, s := range results {
		m.pinnedPopupFiltered = append(m.pinnedPopupFiltered, s.idx)
		m.pinnedPopupTitlePos = append(m.pinnedPopupTitlePos, s.titlePositions)
	}
}

// pinnedPopupFilteredLen returns the number of visible (filtered) entries.
func (m *AppModel) pinnedPopupFilteredLen() int {
	return len(m.pinnedPopupFiltered)
}

// pinnedPopupClampCursor clamps the cursor to the filtered list bounds.
func (m *AppModel) pinnedPopupClampCursor() {
	n := m.pinnedPopupFilteredLen()
	if n == 0 {
		m.pinnedPopupCursor = 0
	} else if m.pinnedPopupCursor >= n {
		m.pinnedPopupCursor = n - 1
	}
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
		titleWidth := max(perEntryWidth-prefixWidth, 3)

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
func (m *AppModel) handlePinnedPopupKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	// Handle second key of a pending prefix
	if m.pinnedPendingPrefix != "" {
		return m.handlePinnedPopupPrefixKey(msg)
	}
	if m.pinnedPopupFiltering {
		return m.handlePinnedPopupFilterKey(msg)
	}
	return m.handlePinnedPopupNormalKey(msg)
}

// handlePinnedPopupPrefixKey dispatches the second key of a `,` or `!` prefix.
func (m *AppModel) handlePinnedPopupPrefixKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	prefix := m.pinnedPendingPrefix
	m.pinnedPendingPrefix = ""
	key := msg.String()

	if key == "esc" {
		return m, nil
	}

	// Resolve the real index from filtered list
	if m.pinnedPopupCursor < 0 || m.pinnedPopupCursor >= m.pinnedPopupFilteredLen() {
		return m, nil
	}
	realIdx := m.pinnedPopupFiltered[m.pinnedPopupCursor]
	entry := m.pinnedPopupItems[realIdx]

	if prefix == "!" {
		priority, ok := PriorityForPrefixKey(key)
		if !ok {
			return m, nil
		}
		result := applyPriorityChange(simplePersist(m.persister), entry.Project, entry.Task.ID, priority)
		m.setSuccessStatus(result.StatusMessage)
	}

	if prefix == "," {
		status, ok := StatusForPrefixKey(key)
		if !ok {
			return m, nil
		}
		result := applyStatusChange(simplePersist(m.persister), entry.Project, entry.Task.ID, status)
		m.setSuccessStatus(result.StatusMessage)
	}

	// Rebuild list (task may have been completed and filtered out)
	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.showPinnedPopup = false
	} else {
		m.rebuildPinnedPopupFilter()
		m.pinnedPopupClampCursor()
	}

	return m, nil
}

// handlePinnedPopupNormalKey handles keys in normal (non-filtering) mode.
func (m *AppModel) handlePinnedPopupNormalKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	switch msg.String() {
	case ",":
		m.pinnedPendingPrefix = ","
		return m, nil
	case "!":
		m.pinnedPendingPrefix = "!"
		return m, nil
	case "j", "down":
		if m.pinnedPopupCursor < m.pinnedPopupFilteredLen()-1 {
			m.pinnedPopupCursor++
		}
		return m, nil
	case "k", "up":
		if m.pinnedPopupCursor > 0 {
			m.pinnedPopupCursor--
		}
		return m, nil
	case "enter":
		return m.pinnedPopupSelectFiltered(m.pinnedPopupCursor)
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
	case "esc":
		if m.pinnedPopupFilter != "" {
			// Filter active: clear it, stay in popup
			m.pinnedPopupFilter = ""
			m.pinnedPopupCursor = 0
			m.rebuildPinnedPopupFilter()
			return m, nil
		}
		m.showPinnedPopup = false
		m.pinnedPendingPrefix = ""
		m.pinnedLastUnpinned = nil
		m.invalidatePinnedPreview()
		return m, nil
	case "#":
		m.showPinnedPopup = false
		m.pinnedPendingPrefix = ""
		m.pinnedLastUnpinned = nil
		m.invalidatePinnedPreview()
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		return m.pinnedPopupSelectFiltered(idx)
	case "/":
		m.pinnedPopupFiltering = true
		m.pinnedPopupFilter = ""
		m.pinnedPopupCursor = 0
		m.rebuildPinnedPopupFilter()
		return m, nil
	default:
		// Ignore unbound keys (don't close popup)
		return m, nil
	}
}

// handlePinnedPopupFilterKey handles keys in filter (type-to-search) mode.
func (m *AppModel) handlePinnedPopupFilterKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) {
	key := msg.String()
	switch key {
	case "up":
		if m.pinnedPopupCursor > 0 {
			m.pinnedPopupCursor--
		}
		return m, nil
	case "down":
		if m.pinnedPopupCursor < m.pinnedPopupFilteredLen()-1 {
			m.pinnedPopupCursor++
		}
		return m, nil
	case "enter":
		// Exit filter mode but keep filter text for navigation
		m.pinnedPopupFiltering = false
		return m, nil
	case "esc":
		// Clear filter and exit filter mode
		m.pinnedPopupFilter = ""
		m.pinnedPopupFiltering = false
		m.pinnedPopupCursor = 0
		m.rebuildPinnedPopupFilter()
		return m, nil
	case "backspace":
		if len(m.pinnedPopupFilter) > 0 {
			m.pinnedPopupFilter = m.pinnedPopupFilter[:len(m.pinnedPopupFilter)-1]
			if m.pinnedPopupFilter == "" {
				m.pinnedPopupFiltering = false
			}
			m.pinnedPopupCursor = 0
			m.rebuildPinnedPopupFilter()
		}
		return m, nil
	default:
		runes := []rune(msg.Text)
		if len(runes) == 1 && unicode.IsPrint(runes[0]) {
			m.pinnedPopupFilter += string(runes[0])
			m.pinnedPopupCursor = 0
			m.rebuildPinnedPopupFilter()
			return m, nil
		}
		return m, nil
	}
}

// pinnedPopupSelectFiltered selects the entry at the given filtered index.
func (m *AppModel) pinnedPopupSelectFiltered(filteredIdx int) (*AppModel, tea.Cmd) {
	if filteredIdx < 0 || filteredIdx >= m.pinnedPopupFilteredLen() {
		m.showPinnedPopup = false
		m.invalidatePinnedPreview()
		return m, nil
	}
	realIdx := m.pinnedPopupFiltered[filteredIdx]
	return m.pinnedPopupSelect(realIdx)
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
	return m, m.navigateToTaskInProject(entry.Task, entry.Project)
}

// reorderPinnedTask moves the current pinned item by delta (+1 = down, -1 = up)
// by swapping PinnedAt timestamps and persisting the affected projects.
func (m *AppModel) reorderPinnedTask(delta int) (*AppModel, tea.Cmd) {
	target := m.pinnedPopupCursor + delta
	if target < 0 || target >= m.pinnedPopupFilteredLen() {
		return m, nil
	}

	realA := m.pinnedPopupFiltered[m.pinnedPopupCursor]
	realB := m.pinnedPopupFiltered[target]
	a := m.pinnedPopupItems[realA]
	b := m.pinnedPopupItems[realB]

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
	if err := m.persister.SaveWithBackupSync(a.Project); err != nil {
		m.setInfoStatus(fmt.Sprintf("Failed to save project %s: %v", a.Project.Name, err))
		return m, nil
	}
	if b.Project.Name != a.Project.Name {
		if err := m.persister.SaveWithBackupSync(b.Project); err != nil {
			m.setInfoStatus(fmt.Sprintf("Failed to save project %s: %v", b.Project.Name, err))
			return m, nil
		}
	}

	// Rebuild the list and follow the cursor to the moved item
	m.pinnedPopupItems = m.collectPinnedTasks()
	m.rebuildPinnedPopupFilter()
	m.pinnedPopupCursor = target

	return m, nil
}

// unpinFromPopup unpins the currently selected task and removes it from the popup.
func (m *AppModel) unpinFromPopup() (*AppModel, tea.Cmd) {
	if m.pinnedPopupCursor < 0 || m.pinnedPopupCursor >= m.pinnedPopupFilteredLen() {
		return m, nil
	}

	realIdx := m.pinnedPopupFiltered[m.pinnedPopupCursor]
	entry := m.pinnedPopupItems[realIdx]
	saved := entry
	m.pinnedLastUnpinned = &saved
	entry.Project.MutateTask(entry.Task.ID, func(t *models.Task) {
		t.Unpin()
	})
	if err := m.persister.SaveWithBackupSync(entry.Project); err != nil {
		m.setInfoStatus(fmt.Sprintf("Failed to save project %s: %v", entry.Project.Name, err))
		return m, nil
	}

	m.pinnedPopupItems = m.collectPinnedTasks()
	if len(m.pinnedPopupItems) == 0 {
		m.showPinnedPopup = false
	} else {
		m.rebuildPinnedPopupFilter()
		if m.pinnedPopupFilteredLen() == 0 {
			// Filter matches nothing after removal
			m.pinnedPopupFilter = ""
			m.pinnedPopupFiltering = false
			m.rebuildPinnedPopupFilter()
		}
		m.pinnedPopupClampCursor()
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
	if err := m.persister.SaveWithBackupSync(entry.Project); err != nil {
		m.setInfoStatus(fmt.Sprintf("Failed to save project %s: %v", entry.Project.Name, err))
		return m, nil
	}

	m.pinnedLastUnpinned = nil
	m.pinnedPopupItems = m.collectPinnedTasks()
	m.rebuildPinnedPopupFilter()
	m.pinnedPopupClampCursor()

	return m, nil
}

// renderPinnedPopup renders the pinned tasks overlay popup.
func (m *AppModel) renderPinnedPopup() string {
	if !m.showPinnedPopup || len(m.pinnedPopupItems) == 0 {
		return ""
	}

	borderStyle := InfoStatusStyle
	titleStyle := TitleStyle
	title := titleStyle.Render("Pinned Tasks")
	filterStyle := InfoStatusStyle

	popupWidth := max(m.width-4, 30)

	cursorStyle := SelectionRowStyle
	badgeStyle := InfoStatusStyle
	dimBadgeStyle := MutedStyle
	projectStyle := MutedStyle

	filtered := m.pinnedPopupFiltered
	innerWidth := popupWidth - 4

	maxProjLen := 0
	for _, realIdx := range filtered {
		entry := m.pinnedPopupItems[realIdx]
		if len(entry.ProjectName) > maxProjLen {
			maxProjLen = len(entry.ProjectName)
		}
	}

	projColWidth := maxProjLen
	fixedWidth := 1 + 1 + 1 + 1 + 3 + 1 + 2 + projColWidth // badge, space, icon, space, priority(3), space, gap, project
	titleWidth := max(innerWidth-fixedWidth, 10)

	var contentLines []string

	// Filter input line
	if m.pinnedPopupFiltering {
		filterLine := filterStyle.Render("> "+m.pinnedPopupFilter) + MutedStyle.Render("_")
		contentLines = append(contentLines, filterLine)
	} else if m.pinnedPopupFilter != "" {
		// Show locked-in filter (no cursor)
		filterLine := filterStyle.Render("> " + m.pinnedPopupFilter)
		contentLines = append(contentLines, filterLine)
	}

	if len(filtered) == 0 {
		noMatch := MutedStyle.Render("No matching tasks")
		contentLines = append(contentLines, noMatch)
	} else {
		for i, realIdx := range filtered {
			entry := m.pinnedPopupItems[realIdx]

			var badge string
			if !m.pinnedPopupFiltering && m.pinnedPopupFilter == "" && i < 9 {
				badge = badgeStyle.Render(fmt.Sprintf("%d", i+1))
			} else {
				badge = dimBadgeStyle.Render("·")
			}

			statusIcon := GetStatusStyle(string(entry.Task.Status)).Render(entry.Task.Status.Icon())
			priorityBadge := renderPriorityBadge(entry.Task.Priority)
			projectName := entry.ProjectName

			taskTitle := entry.Title
			titleRunes := []rune(taskTitle)
			truncated := false
			if len(titleRunes) > titleWidth {
				titleRunes = titleRunes[:titleWidth-3]
				truncated = true
			}

			// Apply fuzzy match highlighting to title
			if i < len(m.pinnedPopupTitlePos) && len(m.pinnedPopupTitlePos[i]) > 0 {
				maxPos := len(titleRunes)
				var validPositions []int
				for _, p := range m.pinnedPopupTitlePos[i] {
					if p < maxPos {
						validPositions = append(validPositions, p)
					}
				}
				taskTitle = HighlightText(string(titleRunes), validPositions, NormalStyle)
				if truncated {
					taskTitle += "..."
				}
				visualWidth := lipgloss.Width(taskTitle)
				if visualWidth < titleWidth {
					taskTitle += strings.Repeat(" ", titleWidth-visualWidth)
				}
			} else {
				taskTitle = string(titleRunes)
				if truncated {
					taskTitle += "..."
				}
				if len([]rune(taskTitle)) < titleWidth {
					taskTitle += strings.Repeat(" ", titleWidth-len([]rune(taskTitle)))
				}
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
		var footer string
		if m.pinnedPopupFiltering {
			footer = MutedStyle.Render("↑↓ nav  enter confirm  esc clear")
		} else if m.pinnedPopupFilter != "" {
			footer = MutedStyle.Render("j/k nav  enter select  ,status  !pri  / filter  esc clear")
		} else {
			footer = MutedStyle.Render("j/k nav  / filter  J/K reorder  ,status  !pri  u unpin  r repin  ctrl+j/k scroll  ctrl+h/l section  esc close")
		}
		lines[len(lines)-1] = renderTitledBottomBorder(footer, actualWidth, borderStyle)
	}

	return strings.Join(lines, "\n")
}
