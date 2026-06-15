package tui

import (
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ShowSaveFilterDialogMsg triggers the save view name input dialog.
type ShowSaveFilterDialogMsg struct{}

// SaveFilterMsg saves the current filter state as a SavedFilter with the given name.
type SaveFilterMsg struct {
	Name string
}

// LoadSavedFilterMsg applies a saved filter's state to the current view.
type LoadSavedFilterMsg struct {
	Filter config.SavedFilter
}

// DeleteSavedFilterMsg removes a saved filter by ID.
type DeleteSavedFilterMsg struct {
	ID string
}

// ItemCategory represents the type of searchable entity in the command palette.
// This is distinct from CommandCategory, which organizes commands by their function.
// ItemCategory identifies what kind of item is being searched (command vs task vs project, etc).
type ItemCategory string

const (
	ItemCategoryCommands    ItemCategory = "Commands"
	ItemCategoryTasks       ItemCategory = "Tasks"
	ItemCategoryProjects    ItemCategory = "Projects"
	ItemCategoryTags        ItemCategory = "Tags"
	ItemCategorySections    ItemCategory = "Sections"
	ItemCategorySavedSearch ItemCategory = "Saved"
	ItemCategoryNotes       ItemCategory = "Notes"
	ItemCategorySavedFilter ItemCategory = "Saved Filters"
	ItemCategoryViews       ItemCategory = "Views"
)

// categoryPrefixes maps short prefixes to item categories for filtering.
// Users can type ":cmd " to filter to commands only, ":task " for tasks, etc.
var categoryPrefixes = map[string]ItemCategory{
	"cmd":    ItemCategoryCommands,
	"task":   ItemCategoryTasks,
	"prj":    ItemCategoryProjects,
	"tag":    ItemCategoryTags,
	"sec":    ItemCategorySections,
	"saved":  ItemCategorySavedSearch,
	"recent": ItemCategoryCommands, // Special: filters to recently executed commands
	"note":   ItemCategoryNotes,
	"sv":     ItemCategorySavedFilter,
	"view":   ItemCategoryViews,
}

// commandCategoryPrefixes maps short prefixes to CommandCategory for sub-filtering commands.
// Users can type ":nav " to filter commands to Navigation category only, etc.
var commandCategoryPrefixes = map[string]CommandCategory{
	"nav":       CategoryNavigation,
	"sort":      CategorySort,
	"pri":       CategoryPriority,
	"status":    CategoryStatus,
	"due":       CategoryDueDate,
	"fold":      CategoryFold,
	"link":      CategoryLinkedTasks,
	"attach":    CategoryAttachments,
	"block":     CategoryBlockers,
	"comment":   CategoryComments,
	"structure": CategoryStructure,
}

// categoryOrder defines the display order for categories when showing headers.
var categoryOrder = []ItemCategory{
	ItemCategorySavedFilter,
	ItemCategorySavedSearch,
	ItemCategoryCommands,
	ItemCategoryProjects,
	ItemCategoryTasks,
	ItemCategoryTags,
	ItemCategorySections,
	ItemCategoryNotes,
	ItemCategoryViews,
}

// recentSelection tracks a recently selected item for boosting in search results.
type recentSelection struct {
	Category ItemCategory
	ID       string // command name, task ID, project slug, tag, or section name
	Count    int    // total number of times this item has been selected
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
	Command     *Command            // Set when Category is CategoryCommands
	Task        *models.Task        // Set when Category is CategoryTasks
	Project     *models.Project     // Set when Category is CategoryProjects
	Tag         string              // Set when Category is CategoryTags
	Section     string              // Set when Category is CategorySections
	SavedSearch string              // Set when Category is CategorySavedSearch (the query)
	Note        *models.Note        // Set when Category is CategoryNotes
	SavedFilter *config.SavedFilter // Set when Category is CategorySavedView
	ViewTarget  ViewType            // Set when Category is CategoryViews
}

// PaletteAction represents an inline action that can be performed on a palette item.
// When a user presses Tab on a non-command item, a submenu of available actions is shown.
type PaletteAction struct {
	Name    string         // Display name (e.g., "Complete", "Edit")
	Icon    string         // Visual indicator
	KeyHint string         // Keyboard shortcut hint (e.g., "m" for complete)
	Handler func() tea.Msg // Returns message when the action is selected
}

// MatchField identifies which field of an item was matched during search.
type MatchField int

const (
	MatchFieldNone        MatchField = iota
	MatchFieldName                   // Matched in the item's name/title
	MatchFieldDescription            // Matched in the description
	MatchFieldAlias                  // Matched in command aliases
	MatchFieldTag                    // Matched in task tags
	MatchFieldSection                // Matched in task section
)

// filteredPaletteItem wraps a PaletteItem with its match positions for highlighting.
type filteredPaletteItem struct {
	Item          PaletteItem
	NamePositions []int      // Positions of matched characters in Name
	MatchedField  MatchField // Which field was matched (for preview display)
	MatchedTerm   string     // The search term that matched (for preview display)
	IsFrequent    bool       // Item appears in the Recent section (duplicate of a regular item)
	IsSuggested   bool       // Item appears in the Suggested section (commands not yet used)
}

// inlineTaskPreview holds parsed metadata for inline task creation preview.
// Syntax: "+Task title !priority #tag @due"
type inlineTaskPreview struct {
	title    string
	priority int
	tags     []string
	dueDate  *time.Time
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

	// Current project for task filtering (nil = show tasks from all projects)
	currentProject *models.Project

	// Input for filtering
	searchInput textinput.Model

	// Items - using PaletteItem for unified representation of all searchable types
	allItems      []PaletteItem
	filteredItems []filteredPaletteItem

	// Navigation state
	cursor           int
	scrollOffset     int    // Current scroll offset (persisted between renders)
	scrollAnchor     int    // When >= 0, forces scroll to position this item at top of view, then resets to -1
	lastCursorItemID string // ID of item at cursor before filtering, for restoration

	// Action mode state - when true, showing inline actions for selected item
	actionMode         bool
	actionCursor       int
	actionScrollOffset int
	availableActions   []PaletteAction
	actionItem         PaletteItem // The item whose actions are being shown

	// Dimensions
	width        int
	height       int
	listWidth    int
	previewWidth int

	// Onboarding state
	showTips bool // Show tips until first keystroke

	// Feature toggles
	includeCompleted bool      // When true, include completed/canceled tasks (Ctrl+H)
	sortField        SortField // Current sort mode for results (Ctrl+O to cycle)

	// Inline task creation state
	inlineTaskMode   bool   // When true, showing inline task creation preview
	inlineTaskTitle  string // Parsed title from +query
	inlineTaskParsed *inlineTaskPreview

	// Category cursor persistence - remembers cursor position per category
	categoryCursors map[ItemCategory]int

	// View mode toggle - compact mode hides preview pane
	compactMode    bool
	userSetCompact bool // true when user explicitly toggled compact via Ctrl+P

	// Search history navigation
	searchHistory      []string // Copy of history from config
	searchHistoryIndex int      // -1 = not browsing history, 0+ = index in history

	// Pinned commands - set of pinned command names for quick lookup
	pinnedCommands map[string]bool

	// Multi-select state for bulk actions on tasks
	selectedTasks map[string]bool // Task IDs that are selected
}

// searchTermGroup represents a group of OR-ed alternatives within a search term.
// For example, "bug|fix" becomes a group with alternatives ["bug", "fix"].
// A simple term like "task" becomes a group with one alternative ["task"].
// Quoted strings are treated as exact phrase matches.
type searchTermGroup struct {
	alternatives []string // OR-ed alternatives (at least one must match)
	isNegative   bool     // If true, item must NOT match any alternative
	isExact      bool     // If true, require exact substring match (not fuzzy)
}

// fieldFilter represents a field-specific filter parsed from the query.
// Supports: priority:1-2, status:todo|progress, tag:name, section:name, @overdue, @today, @this-week
type fieldFilter struct {
	field string   // "priority", "status", "date", "tag", or "section"
	op    string   // "eq", "range", "in" for comparison type
	value string   // single value or range (e.g., "1-3")
	alt   []string // alternatives for OR logic (e.g., ["todo", "progress"])
}

// InlineTaskCreateMsg is sent when a task is created from the command palette.
type InlineTaskCreateMsg struct {
	Project  *models.Project
	Title    string
	Priority int
	Tags     []string
	DueDate  *time.Time
}

// toggleSavedFilterTabMsg toggles the ShowAsTab flag on a saved filter.
type toggleSavedFilterTabMsg struct {
	ID        string
	ShowAsTab bool
}

// CloseCommandPaletteMsg is sent when the command palette should close.
type CloseCommandPaletteMsg struct{ PopViewMsg }

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
// PreFilter can be set to pre-fill the search input (e.g., ":sv " for saved filters).
type ShowCommandPaletteMsg struct {
	PreFilter string
}

// CommandPaletteTaskActionMsg is sent when an inline action is performed on a task.
// This message triggers status changes (complete, start, block, cancel, reopen) that need to be
// saved to the project file.
type CommandPaletteTaskActionMsg struct {
	Task    *models.Task
	Project *models.Project
	Action  string // "complete", "start", "block", "cancel", "reopen"
}

// CommandPaletteBulkActionMsg is sent when a bulk action is performed on multiple selected tasks.
type CommandPaletteBulkActionMsg struct {
	Action string // "complete", "start"
	Tasks  []TaskProjectRef
}

// CommandPaletteClearSelectionMsg is sent to clear the multi-select state.
type CommandPaletteClearSelectionMsg struct{}

// ShowSaveSearchDialogMsg is sent to show a dialog for saving the current search.
type ShowSaveSearchDialogMsg struct {
	Query string // The search query to save
}

// SaveSearchMsg is sent when a search is saved with a name.
type SaveSearchMsg struct {
	Name  string // Name for the saved search
	Query string // The search query
}

// DeleteSavedSearchMsg is sent to delete a saved search by name.
type DeleteSavedSearchMsg struct {
	Name string // Name of the saved search to delete
}

// ExecuteSavedSearchMsg is sent to execute a saved search.
type ExecuteSavedSearchMsg struct {
	Query string // The search query to execute
}

// PaletteBulkTaskItem represents a task and its project for bulk operations from the palette.
type PaletteBulkTaskItem = TaskProjectRef

// PaletteBulkPriorityMsg is sent to set priority for multiple tasks from the palette.
type PaletteBulkPriorityMsg struct {
	Tasks []PaletteBulkTaskItem
}

// PaletteBulkTagMsg is sent to add tags to multiple tasks from the palette.
type PaletteBulkTagMsg struct {
	Tasks []PaletteBulkTaskItem
}

// PaletteBulkDueDateMsg is sent to set due date for multiple tasks from the palette.
type PaletteBulkDueDateMsg struct {
	Tasks []PaletteBulkTaskItem
}

// NavigateToViewMsg is sent when a view is selected from the command palette.
type NavigateToViewMsg struct {
	ViewType ViewType
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
	case ItemCategorySavedSearch:
		return weightCommands // Saved searches rank high like commands
	case ItemCategorySavedFilter:
		return weightCommands // Saved filters rank high like commands
	case ItemCategoryViews:
		return weightProjects // Views are navigational like projects
	default:
		return weightDefault
	}
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
	case ItemCategorySavedSearch:
		return "Saved Searches"
	case ItemCategoryNotes:
		return "Notes"
	case ItemCategorySavedFilter:
		return "Saved Filters"
	case ItemCategoryViews:
		return "Views"
	default:
		return "Other"
	}
}
