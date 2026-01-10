package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// ItemCategory represents the type of searchable entity in the command palette.
// This is distinct from CommandCategory, which organizes commands by their function.
// ItemCategory identifies what kind of item is being searched (command vs task vs project, etc).
type ItemCategory string

const (
	ItemCategoryCommands ItemCategory = "Commands"
	ItemCategoryTasks    ItemCategory = "Tasks"
	ItemCategoryProjects ItemCategory = "Projects"
	ItemCategoryTags     ItemCategory = "Tags"
	ItemCategorySections ItemCategory = "Sections"
)

// categoryPrefixes maps short prefixes to item categories for filtering.
// Users can type ":cmd " to filter to commands only, ":task " for tasks, etc.
var categoryPrefixes = map[string]ItemCategory{
	"cmd":  ItemCategoryCommands,
	"task": ItemCategoryTasks,
	"prj":  ItemCategoryProjects,
	"tag":  ItemCategoryTags,
	"sec":  ItemCategorySections,
}

// categoryOrder defines the display order for categories when showing headers.
var categoryOrder = []ItemCategory{
	ItemCategoryCommands,
	ItemCategoryProjects,
	ItemCategoryTasks,
	ItemCategoryTags,
	ItemCategorySections,
}

// recentSelection tracks a recently selected item for boosting in search results.
type recentSelection struct {
	Category ItemCategory
	ID       string // command name, task ID, project slug, tag, or section name
}

// recentPaletteSelections stores recently selected items for the current session.
// Items at the front of the slice are most recent. Capped at maxRecentItems.
var recentPaletteSelections []recentSelection

const maxRecentItems = 10

// recentBoostMultiplier is applied to items that appear in the recent selections.
const recentBoostMultiplier = 2

// PaletteItem represents a unified search result in the command palette.
// It provides a common interface for displaying and selecting items regardless
// of their underlying type (command, task, project, tag, or section).
type PaletteItem struct {
	Name        string       // Display name shown in the palette
	Description string       // Brief description or context
	Shortcut    string       // Keyboard shortcut if applicable (mainly for commands)
	Category    ItemCategory // The type of item (Commands, Tasks, Projects, Tags, Sections)

	// Underlying data - only one of these will be set based on Category
	Command *Command        // Set when Category is CategoryCommands
	Task    *models.Task    // Set when Category is CategoryTasks
	Project *models.Project // Set when Category is CategoryProjects
	Tag     string          // Set when Category is CategoryTags
	Section string          // Set when Category is CategorySections
}

// CommandPaletteItem represents a searchable item in the command palette.
// This wraps Command to allow future extension with other searchable types
// (tasks, projects, tags, sections) as per the requirement.
// Deprecated: Use PaletteItem instead for unified search result representation.
type CommandPaletteItem struct {
	Command Command
}

// PaletteAction represents an inline action that can be performed on a palette item.
// When a user presses Tab on a non-command item, a submenu of available actions is shown.
type PaletteAction struct {
	Name    string         // Display name (e.g., "Complete", "Edit")
	Icon    string         // Visual indicator
	KeyHint string         // Keyboard shortcut hint (e.g., "m" for complete)
	Handler func() tea.Msg // Returns message when the action is selected
}

// filteredPaletteItem wraps a PaletteItem with its match positions for highlighting.
type filteredPaletteItem struct {
	Item          PaletteItem
	NamePositions []int // Positions of matched characters in Name
}

// CommandPaletteModel provides a fullscreen fuzzy search palette for commands.
// It implements the View interface and follows the existing view stack pattern
// used by GlobalSearchModel and HelpViewModel.
type CommandPaletteModel struct {
	config        *config.Config
	configManager *config.Manager
	registry      *CommandRegistry

	// Current view context for filtering commands
	currentView ViewType

	// Input for filtering
	searchInput textinput.Model

	// Items - using PaletteItem for unified representation of all searchable types
	allItems      []PaletteItem
	filteredItems []filteredPaletteItem

	// Navigation state
	cursor       int
	scrollAnchor int // When >= 0, forces scroll to position this item at top of view, then resets to -1

	// Action mode state - when true, showing inline actions for selected item
	actionMode       bool
	actionCursor     int
	availableActions []PaletteAction
	actionItem       PaletteItem // The item whose actions are being shown

	// Dimensions
	width        int
	height       int
	listWidth    int
	previewWidth int

	// Onboarding state
	showTips bool // Show tips until first keystroke
}

// NewCommandPaletteModel creates a new command palette model.
// The currentView parameter determines which commands are shown (context-aware filtering).
func NewCommandPaletteModel(cfg *config.Config, manager *config.Manager, registry *CommandRegistry, currentView ViewType) *CommandPaletteModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search (use -term to exclude) :cmd :task :prj :tag :sec"
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

	m := &CommandPaletteModel{
		config:        cfg,
		configManager: manager,
		registry:      registry,
		currentView:   currentView,
		searchInput:   searchInput,
		cursor:        0,
		scrollAnchor:  -1,
		showTips:      true,
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

// SetSize sets the view dimensions.
func (m *CommandPaletteModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.searchInput.Width = width - 4

	// Calculate split pane widths: 55% list, 45% preview
	m.listWidth = width * 55 / 100
	if m.listWidth < 35 {
		m.listWidth = 35
	}
	if m.listWidth > width-25 {
		m.listWidth = width - 25
	}
	m.previewWidth = width - m.listWidth - 1 // -1 for border
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
	projects := m.config.GetActiveProjects()
	for _, project := range projects {
		// Add project as searchable item
		items = append(items, PaletteItem{
			Name:        project.Name,
			Description: fmt.Sprintf("Project: %s", project.Slug),
			Category:    ItemCategoryProjects,
			Project:     project,
		})

		// Load project tasks
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		// Add tasks and collect tags/sections (only active tasks)
		for _, task := range project.Tasks {
			if !task.Status.IsActive() {
				continue
			}
			items = append(items, PaletteItem{
				Name:        task.Title,
				Description: fmt.Sprintf("#%d in %s", task.ShortID, project.Slug),
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
		desc := "Tag"
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
		desc := "Section"
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

	return items
}

// Category weight multipliers for fuzzy scoring.
// Commands rank highest for short queries, projects next, others equal.
const (
	weightCommands = 3
	weightProjects = 2
	weightDefault  = 1
)

// categoryWeight returns the scoring multiplier for an ItemCategory.
func categoryWeight(cat ItemCategory) int {
	switch cat {
	case ItemCategoryCommands:
		return weightCommands
	case ItemCategoryProjects:
		return weightProjects
	default:
		return weightDefault
	}
}

// parseQueryPrefix extracts a category filter prefix from the query.
// Returns the category to filter by (or empty string for no filter) and the remaining search query.
// Prefix format: ":cmd ", ":task ", ":prj ", ":tag ", ":sec " (note the trailing space).
func (m *CommandPaletteModel) parseQueryPrefix(query string) (ItemCategory, string) {
	if !strings.HasPrefix(query, ":") {
		return "", query
	}

	// Find the first space after the prefix
	spaceIdx := strings.Index(query, " ")
	if spaceIdx == -1 {
		// No space yet - check if it's a partial prefix (user still typing)
		prefix := strings.ToLower(query[1:])
		for p := range categoryPrefixes {
			if strings.HasPrefix(p, prefix) {
				// Partial match - filter to this category but no search query yet
				return categoryPrefixes[p], ""
			}
		}
		return "", query
	}

	// Extract the prefix (without the colon)
	prefix := strings.ToLower(query[1:spaceIdx])
	if cat, ok := categoryPrefixes[prefix]; ok {
		return cat, strings.TrimSpace(query[spaceIdx+1:])
	}

	// Unknown prefix - treat as regular search
	return "", query
}

// sortItemsByCategory sorts items by the predefined category order for display without search.
func (m *CommandPaletteModel) sortItemsByCategory(items []PaletteItem) []PaletteItem {
	// Create a map for category order lookup
	orderMap := make(map[ItemCategory]int)
	for i, cat := range categoryOrder {
		orderMap[cat] = i
	}

	sorted := make([]PaletteItem, len(items))
	copy(sorted, items)

	sort.SliceStable(sorted, func(i, j int) bool {
		return orderMap[sorted[i].Category] < orderMap[sorted[j].Category]
	})

	return sorted
}

// parseSearchTerms splits a query into positive and negative terms.
// Terms prefixed with "-" are negative (exclude items matching them).
// Returns (positiveTerms, negativeTerms).
func parseSearchTerms(query string) ([]string, []string) {
	terms := strings.Fields(query)
	var positive, negative []string

	for _, term := range terms {
		if strings.HasPrefix(term, "-") && len(term) > 1 {
			// Negative term - remove the "-" prefix
			negative = append(negative, term[1:])
		} else if term != "-" {
			// Positive term (ignore standalone "-")
			positive = append(positive, term)
		}
	}

	return positive, negative
}

// itemMatchesNegativeTerms returns true if the item matches any negative term.
func (m *CommandPaletteModel) itemMatchesNegativeTerms(item PaletteItem, negativeTerms []string) bool {
	for _, term := range negativeTerms {
		// Check if term matches name or description
		if fuzzy.Match(item.Name, term) || fuzzy.Match(item.Description, term) {
			return true
		}
	}
	return false
}

// filterItems filters items based on the current search query using category-weighted fuzzy scoring.
// Supports category prefix filtering (e.g., ":cmd help" filters to commands matching "help").
// Supports negation with "-" prefix (e.g., "task -bug" excludes items matching "bug").
func (m *CommandPaletteModel) filterItems() {
	query := strings.TrimSpace(m.searchInput.Value())

	// Parse category prefix if present
	categoryFilter, searchQuery := m.parseQueryPrefix(query)

	// Determine which items to search
	itemsToSearch := m.allItems
	if categoryFilter != "" {
		// Filter to only the specified category
		var filtered []PaletteItem
		for _, item := range m.allItems {
			if item.Category == categoryFilter {
				filtered = append(filtered, item)
			}
		}
		itemsToSearch = filtered
	}

	// Empty search query shows all (filtered) items sorted by category (no highlighting)
	if searchQuery == "" {
		sorted := m.sortItemsByCategory(itemsToSearch)
		m.filteredItems = make([]filteredPaletteItem, len(sorted))
		for i, item := range sorted {
			m.filteredItems[i] = filteredPaletteItem{Item: item}
		}
		m.cursor = 0
		return
	}

	// Parse positive and negative terms
	positiveTerms, negativeTerms := parseSearchTerms(searchQuery)

	// Build the positive query string for scoring
	positiveQuery := strings.Join(positiveTerms, " ")

	// Score each item and collect matches with positions
	type scoredItem struct {
		item          PaletteItem
		score         int
		namePositions []int
	}
	var scored []scoredItem

	for _, item := range itemsToSearch {
		// Skip items that match any negative term
		if len(negativeTerms) > 0 && m.itemMatchesNegativeTerms(item, negativeTerms) {
			continue
		}

		// If only negative terms, include all non-excluded items
		if positiveQuery == "" {
			scored = append(scored, scoredItem{item: item, score: 1})
			continue
		}

		// Score against positive terms
		score, positions := m.scoreItemWithPositions(item, positiveQuery)
		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score, namePositions: positions})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results with positions
	m.filteredItems = make([]filteredPaletteItem, len(scored))
	for i, s := range scored {
		m.filteredItems[i] = filteredPaletteItem{
			Item:          s.item,
			NamePositions: s.namePositions,
		}
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filteredItems) {
		m.cursor = len(m.filteredItems) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// scoreItem returns a category-weighted fuzzy match score for an item against the query.
// Returns 0 if no match. Higher scores indicate better matches.
// Commands receive 3x weight, projects 2x, tasks/tags/sections 1x.
// Recently selected items receive an additional boost.
func (m *CommandPaletteModel) scoreItem(item PaletteItem, query string) int {
	score, _ := m.scoreItemWithPositions(item, query)
	return score
}

// scoreItemWithPositions returns a category-weighted fuzzy match score and match positions.
// Returns 0 and nil if no match. Higher scores indicate better matches.
// Positions are for the name field only (primary display target).
// For commands, also scores against aliases for better discoverability.
func (m *CommandPaletteModel) scoreItemWithPositions(item PaletteItem, query string) (int, []int) {
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

	// Use the best score between name, description, and aliases
	bestScore := nameScore
	var positions []int
	if descScore > bestScore {
		bestScore = descScore
		positions = nil // Description matched better, don't highlight name
	} else if aliasScore > bestScore {
		bestScore = aliasScore
		positions = nil // Alias matched better, don't highlight name
	} else if nameScore > 0 {
		positions = namePositions
	}

	// If name matched, give it extra weight (similar to GlobalSearchModel's title weighting)
	if nameScore > 0 && nameScore >= descScore && nameScore >= aliasScore {
		bestScore = nameScore * 2
	}

	if bestScore == 0 {
		return 0, nil
	}

	// Apply category weight multiplier
	score := bestScore * categoryWeight(item.Category)

	// Apply recent selection boost
	if m.isRecentSelection(item) {
		score *= recentBoostMultiplier
	}

	return score, positions
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
	m.configManager.Save(m.config)
}

// Init implements tea.Model.
func (m *CommandPaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m *CommandPaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Handle action mode separately
		if m.actionMode {
			return m.updateActionMode(msg)
		}
		return m.updateSearchMode(msg)
	}
	return m, nil
}

// updateSearchMode handles key events in search mode (the default mode).
func (m *CommandPaletteModel) updateSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, func() tea.Msg {
			return CloseCommandPaletteMsg{}
		}
	case "enter":
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			return m.executeItem(m.filteredItems[m.cursor].Item)
		}
		return m, nil
	case "tab":
		// Enter action mode for non-command items
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category != ItemCategoryCommands {
				actions := m.getActionsForItem(item)
				if len(actions) > 0 {
					m.actionMode = true
					m.actionCursor = 0
					m.availableActions = actions
					m.actionItem = item
					return m, nil
				}
			}
		}
		return m, nil
	case "up", "ctrl+k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.cursor < len(m.filteredItems)-1 {
			m.cursor++
		}
		return m, nil
	case "ctrl+u":
		m.cursor -= 10
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil
	case "ctrl+d":
		m.cursor += 10
		if m.cursor >= len(m.filteredItems) {
			m.cursor = len(m.filteredItems) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil
	case "ctrl+n":
		// Jump to next section and scroll to show it at top
		nextIdx := m.findNextSectionStart()
		if nextIdx >= 0 {
			m.cursor = nextIdx
			m.scrollAnchor = nextIdx
		}
		return m, nil
	case "ctrl+x":
		// Quick complete task (ctrl+m is same as Enter in terminals)
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category == ItemCategoryTasks && item.Task != nil && item.Project != nil {
				task := item.Task
				project := item.Project
				if task.Status != models.StatusDone && task.Status != models.StatusCancelled {
					return m, func() tea.Msg {
						return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteTaskActionMsg{
							Task: task, Project: project, Action: "complete",
						}}
					}
				}
			}
		}
		return m, nil
	case "ctrl+e":
		// Quick edit task
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category == ItemCategoryTasks && item.Task != nil {
				task := item.Task
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: EditTaskMsg{Task: task}}
				}
			}
		}
		return m, nil
	case "ctrl+g":
		// Quick view in glow (ctrl+v conflicts with paste)
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category == ItemCategoryTasks && item.Task != nil {
				task := item.Task
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: GlowViewMsg{Task: task}}
				}
			}
		}
		return m, nil
	case "ctrl+s":
		// Quick start task
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor].Item
			if item.Category == ItemCategoryTasks && item.Task != nil && item.Project != nil {
				task := item.Task
				project := item.Project
				if task.Status == models.StatusTodo {
					return m, func() tea.Msg {
						return CommandPaletteExecuteMsg{ResultMsg: CommandPaletteTaskActionMsg{
							Task: task, Project: project, Action: "start",
						}}
					}
				}
			}
		}
		return m, nil
	default:
		// Handle number keys for direct jump when enabled
		key := msg.String()
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' && m.isNumberJumpEnabled() {
			idx := int(key[0] - '1') // "1" -> 0, "9" -> 8
			if idx < len(m.filteredItems) {
				return m.executeItem(m.filteredItems[idx].Item)
			}
			return m, nil
		}
		// Update search input and filter items
		m.showTips = false // Hide tips on first input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	}
}

// isNumberJumpEnabled returns true if number keys should trigger direct selection.
// This is enabled when the search input is empty or contains only a category prefix.
func (m *CommandPaletteModel) isNumberJumpEnabled() bool {
	query := strings.TrimSpace(m.searchInput.Value())
	if query == "" {
		return true
	}
	// Check if query is just a category prefix (e.g., ":cmd", ":task ")
	_, searchQuery := m.parseQueryPrefix(query)
	return searchQuery == ""
}

// findNextSectionStart returns the index of the first item in the next category.
// Categories cycle in the order: Commands → Projects → Tasks → Tags → Sections → Commands.
// Returns -1 if there are no items or only one category exists.
func (m *CommandPaletteModel) findNextSectionStart() int {
	if len(m.filteredItems) == 0 {
		return -1
	}

	currentCategory := m.filteredItems[m.cursor].Item.Category

	// Find current category's position in the order
	currentIdx := -1
	for i, cat := range categoryOrder {
		if cat == currentCategory {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		return -1
	}

	// Try each subsequent category in order (wrapping around)
	for offset := 1; offset <= len(categoryOrder); offset++ {
		nextCatIdx := (currentIdx + offset) % len(categoryOrder)
		targetCategory := categoryOrder[nextCatIdx]

		// Find first item with this category
		for i, filtered := range m.filteredItems {
			if filtered.Item.Category == targetCategory {
				return i
			}
		}
	}

	return -1
}

// updateActionMode handles key events when showing inline actions for an item.
func (m *CommandPaletteModel) updateActionMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "tab":
		// Exit action mode, return to search
		m.actionMode = false
		m.availableActions = nil
		return m, nil
	case "enter":
		// Execute the selected action
		if m.actionCursor < len(m.availableActions) {
			action := m.availableActions[m.actionCursor]
			if action.Handler != nil {
				handler := action.Handler
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: handler()}
				}
			}
		}
		return m, nil
	case "up", "ctrl+k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.actionCursor < len(m.availableActions)-1 {
			m.actionCursor++
		}
		return m, nil
	default:
		// Check if the key matches an action's KeyHint for quick execution
		for _, action := range m.availableActions {
			if action.KeyHint == key && action.Handler != nil {
				handler := action.Handler
				return m, func() tea.Msg {
					return CommandPaletteExecuteMsg{ResultMsg: handler()}
				}
			}
		}
		// Any other key exits action mode and processes as search input
		m.actionMode = false
		m.availableActions = nil
		// Process the key as search input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	}
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

	switch item.Category {
	case ItemCategoryCommands:
		if item.Command != nil && item.Command.Handler != nil {
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
	}
	return m, nil
}

// findProjectWithTag returns the first project that contains the specified tag.
func (m *CommandPaletteModel) findProjectWithTag(tag string) *models.Project {
	for _, item := range m.allItems {
		if item.Category == ItemCategoryTasks && item.Task != nil {
			for _, t := range item.Task.Tags {
				if t == tag {
					return item.Project
				}
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

// getActionsForItem returns the available inline actions for a palette item.
// Commands have no inline actions (Enter executes them directly).
func (m *CommandPaletteModel) getActionsForItem(item PaletteItem) []PaletteAction {
	switch item.Category {
	case ItemCategoryTasks:
		return m.getTaskActions(item)
	case ItemCategoryProjects:
		return m.getProjectActions(item)
	case ItemCategoryTags:
		return m.getTagActions(item)
	case ItemCategorySections:
		return m.getSectionActions(item)
	default:
		return nil
	}
}

// getTaskActions returns inline actions for a task item.
func (m *CommandPaletteModel) getTaskActions(item PaletteItem) []PaletteAction {
	if item.Task == nil || item.Project == nil {
		return nil
	}
	task := item.Task
	project := item.Project

	actions := []PaletteAction{
		{
			Name:    "Go to task",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return GlobalSearchSelectMsg{Task: task, Project: project}
			},
		},
		{
			Name:    "View",
			Icon:    "👁",
			KeyHint: "v",
			Handler: func() tea.Msg {
				return GlowViewMsg{Task: task}
			},
		},
		{
			Name:    "Edit",
			Icon:    "✏",
			KeyHint: "e",
			Handler: func() tea.Msg {
				return EditTaskMsg{Task: task}
			},
		},
	}

	// Add status-based actions
	if task.Status != models.StatusDone && task.Status != models.StatusCancelled {
		actions = append(actions, PaletteAction{
			Name:    "Complete",
			Icon:    "✓",
			KeyHint: "x",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "complete"}
			},
		})
	}

	if task.Status == models.StatusTodo {
		actions = append(actions, PaletteAction{
			Name:    "Start",
			Icon:    "▶",
			KeyHint: "s",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "start"}
			},
		})
	}

	if task.Status == models.StatusDone || task.Status == models.StatusCancelled {
		actions = append(actions, PaletteAction{
			Name:    "Reopen",
			Icon:    "↺",
			KeyHint: "r",
			Handler: func() tea.Msg {
				return CommandPaletteTaskActionMsg{Task: task, Project: project, Action: "reopen"}
			},
		})
	}

	// Add due date action
	actions = append(actions, PaletteAction{
		Name:    "Set due date",
		Icon:    "📅",
		KeyHint: "d",
		Handler: func() tea.Msg {
			return SetDueDateMsg{Task: task, Project: project}
		},
	})

	// Add subtask action for non-leaf types
	if task.Type == models.TaskTypeEpic || task.Type == models.TaskTypeStory {
		actions = append(actions, PaletteAction{
			Name:    "Add subtask",
			Icon:    "+",
			KeyHint: "a",
			Handler: func() tea.Msg {
				return CreateTaskMsg{ParentTask: task}
			},
		})
	}

	return actions
}

// getProjectActions returns inline actions for a project item.
func (m *CommandPaletteModel) getProjectActions(item PaletteItem) []PaletteAction {
	if item.Project == nil {
		return nil
	}
	project := item.Project

	actions := []PaletteAction{
		{
			Name:    "Open",
			Icon:    "→",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				return SelectProjectMsg{Project: project}
			},
		},
		{
			Name:    "Edit",
			Icon:    "✏",
			KeyHint: "e",
			Handler: func() tea.Msg {
				return EditProjectMsg{Project: project}
			},
		},
	}

	if project.Archived {
		actions = append(actions, PaletteAction{
			Name:    "Restore",
			Icon:    "↺",
			KeyHint: "r",
			Handler: func() tea.Msg {
				return ProjectRestoredMsg{Project: project}
			},
		})
	} else {
		actions = append(actions, PaletteAction{
			Name:    "Archive",
			Icon:    "📦",
			KeyHint: "x",
			Handler: func() tea.Msg {
				return ProjectArchivedMsg{Project: project}
			},
		})
	}

	return actions
}

// getTagActions returns inline actions for a tag item.
func (m *CommandPaletteModel) getTagActions(item PaletteItem) []PaletteAction {
	if item.Tag == "" {
		return nil
	}
	tag := item.Tag
	project := m.findProjectWithTag(tag)

	actions := []PaletteAction{
		{
			Name:    "Filter by tag",
			Icon:    "🏷",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				if project != nil {
					return GlobalTagNavigateMsg{Project: project, Tag: tag}
				}
				return nil
			},
		},
		{
			Name:    "Manage tags",
			Icon:    "⚙",
			KeyHint: "t",
			Handler: func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			},
		},
	}

	return actions
}

// getSectionActions returns inline actions for a section item.
func (m *CommandPaletteModel) getSectionActions(item PaletteItem) []PaletteAction {
	if item.Section == "" {
		return nil
	}
	section := item.Section
	project := m.findProjectWithSection(section)

	actions := []PaletteAction{
		{
			Name:    "Filter by section",
			Icon:    "📂",
			KeyHint: "enter",
			Handler: func() tea.Msg {
				if project != nil {
					return CommandPaletteSectionSelectMsg{Section: section, Project: project}
				}
				return nil
			},
		},
	}

	return actions
}

// View implements tea.Model.
func (m *CommandPaletteModel) View() string {
	var b strings.Builder

	// Title - changes based on mode
	if m.actionMode {
		b.WriteString(TitleStyle.Render("Actions"))
	} else {
		b.WriteString(TitleStyle.Render("Command Palette"))
	}
	b.WriteString("\n")

	// In action mode, show which item we're acting on
	if m.actionMode {
		itemDesc := m.actionItem.Name
		if m.actionItem.Description != "" {
			itemDesc = fmt.Sprintf("%s · %s", m.actionItem.Name, m.actionItem.Description)
		}
		b.WriteString(MutedStyle.Render(itemDesc))
		b.WriteString("\n\n")
	} else {
		// Search input (only in search mode)
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	}

	// Handle edge cases
	if len(m.allItems) == 0 {
		b.WriteString(MutedStyle.Render("No items available."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
		return b.String()
	}

	query := strings.TrimSpace(m.searchInput.Value())
	if !m.actionMode && len(m.filteredItems) == 0 && query != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("No matches for \"%s\"", query)))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Try: :cmd for commands • :task for tasks • :prj for projects"))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("     :tag for tags • :sec for sections • or clear to browse all"))
		return b.String()
	}

	// Render content based on mode
	var listContent string
	if m.actionMode {
		listContent = m.renderActionList()
	} else {
		listContent = m.renderItemList()
	}

	paneHeight := m.height - 6 // Account for title, input, spacing, and footer
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Split pane: list on left, preview on right
	previewContent := m.renderPreview()

	listPane := TaskJumperListStyle.
		Width(m.listWidth).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(listContent)

	previewPane := TaskJumperPreviewStyle.
		Width(m.previewWidth).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(previewContent)

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane))
	b.WriteString("\n")

	// Footer with controls - different for each mode
	var footer string
	if m.actionMode {
		footer = HelpStyle.Render("↑/↓ navigate • Enter execute • Tab/Esc back")
	} else {
		footer = HelpStyle.Render(m.buildFooter())
	}
	b.WriteString(footer)

	return b.String()
}

// buildFooter constructs the footer text with navigation hints and category counts.
func (m *CommandPaletteModel) buildFooter() string {
	// Count items by category in the filtered results
	counts := make(map[ItemCategory]int)
	for _, filtered := range m.filteredItems {
		counts[filtered.Item.Category]++
	}

	// Build count summary (only non-zero categories)
	var countParts []string
	if c := counts[ItemCategoryCommands]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%d cmd", c))
	}
	if c := counts[ItemCategoryProjects]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%d prj", c))
	}
	if c := counts[ItemCategoryTasks]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%d task", c))
	}
	if c := counts[ItemCategoryTags]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%d tag", c))
	}
	if c := counts[ItemCategorySections]; c > 0 {
		countParts = append(countParts, fmt.Sprintf("%d sec", c))
	}

	countSummary := strings.Join(countParts, ", ")
	if countSummary == "" {
		countSummary = "0 items"
	}

	// Build context-aware hints based on selected item type
	hints := "↑/↓ navigate • Enter select"
	if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
		item := m.filteredItems[m.cursor].Item
		switch item.Category {
		case ItemCategoryTasks:
			hints += " • ^X done • ^E edit • ^G view • ^S start"
		default:
			hints += " • Tab actions"
		}
	} else {
		hints += " • Tab actions"
	}
	hints += " • Esc close"

	return fmt.Sprintf("%s • %s", hints, countSummary)
}

// renderActionList renders the list of available actions for the selected item.
func (m *CommandPaletteModel) renderActionList() string {
	if len(m.availableActions) == 0 {
		return MutedStyle.Render("No actions available.")
	}

	var b strings.Builder
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	for i, action := range m.availableActions {
		line := m.renderActionItem(action, i == m.actionCursor, contentWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// renderActionItem renders a single action row.
// Layout: [cursor] [icon] [name] ... [key hint]
func (m *CommandPaletteModel) renderActionItem(action PaletteAction, selected bool, width int) string {
	// Cursor indicator (2 chars)
	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Icon (2-3 chars)
	icon := action.Icon
	if icon == "" {
		icon = "•"
	}
	icon = icon + " "

	// Key hint for right alignment
	keyHint := ""
	keyHintLen := 0
	if action.KeyHint != "" {
		keyHint = action.KeyHint
		keyHintLen = len(keyHint) + 2 // +2 for spacing
	}

	// Calculate remaining width for name
	fixedWidth := 2 + 3 + keyHintLen + 2 // cursor + icon + key hint + gap
	nameWidth := width - fixedWidth
	if nameWidth < 10 {
		nameWidth = 10
	}

	// Truncate name if needed
	name := action.Name
	if len(name) > nameWidth {
		name = name[:nameWidth-1] + "…"
	}

	// Calculate gap for right-alignment
	gap := nameWidth - len(name)
	if gap < 1 {
		gap = 1
	}

	// Build the line
	var line strings.Builder
	line.WriteString(cursor)
	line.WriteString(icon)
	line.WriteString(BoldStyle.Render(name))

	if keyHint != "" {
		line.WriteString(strings.Repeat(" ", gap))
		line.WriteString(HelpKeyStyle.Render(keyHint))
	}

	result := line.String()
	if selected {
		result = TaskJumperSelectedStyle.Render(result)
	}

	return result
}

// renderItemList renders the filtered item list with cursor indicator.
// Each row displays: cursor + category badge + name (prominent) + extra info + shortcut (right-aligned)
// When no search query is active, items are grouped by category with section headers.
func (m *CommandPaletteModel) renderItemList() string {
	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("Type to search commands, tasks, projects...")
	}

	// Determine if we should show category headers (only when browsing, not searching)
	query := strings.TrimSpace(m.searchInput.Value())
	categoryFilter, searchQuery := m.parseQueryPrefix(query)
	showHeaders := searchQuery == "" && categoryFilter == ""

	var b strings.Builder

	// Show tips when palette first opens with empty query
	if m.showTips && query == "" {
		tips := []string{
			"Tip: Type :cmd, :task, :prj, :tag, or :sec to filter by type",
			"Tip: Use -term to exclude matches (e.g., bug -fixed)",
			"Tip: Press Tab on any item for quick actions",
			"Tip: Keys 1-9 select the first 9 items directly",
			"Tip: ^M completes a task, ^E edits, ^G views in glow",
		}
		// Pick one tip based on current second for variety
		tipIndex := time.Now().Second() % len(tips)
		b.WriteString(HelpStyle.Render(tips[tipIndex]))
		b.WriteString("\n\n")
	}

	maxVisible := m.height - 8 // Account for chrome
	if m.showTips && query == "" {
		maxVisible -= 2 // Account for tip line
	}
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.scrollAnchor >= 0 {
		// When scrollAnchor is set, position that item at the top of the view
		scrollOffset = m.scrollAnchor
		m.scrollAnchor = -1 // Reset after use
	} else if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

	// Available width for content (account for padding)
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Track current category for header insertion
	var currentCategory ItemCategory
	linesRendered := 0

	for i := scrollOffset; i < len(m.filteredItems) && linesRendered < maxVisible; i++ {
		filtered := m.filteredItems[i]
		item := filtered.Item

		// Insert category header when category changes (only in browse mode)
		if showHeaders && item.Category != currentCategory {
			// Don't count the header against visible items for first category
			if currentCategory != "" {
				// Add spacing before new category (except first)
				b.WriteString("\n")
				linesRendered++
				if linesRendered >= maxVisible {
					break
				}
			}
			header := m.renderCategoryHeader(item.Category)
			b.WriteString(header)
			b.WriteString("\n")
			linesRendered++
			currentCategory = item.Category
			if linesRendered >= maxVisible {
				break
			}
		}

		// Pass the index for number hint display (only first 9 items when jump is enabled)
		numberHint := -1
		if m.isNumberJumpEnabled() && i < 9 {
			numberHint = i + 1 // 1-9 for first 9 items
		}
		line := m.renderPaletteItemWithHighlight(filtered, i == m.cursor, contentWidth, numberHint)
		b.WriteString(line)
		b.WriteString("\n")
		linesRendered++
	}

	// Show scroll indicator if there are more items
	if len(m.filteredItems) > maxVisible {
		showing := scrollOffset + 1
		showingEnd := scrollOffset + maxVisible
		if showingEnd > len(m.filteredItems) {
			showingEnd = len(m.filteredItems)
		}
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", showing, showingEnd, len(m.filteredItems))
		b.WriteString(MutedStyle.Render(scrollInfo))
	}

	return b.String()
}

// renderPreview renders the preview pane for the selected item.
func (m *CommandPaletteModel) renderPreview() string {
	// In action mode, show action help
	if m.actionMode {
		return m.renderActionPreview()
	}

	if len(m.filteredItems) == 0 || m.cursor >= len(m.filteredItems) {
		return MutedStyle.Render("No item selected.")
	}

	item := m.filteredItems[m.cursor].Item

	switch item.Category {
	case ItemCategoryCommands:
		return m.renderCommandPreview(item)
	case ItemCategoryTasks:
		return m.renderTaskPreview(item)
	case ItemCategoryProjects:
		return m.renderProjectPreview(item)
	case ItemCategoryTags:
		return m.renderTagPreview(item)
	case ItemCategorySections:
		return m.renderSectionPreview(item)
	default:
		return MutedStyle.Render("Unknown item type.")
	}
}

// renderCommandPreview shows details for a command.
func (m *CommandPaletteModel) renderCommandPreview(item PaletteItem) string {
	if item.Command == nil {
		return MutedStyle.Render("No command details.")
	}
	cmd := item.Command
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(cmd.Name))
	b.WriteString("\n\n")

	// Category
	b.WriteString(MutedStyle.Render("Category: "))
	b.WriteString(string(cmd.Category))
	b.WriteString("\n")

	// Shortcut
	if cmd.Shortcut != "" {
		b.WriteString(MutedStyle.Render("Shortcut: "))
		b.WriteString(HelpKeyStyle.Render(cmd.Shortcut))
		b.WriteString("\n")
	}

	// Description
	if cmd.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(cmd.Description)
		b.WriteString("\n")
	}

	// Aliases
	if len(cmd.Aliases) > 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Also searchable as: "))
		b.WriteString(strings.Join(cmd.Aliases, ", "))
		b.WriteString("\n")
	}

	return b.String()
}

// renderTaskPreview shows details for a task.
func (m *CommandPaletteModel) renderTaskPreview(item PaletteItem) string {
	if item.Task == nil {
		return MutedStyle.Render("No task details.")
	}
	task := item.Task
	var b strings.Builder

	// Project header if available
	if item.Project != nil {
		b.WriteString(MutedStyle.Render("Project: "))
		b.WriteString(BoldStyle.Render(item.Project.Name))
		b.WriteString("\n\n")
	}

	// Header with type and title
	typeStyle := GetTypeStyle(string(task.Type))
	header := fmt.Sprintf("%s #%d: %s",
		typeStyle.Render(task.Type.HeadingPrefix()),
		task.ShortID,
		task.Title)
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(header))
	b.WriteString("\n\n")

	// Status
	statusStyle := GetStatusStyle(string(task.Status))
	b.WriteString(fmt.Sprintf("%s %s %s\n",
		MutedStyle.Render("Status:"),
		statusStyle.Render(task.Status.Icon()),
		statusStyle.Render(string(task.Status))))

	// Priority
	priorityStyle := NormalStyle
	if task.Priority <= 2 {
		priorityStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	}
	b.WriteString(fmt.Sprintf("%s %s\n",
		MutedStyle.Render("Priority:"),
		priorityStyle.Render(fmt.Sprintf("%d", task.Priority))))

	// Tags
	if len(task.Tags) > 0 {
		tagStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
		tags := make([]string, len(task.Tags))
		for i, tag := range task.Tags {
			tags[i] = tagStyle.Render(tag)
		}
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Tags:"),
			strings.Join(tags, ", ")))
	}

	// Section
	if task.Section != "" {
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Section:"),
			task.Section))
	}

	// Due date
	if task.DueDate != nil {
		dueStyle := DueDateStyle
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
		if dueDay.Before(today) {
			dueStyle = OverdueStyle
		} else if dueDay.Equal(today) {
			dueStyle = DueTodayStyle
		}
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Due:"),
			dueStyle.Render(task.DueDate.Format(util.DateFormat))))
	}

	// Parent reference
	if task.Parent != nil {
		parentRef := fmt.Sprintf("#%d: %s", task.Parent.ShortID, task.Parent.Title)
		b.WriteString(fmt.Sprintf("%s %s\n",
			MutedStyle.Render("Parent:"),
			MutedStyle.Render(util.Truncate(parentRef, m.previewWidth-10))))
	}

	// Subtasks count
	if len(task.Children) > 0 {
		completed := 0
		for _, child := range task.Children {
			if child.Status.IsComplete() {
				completed++
			}
		}
		b.WriteString(fmt.Sprintf("%s %d/%d done\n",
			MutedStyle.Render("Subtasks:"),
			completed, len(task.Children)))
	}

	// Description (truncated)
	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		desc := util.Truncate(task.Description, m.previewWidth*3) // Allow ~3 lines
		b.WriteString(desc)
		b.WriteString("\n")
	}

	return b.String()
}

// renderProjectPreview shows details for a project.
func (m *CommandPaletteModel) renderProjectPreview(item PaletteItem) string {
	if item.Project == nil {
		return MutedStyle.Render("No project details.")
	}
	project := item.Project
	var b strings.Builder

	// Header
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(project.Name))
	b.WriteString("\n\n")

	// Slug
	b.WriteString(MutedStyle.Render("Slug: "))
	b.WriteString(project.Slug)
	b.WriteString("\n")

	// Context
	if project.Context != "" {
		b.WriteString(MutedStyle.Render("Context: "))
		b.WriteString(project.Context)
		b.WriteString("\n")
	}

	// Description
	if project.Description != "" {
		b.WriteString("\n")
		b.WriteString(SubtitleStyle.Render("Description"))
		b.WriteString("\n")
		b.WriteString(util.Truncate(project.Description, m.previewWidth*2))
		b.WriteString("\n")
	}

	// Task counts (if loaded)
	if len(project.Tasks) > 0 {
		active, completed := 0, 0
		for _, task := range project.Tasks {
			if task.Status.IsComplete() {
				completed++
			} else {
				active++
			}
		}
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Tasks: "))
		b.WriteString(fmt.Sprintf("%d active, %d completed", active, completed))
		b.WriteString("\n")
	}

	return b.String()
}

// renderTagPreview shows tasks with this tag.
func (m *CommandPaletteModel) renderTagPreview(item PaletteItem) string {
	if item.Tag == "" {
		return MutedStyle.Render("No tag details.")
	}
	var b strings.Builder

	// Header
	tagStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(tagStyle.Render("#" + item.Tag)))
	b.WriteString("\n\n")

	// Find tasks with this tag
	var tasksWithTag []PaletteItem
	for _, fi := range m.filteredItems {
		if fi.Item.Category == ItemCategoryTasks && fi.Item.Task != nil {
			for _, tag := range fi.Item.Task.Tags {
				if tag == item.Tag {
					tasksWithTag = append(tasksWithTag, fi.Item)
					break
				}
			}
		}
	}

	if len(tasksWithTag) == 0 {
		b.WriteString(MutedStyle.Render("No active tasks with this tag."))
		return b.String()
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Tasks (%d)", len(tasksWithTag))))
	b.WriteString("\n")

	// Show up to 5 tasks
	maxShow := 5
	if len(tasksWithTag) < maxShow {
		maxShow = len(tasksWithTag)
	}
	for i := 0; i < maxShow; i++ {
		task := tasksWithTag[i].Task
		typeStyle := GetTypeStyle(string(task.Type))
		b.WriteString(fmt.Sprintf("  %s #%d: %s\n",
			typeStyle.Render(task.Type.Icon()),
			task.ShortID,
			util.Truncate(task.Title, m.previewWidth-15)))
	}
	if len(tasksWithTag) > maxShow {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more", len(tasksWithTag)-maxShow)))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSectionPreview shows tasks in this section.
func (m *CommandPaletteModel) renderSectionPreview(item PaletteItem) string {
	if item.Section == "" {
		return MutedStyle.Render("No section details.")
	}
	var b strings.Builder

	// Header
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("135"))
	b.WriteString(TaskJumperPreviewHeaderStyle.Render(sectionStyle.Render("§ " + item.Section)))
	b.WriteString("\n\n")

	// Find tasks in this section
	var tasksInSection []PaletteItem
	for _, fi := range m.filteredItems {
		if fi.Item.Category == ItemCategoryTasks && fi.Item.Task != nil {
			if fi.Item.Task.Section == item.Section {
				tasksInSection = append(tasksInSection, fi.Item)
			}
		}
	}

	if len(tasksInSection) == 0 {
		b.WriteString(MutedStyle.Render("No active tasks in this section."))
		return b.String()
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Tasks (%d)", len(tasksInSection))))
	b.WriteString("\n")

	// Show up to 5 tasks
	maxShow := 5
	if len(tasksInSection) < maxShow {
		maxShow = len(tasksInSection)
	}
	for i := 0; i < maxShow; i++ {
		task := tasksInSection[i].Task
		typeStyle := GetTypeStyle(string(task.Type))
		b.WriteString(fmt.Sprintf("  %s #%d: %s\n",
			typeStyle.Render(task.Type.Icon()),
			task.ShortID,
			util.Truncate(task.Title, m.previewWidth-15)))
	}
	if len(tasksInSection) > maxShow {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more", len(tasksInSection)-maxShow)))
		b.WriteString("\n")
	}

	return b.String()
}

// renderActionPreview shows help for the current action.
func (m *CommandPaletteModel) renderActionPreview() string {
	var b strings.Builder

	// Show item context
	b.WriteString(SubtitleStyle.Render("Acting on:"))
	b.WriteString("\n")
	b.WriteString(BoldStyle.Render(m.actionItem.Name))
	b.WriteString("\n")
	if m.actionItem.Description != "" {
		b.WriteString(MutedStyle.Render(m.actionItem.Description))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Show available actions with hints
	b.WriteString(SubtitleStyle.Render("Available Actions:"))
	b.WriteString("\n")
	for _, action := range m.availableActions {
		hint := ""
		if action.KeyHint != "" {
			hint = fmt.Sprintf(" [%s]", action.KeyHint)
		}
		b.WriteString(fmt.Sprintf("  %s %s%s\n",
			action.Icon,
			action.Name,
			MutedStyle.Render(hint)))
	}

	return b.String()
}

// renderCategoryHeader renders a section header for a category.
func (m *CommandPaletteModel) renderCategoryHeader(cat ItemCategory) string {
	name := getCategoryDisplayName(cat)
	// Count items in this category
	count := 0
	for _, filtered := range m.filteredItems {
		if filtered.Item.Category == cat {
			count++
		}
	}
	header := fmt.Sprintf("── %s (%d) ", name, count)
	return MutedStyle.Render(header)
}

// renderPaletteItemWithHighlight renders a palette item with match highlighting.
// numberHint is 1-9 for items that can be selected with number keys, or -1 to hide.
func (m *CommandPaletteModel) renderPaletteItemWithHighlight(filtered filteredPaletteItem, selected bool, width int, numberHint int) string {
	item := filtered.Item

	// Cursor/number indicator (2 chars)
	cursor := "  "
	if selected {
		cursor = "> "
	} else if numberHint > 0 && numberHint <= 9 {
		// Show number hint in muted style
		cursor = MutedStyle.Render(fmt.Sprintf("%d ", numberHint))
	}

	// Category badge with color (5-6 chars including brackets)
	catBadge, catBadgeLen := m.renderCategoryBadge(item.Category)

	// Shortcut string for right alignment
	shortcutText := ""
	shortcutLen := 0
	if item.Shortcut != "" {
		shortcutText = item.Shortcut
		shortcutLen = len(shortcutText) + 2
	} else if item.Category != ItemCategoryCommands {
		shortcutText = "Tab"
		shortcutLen = 5
	}

	// Calculate space for name + extra info + description
	fixedWidth := 2 + catBadgeLen + 1 + shortcutLen + 2
	remainingWidth := width - fixedWidth
	if remainingWidth < 20 {
		remainingWidth = 20
	}

	// Build the left content with highlighting
	leftContent, leftLen := m.renderItemContentWithHighlight(item, filtered.NamePositions, remainingWidth)

	// Calculate gap to right-align shortcut
	gap := remainingWidth - leftLen
	if gap < 1 {
		gap = 1
	}

	// Build the complete line
	var line strings.Builder
	line.WriteString(cursor)
	line.WriteString(catBadge)
	line.WriteString(" ")
	line.WriteString(leftContent)

	if shortcutText != "" {
		line.WriteString(strings.Repeat(" ", gap))
		line.WriteString(HelpKeyStyle.Render(shortcutText))
	}

	result := line.String()
	if selected {
		result = TaskJumperSelectedStyle.Render(result)
	}

	return result
}

// renderPaletteItem renders a single palette item row.
// Layout: [cursor][category badge] [name] [extra info] ... [shortcut]
// The shortcut is right-aligned, with the name and info filling the remaining space.
func (m *CommandPaletteModel) renderPaletteItem(item PaletteItem, selected bool, width int) string {
	// Cursor indicator (2 chars)
	cursor := "  "
	if selected {
		cursor = "> "
	}

	// Category badge with color (5-6 chars including brackets)
	catBadge, catBadgeLen := m.renderCategoryBadge(item.Category)

	// Shortcut string for right alignment
	// Commands show their keyboard shortcut, other items show "Tab" hint for actions
	shortcutText := ""
	shortcutLen := 0
	if item.Shortcut != "" {
		shortcutText = item.Shortcut
		shortcutLen = len(shortcutText) + 2 // +2 for spacing
	} else if item.Category != ItemCategoryCommands {
		// Show "Tab" hint for non-command items that have actions
		shortcutText = "Tab"
		shortcutLen = 5 // "Tab" + spacing
	}

	// Calculate space for name + extra info + description
	// Layout: cursor(2) + badge(6) + space(1) + name + extras + gap + shortcut
	fixedWidth := 2 + catBadgeLen + 1 + shortcutLen + 2 // cursor + badge + space + shortcut + min gap
	remainingWidth := width - fixedWidth
	if remainingWidth < 20 {
		remainingWidth = 20
	}

	// Build the left content: name + task extras + description
	leftContent, leftLen := m.renderItemContent(item, remainingWidth)

	// Calculate gap to right-align shortcut
	gap := remainingWidth - leftLen
	if gap < 1 {
		gap = 1
	}

	// Build the complete line
	var line strings.Builder
	line.WriteString(cursor)
	line.WriteString(catBadge)
	line.WriteString(" ")
	line.WriteString(leftContent)

	// Add shortcut with right-alignment if present
	if shortcutText != "" {
		line.WriteString(strings.Repeat(" ", gap))
		line.WriteString(HelpKeyStyle.Render(shortcutText))
	}

	result := line.String()
	if selected {
		result = TaskJumperSelectedStyle.Render(result)
	}

	return result
}

// renderCategoryBadge returns the styled category badge and its display length.
// Badges include an icon prefix for faster visual scanning.
func (m *CommandPaletteModel) renderCategoryBadge(cat ItemCategory) (string, int) {
	var badge string
	var style lipgloss.Style

	switch cat {
	case ItemCategoryCommands:
		badge = "> CMD"
		style = lipgloss.NewStyle().Foreground(ColorPrimary)
	case ItemCategoryTasks:
		badge = "☐ TSK"
		style = lipgloss.NewStyle().Foreground(ColorSuccess)
	case ItemCategoryProjects:
		badge = "◈ PRJ"
		style = lipgloss.NewStyle().Foreground(ColorSecondary)
	case ItemCategoryTags:
		badge = "# TAG"
		style = lipgloss.NewStyle().Foreground(ColorWarning)
	case ItemCategorySections:
		badge = "§ SEC"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("135")) // Purple
	default:
		badge = "? ???"
		style = MutedStyle
	}

	return style.Render(badge), 5 // All badges are 5 display characters
}

// getCategoryDisplayName returns a human-readable name for a category.
func getCategoryDisplayName(cat ItemCategory) string {
	switch cat {
	case ItemCategoryCommands:
		return "Commands"
	case ItemCategoryTasks:
		return "Tasks"
	case ItemCategoryProjects:
		return "Projects"
	case ItemCategoryTags:
		return "Tags"
	case ItemCategorySections:
		return "Sections"
	default:
		return "Other"
	}
}

// getDueDateIndicator returns a colored indicator for task due date urgency.
// Returns empty string if no due date or not urgent.
func getDueDateIndicator(task *models.Task) (string, int) {
	if task == nil || task.DueDate == nil {
		return "", 0
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	if dueDay.Before(today) {
		// Overdue - red exclamation
		return OverdueStyle.Render("!") + " ", 2
	}
	if dueDay.Equal(today) {
		// Due today - orange exclamation
		return DueTodayStyle.Render("!") + " ", 2
	}
	weekFromNow := today.AddDate(0, 0, 7)
	if dueDay.Before(weekFromNow) {
		// Due this week - yellow tilde
		return lipgloss.NewStyle().Foreground(ColorWarning).Render("~") + " ", 2
	}

	return "", 0
}

// HighlightStyle is used to highlight matched characters in search results.
var HighlightStyle = lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)

// highlightText renders text with certain character positions highlighted.
// Positions are rune indices (0-based) that should be highlighted.
func highlightText(text string, positions []int, baseStyle lipgloss.Style) string {
	if len(positions) == 0 {
		return baseStyle.Render(text)
	}

	// Create a set for quick lookup
	posSet := make(map[int]struct{})
	for _, p := range positions {
		posSet[p] = struct{}{}
	}

	var result strings.Builder
	runes := []rune(text)

	for i, r := range runes {
		char := string(r)
		if _, highlight := posSet[i]; highlight {
			result.WriteString(HighlightStyle.Render(char))
		} else {
			result.WriteString(baseStyle.Render(char))
		}
	}

	return result.String()
}

// renderItemContentWithHighlight renders item content with match highlighting on the name.
func (m *CommandPaletteModel) renderItemContentWithHighlight(item PaletteItem, namePositions []int, maxWidth int) (string, int) {
	var content strings.Builder
	totalLen := 0

	// Task-specific: show due date indicator and type/status icons before name
	if item.Category == ItemCategoryTasks && item.Task != nil {
		dueIndicator, dueLen := getDueDateIndicator(item.Task)
		if dueLen > 0 {
			content.WriteString(dueIndicator)
			totalLen += dueLen
		}

		typeStyle := GetTypeStyle(string(item.Task.Type))
		statusStyle := GetStatusStyle(string(item.Task.Status))
		icons := fmt.Sprintf("%s %s ",
			typeStyle.Render(item.Task.Type.Icon()),
			statusStyle.Render(item.Task.Status.Icon()))
		content.WriteString(icons)
		totalLen += 4
	}

	// Calculate max name length
	maxNameLen := maxWidth - totalLen
	if item.Description != "" && maxNameLen > 25 {
		maxNameLen = maxWidth/2 - totalLen
		if maxNameLen < 15 {
			maxNameLen = 15
		}
	}
	if maxNameLen < 10 {
		maxNameLen = 10
	}

	// Name with highlighting (truncated if needed)
	name := util.Truncate(item.Name, maxNameLen)
	// Adjust positions if name was truncated
	truncatedPositions := namePositions
	if len(name) < len(item.Name) {
		var validPositions []int
		nameRunes := []rune(name)
		for _, p := range namePositions {
			if p < len(nameRunes) {
				validPositions = append(validPositions, p)
			}
		}
		truncatedPositions = validPositions
	}
	content.WriteString(highlightText(name, truncatedPositions, BoldStyle))
	totalLen += len([]rune(name))

	// Description (dimmed, if space allows)
	if item.Description != "" {
		descMaxLen := maxWidth - totalLen - 3
		if descMaxLen > 8 {
			desc := util.Truncate(item.Description, descMaxLen)
			content.WriteString(" ")
			content.WriteString(MutedStyle.Render("· " + desc))
			totalLen += 3 + len(desc)
		}
	}

	return content.String(), totalLen
}

// renderItemContent renders the main content of a palette item (name + extras).
// Returns the styled string and its approximate display length.
func (m *CommandPaletteModel) renderItemContent(item PaletteItem, maxWidth int) (string, int) {
	var content strings.Builder
	totalLen := 0

	// Task-specific: show due date indicator and type/status icons before name
	if item.Category == ItemCategoryTasks && item.Task != nil {
		// Due date urgency indicator
		dueIndicator, dueLen := getDueDateIndicator(item.Task)
		if dueLen > 0 {
			content.WriteString(dueIndicator)
			totalLen += dueLen
		}

		typeStyle := GetTypeStyle(string(item.Task.Type))
		statusStyle := GetStatusStyle(string(item.Task.Status))
		icons := fmt.Sprintf("%s %s ",
			typeStyle.Render(item.Task.Type.Icon()),
			statusStyle.Render(item.Task.Status.Icon()))
		content.WriteString(icons)
		totalLen += 4 // 2 icons + 2 spaces
	}

	// Calculate max name length based on remaining space
	// Reserve some space for description if present
	maxNameLen := maxWidth - totalLen
	if item.Description != "" && maxNameLen > 25 {
		maxNameLen = maxWidth/2 - totalLen
		if maxNameLen < 15 {
			maxNameLen = 15
		}
	}
	if maxNameLen < 10 {
		maxNameLen = 10
	}

	// Name (prominent, truncated if needed)
	name := util.Truncate(item.Name, maxNameLen)
	content.WriteString(BoldStyle.Render(name))
	totalLen += len(name)

	// Description (dimmed, if space allows)
	if item.Description != "" {
		descMaxLen := maxWidth - totalLen - 3 // -3 for " · " separator
		if descMaxLen > 8 {
			desc := util.Truncate(item.Description, descMaxLen)
			content.WriteString(" ")
			content.WriteString(MutedStyle.Render("· " + desc))
			totalLen += 3 + len(desc) // separator + desc
		}
	}

	return content.String(), totalLen
}

// CloseCommandPaletteMsg is sent when the command palette should close.
type CloseCommandPaletteMsg struct{}

// CommandPaletteExecuteMsg wraps a command's result and signals the palette should close.
// This ensures the palette closes and the command executes in the same update cycle.
type CommandPaletteExecuteMsg struct {
	ResultMsg tea.Msg
}

// CommandPaletteSelectMsg is sent when a command is selected from the palette.
type CommandPaletteSelectMsg struct {
	Command Command
}

// CommandPaletteTaskSelectMsg is sent when a task is selected from the palette.
type CommandPaletteTaskSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// CommandPaletteProjectSelectMsg is sent when a project is selected from the palette.
type CommandPaletteProjectSelectMsg struct {
	Project *models.Project
}

// CommandPaletteTagSelectMsg is sent when a tag is selected from the palette.
type CommandPaletteTagSelectMsg struct {
	Tag string
}

// CommandPaletteSectionSelectMsg is sent when a section is selected from the palette.
type CommandPaletteSectionSelectMsg struct {
	Section string
	Project *models.Project
}

// ShowCommandPaletteMsg is sent to open the command palette.
type ShowCommandPaletteMsg struct{}

// CommandPaletteTaskActionMsg is sent when an inline action is performed on a task.
// This message triggers status changes (complete, start, reopen) that need to be
// saved to the project file.
type CommandPaletteTaskActionMsg struct {
	Task    *models.Task
	Project *models.Project
	Action  string // "complete", "start", "reopen"
}
