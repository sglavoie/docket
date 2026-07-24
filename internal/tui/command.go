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
	CategoryWorkflowRules CommandCategory = "Workflow Rules"
)

// Command represents an executable action in the command palette.
// Commands are first-class data structures that can be searched, displayed,
// and executed from the fuzzy-find command palette.
type Command struct {
	OperationID          OperationID               // Stable semantic identity used by the operation ledger
	Reachability         CommandReachability       // Final discoverability classification during consolidation
	FinalTarget          string                    // Binding, menu, hub, overlay, alias, or retirement target
	Name                 string                    // Display name shown in the palette (e.g., "Create Task")
	Description          string                    // Brief description of what the command does
	Shortcut             string                    // Keyboard shortcut if any (e.g., "n", "ctrl+n")
	Category             CommandCategory           // Logical grouping for organization
	Views                []PresentationID          // Renderer scope (empty = global/all views)
	Aliases              []string                  // Alternative names for discoverability
	RequiredCapabilities CapabilitySet             // Capabilities required by the migrated command
	ContextAvailable     func(CommandContext) bool // Optional selection/scope/entity predicate
	Available            func() bool               // Legacy dynamic availability predicate
	Handler              func() tea.Msg            // Execution closure returning a BubbleTea message
}
