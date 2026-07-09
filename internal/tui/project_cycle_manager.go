package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
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
	m.projectCycleAllItems = buildProjectInfoListFiltered(m.config, m.config.FilterContext, m.config.FilterCustomContext)
	m.sortCycleItemsSelectedFirst()
}

// sortCycleItemsSelectedFirst sorts projectCycleItems so that projects
// in the cycle appear first, preserving existing order within each group.
func (m *AppModel) sortCycleItemsSelectedFirst() {
	if len(m.projectCycleAllItems) == 0 && len(m.projectCycleItems) > 0 {
		m.projectCycleAllItems = m.projectCycleItems
	}
	contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
	cycleIDs := m.config.GetCycleProjectIDsForContext(contextKey)
	cycleIDSet := make(map[string]bool, len(cycleIDs))
	for _, id := range cycleIDs {
		cycleIDSet[id] = true
	}
	sort.SliceStable(m.projectCycleAllItems, func(i, j int) bool {
		iSel := projectCycleItemSelected(m.projectCycleAllItems[i], cycleIDSet)
		jSel := projectCycleItemSelected(m.projectCycleAllItems[j], cycleIDSet)
		if iSel != jSel {
			return iSel
		}
		return false
	})
	m.applyProjectCycleFilter(false)
}

func projectCycleItemSelected(item ProjectInfo, cycleIDSet map[string]bool) bool {
	return item.Project != nil && cycleIDSet[item.Project.ID]
}

func (m *AppModel) ensureProjectCycleSearchInput() {
	if m.projectCycleSearch.searchInput.Placeholder == "" {
		m.projectCycleSearch.InitSearchInput("Filter projects...", 100, SearchInputWidth(m.width))
	}
	m.projectCycleSearch.searchInput.SetWidth(SearchInputWidth(m.width))
}

func (m *AppModel) resetProjectCycleSearch() {
	m.ensureProjectCycleSearchInput()
	m.projectCycleSearch.ClearSearch()
}

func (m *AppModel) projectCycleSearchQuery() string {
	return strings.TrimSpace(m.projectCycleSearch.SearchQuery())
}

func (m *AppModel) applyProjectCycleFilter(resetCursor bool) {
	query := m.projectCycleSearchQuery()
	if query == "" {
		m.projectCycleItems = m.projectCycleAllItems
	} else {
		filtered := make([]ProjectInfo, 0, len(m.projectCycleAllItems))
		for _, item := range m.projectCycleAllItems {
			if item.Project != nil && fuzzy.MatchTerms(item.FilterText(), query) {
				filtered = append(filtered, item)
			}
		}
		m.projectCycleItems = filtered
	}
	if resetCursor {
		m.projectCycleCursor = 0
		m.projectCycleScroll = 0
		m.invalidateCyclePreview()
		return
	}
	m.clampProjectCycleCursor()
}

func (m *AppModel) clampProjectCycleCursor() {
	if len(m.projectCycleItems) == 0 {
		m.projectCycleCursor = 0
		m.projectCycleScroll = 0
		m.invalidateCyclePreview()
		return
	}
	if m.projectCycleCursor < 0 {
		m.projectCycleCursor = 0
	}
	if m.projectCycleCursor >= len(m.projectCycleItems) {
		m.projectCycleCursor = len(m.projectCycleItems) - 1
	}
	m.adjustCycleScroll()
}

func (m *AppModel) restoreProjectCycleCursor(projectID string) {
	for i, item := range m.projectCycleItems {
		if item.Project != nil && item.Project.ID == projectID {
			m.projectCycleCursor = i
			m.adjustCycleScroll()
			return
		}
	}
	m.clampProjectCycleCursor()
}

// closeProjectCyclePopup closes the popup and does cleanup.
func (m *AppModel) closeProjectCyclePopup() tea.Cmd {
	m.showProjectCycle = false
	m.projectCycleScroll = 0
	m.resetProjectCycleSearch()
	m.projectCycleAllItems = nil
	m.projectCycleItems = nil
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
	if m.crossOutliner == nil {
		return nil
	}
	return m.crossOutliner.reloadCrossProjects()
}

// handleProjectCycleKey handles key events when the project cycle popup is open.
func (m *AppModel) handleProjectCycleKey(msg tea.KeyPressMsg) (*AppModel, tea.Cmd) { //nolint:gocyclo
	if isModifierOnly(msg) {
		return m, nil
	}
	if _, cmd, handled := m.projectCycleSearch.HandleSearchMode(msg, m, func() {
		m.applyProjectCycleFilter(true)
	}); handled {
		return m, cmd
	}
	switch msg.String() {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		if idx < len(m.projectCycleItems) && m.projectCycleItems[idx].Project != nil {
			project := m.projectCycleItems[idx].Project
			reloadCmd := m.closeProjectCyclePopup()
			_, cmd := m.switchToCycleProject(project)
			return m, tea.Batch(reloadCmd, cmd)
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
		if m.projectCycleCursor < len(m.projectCycleItems) && m.projectCycleItems[m.projectCycleCursor].Project != nil {
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
		if m.projectCycleCursor < len(m.projectCycleItems) && m.projectCycleItems[m.projectCycleCursor].Project != nil {
			project := m.projectCycleItems[m.projectCycleCursor].Project
			reloadCmd := m.closeProjectCyclePopup()
			return m, tea.Batch(reloadCmd, func() tea.Msg {
				return SelectProjectMsg{Project: project}
			})
		}
		return m, nil

	case "p":
		// Toggle pin on highlighted project
		if m.projectCycleCursor < len(m.projectCycleItems) && m.projectCycleItems[m.projectCycleCursor].Project != nil {
			item := m.projectCycleItems[m.projectCycleCursor]
			item.Project.TogglePinned()
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
			currentID := item.Project.ID
			m.refreshProjectCycleItems()
			m.restoreProjectCycleCursor(currentID)
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
			m.resetProjectCycleSearch()
			m.refreshProjectCycleItems()
			m.projectCycleCursor = 0
			m.projectCycleScroll = 0
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

	case "/":
		m.ensureProjectCycleSearchInput()
		return m, m.projectCycleSearch.ActivateSearch()

	case "esc", "@":
		if m.projectCycleSearchQuery() != "" {
			m.projectCycleSearch.ClearSearch()
			m.applyProjectCycleFilter(true)
			return m, nil
		}
		reloadCmd := m.closeProjectCyclePopup()
		return m, reloadCmd

	default:
		// Close popup and let the key pass through
		reloadCmd := m.closeProjectCyclePopup()
		return m, reloadCmd
	}
}

// cycleMaxVisibleItems returns the maximum number of project items visible
// in the cycle popup viewport, based on terminal height.
func (m *AppModel) cycleMaxVisibleItems() int {
	maxFromHeight := m.height - OverlayListChrome
	maxFromPercent := (m.height * 50) / 100
	return max(min(maxFromHeight, maxFromPercent)-3, 3)
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
func (m *AppModel) renderProjectCyclePopup() string { //nolint:gocyclo // renderer combines layout, search, and selected/hidden project states.
	if !m.showProjectCycle {
		return ""
	}
	contextKey := getContextKey(m.config.FilterContext, m.config.FilterCustomContext)
	hiddenCount := m.config.GetArchivedCycleProjectCountForContext(contextKey)
	if len(m.projectCycleItems) == 0 && len(m.projectCycleAllItems) == 0 && hiddenCount == 0 {
		return ""
	}

	m.ensureProjectCycleSearchInput()
	borderStyle := InfoStatusStyle
	titleStyle := TitleStyle
	title := titleStyle.Render("Project Cycle")

	badgeStyle := InfoStatusStyle
	dimBadgeStyle := MutedStyle

	// Build subtitle: context indicator + selection count
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
	if hiddenCount > 0 {
		subtitleParts = append(subtitleParts, cycleArchivedHiddenText(hiddenCount))
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

	popupWidth := projectCycleSplitWidth(m.width)
	footerText := m.projectCycleFooterText(popupWidth)

	innerWidth := max(popupWidth-4, 1)

	var contentLines []string
	contentLines = append(contentLines, subtitle)
	headerLineCount := 2
	searchQuery := m.projectCycleSearchQuery()
	if m.projectCycleSearch.IsSearchMode() {
		contentLines = append(contentLines, HelpStyle.Render("Search: ")+m.projectCycleSearch.SearchInputView())
		headerLineCount++
	} else if searchQuery != "" {
		contentLines = append(contentLines, MutedStyle.Render(fmt.Sprintf("Search: %q ", searchQuery))+HelpStyle.Render("(Esc clear)"))
		headerLineCount++
	}
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

	if len(m.projectCycleItems) == 0 {
		if hiddenCount > 0 && searchQuery == "" {
			contentLines = append(contentLines, MutedStyle.Render("No active projects in this cycle"))
			contentLines = append(contentLines, MutedStyle.Render("Unarchive from Dashboard archived mode or update cycle selections."))
		} else {
			contentLines = append(contentLines, MutedStyle.Render("No matching projects"))
		}
	} else {
		for i, item := range m.projectCycleItems {
			if item.Project == nil {
				continue
			}
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
				line = SelectedLine(truncateIntentional(line, innerWidth, ""), innerWidth)
			}

			contentLines = append(contentLines, line)
		}
	}

	// Apply viewport scrolling if there are more items than fit
	maxVisible := m.cycleMaxVisibleItems()
	itemCount := len(m.projectCycleItems)
	if itemCount > maxVisible {
		headerLines := contentLines[:headerLineCount]
		itemLines := contentLines[headerLineCount:]
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

	footer := MutedStyle.Render(footerText)
	return renderTitledBox(content, popupWidth, title, footer, borderStyle)
}

func projectCycleSplitWidth(totalWidth int) int {
	treeWidth := max((totalWidth*55)/100, TreeMinWidth)
	const dividerWidth = 1
	sidebarWidth := totalWidth - treeWidth - dividerWidth
	if sidebarWidth < SidebarMinWidth {
		treeWidth = totalWidth - SidebarMinWidth - dividerWidth
	}
	return max(min(treeWidth, totalWidth), 1)
}

func (m *AppModel) projectCycleFooterText(popupWidth int) string {
	footerLimit := max(popupWidth-6, 1)
	if m.projectCycleSearch.IsSearchMode() {
		return projectCycleFooterVariant(footerLimit,
			"enter validate · esc clear",
			"enter · esc clear",
			"enter · esc",
		)
	}
	if m.projectCycleSearchQuery() != "" {
		return projectCycleFooterVariant(footerLimit,
			"j/k nav · / edit · enter go · esc clear",
			"j/k · / edit · enter · esc",
			"/ edit · enter · esc",
		)
	}
	return projectCycleFooterVariant(footerLimit,
		"1-9 jump · / filter · space · p pin · enter · esc",
		"1-9 · / filter · space · enter · esc",
		"1-9 · / filter · space · esc",
	)
}

func projectCycleFooterVariant(limit int, variants ...string) string {
	if len(variants) == 0 {
		return ""
	}
	for _, variant := range variants {
		if lipgloss.Width(variant) <= limit {
			return variant
		}
	}
	return variants[len(variants)-1]
}
