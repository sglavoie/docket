package tui

import tea "charm.land/bubbletea/v2"

// CommandCategory represents a logical grouping of commands.
type CommandCategory string

const (
	CategoryNavigation    CommandCategory = "Navigation"
	CategoryActions       CommandCategory = "Actions"
	CategoryViews         CommandCategory = "Views"
	CategorySearch        CommandCategory = "Search"
	CategoryTasks         CommandCategory = "Tasks"
	CategoryProjects      CommandCategory = "Projects"
	CategoryView          CommandCategory = "View"
	CategoryFilters       CommandCategory = "Filters"
	CategoryHelp          CommandCategory = "Help"
	CategoryStatus        CommandCategory = "Status"
	CategoryDueDate       CommandCategory = "Due Date"
	CategoryNotes         CommandCategory = "Notes"
	CategoryPriority      CommandCategory = "Priority"
	CategorySort          CommandCategory = "Sort"
	CategoryLinkedTasks   CommandCategory = "Linked Tasks"
	CategoryStructure     CommandCategory = "Structure"
	CategoryMultiSelect   CommandCategory = "Multi-Select"
	CategoryFold          CommandCategory = "Fold"
	CategoryAttachments   CommandCategory = "Attachments"
	CategoryBlockers      CommandCategory = "Blockers"
	CategoryComments      CommandCategory = "Comments"
	CategoryDecisions     CommandCategory = "Decisions"
	CategoryUndoHistory   CommandCategory = "Undo History"
	CategoryWorkflowRules CommandCategory = "Workflow Rules"
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
	Aliases     []string        // Alternative names for discoverability (e.g., "create task", "add task")
	Handler     func() tea.Msg  // Execution closure returning a BubbleTea message
}

// projectTreeAndOutlinerViews returns the Views slice for commands that work
// in both the project tree and outliner views.
func projectTreeAndOutlinerViews() []ViewType {
	return []ViewType{ViewProjectTree, ViewOutliner}
}
