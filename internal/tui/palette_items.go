package tui

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// NewCommandPaletteModel creates a new command palette model.
// The currentView parameter determines which commands are shown (context-aware filtering).
// The currentProject parameter filters tasks to only show those from the specified project (nil = show all).
func NewCommandPaletteModel(cfg *config.Config, manager *config.Manager, registry *CommandRegistry, currentView ViewType, currentProject *models.Project) *CommandPaletteModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search · :cmd :task :prj :tag :sec :saved :sv :note :view · Alt+↑↓ history"
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

	// Load persisted recent items from config
	recentPaletteSelections = nil
	for _, item := range cfg.GetRecentPaletteItems() {
		recentPaletteSelections = append(recentPaletteSelections, recentSelection{
			Category: ItemCategory(item.Category),
			ID:       item.ID,
		})
	}

	// Build pinned commands set for quick lookup
	pinnedSet := make(map[string]bool)
	for _, name := range cfg.GetPinnedCommands() {
		pinnedSet[name] = true
	}

	m := &CommandPaletteModel{
		config:             cfg,
		configManager:      manager,
		registry:           registry,
		currentView:        currentView,
		currentProject:     currentProject,
		searchInput:        searchInput,
		cursor:             0,
		scrollAnchor:       -1,
		showTips:           true,
		categoryCursors:    make(map[ItemCategory]int),
		searchHistory:      cfg.GetPaletteSearchHistory(),
		searchHistoryIndex: -1, // Not browsing history
		pinnedCommands:     pinnedSet,
		selectedTasks:      make(map[string]bool),
	}

	// Pre-fill with last category filter if set
	if lastCat := cfg.GetLastPaletteCategory(); lastCat != "" {
		m.searchInput.SetValue(lastCat)
		m.showTips = false // Don't show tips when pre-filled
	}

	// Load all searchable items (lazy loading on palette open per design decision)
	m.allItems = m.loadAllItems()
	// Start with all items visible, sorted by category (no search filter, no highlights)
	m.filterItems()
	return m
}

// HasActiveSearch returns true (search input is always active in the palette).
func (m *CommandPaletteModel) HasActiveSearch() bool {
	return true
}

// HasTextInput returns true because the palette always has input focus.
func (m *CommandPaletteModel) HasTextInput() bool {
	return true
}

// IsFullHeight returns true because the command palette renders its own footer.
func (m *CommandPaletteModel) IsFullHeight() bool {
	return true
}

// IsActionMode returns true if the palette is showing inline actions for an item.
func (m *CommandPaletteModel) IsActionMode() bool {
	return m.actionMode
}

// ShouldCaptureEsc returns true when inline actions are shown.
func (m *CommandPaletteModel) ShouldCaptureEsc() bool {
	return m.IsActionMode()
}

// SetSize sets the view dimensions.
func (m *CommandPaletteModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate inner content width for split-pane overlay:
	// cardWidth = viewWidth - 4 (margin)
	// lipgloss Width = cardWidth - 2 (border adds 2 to total)
	// usable text area = Width - 4 (padding inside) = viewWidth - 10
	innerWidth := max(width-10, 60)

	m.searchInput.Width = innerWidth - 4

	// Calculate split pane widths: 65% list, 35% preview based on inner width
	m.listWidth = min(max(innerWidth*65/100, 35), innerWidth-25)
	m.previewWidth = innerWidth - m.listWidth - 1 // -1 for separator

	// Auto-enable compact mode on narrow terminals unless user explicitly toggled
	if !m.userSetCompact {
		m.compactMode = width < 100 || m.previewWidth < 25
	}
}

// loadAllItems aggregates commands, tasks, projects, tags, and sections into PaletteItems.
// This follows the lazy loading pattern from GlobalSearchModel, loading all searchable
// items when the palette opens rather than maintaining a persistent index.
// Commands are filtered by the current view context (only showing relevant commands).
func (m *CommandPaletteModel) loadAllItems() []PaletteItem {
	var items []PaletteItem

	// Load commands from registry (filtered by current view)
	for _, cmd := range m.registry.GetForView(m.currentView) {
		cmdCopy := cmd // Copy loop variable to avoid pointer aliasing
		items = append(items, PaletteItem{
			Name:        cmdCopy.Name,
			Description: cmdCopy.Description,
			Shortcut:    cmdCopy.Shortcut,
			Category:    ItemCategoryCommands,
			Command:     &cmdCopy,
		})
	}

	// Track tags and sections with task counts across all projects
	tagCounts := make(map[string]int)
	sectionCounts := make(map[string]int)

	// Load tasks, projects, tags, and sections from active projects
	// When currentProject is set, only load tasks from that project
	projects := m.config.GetActiveProjects()
	// Include sandbox if it's the current project (GetActiveProjects excludes it)
	if m.currentProject != nil && m.currentProject.Sandbox {
		projects = append(projects, m.currentProject)
	}
	for _, project := range projects {
		// Add project as searchable item (always include all projects for switching)
		items = append(items, PaletteItem{
			Name:        project.Name,
			Description: fmt.Sprintf("Project: %s", project.Slug),
			Category:    ItemCategoryProjects,
			Project:     project,
		})

		// Skip loading tasks from other projects when filtering by current project
		if m.currentProject != nil && project.ID != m.currentProject.ID {
			continue
		}

		// Load project tasks
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		// Add tasks and collect tags/sections
		// By default only show active tasks; when includeCompleted is true, show all
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			if !m.includeCompleted && !task.Status.IsActive() {
				continue
			}
			// Use simpler description when filtering by current project
			description := fmt.Sprintf("#%d in %s", task.ShortID, project.Slug)
			if m.currentProject != nil {
				description = fmt.Sprintf("#%d", task.ShortID)
			}
			items = append(items, PaletteItem{
				Name:        task.Title,
				Description: description,
				Category:    ItemCategoryTasks,
				Task:        task,
				Project:     project,
			})

			// Count tasks per tag
			for _, tag := range task.Tags {
				tagCounts[tag]++
			}

			// Count tasks per section
			if task.Section != "" {
				sectionCounts[task.Section]++
			}
		}
	}

	// Add unique tags as searchable items with task counts
	for tag, count := range tagCounts {
		var desc string
		if count == 1 {
			desc = "Tag (1 task)"
		} else {
			desc = fmt.Sprintf("Tag (%d tasks)", count)
		}
		items = append(items, PaletteItem{
			Name:        tag,
			Description: desc,
			Category:    ItemCategoryTags,
			Tag:         tag,
		})
	}

	// Add unique sections as searchable items with task counts
	for section, count := range sectionCounts {
		var desc string
		if count == 1 {
			desc = "Section (1 task)"
		} else {
			desc = fmt.Sprintf("Section (%d tasks)", count)
		}
		items = append(items, PaletteItem{
			Name:        section,
			Description: desc,
			Category:    ItemCategorySections,
			Section:     section,
		})
	}

	// Add saved searches from config
	savedSearches := m.config.GetSavedSearches()
	for name, query := range savedSearches {
		items = append(items, PaletteItem{
			Name:        name,
			Description: query,
			Category:    ItemCategorySavedSearch,
			SavedSearch: query,
		})
	}

	// Load notes from all projects
	for _, project := range projects {
		notes, err := storage.ListNotes(project)
		if err != nil {
			continue
		}
		for _, note := range notes {
			desc := project.Name
			if len(note.Tags) > 0 {
				desc += " · " + strings.Join(note.Tags, ", ")
			}
			items = append(items, PaletteItem{
				Name:        note.Title,
				Description: desc,
				Category:    ItemCategoryNotes,
				Note:        note,
				Project:     project,
			})
		}
	}

	// Add saved filters from config
	for i := range m.config.GetSavedFilters() {
		sf := m.config.GetSavedFilters()[i]
		desc := describeSavedFilterSummary(&sf)
		modeBadge := "[S] "
		if sf.Mode == "criteria" {
			modeBadge = "[C] "
		}
		items = append(items, PaletteItem{
			Name:        modeBadge + sf.Name,
			Description: desc,
			Category:    ItemCategorySavedFilter,
			SavedFilter: &sf,
		})
	}

	// Add navigable views as static items
	viewItems := []struct {
		name        string
		description string
		viewType    ViewType
	}{
		{"Daily Review", "Review and plan your day", ViewDailyReview},
		{"Weekly Review", "Strategic project health review", ViewWeeklyReview},
		{"What's Next", "Cross-project task suggestions and work queue", ViewFocusView},
		{"Due Date Overview", "Tasks organized by due date", ViewDueDateOverview},
		{"Priority View", "Tasks sorted by priority level", ViewPriorityView},
		{"Blockers", "View blocked and blocking tasks", ViewBlockers},
		{"All Tasks", "View tasks across all projects", ViewAllTasks},
		{"Global Blockers", "Blockers across all projects", ViewGlobalBlockers},
		{"Global Inbox", "Process unassigned tasks across projects", ViewGlobalInboxProcessor},
		{"Global Tag Manager", "Manage tags across all projects", ViewGlobalTagManager},
		{"Notes", "Project notes and documents", ViewNotes},
		{"Global Notes", "Notes not tied to any project", ViewGlobalNotes},
		{"Workflow Rules", "Manage workflow automation rules", ViewWorkflowRules},
		{"Settings", "Application preferences and configuration", ViewSettings},
		{"Undo History", "View and restore previous changes", ViewUndoHistory},
	}
	for _, v := range viewItems {
		items = append(items, PaletteItem{
			Name:        v.name,
			Description: v.description,
			Category:    ItemCategoryViews,
			ViewTarget:  v.viewType,
		})
	}

	return items
}

// scoreItemWithDetails scores an item and returns which field matched.
// Returns (score, namePositions, matchedField).
func (m *CommandPaletteModel) scoreItemWithDetails(item PaletteItem, query string) (int, []int, MatchField) {
	// Score against name (primary match target) with positions
	nameScore, namePositions := fuzzy.ScoreTermsWithPositions(item.Name, query)

	// Score against description (secondary match target)
	descScore := fuzzy.ScoreTerms(item.Description, query)

	// For commands, also score against aliases
	aliasScore := 0
	if item.Category == ItemCategoryCommands && item.Command != nil {
		for _, alias := range item.Command.Aliases {
			score := fuzzy.ScoreTerms(alias, query)
			if score > aliasScore {
				aliasScore = score
			}
		}
	}

	// Always preserve name positions when name matches (for highlighting)
	var positions []int
	if nameScore > 0 {
		positions = namePositions
	}

	// Use the best score between name, description, and aliases
	bestScore := nameScore
	matchedField := MatchFieldName
	if descScore > bestScore {
		bestScore = descScore
		matchedField = MatchFieldDescription
	} else if aliasScore > bestScore {
		bestScore = aliasScore
		matchedField = MatchFieldAlias
	} else if nameScore > 0 {
		matchedField = MatchFieldName
	}

	// If name matched, give it extra weight (similar to GlobalSearchModel's title weighting)
	if nameScore > 0 && nameScore >= descScore && nameScore >= aliasScore {
		bestScore = nameScore * 2
	}

	if bestScore == 0 {
		return 0, nil, MatchFieldNone
	}

	// Apply category weight multiplier
	score := bestScore * categoryWeight(item.Category)

	// Apply recent selection boost
	if m.isRecentSelection(item) {
		score *= recentBoostMultiplier
	}

	return score, positions, matchedField
}

// isRecentSelection checks if an item appears in the recent selections list.
func (m *CommandPaletteModel) isRecentSelection(item PaletteItem) bool {
	itemID := m.getItemID(item)
	for _, recent := range recentPaletteSelections {
		if recent.Category == item.Category && recent.ID == itemID {
			return true
		}
	}
	return false
}

// getItemID returns a unique identifier for a palette item.
func (m *CommandPaletteModel) getItemID(item PaletteItem) string {
	switch item.Category {
	case ItemCategoryCommands:
		if item.Command != nil {
			return item.Command.Name
		}
	case ItemCategoryTasks:
		if item.Task != nil {
			return item.Task.ID
		}
	case ItemCategoryProjects:
		if item.Project != nil {
			return item.Project.Slug
		}
	case ItemCategoryTags:
		return item.Tag
	case ItemCategorySections:
		return item.Section
	case ItemCategorySavedSearch:
		return item.Name // Use the saved search name as ID
	case ItemCategorySavedFilter:
		if item.SavedFilter != nil {
			return item.SavedFilter.ID
		}
		return item.Name
	case ItemCategoryNotes:
		if item.Note != nil {
			return item.Note.Filename
		}
	case ItemCategoryViews:
		return item.Name
	}
	return ""
}

// recordRecentSelection adds an item to the recent selections list.
// Most recent items are at the front. Duplicates are moved to the front.
func recordRecentSelection(item PaletteItem, itemID string) {
	// Remove existing entry if present
	for i, recent := range recentPaletteSelections {
		if recent.Category == item.Category && recent.ID == itemID {
			recentPaletteSelections = append(recentPaletteSelections[:i], recentPaletteSelections[i+1:]...)
			break
		}
	}

	// Prepend to front
	recentPaletteSelections = append([]recentSelection{{Category: item.Category, ID: itemID}}, recentPaletteSelections...)

	// Cap at maxRecentItems
	if len(recentPaletteSelections) > maxRecentItems {
		recentPaletteSelections = recentPaletteSelections[:maxRecentItems]
	}
}

// persistRecentSelections saves the current recent selections to config.
func (m *CommandPaletteModel) persistRecentSelections() {
	if m.configManager == nil {
		return
	}
	var configItems []config.RecentPaletteItem
	for _, r := range recentPaletteSelections {
		configItems = append(configItems, config.RecentPaletteItem{
			Category: string(r.Category),
			ID:       r.ID,
		})
	}
	m.config.SetRecentPaletteItems(configItems)
	_ = m.configManager.Save(m.config)
}

// recordSearchHistory adds the current search query to history if meaningful.
// Queries are deduplicated, with the most recent occurrence moved to the front.
// Category prefixes alone (e.g., ":task ") are not recorded.
func (m *CommandPaletteModel) recordSearchHistory() {
	if m.configManager == nil {
		return
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		return
	}

	// Don't record category prefixes alone
	_, searchQuery := m.parseQueryPrefix(query)
	if searchQuery == "" && strings.HasPrefix(query, ":") {
		return
	}

	// Don't record inline task creation queries
	if strings.HasPrefix(query, "+") {
		return
	}

	// Don't record task ID jumps
	if taskIDPattern.MatchString(query) {
		return
	}

	// Remove existing occurrence if present (to move it to front)
	newHistory := []string{query}
	for _, h := range m.searchHistory {
		if h != query {
			newHistory = append(newHistory, h)
		}
	}

	// Cap at 10 entries
	const maxHistorySize = 10
	if len(newHistory) > maxHistorySize {
		newHistory = newHistory[:maxHistorySize]
	}

	m.searchHistory = newHistory
	m.config.SetPaletteSearchHistory(newHistory)
	_ = m.configManager.Save(m.config)
}

// parseInlineTask extracts metadata from inline task creation input.
// Syntax: "Task title !priority #tag @due"
// Similar to quick capture but adapted for command palette.
func (m *CommandPaletteModel) parseInlineTask(input string) *inlineTaskPreview {
	result := &inlineTaskPreview{
		priority: 3, // default priority
	}

	// Extract priority: !1 through !5
	priorityRe := regexp.MustCompile(`\s*!([1-5])\s*`)
	if match := priorityRe.FindStringSubmatch(input); len(match) > 1 {
		result.priority, _ = strconv.Atoi(match[1])
		input = priorityRe.ReplaceAllString(input, " ")
	}

	// Extract tags: #word
	tagRe := regexp.MustCompile(`\s*#(\w+)\s*`)
	tagMatches := tagRe.FindAllStringSubmatch(input, -1)
	for _, match := range tagMatches {
		if len(match) > 1 {
			result.tags = append(result.tags, match[1])
		}
	}
	input = tagRe.ReplaceAllString(input, " ")

	// Extract due date: @today, @tomorrow, @YYYY-MM-DD
	dueDateRe := regexp.MustCompile(`\s*@(\S+)\s*`)
	if match := dueDateRe.FindStringSubmatch(input); len(match) > 1 {
		dateStr := match[1]
		now := time.Now()
		var dueDate time.Time

		switch strings.ToLower(dateStr) {
		case "today":
			dueDate = util.StartOfDay(now)
		case "tomorrow":
			dueDate = util.StartOfDay(now).AddDate(0, 0, 1)
		default:
			// Try parsing as date
			if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
				dueDate = parsed
			}
		}

		if !dueDate.IsZero() {
			result.dueDate = &dueDate
		}
		input = dueDateRe.ReplaceAllString(input, " ")
	}

	// Remaining text is the title
	result.title = strings.TrimSpace(input)

	return result
}

// createInlineTask creates a task from the inline task preview and closes the palette.
func (m *CommandPaletteModel) createInlineTask() (tea.Model, tea.Cmd) {
	if m.inlineTaskParsed == nil || m.inlineTaskParsed.title == "" {
		return m, nil
	}

	// Find current project to create task in
	var targetProject *models.Project
	projects := m.config.GetActiveProjects()
	if len(projects) > 0 {
		// Use current project if set, otherwise first active project
		if current := m.config.GetCurrentProject(); current != nil {
			targetProject = current
		} else {
			targetProject = projects[0]
		}
	}

	if targetProject == nil {
		return m, nil // No project available
	}

	// Load project if needed
	if err := storage.LoadProject(targetProject); err != nil {
		return m, nil
	}

	parsed := m.inlineTaskParsed

	return m, func() tea.Msg {
		return CommandPaletteExecuteMsg{
			ResultMsg: InlineTaskCreateMsg{
				Project:  targetProject,
				Title:    parsed.title,
				Priority: parsed.priority,
				Tags:     parsed.tags,
				DueDate:  parsed.dueDate,
			},
		}
	}
}

// findProjectWithTag returns the first project that contains the specified tag.
func (m *CommandPaletteModel) findProjectWithTag(tag string) *models.Project {
	for _, item := range m.allItems {
		if item.Category == ItemCategoryTasks && item.Task != nil {
			if slices.Contains(item.Task.Tags, tag) {
				return item.Project
			}
		}
	}
	return nil
}

// findProjectWithSection returns the first project that contains the specified section.
func (m *CommandPaletteModel) findProjectWithSection(section string) *models.Project {
	for _, item := range m.allItems {
		if item.Category == ItemCategoryTasks && item.Task != nil && item.Task.Section == section {
			return item.Project
		}
	}
	return nil
}

// executeItem handles selection of an item from the palette.
// For commands, it invokes the handler directly and returns its message.
// For tasks, it emits GlobalSearchSelectMsg to navigate to the task.
// For projects, it emits SelectProjectMsg to switch to that project.
// For tags and sections, it emits navigation messages with the first associated project.
// All item types use CommandPaletteExecuteMsg to ensure the palette closes and action
// executes in the same update cycle.
// The selected item is recorded for recent items boosting.
func (m *CommandPaletteModel) executeItem(item PaletteItem) (tea.Model, tea.Cmd) {
	// Record the selection for recent items boosting and persist to config
	itemID := m.getItemID(item)
	if itemID != "" {
		recordRecentSelection(item, itemID)
		m.persistRecentSelections()
	}

	// Record search query in history if it's a meaningful search
	m.recordSearchHistory()

	switch item.Category {
	case ItemCategoryCommands:
		if item.Command != nil && item.Command.Handler != nil {
			// Record command execution in history
			m.config.AddCommandToHistory(item.Command.Name)
			if m.configManager != nil {
				_ = m.configManager.Save(m.config)
			}

			handler := item.Command.Handler
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: handler()}
			}
		}
	case ItemCategoryTasks:
		if item.Task != nil && item.Project != nil {
			task := item.Task
			project := item.Project
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: GlobalSearchSelectMsg{Task: task, Project: project}}
			}
		}
	case ItemCategoryProjects:
		if item.Project != nil {
			project := item.Project
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: SelectProjectMsg{Project: project}}
			}
		}
	case ItemCategoryTags:
		if item.Tag != "" {
			tag := item.Tag
			project := m.findProjectWithTag(tag)
			if project != nil {
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: GlobalTagNavigateMsg{Project: project, Tag: tag}}
				}
			}
		}
	case ItemCategorySections:
		if item.Section != "" {
			section := item.Section
			project := m.findProjectWithSection(section)
			if project != nil {
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteSectionSelectMsg{Section: section, Project: project}}
				}
			}
		}
	case ItemCategorySavedSearch:
		if item.SavedSearch != "" {
			// Execute the saved search by setting the search input and re-filtering
			query := item.SavedSearch
			m.searchInput.SetValue(query)
			m.filterItems()
			// Don't close the palette - let user see the results
			return m, nil
		}
	case ItemCategorySavedFilter:
		if item.SavedFilter != nil {
			sf := *item.SavedFilter
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: LoadSavedFilterMsg{Filter: sf}}
			}
		}
	case ItemCategoryNotes:
		if item.Project != nil {
			project := item.Project
			return m, func() tea.Msg {
				return CommandPaletteExecuteMsg{ResultMsg: ShowNotesMsg{Project: project}}
			}
		}
	case ItemCategoryViews:
		viewType := item.ViewTarget
		return m, func() tea.Msg {
			return CommandPaletteExecuteMsg{ResultMsg: NavigateToViewMsg{ViewType: viewType}}
		}
	}
	return m, nil
}
