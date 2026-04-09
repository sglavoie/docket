package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// ShowProjectCycleManagerMsg is sent to open the project cycle manager.
type ShowProjectCycleManagerMsg struct{}

// ShowContextPickerForCycleMsg is sent to open context picker from cycle manager.
type ShowContextPickerForCycleMsg struct{}

// getContextKey returns the context key for cycle project storage.
// Custom contexts use "+" prefix to distinguish from regular contexts.
func getContextKey(filterContext, filterCustomContext string) string {
	if filterCustomContext != "" {
		return "+" + filterCustomContext
	}
	return filterContext
}

// buildProjectInfoListFiltered gathers active projects filtered by context.
// If filterCustomContext is set, filters by custom context membership.
// Otherwise, if filterContext is set, filters by project context.
// If both are empty, returns all active projects.
func buildProjectInfoListFiltered(cfg *config.Config, filterContext, filterCustomContext string) []ProjectInfo {
	activeProjects := cfg.GetActiveProjects()
	projects := make([]ProjectInfo, 0, len(activeProjects))

	for _, p := range activeProjects {
		// Custom context filter takes precedence
		if filterCustomContext != "" {
			if !cfg.IsProjectInCustomContext(filterCustomContext, p.ID) {
				continue
			}
		} else if filterContext != "" && p.Context != filterContext {
			continue
		}
		if err := storage.LoadProject(p); err != nil {
			logging.Warn("failed to load project %s: %v", p.Name, err)
		}
		projects = append(projects, ProjectInfo{
			Project:        p,
			TaskCount:      p.CountActiveTasks(),
			CompletedCount: p.CountCompletedTasks(),
		})
	}

	return projects
}

// --- Popup overlay methods (on *AppModel) ---

// refreshProjectCycleItems rebuilds the projectCycleItems list from current config.
func (m *AppModel) refreshProjectCycleItems() {
	m.projectCycleItems = buildProjectInfoListFiltered(m.config, m.config.FilterContext, m.config.FilterCustomContext)
}

// sortCycleItemsSelectedFirst sorts projectCycleItems so that projects
// in the cycle appear first, preserving existing order within each group.
func (m *AppModel) sortCycleItemsSelectedFirst() {
	contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
	cycleIDs := m.config.GetCycleProjectIDsForContext(contextKey)
	cycleIDSet := make(map[string]bool, len(cycleIDs))
	for _, id := range cycleIDs {
		cycleIDSet[id] = true
	}
	sort.SliceStable(m.projectCycleItems, func(i, j int) bool {
		iSel := cycleIDSet[m.projectCycleItems[i].Project.ID]
		jSel := cycleIDSet[m.projectCycleItems[j].Project.ID]
		if iSel != jSel {
			return iSel
		}
		return false
	})
}

// closeProjectCyclePopup closes the popup and does cleanup.
func (m *AppModel) closeProjectCyclePopup() {
	m.showProjectCycle = false
	m.projectCycleScroll = 0
	m.invalidateCyclePreview()
	if m.projectTree != nil {
		m.projectTree.buildTree()
	}
	var dash *DashboardModel
	if v := m.views.Get(ViewDashboard); v != nil {
		dash, _ = v.(*DashboardModel)
	}
	if dash == nil {
		dash = m.dashboard
	}
	if dash != nil {
		dash.Refresh(m.config)
	}
}

// handleProjectCycleKey handles key events when the project cycle popup is open.
func (m *AppModel) handleProjectCycleKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) { //nolint:gocyclo
	switch msg.String() {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		if idx < len(m.projectCycleItems) {
			m.closeProjectCyclePopup()
			_, cmd := m.switchToCycleProject(m.projectCycleItems[idx].Project)
			return m, cmd
		}
		return m, nil

	case "j", "down":
		if m.projectCycleCursor < len(m.projectCycleItems)-1 {
			m.projectCycleCursor++
			m.adjustCycleScroll()
		}
		return m, nil

	case "k", "up":
		if m.projectCycleCursor > 0 {
			m.projectCycleCursor--
			m.adjustCycleScroll()
		}
		return m, nil

	case "space":
		// Toggle cycle membership for highlighted project
		if m.projectCycleCursor < len(m.projectCycleItems) {
			item := m.projectCycleItems[m.projectCycleCursor]
			contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
			cycleIDs := m.config.GetCycleProjectIDsForContext(contextKey)
			// Check if already in cycle
			found := false
			var newIDs []string
			for _, id := range cycleIDs {
				if id == item.Project.ID {
					found = true
				} else {
					newIDs = append(newIDs, id)
				}
			}
			if !found {
				newIDs = append(newIDs, item.Project.ID)
			}
			m.config.SetCycleProjectsForContext(contextKey, newIDs)
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
		}
		return m, nil

	case "enter", "g":
		// Navigate to highlighted project
		if m.projectCycleCursor < len(m.projectCycleItems) {
			project := m.projectCycleItems[m.projectCycleCursor].Project
			m.closeProjectCyclePopup()
			return m, func() tea.Msg {
				return SelectProjectMsg{Project: project}
			}
		}
		return m, nil

	case "p":
		// Toggle pin on highlighted project
		if m.projectCycleCursor < len(m.projectCycleItems) {
			item := m.projectCycleItems[m.projectCycleCursor]
			item.Project.TogglePinned()
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
			currentID := item.Project.ID
			m.refreshProjectCycleItems()
			// Restore cursor to same project
			for i, pi := range m.projectCycleItems {
				if pi.Project.ID == currentID {
					m.projectCycleCursor = i
					break
				}
			}
		}
		return m, nil

	case "c":
		// Open context picker
		m.showProjectCycle = false
		return m, func() tea.Msg {
			return ShowContextPickerForCycleMsg{}
		}

	case "C":
		// Clear context filter
		if m.config.FilterContext != "" || m.config.FilterCustomContext != "" {
			m.config.FilterContext = ""
			m.config.FilterCustomContext = ""
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
			m.refreshProjectCycleItems()
			m.projectCycleCursor = 0
		}
		return m, nil

	case "u":
		// Clear all cycle selections for current context
		contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
		m.config.SetCycleProjectsForContext(contextKey, nil)
		if err := m.configManager.Save(m.config); err != nil {
			logging.Warn("failed to save config: %v", err)
		}
		return m, nil

	case "ctrl+j":
		// Scroll preview background down
		if m.projectCyclePreview != nil {
			visible := m.projectCyclePreview.visibleNodes()
			if m.projectCyclePreview.cursor < len(visible)-1 {
				m.projectCyclePreview.cursor++
			}
		}
		return m, nil

	case "ctrl+k":
		// Scroll preview background up
		if m.projectCyclePreview != nil {
			if m.projectCyclePreview.cursor > 0 {
				m.projectCyclePreview.cursor--
			}
		}
		return m, nil

	case "ctrl+h":
		if m.projectCyclePreview != nil {
			m.projectCyclePreview.cycleViewMode(-1)
		}
		return m, nil

	case "ctrl+l":
		if m.projectCyclePreview != nil {
			m.projectCyclePreview.cycleViewMode(1)
		}
		return m, nil

	case "esc", "@":
		m.closeProjectCyclePopup()
		return m, nil

	default:
		// Close popup and let the key pass through
		m.closeProjectCyclePopup()
		return m, nil
	}
}

// cycleMaxVisibleItems returns the maximum number of project items visible
// in the cycle popup viewport, based on terminal height.
func (m *AppModel) cycleMaxVisibleItems() int {
	maxFromHeight := m.height - OverlayListChrome
	maxFromPercent := (m.height * 50) / 100
	return max(min(maxFromHeight, maxFromPercent), 3)
}

// adjustCycleScroll ensures projectCycleScroll keeps the cursor visible.
func (m *AppModel) adjustCycleScroll() {
	maxVisible := m.cycleMaxVisibleItems()
	if len(m.projectCycleItems) <= maxVisible {
		m.projectCycleScroll = 0
		return
	}
	if m.projectCycleCursor < m.projectCycleScroll {
		m.projectCycleScroll = m.projectCycleCursor
	}
	if m.projectCycleCursor >= m.projectCycleScroll+maxVisible {
		m.projectCycleScroll = m.projectCycleCursor - maxVisible + 1
	}
}

// renderProjectCyclePopup renders the project cycle overlay popup.
func (m *AppModel) renderProjectCyclePopup() string {
	if !m.showProjectCycle || len(m.projectCycleItems) == 0 {
		return ""
	}

	borderStyle := InfoStatusStyle
	titleStyle := TitleStyle
	title := titleStyle.Render("Project Cycle")

	badgeStyle := InfoStatusStyle
	dimBadgeStyle := MutedStyle

	// Build subtitle: context indicator + selection count
	contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
	cycleIDs := m.config.GetCycleProjectIDsForContext(contextKey)
	cycleIDSet := make(map[string]bool, len(cycleIDs))
	for _, id := range cycleIDs {
		cycleIDSet[id] = true
	}

	var subtitleParts []string
	contextStyle := InfoStatusStyle
	if m.config.FilterCustomContext != "" {
		subtitleParts = append(subtitleParts, fmt.Sprintf("Context: %s", contextStyle.Render("+ "+m.config.FilterCustomContext)))
	} else if m.config.FilterContext != "" {
		subtitleParts = append(subtitleParts, fmt.Sprintf("Context: %s", contextStyle.Render(m.config.FilterContext)))
	} else {
		subtitleParts = append(subtitleParts, "Context: All")
	}
	selectedCount := len(cycleIDs)
	if selectedCount == 0 {
		subtitleParts = append(subtitleParts, "No projects selected")
	} else {
		subtitleParts = append(subtitleParts, fmt.Sprintf("%d %s selected", selectedCount, pluralize(selectedCount, "project", "projects")))
	}
	subtitle := MutedStyle.Render(strings.Join(subtitleParts, " · "))

	// Find max task count width for right-alignment
	maxCountWidth := 0
	for _, item := range m.projectCycleItems {
		w := len(fmt.Sprintf("%d", item.TaskCount))
		if w > maxCountWidth {
			maxCountWidth = w
		}
	}

	// Calculate ideal width from content
	maxNameLen := 0
	for _, item := range m.projectCycleItems {
		if width := lipgloss.Width(item.Project.Name); width > maxNameLen {
			maxNameLen = width
		}
	}
	// badge(1) + space(1) + checkbox(3) + space(1) + name + gap(2) + count + border/padding(4)
	idealWidth := 1 + 1 + 3 + 1 + maxNameLen + 2 + maxCountWidth + 4
	// Footer text must fit within the border: "╰─ " + footer + " ─╯" = 5 + footerLen
	footerText := "1-9 jump · space toggle · p pin · enter go · esc close"
	minWidth := lipgloss.Width(footerText) + 6
	popupWidth := min(max(idealWidth, minWidth), 60)
	// Constrain to tree area width so popup doesn't cover the sidebar
	treeWidth := (m.width * 55) / 100
	if maxTreePopup := treeWidth - 2; popupWidth > maxTreePopup && maxTreePopup > minWidth {
		popupWidth = maxTreePopup
	}
	if popupWidth > m.width-4 {
		popupWidth = max(m.width-4, minWidth)
	}

	innerWidth := max(popupWidth-4, 1)

	var contentLines []string
	contentLines = append(contentLines, subtitle)
	contentLines = append(contentLines, "")

	// Compute min/max task counts for dynamic color scaling
	minCount, maxCount := 0, 0
	if len(m.projectCycleItems) > 0 {
		minCount = m.projectCycleItems[0].TaskCount
		maxCount = m.projectCycleItems[0].TaskCount
		for _, item := range m.projectCycleItems[1:] {
			if item.TaskCount < minCount {
				minCount = item.TaskCount
			}
			if item.TaskCount > maxCount {
				maxCount = item.TaskCount
			}
		}
	}

	for i, item := range m.projectCycleItems {
		// Badge
		var badge string
		if i < 9 {
			badge = badgeStyle.Render(fmt.Sprintf("%d", i+1))
		} else {
			badge = dimBadgeStyle.Render("·")
		}

		// Checkbox
		var checkbox string
		if cycleIDSet[item.Project.ID] {
			checkbox = SelectionCheckStyle.Render("[x]")
		} else {
			checkbox = MutedStyle.Render("[ ]")
		}

		// Task count (right-aligned) — colored on a red→green gradient
		countText := fmt.Sprintf("%*d", maxCountWidth, item.TaskCount)
		color := TaskCountColor(item.TaskCount, minCount, maxCount)
		countStr := lipgloss.NewStyle().Foreground(color).Render(countText)

		// Layout: badge(1) + space(1) + checkbox(3) + space(1) + name + gap(2) + count
		fixedWidth := 1 + 1 + 3 + 1 + 2 + maxCountWidth
		nameWidth := max(innerWidth-fixedWidth, 10)

		plainName := truncateAndPad(item.Project.Name, nameWidth)
		displayName := plainName
		if item.Project.Pinned {
			displayName = PinnedProjectStyle.Render(plainName)
		}

		line := badge + " " + checkbox + " " + displayName + "  " + countStr

		if i == m.projectCycleCursor {
			lineWidth := lipgloss.Width(line)
			if lineWidth < innerWidth {
				line += strings.Repeat(" ", innerWidth-lineWidth)
			}
			line = SelectedLine(ansi.Truncate(line, innerWidth, ""), innerWidth)
		}

		contentLines = append(contentLines, line)
	}

	// Apply viewport scrolling if there are more items than fit
	maxVisible := m.cycleMaxVisibleItems()
	itemCount := len(m.projectCycleItems)
	if itemCount > maxVisible {
		// contentLines has 2 header lines (subtitle + blank), then item lines
		headerLines := contentLines[:2]
		itemLines := contentLines[2:]
		end := min(m.projectCycleScroll+maxVisible, len(itemLines))
		visibleItems := itemLines[m.projectCycleScroll:end]
		contentLines = append(headerLines, visibleItems...)
		// Add scroll indicators
		if m.projectCycleScroll > 0 {
			contentLines = append(contentLines, MutedStyle.Render(fmt.Sprintf("  ↑ %d more above", m.projectCycleScroll)))
		}
		below := itemCount - end
		if below > 0 {
			contentLines = append(contentLines, MutedStyle.Render(fmt.Sprintf("  ↓ %d more below", below)))
		}
	}

	content := strings.Join(contentLines, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		// Width includes the border and padding, so use the full popup width
		// to preserve the content area assumed by innerWidth above.
		Width(popupWidth)

	rendered := boxStyle.Render(content)

	lines := strings.Split(rendered, "\n")
	if len(lines) >= 2 {
		actualWidth := lipgloss.Width(lines[0])
		lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)
		footer := MutedStyle.Render("1-9 jump · space toggle · p pin · enter go · esc close")
		lines[len(lines)-1] = renderTitledBottomBorder(footer, actualWidth, borderStyle)
	}

	return strings.Join(lines, "\n")
}
