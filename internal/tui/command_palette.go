package tui

import (
	"fmt"
	"sort"
	"strings"

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

// CommandPaletteModel provides a fullscreen fuzzy search palette for commands.
// It implements the View interface and follows the existing view stack pattern
// used by GlobalSearchModel and HelpViewModel.
type CommandPaletteModel struct {
	config   *config.Config
	registry *CommandRegistry

	// Current view context for filtering commands
	currentView ViewType

	// Input for filtering
	searchInput textinput.Model

	// Items - using PaletteItem for unified representation of all searchable types
	allItems      []PaletteItem
	filteredItems []PaletteItem

	// Navigation state
	cursor int

	// Action mode state - when true, showing inline actions for selected item
	actionMode       bool
	actionCursor     int
	availableActions []PaletteAction
	actionItem       PaletteItem // The item whose actions are being shown

	// Dimensions
	width  int
	height int
}

// NewCommandPaletteModel creates a new command palette model.
// The currentView parameter determines which commands are shown (context-aware filtering).
func NewCommandPaletteModel(cfg *config.Config, registry *CommandRegistry, currentView ViewType) *CommandPaletteModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search commands, tasks, projects..."
	searchInput.CharLimit = 100
	searchInput.Width = 40
	searchInput.Focus()

	m := &CommandPaletteModel{
		config:      cfg,
		registry:    registry,
		currentView: currentView,
		searchInput: searchInput,
		cursor:      0,
	}
	// Load all searchable items (lazy loading on palette open per design decision)
	m.allItems = m.loadAllItems()
	// Start with all items visible (no filter applied)
	m.filteredItems = m.allItems
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

	// Track unique tags and sections across all projects
	uniqueTags := make(map[string]struct{})
	uniqueSections := make(map[string]struct{})

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

			// Collect unique tags
			for _, tag := range task.Tags {
				uniqueTags[tag] = struct{}{}
			}

			// Collect unique sections
			if task.Section != "" {
				uniqueSections[task.Section] = struct{}{}
			}
		}
	}

	// Add unique tags as searchable items
	for tag := range uniqueTags {
		items = append(items, PaletteItem{
			Name:        tag,
			Description: "Tag",
			Category:    ItemCategoryTags,
			Tag:         tag,
		})
	}

	// Add unique sections as searchable items
	for section := range uniqueSections {
		items = append(items, PaletteItem{
			Name:        section,
			Description: "Section",
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

// filterItems filters items based on the current search query using category-weighted fuzzy scoring.
func (m *CommandPaletteModel) filterItems() {
	query := strings.TrimSpace(m.searchInput.Value())

	// Empty query shows all items (commands first due to weighting)
	if query == "" {
		m.filteredItems = m.allItems
		m.cursor = 0
		return
	}

	// Score each item and collect matches
	type scoredItem struct {
		item  PaletteItem
		score int
	}
	var scored []scoredItem

	for _, item := range m.allItems {
		score := m.scoreItem(item, query)
		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results
	m.filteredItems = make([]PaletteItem, len(scored))
	for i, s := range scored {
		m.filteredItems[i] = s.item
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
func (m *CommandPaletteModel) scoreItem(item PaletteItem, query string) int {
	// Score against name (primary match target)
	nameScore := fuzzy.ScoreTerms(item.Name, query)

	// Score against description (secondary match target)
	descScore := fuzzy.ScoreTerms(item.Description, query)

	// Use the best score between name and description
	bestScore := nameScore
	if descScore > bestScore {
		bestScore = descScore
	}

	// If name matched, give it extra weight (similar to GlobalSearchModel's title weighting)
	if nameScore > 0 && nameScore >= descScore {
		bestScore = nameScore * 2
	}

	if bestScore == 0 {
		return 0
	}

	// Apply category weight multiplier
	return bestScore * categoryWeight(item.Category)
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
			return m.executeItem(m.filteredItems[m.cursor])
		}
		return m, nil
	case "tab":
		// Enter action mode for non-command items
		if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
			item := m.filteredItems[m.cursor]
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
	default:
		// Update search input and filter items
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.filterItems()
		return m, cmd
	}
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
func (m *CommandPaletteModel) executeItem(item PaletteItem) (tea.Model, tea.Cmd) {
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
			KeyHint: "m",
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
		b.WriteString(MutedStyle.Render(fmt.Sprintf("No matches for \"%s\".", query)))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press Esc to close"))
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

	listPane := lipgloss.NewStyle().
		Width(m.width - 2).
		Height(paneHeight).
		MaxHeight(paneHeight).
		Render(listContent)

	b.WriteString(listPane)
	b.WriteString("\n")

	// Footer with controls - different for each mode
	var footer string
	if m.actionMode {
		footer = HelpStyle.Render("↑/↓ navigate • Enter execute • Tab/Esc back")
	} else {
		footer = HelpStyle.Render(fmt.Sprintf("↑/↓ navigate • Enter select • Tab actions • Esc close • %d item(s)", len(m.filteredItems)))
	}
	b.WriteString(footer)

	return b.String()
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
func (m *CommandPaletteModel) renderItemList() string {
	if len(m.filteredItems) == 0 {
		return MutedStyle.Render("Type to search commands, tasks, projects...")
	}

	var b strings.Builder
	maxVisible := m.height - 8 // Account for chrome
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Calculate scroll offset to keep cursor visible
	scrollOffset := 0
	if m.cursor >= maxVisible {
		scrollOffset = m.cursor - maxVisible + 1
	}

	// Available width for content (account for padding)
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	for i := scrollOffset; i < len(m.filteredItems) && i < scrollOffset+maxVisible; i++ {
		item := m.filteredItems[i]
		line := m.renderPaletteItem(item, i == m.cursor, contentWidth)
		b.WriteString(line)
		b.WriteString("\n")
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

	// Shortcut string for right alignment (commands only, typically)
	shortcutText := ""
	shortcutLen := 0
	if item.Shortcut != "" {
		shortcutText = item.Shortcut
		shortcutLen = len(shortcutText) + 2 // +2 for spacing
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
func (m *CommandPaletteModel) renderCategoryBadge(cat ItemCategory) (string, int) {
	var badge string
	var style lipgloss.Style

	switch cat {
	case ItemCategoryCommands:
		badge = "[CMD]"
		style = lipgloss.NewStyle().Foreground(ColorPrimary)
	case ItemCategoryTasks:
		badge = "[TSK]"
		style = lipgloss.NewStyle().Foreground(ColorSuccess)
	case ItemCategoryProjects:
		badge = "[PRJ]"
		style = lipgloss.NewStyle().Foreground(ColorSecondary)
	case ItemCategoryTags:
		badge = "[TAG]"
		style = lipgloss.NewStyle().Foreground(ColorWarning)
	case ItemCategorySections:
		badge = "[SEC]"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("135")) // Purple
	default:
		badge = "[???]"
		style = MutedStyle
	}

	return style.Render(badge), len(badge)
}

// renderItemContent renders the main content of a palette item (name + extras).
// Returns the styled string and its approximate display length.
func (m *CommandPaletteModel) renderItemContent(item PaletteItem, maxWidth int) (string, int) {
	var content strings.Builder
	totalLen := 0

	// Task-specific: show type and status icons before name
	if item.Category == ItemCategoryTasks && item.Task != nil {
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
