package tui

import tea "github.com/charmbracelet/bubbletea"

// CommandCategory represents a logical grouping of commands.
type CommandCategory string

const (
	CategoryNavigation CommandCategory = "Navigation"
	CategoryActions    CommandCategory = "Actions"
	CategoryViews      CommandCategory = "Views"
	CategorySearch     CommandCategory = "Search"
	CategoryTasks      CommandCategory = "Tasks"
	CategoryProjects   CommandCategory = "Projects"
	CategoryView       CommandCategory = "View"
	CategoryFilters    CommandCategory = "Filters"
	CategoryHelp       CommandCategory = "Help"
)

// Command represents an executable action in the command palette.
// Commands are first-class data structures that can be searched, displayed,
// and executed from the fuzzy-find command palette.
type Command struct {
	Name        string          // Display name shown in the palette (e.g., "Create Task")
	Description string          // Brief description of what the command does
	Shortcut    string          // Keyboard shortcut if any (e.g., "n", "ctrl+n")
	Category    CommandCategory // Logical grouping for organization
	Views       []ViewType      // Views where this command is available (empty = global/all views)
	Handler     func() tea.Msg  // Execution closure returning a BubbleTea message
}
