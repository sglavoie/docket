package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ViewType represents the different views in the TUI.
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewProjectList
	ViewProjectTree
	ViewProjectCreator
	ViewProjectEditor
	ViewTaskCreator
	ViewTaskEditor
	ViewSettings
	ViewHelp
	ViewDateInput
	ViewDependencySelector
	ViewDependencyViewer
	ViewFilter
	ViewTaskJumper
	ViewProjectPreview
	ViewDueDateOverview
	ViewTagManager
	ViewGlobalTagManager
	ViewTagFilterSelector
	ViewSectionFilterSelector
	ViewSectionPicker
	ViewSectionManager
	ViewPriorityPicker
	ViewContextPicker
	ViewCustomContextEditor
	ViewGlobalSearch
	ViewParentSelector
	ViewPriorityView
	ViewQuickCapture
	ViewBulkCapture
	ViewInboxProcessor
	ViewKanban
	ViewTagSelector
	ViewBulkTagSelector
	ViewUndoHistory
	ViewTaskComments
	ViewProjectCycleManager
	ViewCommandPalette
	ViewBulkEdit
	ViewSaveSearchInput
	ViewTrashedProjects
	ViewProjectSelector
	ViewDailyReview
	ViewWorkflowRules
	ViewWorkflowRuleCreator
	ViewWorkflowRuleEditor
	ViewRecurrenceEditor
	ViewTextEditor
	ViewGlobalInboxProcessor
	ViewFocusView
	ViewProjectFocusView
	ViewBlockers
	ViewTaskAttachments
	ViewLinkedTaskCreator
	ViewLinkedTaskSelector
	ViewCrossProjectLinker
	ViewKanbanTagFilterSelector
	ViewNotes
	ViewNoteTaskLinker
	ViewNoteTagSelector
	ViewNotesSearch
	ViewNoteTagFilterSelector
	ViewTaskActivity
	ViewRecentActivity
	ViewRecentActivityPreview
)

// AppModel is the main TUI model with a view stack.
type AppModel struct {
	config        *config.Config
	configManager *config.Manager

	// Persister for project persistence operations (enables testing with mocks)
	persister Persister

	// View stack for navigation
	viewStack []ViewType
	width     int
	height    int

	// View registry for centralized view management
	views *ViewRegistry

	// Command registry for centralized command lookup (used by command palette)
	commandRegistry *CommandRegistry

	// Typed accessors for views that need direct access (e.g., for HasActiveSearch)
	dashboard            *DashboardModel
	projectTree          *ProjectTreeModel
	helpView             *HelpViewModel
	taskJumper           *TaskJumperModel
	globalSearch         *GlobalSearchModel
	inboxProcessor       *InboxProcessorModel
	globalInboxProcessor *GlobalInboxProcessorModel
	kanban               *KanbanViewModel

	// Current project context
	currentProject *models.Project

	// State
	err           error
	quitting      bool
	statusMessage string // Temporary status message (auto-clears after timeout)

	// Loading state for async operations
	isLoading      bool
	loadingMessage string
	spinner        SpinnerModel

	// Pending bulk tasks for palette operations (priority, date assignment)
	pendingBulkTasks []PaletteBulkTaskItem

	// Pending bulk notes for multi-select operations
	pendingBulkNotes []*models.Note
}

// AppOption is a functional option for configuring AppModel.
type AppOption func(*AppModel)

// WithPersister sets a custom Persister for the AppModel.
// This is primarily used for testing with mock implementations.
func WithPersister(p Persister) AppOption {
	return func(app *AppModel) {
		app.persister = p
	}
}

// NewAppModel creates a new TUI application model.
func NewAppModel(cfg *config.Config, manager *config.Manager, opts ...AppOption) *AppModel {
	app := &AppModel{
		config:          cfg,
		configManager:   manager,
		persister:       NewProjectService(),
		viewStack:       []ViewType{ViewDashboard},
		width:           80,
		height:          24,
		views:           NewViewRegistry(),
		commandRegistry: NewCommandRegistry(),
		spinner:         NewSpinner(),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(app)
	}

	// Ensure sandbox project exists
	if cfg.EnsureSandboxProject() {
		_ = manager.Save(cfg)
	}

	// Initialize dashboard and register it
	app.dashboard = NewDashboardModel(cfg)
	app.views.Register(ViewDashboard, app.dashboard)

	// Register global commands
	app.registerGlobalCommands()

	// Load current project if set
	if current := cfg.GetCurrentProject(); current != nil {
		app.currentProject = current
	}

	return app
}

// makePriorityHandler creates a command handler that sets task priority.
func (m *AppModel) makePriorityHandler(priority int) func() tea.Msg {
	return func() tea.Msg {
		if m.projectTree == nil {
			return nil
		}
		visible := m.projectTree.visibleNodes()
		if m.projectTree.multiSelectMode && len(m.projectTree.selectedTasks) > 0 {
			for id := range m.projectTree.selectedTasks {
				if task := m.projectTree.project.GetTaskByID(id); task != nil {
					task.Priority = priority
					task.UpdatedAt = time.Now()
				}
			}
			m.projectTree.clearSelection()
			m.projectTree.buildTree()
			return nil
		}
		if CursorInBounds(m.projectTree.cursor, len(visible)) {
			task := visible[m.projectTree.cursor].Task
			task.Priority = priority
			task.UpdatedAt = time.Now()
			return TaskUpdatedMsg{Task: task}
		}
		return nil
	}
}

// registerGlobalCommands registers commands that are available from any view.
func (m *AppModel) registerGlobalCommands() {
	// Global commands (available in all views)
	globalCommands := []Command{
		{
			Name:        "help",
			Description: "Show help for keyboard shortcuts",
			Shortcut:    "?",
			Category:    CategoryHelp,
			Aliases:     []string{"keyboard shortcuts", "keybindings", "hotkeys"},
			Handler: func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewDashboard,
					ViewName:   "tt",
					Items:      getGlobalHelpItems(),
				}
			},
		},
		{
			Name:        "quit",
			Description: "Quit the application",
			Shortcut:    "ctrl+c",
			Category:    CategoryNavigation,
			Aliases:     []string{"exit", "close"},
			Handler: func() tea.Msg {
				return QuitMsg{}
			},
		},
		{
			Name:        "global search",
			Description: "Search tasks across all projects",
			Shortcut:    "/",
			Category:    CategorySearch,
			Aliases:     []string{"find tasks", "search all"},
			Handler: func() tea.Msg {
				return ShowGlobalSearchMsg{}
			},
		},
		{
			Name:        "settings",
			Description: "Open application settings",
			Shortcut:    ",",
			Category:    CategoryNavigation,
			Aliases:     []string{"preferences", "config", "options"},
			Handler: func() tea.Msg {
				return SettingsMsg{}
			},
		},
		{
			Name:        "back",
			Description: "Go back to previous view",
			Shortcut:    "esc",
			Category:    CategoryNavigation,
			Aliases:     []string{"go back", "return", "previous"},
			Handler: func() tea.Msg {
				return BackMsg{}
			},
		},
		{
			Name:        "workflow rules",
			Description: "Manage workflow automation rules",
			Shortcut:    "ctrl+W",
			Category:    CategoryNavigation,
			Aliases:     []string{"automation", "rules", "workflow"},
			Views:       []ViewType{ViewDashboard},
			Handler: func() tea.Msg {
				return ShowWorkflowRulesMsg{}
			},
		},
		{
			Name:        "create workflow rule",
			Description: "Create a new workflow automation rule",
			Category:    CategoryActions,
			Aliases:     []string{"new rule", "add rule", "new automation"},
			Handler: func() tea.Msg {
				return ShowWorkflowRuleCreatorMsg{}
			},
		},
	}

	// Filter and search commands (ProjectTree view)
	filterCommands := []Command{
		{
			Name:        "filter by tag",
			Description: "Open tag filter selector",
			Shortcut:    "T",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowTagFilterSelectorMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "filter by section",
			Description: "Open section filter selector",
			Shortcut:    "N",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowSectionFilterSelectorMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "view completed",
			Description: "Switch to Completed view",
			Shortcut:    "C",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeCompleted
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "toggle high priority",
			Description: "Filter high priority tasks only",
			Shortcut:    "P",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					if m.projectTree.filter.Priority == 0 {
						m.projectTree.filter.Priority = 1
					} else {
						m.projectTree.filter.Priority = 0
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "clear filters",
			Description: "Reset all task filters",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Type = ""
					m.projectTree.filter.Status = ""
					m.projectTree.filter.Priority = 0
					m.projectTree.filter.Tags = make(map[string]bool)
					m.projectTree.filter.Sections = make(map[string]bool)
					m.projectTree.filter.DueDate = 0
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "jump to task",
			Description: "Jump to task by ID or name",
			Shortcut:    "\\",
			Category:    CategorySearch,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowTaskJumperMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "tag manager",
			Description: "Manage project tags",
			Shortcut:    "ctrl+t",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowTagManagerMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "global tag manager",
			Description: "Manage tags across all projects",
			Shortcut:    "ctrl+t",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDashboard},
			Handler: func() tea.Msg {
				return ShowGlobalTagManagerMsg{}
			},
		},
		{
			Name:        "section manager",
			Description: "Manage project sections",
			Shortcut:    "ctrl+s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowSectionManagerMsg{ProjectTree: m.projectTree}
			},
		},
		{
			Name:        "add section",
			Description: "Add a new section to project",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"create section", "new section"},
			Handler: func() tea.Msg {
				return ShowSectionManagerMsg{ProjectTree: m.projectTree, StartAddMode: true}
			},
		},
		{
			Name:        "open data folder",
			Description: "Open root data folder in editor",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"open folder", "reveal data", "data directory"},
			Handler: func() tea.Msg {
				return OpenDataFolderMsg{Project: nil}
			},
		},
		{
			Name:        "open project folder",
			Description: "Open project data folder in editor",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"open folder", "reveal data", "data directory", "open data folder"},
			Handler: func() tea.Msg {
				return OpenDataFolderMsg{Project: m.currentProject}
			},
		},
		{
			Name:        "search projects",
			Description: "Filter projects by name",
			Shortcut:    "\\",
			Category:    CategorySearch,
			Views:       []ViewType{ViewDashboard},
			Handler: func() tea.Msg {
				if m.dashboard != nil {
					return m.dashboard.ActivateSearch()
				}
				return nil
			},
		},
	}

	// View commands
	viewCommands := []Command{
		{
			Name:        "due date overview",
			Description: "View tasks by due date",
			Shortcut:    "ctrl+d",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewDashboard},
			Handler: func() tea.Msg {
				return ShowDueDateOverviewMsg{}
			},
		},
		{
			Name:        "daily review",
			Description: "Daily planning and review view",
			Shortcut:    "r",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"planning", "today", "morning review"},
			Handler: func() tea.Msg {
				return ShowDailyReviewMsg{}
			},
		},
		{
			Name:        "latest activity",
			Description: "View recently modified tasks across all projects",
			Shortcut:    "l",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"recent", "activity", "latest", "recent tasks"},
			Handler: func() tea.Msg {
				return ShowRecentActivityMsg{}
			},
		},
		{
			Name:        "focus view",
			Description: "View all active-flagged tasks across projects",
			Shortcut:    "f",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"active tasks", "focus", "today tasks"},
			Handler: func() tea.Msg {
				return ShowFocusViewMsg{}
			},
		},
		{
			Name:        "priority view",
			Description: "View tasks by priority",
			Shortcut:    "p",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowPriorityViewMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "inbox processor",
			Description: "Process inbox items one by one",
			Shortcut:    "w",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowInboxProcessorMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "kanban board",
			Description: "View tasks in Kanban board layout",
			Shortcut:    "ctrl+k",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"kanban", "board", "columns"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowKanbanMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "global inbox processor",
			Description: "Process inbox items across all projects",
			Shortcut:    "w",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"global inbox", "all inbox", "process all inbox"},
			Handler: func() tea.Msg {
				return ShowGlobalInboxProcessorMsg{}
			},
		},
		{
			Name:        "undo history",
			Description: "View and restore from undo history",
			Shortcut:    "ctrl+z",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowUndoHistoryMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "toggle filter bar",
			Description: "Show/hide the filter bar",
			Shortcut:    "F",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filterBar.Toggle()
				}
				return nil
			},
		},
	}

	// Navigation commands
	navCommands := []Command{
		{
			Name:        "dashboard",
			Description: "Go to dashboard",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewProjectTree, ViewProjectList},
			Handler: func() tea.Msg {
				return BackMsg{}
			},
		},
		{
			Name:        "project cycle",
			Description: "Configure project cycle, jump to project",
			Shortcut:    "@",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree},
			Handler: func() tea.Msg {
				return ShowProjectCycleManagerMsg{}
			},
		},
		{
			Name:        "next project",
			Description: "Switch to next project in cycle",
			Shortcut:    "`",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree},
			Handler: func() tea.Msg {
				return CycleProjectMsg{Forward: true}
			},
		},
		{
			Name:        "previous project",
			Description: "Switch to previous project in cycle",
			Shortcut:    "~",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree},
			Handler: func() tea.Msg {
				return CycleProjectMsg{Forward: false}
			},
		},
	}

	// Task commands (ProjectTree view) - only message-based commands that work through handlers
	taskCommands := []Command{
		{
			Name:        "new task",
			Description: "Create a new task",
			Shortcut:    "n",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"create task", "add task"},
			Handler: func() tea.Msg {
				return CreateTaskMsg{}
			},
		},
		{
			Name:        "quick capture",
			Description: "Quick capture a single task",
			Shortcut:    "i",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"capture", "inbox"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowQuickCaptureMsg{Project: m.currentProject, AllProjects: m.config.Projects}
				}
				return nil
			},
		},
		{
			Name:        "bulk capture",
			Description: "Bulk capture multiple tasks",
			Shortcut:    "I",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"multi capture", "batch add"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowBulkCaptureMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "edit last task",
			Description: "Edit the most recently modified task",
			Shortcut:    "ctrl+l",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"edit recent", "last modified", "recent task"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					recentTasks := storage.GetRecentTasks(m.currentProject.Tasks, 1)
					if len(recentTasks) > 0 {
						return EditTaskMsg{Task: recentTasks[0]}
					}
				}
				return nil
			},
		},
		{
			Name:        "expand all",
			Description: "Expand all tasks in tree",
			Shortcut:    "E",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"unfold all", "open all"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					for _, node := range m.projectTree.nodes {
						node.Expanded = true
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "collapse all",
			Description: "Collapse all tasks in tree",
			Shortcut:    "z",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"fold all", "close all"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					for _, node := range m.projectTree.nodes {
						if node.Level > 0 {
							node.Expanded = false
						}
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "archive completed tasks",
			Description: "Move completed tasks to archive file",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree, ViewKanban},
			Aliases:     []string{"archive", "archive done", "archive completed"},
			Handler: func() tea.Msg {
				return ArchiveCompletedTasksMsg{}
			},
		},
	}

	// Project commands (Dashboard view)
	projectCommands := []Command{
		{
			Name:        "new project",
			Description: "Create a new project",
			Shortcut:    "n",
			Category:    CategoryProjects,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"create project", "add project"},
			Handler: func() tea.Msg {
				return CreateProjectMsg{}
			},
		},
		{
			Name:        "filter by context",
			Description: "Filter projects by context",
			Shortcut:    "c",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"context filter", "workspace"},
			Handler: func() tea.Msg {
				if m.dashboard != nil {
					return ShowContextPickerMsg{Dashboard: m.dashboard}
				}
				return nil
			},
		},
		{
			Name:        "toggle recent tasks",
			Description: "Show/hide recent tasks section",
			Shortcut:    "R",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"recent", "show recent"},
			Handler: func() tea.Msg {
				return ToggleRecentTasksMsg{}
			},
		},
		{
			Name:        "open sandbox",
			Description: "Open the sandbox project",
			Shortcut:    "S",
			Category:    CategoryProjects,
			Aliases:     []string{"sandbox", "scratch", "scratchpad"},
			Handler: func() tea.Msg {
				sandbox := m.config.GetSandboxProject()
				if sandbox != nil {
					return SelectProjectMsg{Project: sandbox}
				}
				return nil
			},
		},
		{
			Name:        "restore from trash",
			Description: "View and restore deleted projects",
			Shortcut:    "T",
			Category:    CategoryProjects,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"trash", "undelete", "recover project"},
			Handler: func() tea.Msg {
				return ShowTrashedProjectsMsg{}
			},
		},
	}

	// Task action commands (ProjectTree view) - operations on the selected task
	taskActionCommands := []Command{
		{
			Name:        "edit task",
			Description: "Edit selected task or bulk edit if multi-select",
			Shortcut:    "e",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"open editor", "modify task"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				if m.projectTree.multiSelectMode && len(m.projectTree.selectedTasks) > 0 {
					return ShowBulkEditMsg{
						Tasks:   m.projectTree.getSelectedTasks(),
						Project: m.currentProject,
					}
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return EditTaskMsg{Task: visible[m.projectTree.cursor].Task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "quick edit",
			Description: "Inline quick edit with modifier syntax",
			Shortcut:    "E",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"inline edit", "fast edit"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					m.projectTree.enterQuickEditMode(visible[m.projectTree.cursor].Task, false)
					return nil
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "rename task",
			Description: "Inline rename task title",
			Shortcut:    "r",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"rename", "change title"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					m.projectTree.enterQuickEditMode(visible[m.projectTree.cursor].Task, true)
					return nil
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "delete task",
			Description: "Delete selected task(s)",
			Shortcut:    "D",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"remove task", "trash task"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				if m.projectTree.multiSelectMode && len(m.projectTree.selectedTasks) > 0 {
					m.projectTree.confirmAction = ConfirmBulkDelete
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					if task.IsRecurring() {
						m.projectTree.confirmAction = ConfirmDeleteRecurring
					} else {
						m.projectTree.confirmAction = ConfirmDelete
					}
					m.projectTree.deleteTarget = task
					return nil
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "copy title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"yank title", "clipboard title"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return CopyTaskTitleMsg{Task: visible[m.projectTree.cursor].Task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "copy details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"yank details", "clipboard details"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return CopyTaskDetailsMsg{Task: visible[m.projectTree.cursor].Task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "copy with description",
			Description: "Copy task title and description to clipboard",
			Shortcut:    "ctrl+y",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"yank description", "clipboard description"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return CopyTaskTitleAndDescMsg{Task: visible[m.projectTree.cursor].Task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "move to project",
			Description: "Move task to another project",
			Shortcut:    "M",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"transfer task", "relocate task"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return ShowProjectSelectorForMoveMsg{
						Task:          visible[m.projectTree.cursor].Task,
						SourceProject: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "toggle active",
			Description: "Toggle active flag on task",
			Shortcut:    "*",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"star task", "flag task", "active flag"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					task.ToggleActive()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "toggle pin",
			Description: "Toggle pinned status on task",
			Shortcut:    "ctrl+p",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"pin task", "unpin task"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					task.TogglePinned()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "set due date",
			Description: "Open date picker for task",
			Shortcut:    "ctrl+d",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"deadline", "due date picker"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return SetDueDateMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "move to section",
			Description: "Pick section for task",
			Shortcut:    "ctrl+n",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"change section", "assign section"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return ShowSectionPickerMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "manage tags",
			Description: "Add or remove tags on task",
			Shortcut:    "ctrl+g",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"tag task", "edit tags"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return ShowTagSelectorMsg{
						Task:        task,
						Project:     m.currentProject,
						CurrentTags: task.Tags,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "manage dependencies",
			Description: "Manage task dependencies",
			Shortcut:    "ctrl+b",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add dependency", "link dependency"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return ManageDependenciesMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "view dependencies",
			Description: "View task dependency graph",
			Shortcut:    "R",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"dependency graph", "show dependencies"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return ShowDependencyViewerMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "edit content",
			Description: "Edit task description in external editor",
			Shortcut:    "ctrl+o",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"edit description", "open editor"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return EditTaskContentMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "task comments",
			Description: "View and add task comments",
			Shortcut:    "C",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"comments", "add comment"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return ShowTaskCommentsMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "task attachments",
			Description: "View and manage task attachments",
			Shortcut:    "A",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"attachments", "add attachment"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return ShowTaskAttachmentsMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "task activity",
			Description: "View task activity log",
			Shortcut:    "gh",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"activity log", "task history"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					return OpenTaskActivityMsg{
						Task:    visible[m.projectTree.cursor].Task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "undo",
			Description: "Undo last change",
			Shortcut:    "u",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"undo change", "revert"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return m.projectTree.performUndo()()
				}
				return StatusMsg("No project open")
			},
		},
		{
			Name:        "filter overlay",
			Description: "Open advanced filter overlay",
			Shortcut:    "ctrl+f",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"advanced filter", "filter panel"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return ShowFilterMsg{ProjectTree: m.projectTree}
				}
				return nil
			},
		},
	}

	// View mode commands (ProjectTree, via `g` prefix)
	viewModeCommands := []Command{
		{
			Name:        "active view",
			Description: "Switch to Active view mode",
			Shortcut:    "ga",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"view active", "active tasks"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeActive
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "next view",
			Description: "Switch to Next view mode",
			Shortcut:    "gn",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"view next", "next actions"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeNext
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "inbox view",
			Description: "Switch to Inbox view mode",
			Shortcut:    "gi",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"view inbox"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeInbox
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "waiting view",
			Description: "Switch to Waiting view mode",
			Shortcut:    "gw",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"view waiting", "waiting for"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeWaiting
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "someday view",
			Description: "Switch to Someday view mode",
			Shortcut:    "gs",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"view someday", "someday maybe"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeSomeday
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "cycle view mode",
			Description: "Cycle through all view modes",
			Shortcut:    "gv",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"next view mode", "toggle view"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = (m.projectTree.viewMode + 1) % (ViewModeCompleted + 1)
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
	}

	// Status commands (ProjectTree, via `,` prefix)
	statusCommands := []Command{
		{
			Name:        "start task",
			Description: "Set task status to in progress",
			Shortcut:    ",s",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"begin task", "in progress"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					visible := m.projectTree.visibleNodes()
					m.projectTree.handleStartTask(visible)
				}
				return nil
			},
		},
		{
			Name:        "complete task",
			Description: "Set task status to done",
			Shortcut:    ",d",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"done task", "finish task", "mark done"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					visible := m.projectTree.visibleNodes()
					m.projectTree.handleCompleteTask(visible)
				}
				return nil
			},
		},
		{
			Name:        "block task",
			Description: "Set task status to blocked",
			Shortcut:    ",b",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"blocked task", "mark blocked"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					visible := m.projectTree.visibleNodes()
					m.projectTree.handleBlockTask(visible)
				}
				return nil
			},
		},
		{
			Name:        "cancel task",
			Description: "Set task status to canceled",
			Shortcut:    ",x",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"canceled task", "mark canceled"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					visible := m.projectTree.visibleNodes()
					m.projectTree.handleCancelTask(visible)
				}
				return nil
			},
		},
		{
			Name:        "reopen task",
			Description: "Reopen a completed or canceled task",
			Shortcut:    ",o",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"reopen", "undo complete", "mark todo"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					visible := m.projectTree.visibleNodes()
					m.projectTree.handleReopenTask(visible)
				}
				return nil
			},
		},
	}

	// Due date quick commands (ProjectTree, via `d` prefix)
	dueDateCommands := []Command{
		{
			Name:        "due today",
			Description: "Set due date to today",
			Shortcut:    "dt",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"today", "due now"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					now := time.Now()
					today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
					task.DueDate = &today
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "due tomorrow",
			Description: "Set due date to tomorrow",
			Shortcut:    "dm",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"tomorrow"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					now := time.Now()
					tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
					task.DueDate = &tomorrow
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "due next week",
			Description: "Set due date to +7 days",
			Shortcut:    "dw",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"next week"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					now := time.Now()
					nextWeek := time.Date(now.Year(), now.Month(), now.Day()+7, 0, 0, 0, 0, now.Location())
					task.DueDate = &nextWeek
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "due next month",
			Description: "Set due date to +30 days",
			Shortcut:    "dn",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"next month"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					now := time.Now()
					nextMonth := time.Date(now.Year(), now.Month(), now.Day()+30, 0, 0, 0, 0, now.Location())
					task.DueDate = &nextMonth
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "clear due date",
			Description: "Remove due date from task",
			Shortcut:    "d0",
			Category:    CategoryDueDate,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"remove due date", "no due date"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					task.DueDate = nil
					task.UpdatedAt = time.Now()
					return TaskUpdatedMsg{Task: task}
				}
				return StatusMsg("No task selected")
			},
		},
	}

	// Notes commands (ProjectTree)
	notesCommands := []Command{
		{
			Name:        "show notes",
			Description: "Open notes view",
			Shortcut:    "[",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"notes", "view notes", "open notes"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowNotesMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "create note",
			Description: "Create a new note",
			Shortcut:    "]",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"new note", "add note"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowNoteCreatorMsg{Project: m.currentProject}
				}
				return nil
			},
		},
	}

	// Other missing commands
	otherCommands := []Command{
		{
			Name:        "project focus",
			Description: "Project-scoped focus view",
			Shortcut:    "f",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"focus mode", "project active tasks"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowProjectFocusViewMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "blockers view",
			Description: "Show blockers and blocked tasks",
			Shortcut:    "b",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"blockers", "blocked tasks"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowBlockersMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "blockers view",
			Description: "Show blockers across all projects",
			Shortcut:    "b",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"blockers", "all blockers"},
			Handler: func() tea.Msg {
				return ShowBlockersMsg{}
			},
		},
		{
			Name:        "reload project",
			Description: "Reload project data from storage",
			Shortcut:    "ctrl+r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"refresh", "reload data", "sync"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.clearSelection()
					_ = storage.LoadProject(m.projectTree.project)
					m.projectTree.buildTree()
				}
				return nil
			},
		},
		{
			Name:        "archive project",
			Description: "Archive or unarchive the current project",
			Shortcut:    "ctrl+a",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"unarchive project"},
			Handler: func() tea.Msg {
				if m.projectTree == nil || m.currentProject == nil {
					return nil
				}
				if m.currentProject.Sandbox {
					return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
				}
				if m.currentProject.Archived {
					m.projectTree.confirmAction = ConfirmUnarchive
				} else {
					m.projectTree.confirmAction = ConfirmArchive
				}
				return nil
			},
		},
	}

	// Priority commands (ProjectTree, via `!` prefix)
	priorityCommands := []Command{
		{
			Name:        "priority critical",
			Description: "Set task priority to Critical (1)",
			Shortcut:    "!1",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p1", "critical priority"},
			Handler:     m.makePriorityHandler(1),
		},
		{
			Name:        "priority high",
			Description: "Set task priority to High (2)",
			Shortcut:    "!2",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p2", "high priority"},
			Handler:     m.makePriorityHandler(2),
		},
		{
			Name:        "priority normal",
			Description: "Set task priority to Normal (3)",
			Shortcut:    "!3",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p3", "normal priority"},
			Handler:     m.makePriorityHandler(3),
		},
		{
			Name:        "priority low",
			Description: "Set task priority to Low (4)",
			Shortcut:    "!4",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p4", "low priority"},
			Handler:     m.makePriorityHandler(4),
		},
		{
			Name:        "priority someday",
			Description: "Set task priority to Someday (5)",
			Shortcut:    "!5",
			Category:    CategoryPriority,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"p5", "someday priority"},
			Handler:     m.makePriorityHandler(5),
		},
	}

	// Sort commands (ProjectTree)
	sortCommands := []Command{
		{
			Name:        "sort by priority",
			Description: "Sort tasks by priority",
			Shortcut:    "1",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"priority sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByPriority)
				}
				return nil
			},
		},
		{
			Name:        "sort by due date",
			Description: "Sort tasks by due date",
			Shortcut:    "2",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"due date sort", "deadline sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByDueDate)
				}
				return nil
			},
		},
		{
			Name:        "sort by created",
			Description: "Sort tasks by creation date",
			Shortcut:    "3",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"created sort", "creation date sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByCreatedAt)
				}
				return nil
			},
		},
		{
			Name:        "sort by status",
			Description: "Sort tasks by status",
			Shortcut:    "4",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"status sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByStatus)
				}
				return nil
			},
		},
		{
			Name:        "reset sort",
			Description: "Reset sort to default order",
			Shortcut:    "0",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"default sort", "clear sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.sortField = SortByDefault
					m.projectTree.sortAscending = true
					m.projectTree.buildTree()
				}
				return nil
			},
		},
	}

	// Structure commands (type changes and task movement)
	structureCommands := []Command{
		{
			Name:        "promote type",
			Description: "Promote task type (task → story → epic)",
			Shortcut:    "<",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"promote", "type up"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleTypeKeys("<", visible)
				return nil
			},
		},
		{
			Name:        "demote type",
			Description: "Demote task type (epic → story → task)",
			Shortcut:    ">",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"demote", "type down"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleTypeKeys(">", visible)
				return nil
			},
		},
		{
			Name:        "cycle leaf type left",
			Description: "Cycle leaf type left (task ← bug ← idea)",
			Shortcut:    "H",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"type left", "previous type"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleTypeKeys("H", visible)
				return nil
			},
		},
		{
			Name:        "cycle leaf type right",
			Description: "Cycle leaf type right (task → bug → idea)",
			Shortcut:    "L",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"type right", "next type"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleTypeKeys("L", visible)
				return nil
			},
		},
		{
			Name:        "move task up",
			Description: "Move task up among siblings",
			Shortcut:    "K",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"reorder up", "task up"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("K", visible)
				return nil
			},
		},
		{
			Name:        "move task down",
			Description: "Move task down among siblings",
			Shortcut:    "J",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"reorder down", "task down"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("J", visible)
				return nil
			},
		},
		{
			Name:        "indent task",
			Description: "Indent task (nest under task above)",
			Shortcut:    "→",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"nest task", "make child"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("right", visible)
				return nil
			},
		},
		{
			Name:        "outdent task",
			Description: "Outdent task (move to parent's level)",
			Shortcut:    "←",
			Category:    CategoryStructure,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"unnest task", "make sibling"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.handleMovementKeys("left", visible)
				return nil
			},
		},
	}

	// Section send commands (ProjectTree, via `g` prefix uppercase)
	sectionSendCommands := []Command{
		{
			Name:        "send to active",
			Description: "Move task to Active section",
			Shortcut:    "gA",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"active section", "move to active"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToActive(visible)
				return nil
			},
		},
		{
			Name:        "send to next",
			Description: "Move task to Next section",
			Shortcut:    "gN",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"next section", "move to next"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionNext, visible)
				return nil
			},
		},
		{
			Name:        "send to inbox",
			Description: "Move task to Inbox section",
			Shortcut:    "gI",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"inbox section", "move to inbox"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionInbox, visible)
				return nil
			},
		},
		{
			Name:        "send to waiting",
			Description: "Move task to Waiting section",
			Shortcut:    "gW",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"waiting section", "move to waiting"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionWaiting, visible)
				return nil
			},
		},
		{
			Name:        "send to someday",
			Description: "Move task to Someday section",
			Shortcut:    "gS",
			Category:    CategoryActions,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"someday section", "move to someday"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				m.projectTree.sendTasksToSection(models.SectionSomeday, visible)
				return nil
			},
		},
		{
			Name:        "completed view",
			Description: "Go to Completed view mode",
			Shortcut:    "gc",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"show completed", "view completed tasks"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = ViewModeCompleted
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "jump to home section",
			Description: "Jump to context node's home section",
			Shortcut:    "go",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"home section", "origin section"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					node := visible[m.projectTree.cursor]
					section := node.Task.Section
					if section == "" {
						section = models.SectionInbox
					}
					for i, n := range visible {
						if n.Task.Section == section && n.Task.IsRoot() {
							m.projectTree.cursor = i
							break
						}
					}
				}
				return nil
			},
		},
		{
			Name:        "cycle due date filter",
			Description: "Cycle through due date filter options",
			Shortcut:    "gd",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"due date filter", "filter due"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.DueDate = (m.projectTree.filter.DueDate + 1) % 5
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
	}

	// Linked task commands (ProjectTree, via `c` prefix)
	linkedTaskCommands := []Command{
		{
			Name:        "create blocker",
			Description: "Create a new blocker linked task",
			Shortcut:    "cb",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add blocker", "new blocker"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeBlocker}
				}
				return nil
			},
		},
		{
			Name:        "create dependent",
			Description: "Create a new dependent linked task",
			Shortcut:    "cd",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add dependent", "new dependent"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeDependent}
				}
				return nil
			},
		},
		{
			Name:        "create subtask",
			Description: "Create a new subtask linked task",
			Shortcut:    "cs",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add subtask", "new subtask"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeSubtask}
				}
				return nil
			},
		},
		{
			Name:        "create follow-up",
			Description: "Create a new follow-up linked task",
			Shortcut:    "cf",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"add follow-up", "new follow-up"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return CreateLinkedTaskMsg{SourceTask: task, Project: m.projectTree.project, LinkType: LinkTypeFollowUp}
				}
				return nil
			},
		},
		{
			Name:        "navigate linked tasks",
			Description: "Navigate to linked tasks",
			Shortcut:    "cn",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"show linked", "go to linked"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return ShowLinkedTaskSelectorMsg{Task: task, Project: m.projectTree.project}
				}
				return nil
			},
		},
		{
			Name:        "unlink task",
			Description: "Remove a linked task relationship",
			Shortcut:    "cx",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"remove link", "delete link"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return ShowLinkedTaskSelectorMsg{Task: task, Project: m.projectTree.project, ForUnlink: true}
				}
				return nil
			},
		},
		{
			Name:        "cross-project link",
			Description: "Link task across projects",
			Shortcut:    "cp",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"cross link", "inter-project link"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					return ShowCrossProjectLinkerMsg{SourceTask: task, SourceProject: m.projectTree.project}
				}
				return nil
			},
		},
	}

	// Multi-select commands (ProjectTree)
	multiSelectCommands := []Command{
		{
			Name:        "toggle multi-select",
			Description: "Toggle multi-select mode",
			Shortcut:    "v",
			Category:    CategoryMultiSelect,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"multi select", "select mode"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				if m.projectTree.multiSelectMode {
					m.projectTree.clearSelection()
				} else {
					m.projectTree.multiSelectMode = true
					visible := m.projectTree.visibleNodes()
					if CursorInBounds(m.projectTree.cursor, len(visible)) {
						m.projectTree.toggleSelection(visible[m.projectTree.cursor].Task)
					}
				}
				return nil
			},
		},
		{
			Name:        "select all visible",
			Description: "Select all visible tasks",
			Shortcut:    "a",
			Category:    CategoryMultiSelect,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"select all"},
			Handler: func() tea.Msg {
				if m.projectTree != nil && m.projectTree.multiSelectMode {
					m.projectTree.selectAllVisible()
				}
				return nil
			},
		},
		{
			Name:        "deselect all",
			Description: "Deselect all tasks",
			Shortcut:    "A",
			Category:    CategoryMultiSelect,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"clear selection", "deselect"},
			Handler: func() tea.Msg {
				if m.projectTree != nil && m.projectTree.multiSelectMode {
					m.projectTree.deselectAllVisible()
				}
				return nil
			},
		},
	}

	// Fold commands (ProjectTree, via `z` prefix)
	foldCommands := []Command{
		{
			Name:        "toggle fold",
			Description: "Toggle expand/collapse current node",
			Shortcut:    "za",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"toggle expand", "toggle collapse"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					node := visible[m.projectTree.cursor]
					if len(node.Task.Children) > 0 {
						node.Expanded = !node.Expanded
						m.projectTree.updateVisibility()
					}
				}
				return nil
			},
		},
		{
			Name:        "open fold",
			Description: "Expand current node",
			Shortcut:    "zo",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"expand node", "unfold"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					visible[m.projectTree.cursor].Expanded = true
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "close fold",
			Description: "Collapse current node",
			Shortcut:    "zc",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"collapse node", "fold"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					visible[m.projectTree.cursor].Expanded = false
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "expand all folds",
			Description: "Expand all nodes in tree",
			Shortcut:    "zR",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"unfold all", "open all folds"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					for _, node := range m.projectTree.nodes {
						node.Expanded = true
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "collapse all folds",
			Description: "Collapse all nodes in tree",
			Shortcut:    "zM",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"fold all", "close all folds"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					for _, node := range m.projectTree.nodes {
						if node.Level > 0 {
							node.Expanded = false
						}
					}
					m.projectTree.updateVisibility()
				}
				return nil
			},
		},
		{
			Name:        "focus subtree",
			Description: "Focus on current subtree only",
			Shortcut:    "zv",
			Category:    CategoryFold,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"zoom subtree", "subtree focus"},
			Handler: func() tea.Msg {
				if m.projectTree == nil {
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
					if task.Type.CanHaveChildren() {
						m.projectTree.focusedTask = task
						m.projectTree.cursor = 0
						m.projectTree.buildTree()
					}
				}
				return nil
			},
		},
	}

	// Filter bar F-key commands (ProjectTree)
	filterBarCommands := []Command{
		{
			Name:        "filter by type",
			Description: "Cycle through type filter options",
			Shortcut:    "t",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"type filter", "cycle type"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Type = cycleFilter(m.projectTree.filter.Type, models.AllTaskTypes())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "filter by status",
			Description: "Cycle through status filter options",
			Shortcut:    "S",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"status filter", "cycle status"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Status = cycleFilter(m.projectTree.filter.Status, models.AllTaskStatuses())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "completed view toggle",
			Description: "Toggle Completed view (F1)",
			Shortcut:    "F1",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f1 filter", "show completed"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					if m.projectTree.viewMode == ViewModeCompleted {
						m.projectTree.viewMode = ViewModeActive
					} else {
						m.projectTree.viewMode = ViewModeCompleted
					}
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "high priority filter",
			Description: "Toggle high priority filter (F2)",
			Shortcut:    "F2",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f2 filter", "priority filter toggle"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					if m.projectTree.filter.Priority == 0 {
						m.projectTree.filter.Priority = 1
					} else {
						m.projectTree.filter.Priority = 0
					}
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "type filter cycle",
			Description: "Cycle type filter (F3)",
			Shortcut:    "F3",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f3 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Type = cycleFilter(m.projectTree.filter.Type, models.AllTaskTypes())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "status filter cycle",
			Description: "Cycle status filter (F4)",
			Shortcut:    "F4",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f4 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Status = cycleFilter(m.projectTree.filter.Status, models.AllTaskStatuses())
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "tag filter selector",
			Description: "Open tag filter (F5)",
			Shortcut:    "F5",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f5 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return ShowTagFilterSelectorMsg{ProjectTree: m.projectTree}
				}
				return nil
			},
		},
		{
			Name:        "section filter selector",
			Description: "Open section filter (F6)",
			Shortcut:    "F6",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f6 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					return ShowSectionFilterSelectorMsg{ProjectTree: m.projectTree}
				}
				return nil
			},
		},
		{
			Name:        "due date filter cycle",
			Description: "Cycle due date filter (F7)",
			Shortcut:    "F7",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f7 filter"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.DueDate = (m.projectTree.filter.DueDate + 1) % 5
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
		{
			Name:        "actionable filter",
			Description: "Toggle actionable filter (F8)",
			Shortcut:    "F8",
			Category:    CategoryFilters,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"f8 filter", "actionable only"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.filter.Actionable = !m.projectTree.filter.Actionable
					m.projectTree.updateVisibility()
					m.projectTree.pruneInvisibleSelections()
					m.projectTree.clampCursor()
				}
				return nil
			},
		},
	}

	// Notes view commands (ViewNotes)
	notesViewCommands := []Command{
		{
			Name:        "edit note",
			Description: "Edit note in external editor",
			Shortcut:    "e",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"open note editor", "modify note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
			},
		},
		{
			Name:        "delete note",
			Description: "Delete selected note",
			Shortcut:    "d",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"remove note", "trash note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "undo delete note",
			Description: "Undo last note deletion",
			Shortcut:    "u",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"restore note", "undelete note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
		{
			Name:        "link note to task",
			Description: "Link note to a task",
			Shortcut:    "l",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"attach note", "note task link"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
			},
		},
		{
			Name:        "toggle note pin",
			Description: "Toggle pin status on note",
			Shortcut:    "p",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"pin note", "unpin note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
			},
		},
		{
			Name:        "edit note tags",
			Description: "Add or remove tags on note",
			Shortcut:    "t",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"tag note", "note tags"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
			},
		},
		{
			Name:        "cycle note tag filter",
			Description: "Open note tag filter selector",
			Shortcut:    "T",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"filter notes by tag"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}}
			},
		},
		{
			Name:        "search notes",
			Description: "Search notes by title or content",
			Shortcut:    "/",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"find note", "note search"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
			},
		},
		{
			Name:        "create note",
			Description: "Create a new note",
			Shortcut:    "n",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"new note", "add note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "rename note",
			Description: "Rename selected note inline",
			Shortcut:    "r",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"note rename", "change note title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "cycle note sort",
			Description: "Cycle sort mode (date/title/pinned)",
			Shortcut:    "s",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"sort notes", "note sort mode"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "clear tag filter",
			Description: "Clear all note tag filters",
			Shortcut:    "C",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"reset note filters", "clear note tags"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}}
			},
		},
		{
			Name:        "copy note title",
			Description: "Copy note title to clipboard",
			Shortcut:    "y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"yank note title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy note body",
			Description: "Copy note body to clipboard",
			Shortcut:    "Y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"yank note body"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy note title and body",
			Description: "Copy title and body to clipboard",
			Shortcut:    "ctrl+y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"yank note full"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
		{
			Name:        "toggle multi-select",
			Description: "Toggle note multi-select mode",
			Shortcut:    "V",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"multi select notes"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}}
			},
		},
		{
			Name:        "view note",
			Description: "View selected note",
			Shortcut:    "enter",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"open note", "read note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEnter}
			},
		},
	}

	// Kanban view commands (ViewKanban)
	kanbanCommands := []Command{
		{
			Name:        "move task left column",
			Description: "Move task to left column",
			Shortcut:    "{",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"column left", "move left"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}}
			},
		},
		{
			Name:        "move task right column",
			Description: "Move task to right column",
			Shortcut:    "}",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"column right", "move right"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}}
			},
		},
		{
			Name:        "swap task up in column",
			Description: "Swap task with one above in column",
			Shortcut:    "K",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"reorder up column", "move up column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
			},
		},
		{
			Name:        "swap task down in column",
			Description: "Swap task with one below in column",
			Shortcut:    "J",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"reorder down column", "move down column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
			},
		},
		{
			Name:        "go to active column",
			Description: "Jump to Active column",
			Shortcut:    "ga",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"active column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to next column",
			Description: "Jump to Next column",
			Shortcut:    "gn",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"next column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to inbox column",
			Description: "Jump to Inbox column",
			Shortcut:    "gi",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"inbox column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to waiting column",
			Description: "Jump to Waiting column",
			Shortcut:    "gw",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"waiting column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to someday column",
			Description: "Jump to Someday column",
			Shortcut:    "gs",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"someday column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to active section",
			Description: "Move task to Active section",
			Shortcut:    "gA",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send active"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to next section",
			Description: "Move task to Next section",
			Shortcut:    "gN",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send next"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to inbox section",
			Description: "Move task to Inbox section",
			Shortcut:    "gI",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to waiting section",
			Description: "Move task to Waiting section",
			Shortcut:    "gW",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send waiting"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to someday section",
			Description: "Move task to Someday section",
			Shortcut:    "gS",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send someday"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
	}

	// Extended Kanban commands — task operations, status, priority, due dates,
	// linked tasks, copy, flags, tags/sections, sort, filters, fold, multi-select,
	// type management, structure, views/nav
	kanbanExtCommands := []Command{
		// Task operations
		{Name: "edit task", Description: "Edit selected task", Shortcut: "e", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"open editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "quick edit", Description: "Inline quick edit with modifier syntax", Shortcut: "E", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"inline edit"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "rename task", Description: "Inline rename task title", Shortcut: "r", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"rename"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}} }},
		{Name: "delete task", Description: "Delete selected task", Shortcut: "D", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"remove task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}} }},
		{Name: "new task", Description: "Create a new task", Shortcut: "n", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"create task", "add task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}} }},
		{Name: "quick capture", Description: "Quick capture a single task", Shortcut: "i", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"capture", "inbox"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}} }},
		{Name: "bulk capture", Description: "Bulk capture multiple tasks", Shortcut: "I", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"multi capture"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}} }},
		{Name: "view task detail", Description: "Open task detail view", Shortcut: "enter", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"open task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "task comments", Description: "View and add task comments", Shortcut: "C", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"comments"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}} }},
		{Name: "task attachments", Description: "View and manage task attachments", Shortcut: "A", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"attachments"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}} }},
		{Name: "dependency viewer", Description: "View task dependency graph", Shortcut: "R", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"dependency graph"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}} }},
		{Name: "edit content", Description: "Edit task description in external editor", Shortcut: "ctrl+o", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"edit description"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlO} }},
		{Name: "move to project", Description: "Move task to another project", Shortcut: "M", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"transfer task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}} }},
		{Name: "undo", Description: "Undo last change", Shortcut: "u", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"undo change", "revert"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}} }},
		{Name: "undo history", Description: "View undo history", Shortcut: "ctrl+z", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"undo list"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlZ} }},
		// Status (comma prefix)
		{Name: "start task", Description: "Set task status to in progress", Shortcut: ",s", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"begin task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "complete task", Description: "Set task status to done", Shortcut: ",d", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"done task", "finish task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "block task", Description: "Set task status to blocked", Shortcut: ",b", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"blocked task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "cancel task", Description: "Set task status to canceled", Shortcut: ",x", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"canceled task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "reopen task", Description: "Reopen a completed or canceled task", Shortcut: ",o", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"reopen"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		// Priority (! prefix)
		{Name: "priority critical", Description: "Set priority to Critical (1)", Shortcut: "!1", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p1"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority high", Description: "Set priority to High (2)", Shortcut: "!2", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p2"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority normal", Description: "Set priority to Normal (3)", Shortcut: "!3", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p3"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority low", Description: "Set priority to Low (4)", Shortcut: "!4", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p4"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority someday", Description: "Set priority to Someday (5)", Shortcut: "!5", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p5"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		// Due date (d prefix)
		{Name: "due today", Description: "Set due date to today", Shortcut: "dt", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"today"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "due tomorrow", Description: "Set due date to tomorrow", Shortcut: "dm", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"tomorrow"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "due next week", Description: "Set due date to +7 days", Shortcut: "dw", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"next week"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "due next month", Description: "Set due date to +30 days", Shortcut: "dn", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"next month"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "clear due date", Description: "Remove due date from task", Shortcut: "d0", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"remove due date"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		// Linked tasks (c prefix)
		{Name: "create blocker", Description: "Create blocker linked task", Shortcut: "cb", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "create dependent", Description: "Create dependent linked task", Shortcut: "cd", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add dependent"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "create subtask", Description: "Create subtask linked task", Shortcut: "cs", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add subtask"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "create follow-up", Description: "Create follow-up linked task", Shortcut: "cf", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add follow-up"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "navigate linked tasks", Description: "Navigate to linked tasks", Shortcut: "cn", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"show linked"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "unlink task", Description: "Remove linked task relationship", Shortcut: "cx", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"remove link"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "cross-project link", Description: "Link task across projects", Shortcut: "cp", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"cross link"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		// Copy
		{Name: "copy title", Description: "Copy task title to clipboard", Shortcut: "y", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"yank title"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}} }},
		{Name: "copy details", Description: "Copy task with details to clipboard", Shortcut: "Y", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"yank details"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}} }},
		{Name: "copy with description", Description: "Copy task title and description", Shortcut: "ctrl+y", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"yank description"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlY} }},
		// Flags
		{Name: "toggle active", Description: "Toggle active flag on task", Shortcut: "*", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"star task", "flag task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'*'}} }},
		{Name: "toggle pin", Description: "Toggle pinned status on task", Shortcut: "ctrl+p", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"pin task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlP} }},
		// Tags/sections
		{Name: "manage tags", Description: "Add or remove tags on task", Shortcut: "ctrl+g", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"tag task", "edit tags"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlG} }},
		{Name: "section picker", Description: "Pick section for task", Shortcut: "ctrl+n", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"change section"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlN} }},
		{Name: "tag manager", Description: "Manage project tags", Shortcut: "ctrl+t", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"manage project tags"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlT} }},
		{Name: "section manager", Description: "Manage project sections", Shortcut: "ctrl+s", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"manage sections"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlS} }},
		{Name: "tag filter", Description: "Filter tasks by tag", Shortcut: "T", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"filter tag"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}} }},
		{Name: "manage dependencies", Description: "Manage task dependencies", Shortcut: "ctrl+b", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"add dependency"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlB} }},
		// Sort
		{Name: "sort by priority", Description: "Sort tasks by priority", Shortcut: "1", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"priority sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}} }},
		{Name: "sort by due date", Description: "Sort tasks by due date", Shortcut: "2", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"due date sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}} }},
		{Name: "sort by created", Description: "Sort tasks by creation date", Shortcut: "3", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"created sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}} }},
		{Name: "sort by status", Description: "Sort tasks by status", Shortcut: "4", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"status sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}} }},
		{Name: "reset sort", Description: "Reset sort to default order", Shortcut: "0", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"default sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}} }},
		// Filters
		{Name: "toggle filter bar", Description: "Show/hide the filter bar", Shortcut: "F", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"filter bar"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}} }},
		{Name: "filter overlay", Description: "Open active filters overlay", Shortcut: "ctrl+f", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"advanced filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlF} }},
		{Name: "completed view toggle", Description: "Toggle completed view (F1)", Shortcut: "F1", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"show completed"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF1} }},
		{Name: "high priority filter", Description: "Toggle high priority filter (F2)", Shortcut: "F2", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"priority filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF2} }},
		{Name: "type filter cycle", Description: "Cycle type filter (F3)", Shortcut: "F3", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f3 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF3} }},
		{Name: "status filter cycle", Description: "Cycle status filter (F4)", Shortcut: "F4", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f4 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF4} }},
		{Name: "tag filter selector", Description: "Open tag filter (F5)", Shortcut: "F5", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f5 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF5} }},
		{Name: "due date filter cycle", Description: "Cycle due date filter (F7)", Shortcut: "F7", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f7 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF7} }},
		{Name: "actionable filter", Description: "Toggle actionable filter (F8)", Shortcut: "F8", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f8 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF8} }},
		// Fold (z prefix)
		{Name: "toggle fold", Description: "Toggle expand/collapse current node", Shortcut: "za", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"toggle expand"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		{Name: "open fold", Description: "Expand current node", Shortcut: "zo", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"expand node"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		{Name: "close fold", Description: "Collapse current node", Shortcut: "zc", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"collapse node"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		{Name: "focus subtree", Description: "Focus on current subtree only", Shortcut: "zv", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"zoom subtree"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		// Multi-select
		{Name: "toggle multi-select", Description: "Toggle multi-select mode", Shortcut: "v", Category: CategoryMultiSelect, Views: []ViewType{ViewKanban}, Aliases: []string{"multi select"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}} }},
		{Name: "select all in column", Description: "Select all tasks in column", Shortcut: "a", Category: CategoryMultiSelect, Views: []ViewType{ViewKanban}, Aliases: []string{"select all column"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "toggle selection", Description: "Toggle selection of current task", Shortcut: "space", Category: CategoryMultiSelect, Views: []ViewType{ViewKanban}, Aliases: []string{"select task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeySpace} }},
		// Type management
		{Name: "promote type", Description: "Promote task type (task → story → epic)", Shortcut: "<", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"promote", "type up"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}} }},
		{Name: "demote type", Description: "Demote task type (epic → story → task)", Shortcut: ">", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"demote", "type down"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}} }},
		{Name: "cycle leaf type left", Description: "Cycle leaf type left", Shortcut: "H", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"type left"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}} }},
		{Name: "cycle leaf type right", Description: "Cycle leaf type right", Shortcut: "L", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"type right"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}} }},
		// Structure
		{Name: "outdent task", Description: "Outdent task (move to parent's level)", Shortcut: "←", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"unnest task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyLeft} }},
		{Name: "indent task", Description: "Indent task (nest under task above)", Shortcut: "→", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"nest task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRight} }},
		// Views/nav
		{Name: "show notes", Description: "Open notes view", Shortcut: "[", Category: CategoryNotes, Views: []ViewType{ViewKanban}, Aliases: []string{"notes"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}} }},
		{Name: "create note", Description: "Create a new note", Shortcut: "]", Category: CategoryNotes, Views: []ViewType{ViewKanban}, Aliases: []string{"new note"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}} }},
		{Name: "blockers view", Description: "Show blockers and blocked tasks", Shortcut: "b", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"blockers"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}} }},
		{Name: "priority view", Description: "View tasks by priority", Shortcut: "p", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"priority"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}} }},
		{Name: "inbox processor", Description: "Process inbox items", Shortcut: "w", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"inbox"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}} }},
		{Name: "focus subtree view", Description: "Focus on current subtree", Shortcut: "f", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"focus mode"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}} }},
		{Name: "task jumper", Description: "Jump to task by ID or name", Shortcut: "\\", Category: CategorySearch, Views: []ViewType{ViewKanban}, Aliases: []string{"find task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\\'}} }},
		{Name: "cycle forward project", Description: "Switch to next project in cycle", Shortcut: "`", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"next project"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}} }},
		{Name: "cycle backward project", Description: "Switch to previous project in cycle", Shortcut: "~", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"previous project"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'~'}} }},
		{Name: "project cycle manager", Description: "Configure project cycle", Shortcut: "@", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"cycle manager"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}} }},
		{Name: "archive completed", Description: "Move completed tasks to archive", Shortcut: "ctrl+a", Category: CategoryActions, Views: []ViewType{ViewKanban}, Aliases: []string{"archive"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlA} }},
		{Name: "edit last modified", Description: "Edit the most recently modified task", Shortcut: "ctrl+l", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"recent task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlL} }},
		{Name: "reload project", Description: "Reload project data from storage", Shortcut: "ctrl+r", Category: CategoryActions, Views: []ViewType{ViewKanban}, Aliases: []string{"refresh"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlR} }},
	}

	// Blockers view commands (ViewBlockers)
	blockersCommands := []Command{
		{Name: "add blocker", Description: "Add a new blocker", Shortcut: "a", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"new blocker", "n"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "edit blocker", Description: "Edit selected blocker", Shortcut: "e", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"modify blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "edit in editor", Description: "Edit blocker in external editor", Shortcut: "E", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "toggle resolve", Description: "Toggle resolve/unresolve blocker", Shortcut: "R", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"resolve blocker", "unresolve"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}} }},
		{Name: "delete blocker", Description: "Delete selected blocker", Shortcut: "d", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"remove blocker", "x"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "toggle show resolved", Description: "Toggle showing resolved blockers", Shortcut: "tab", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"show resolved"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyTab} }},
		{Name: "expand blocker", Description: "Expand/view blocker details", Shortcut: "enter", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"view blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
	}

	// Task Comments view commands (ViewTaskComments)
	taskCommentsCommands := []Command{
		{Name: "add comment", Description: "Add a new comment", Shortcut: "a", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"new comment", "n"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "edit comment", Description: "Edit selected comment", Shortcut: "e", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"modify comment"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "edit in editor", Description: "Edit comment in external editor", Shortcut: "E", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "delete comment", Description: "Delete selected comment", Shortcut: "d", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"remove comment", "x"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "expand comment", Description: "Expand/view comment details", Shortcut: "enter", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"view comment"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
	}

	// Undo History view commands (ViewUndoHistory)
	undoHistoryCommands := []Command{
		{Name: "perform undo", Description: "Undo selected entry or view detail", Shortcut: "enter", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"undo entry", "view detail"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "switch pane", Description: "Switch between entry list and diff pane", Shortcut: "tab", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"toggle pane"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyTab} }},
		{Name: "scroll diff left", Description: "Scroll diff pane left", Shortcut: "h", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"diff left"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}} }},
		{Name: "scroll diff right", Description: "Scroll diff pane right", Shortcut: "l", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"diff right"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}} }},
	}

	// Global Tag Manager view commands (ViewGlobalTagManager)
	globalTagManagerCommands := []Command{
		{Name: "navigate to tag project", Description: "Navigate to tag's project", Shortcut: "enter", Category: CategoryActions, Views: []ViewType{ViewGlobalTagManager}, Aliases: []string{"open tag project"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "rename tag", Description: "Rename selected tag", Shortcut: "r", Category: CategoryActions, Views: []ViewType{ViewGlobalTagManager}, Aliases: []string{"edit tag name"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}} }},
	}

	// Task Attachments view commands (ViewTaskAttachments)
	attachmentCommands := []Command{
		{
			Name:        "add attachment",
			Description: "Add a new attachment to task",
			Shortcut:    "a",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"attach file", "new attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
			},
		},
		{
			Name:        "open attachment",
			Description: "Open selected attachment",
			Shortcut:    "o",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"view attachment", "launch attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}
			},
		},
		{
			Name:        "open attachments directory",
			Description: "Open directory containing attachments",
			Shortcut:    "O",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"reveal attachments", "attachments folder"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}
			},
		},
		{
			Name:        "open original path",
			Description: "Open attachment at its original path",
			Shortcut:    "ctrl+o",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"original location", "source path"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlO}
			},
		},
		{
			Name:        "delete attachment",
			Description: "Delete selected attachment",
			Shortcut:    "d",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"remove attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "unlink attachment",
			Description: "Unlink attachment but keep file",
			Shortcut:    "u",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"detach file", "keep and unlink"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
		{
			Name:        "copy attachment path",
			Description: "Copy attachment path to clipboard",
			Shortcut:    "y",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"yank path", "clipboard path"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
	}

	// Focus view commands (ViewFocusView, ViewProjectFocusView)
	focusViewCommands := []Command{
		{
			Name:        "remove from focus",
			Description: "Remove active flag from task",
			Shortcut:    "*",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"unflag task", "unstar task", "deactivate"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'*'}}
			},
		},
		{
			Name:        "go to task in project",
			Description: "Navigate to task in project tree",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"open in project", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}

	// Daily review commands (ViewDailyReview)
	dailyReviewCommands := []Command{
		{
			Name:        "focus on due today",
			Description: "Focus on due today section",
			Shortcut:    "sd",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"due today section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on top priority",
			Description: "Focus on top priority section",
			Shortcut:    "st",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"top priority section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on stale tasks",
			Description: "Focus on stale tasks section",
			Shortcut:    "ss",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"stale section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on recently logged",
			Description: "Focus on recently logged section",
			Shortcut:    "sl",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"recently logged section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on flagged tasks",
			Description: "Focus on flagged/active tasks section",
			Shortcut:    "sf",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"flagged section", "active section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on overdue tasks",
			Description: "Focus on overdue tasks section",
			Shortcut:    "so",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"overdue section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "collapse/expand section",
			Description: "Toggle section collapse/expand",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"fold section", "toggle section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyTab}
			},
		},
		{
			Name:        "next section",
			Description: "Jump to next section",
			Shortcut:    "n",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"section forward"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "previous section",
			Description: "Jump to previous section",
			Shortcut:    "N",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"section backward"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}

	// Priority view commands (ViewPriorityView)
	priorityViewCommands := []Command{
		{
			Name:        "start suggested task",
			Description: "Start the suggested task",
			Shortcut:    "s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"begin suggestion", "accept suggestion"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "dismiss suggestion",
			Description: "Dismiss the current suggestion",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"skip suggestion", "reject suggestion"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
			},
		},
		{
			Name:        "go to task in tree",
			Description: "Navigate to task in project tree",
			Shortcut:    "gt",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"open in tree", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "toggle waiting section",
			Description: "Toggle waiting-on section visibility",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"show waiting", "hide waiting"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyTab}
			},
		},
		{
			Name:        "refresh priority view",
			Description: "Refresh priority data",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"reload priorities"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}

	// Inbox processor commands (ViewInboxProcessor)
	inboxProcessorCommands := []Command{
		{
			Name:        "skip task",
			Description: "Skip current inbox task",
			Shortcut:    "n",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"next inbox task", "pass"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "assign section",
			Description: "Assign task to a section",
			Shortcut:    "s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"pick section", "move to section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "send to waiting",
			Description: "Send task to Waiting section",
			Shortcut:    "w",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"waiting", "defer task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
			},
		},
		{
			Name:        "manage inbox task tags",
			Description: "Add or remove tags on task",
			Shortcut:    "t",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"tag inbox task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
			},
		},
		{
			Name:        "create subtask from inbox",
			Description: "Create a subtask for current task",
			Shortcut:    "c",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"add subtask inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
			},
		},
		{
			Name:        "set inbox task due date",
			Description: "Set due date for current task",
			Shortcut:    "D",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox due date", "deadline"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
			},
		},
		{
			Name:        "manage inbox dependencies",
			Description: "Manage task dependencies",
			Shortcut:    "b",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox dependencies", "add dependency"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
			},
		},
		{
			Name:        "delete inbox task",
			Description: "Delete current inbox task",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"remove inbox task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
			},
		},
		{
			Name:        "undo inbox action",
			Description: "Undo last inbox processing action",
			Shortcut:    "u",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox undo", "revert inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
	}

	// Due date overview commands (ViewDueDateOverview)
	dueDateOverviewCommands := []Command{
		{
			Name:        "refresh due dates",
			Description: "Refresh due date overview data",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"reload due dates"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "go to task project",
			Description: "Navigate to task in project tree",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"open in project", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}

	// Workflow rules view commands (ViewWorkflowRules)
	workflowRulesCommands := []Command{
		{
			Name:        "create workflow rule",
			Description: "Create a new workflow rule",
			Shortcut:    "n",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"new rule", "add rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "edit workflow rule",
			Description: "Edit selected workflow rule",
			Shortcut:    "e",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"modify rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
			},
		},
		{
			Name:        "delete workflow rule",
			Description: "Delete selected workflow rule",
			Shortcut:    "d",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"remove rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "view workflow rule",
			Description: "View rule details",
			Shortcut:    "enter",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"open rule", "rule details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEnter}
			},
		},
	}

	// Register all command batches
	_ = m.commandRegistry.RegisterBatch(globalCommands)
	_ = m.commandRegistry.RegisterBatch(filterCommands)
	_ = m.commandRegistry.RegisterBatch(viewCommands)
	_ = m.commandRegistry.RegisterBatch(navCommands)
	_ = m.commandRegistry.RegisterBatch(taskCommands)
	_ = m.commandRegistry.RegisterBatch(projectCommands)
	_ = m.commandRegistry.RegisterBatch(taskActionCommands)
	_ = m.commandRegistry.RegisterBatch(viewModeCommands)
	_ = m.commandRegistry.RegisterBatch(statusCommands)
	_ = m.commandRegistry.RegisterBatch(dueDateCommands)
	_ = m.commandRegistry.RegisterBatch(notesCommands)
	_ = m.commandRegistry.RegisterBatch(otherCommands)
	_ = m.commandRegistry.RegisterBatch(priorityCommands)
	_ = m.commandRegistry.RegisterBatch(sortCommands)
	_ = m.commandRegistry.RegisterBatch(structureCommands)
	_ = m.commandRegistry.RegisterBatch(sectionSendCommands)
	_ = m.commandRegistry.RegisterBatch(linkedTaskCommands)
	_ = m.commandRegistry.RegisterBatch(multiSelectCommands)
	_ = m.commandRegistry.RegisterBatch(foldCommands)
	_ = m.commandRegistry.RegisterBatch(filterBarCommands)
	_ = m.commandRegistry.RegisterBatch(notesViewCommands)
	_ = m.commandRegistry.RegisterBatch(kanbanCommands)
	_ = m.commandRegistry.RegisterBatch(attachmentCommands)
	_ = m.commandRegistry.RegisterBatch(focusViewCommands)
	_ = m.commandRegistry.RegisterBatch(dailyReviewCommands)
	_ = m.commandRegistry.RegisterBatch(priorityViewCommands)
	_ = m.commandRegistry.RegisterBatch(inboxProcessorCommands)
	_ = m.commandRegistry.RegisterBatch(dueDateOverviewCommands)
	_ = m.commandRegistry.RegisterBatch(kanbanExtCommands)
	_ = m.commandRegistry.RegisterBatch(blockersCommands)
	_ = m.commandRegistry.RegisterBatch(taskCommentsCommands)
	_ = m.commandRegistry.RegisterBatch(undoHistoryCommands)
	_ = m.commandRegistry.RegisterBatch(globalTagManagerCommands)
	_ = m.commandRegistry.RegisterBatch(workflowRulesCommands)

	sidebarCommands := []Command{
		{
			Name:        "enable sidebar for all projects",
			Description: "Turn on sidebar for every active project",
			Category:    CategoryView,
			Views:       []ViewType{},
			Aliases:     []string{"sidebar on all", "show sidebar all"},
			Handler: func() tea.Msg {
				for _, p := range m.config.GetActiveProjects() {
					state, _ := m.config.GetProjectViewState(p.ID)
					state.SidebarEnabled = true
					m.config.SetProjectViewState(p.ID, state)
				}
				_ = m.configManager.Save(m.config)
				if m.projectTree != nil {
					m.projectTree.sidebar.SetEnabled(true)
				}
				return nil
			},
		},
		{
			Name:        "disable sidebar for all projects",
			Description: "Turn off sidebar for every active project",
			Category:    CategoryView,
			Views:       []ViewType{},
			Aliases:     []string{"sidebar off all", "hide sidebar all"},
			Handler: func() tea.Msg {
				for _, p := range m.config.GetActiveProjects() {
					state, _ := m.config.GetProjectViewState(p.ID)
					state.SidebarEnabled = false
					m.config.SetProjectViewState(p.ID, state)
				}
				_ = m.configManager.Save(m.config)
				if m.projectTree != nil {
					m.projectTree.sidebar.SetEnabled(false)
				}
				return nil
			},
		},
	}
	_ = m.commandRegistry.RegisterBatch(sidebarCommands)
}

// getGlobalHelpItems returns help items that apply across all views.
func getGlobalHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "?", Description: "Show help", Category: "General"},
		{Key: "q", Description: "Quit / go back", Category: "General"},
		{Key: "esc", Description: "Go back / cancel", Category: "General"},
		{Key: "ctrl+c", Description: "Quit application", Category: "General"},
		{Key: "/", Description: "Global search", Category: "Navigation"},
		{Key: ":", Description: "Command palette", Category: "Navigation"},
		{Key: "j/k", Description: "Move cursor down/up", Category: "Navigation"},
		{Key: "↑/↓", Description: "Move cursor up/down", Category: "Navigation"},
		{Key: "g", Description: "Go to first item", Category: "Navigation"},
		{Key: "G", Description: "Go to last item", Category: "Navigation"},
		{Key: "Enter", Description: "Select / confirm", Category: "Actions"},
		{Key: ",", Description: "Settings", Category: "Actions"},
	}
}

func (m *AppModel) Init() tea.Cmd {
	return nil
}

// GetCommandRegistry returns the command registry for the application.
// This is used by the command palette and for testing.
func (m *AppModel) GetCommandRegistry() *CommandRegistry {
	return m.commandRegistry
}

// isNotesFullscreen returns true when the notes view is in fullscreen pane mode.
func (m *AppModel) isNotesFullscreen() bool {
	if m.currentView() != ViewNotes {
		return false
	}
	if v := m.views.Get(ViewNotes); v != nil {
		if nv, ok := v.(*NotesViewModel); ok {
			return nv.fullscreenPane
		}
	}
	return false
}

// currentView returns the current view type.
func (m *AppModel) currentView() ViewType {
	if len(m.viewStack) == 0 {
		return ViewDashboard
	}
	return m.viewStack[len(m.viewStack)-1]
}

// pushView pushes a new view onto the stack.
func (m *AppModel) pushView(view ViewType) {
	m.viewStack = append(m.viewStack, view)
}

// popView removes the top view from the stack.
func (m *AppModel) popView() {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
}

// refreshProjectTree recreates the project tree view with current project.
// Preserves filter/sort state across the refresh.
func (m *AppModel) refreshProjectTree() {
	if m.currentProject == nil {
		return
	}
	// Save focus state (transient, not persisted to config)
	var focusedTaskID string
	if m.projectTree != nil && m.projectTree.focusedTask != nil {
		focusedTaskID = m.projectTree.focusedTask.ID
	}

	m.saveProjectTreeState()

	// Rebuild task tree to ensure parent-child relationships are current
	m.currentProject.BuildTaskTree()

	m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.views.Register(ViewProjectTree, m.projectTree)
	m.restoreProjectTreeState()

	// Restore focus state if the task still exists
	if focusedTaskID != "" {
		if task := m.currentProject.GetTaskByID(focusedTaskID); task != nil {
			m.projectTree.focusedTask = task
		}
	}

	// Rebuild tree after restoring state to ensure nodes, visibility, and
	// filter counts reflect the restored filter/sort settings
	m.projectTree.buildTree()
}

// showView is a helper that registers a view, sets its size, and pushes it onto the view stack.
// This consolidates the common pattern used by Show*Msg handlers.
func (m *AppModel) showView(viewType ViewType, view View) {
	view.SetSize(m.width, m.height-2)
	m.views.Register(viewType, view)
	m.pushView(viewType)
}

// saveProjectTreeState persists the current project tree filter/sort state to config.
func (m *AppModel) saveProjectTreeState() {
	if m.projectTree == nil || m.currentProject == nil {
		return
	}

	var filterTags []string
	for tag := range m.projectTree.filter.Tags {
		filterTags = append(filterTags, tag)
	}
	var filterSections []string
	for section := range m.projectTree.filter.Sections {
		filterSections = append(filterSections, section)
	}

	state := config.ProjectViewState{
		FilterType:       string(m.projectTree.filter.Type),
		FilterStatus:     string(m.projectTree.filter.Status),
		FilterPriority:   m.projectTree.filter.Priority,
		FilterTags:       filterTags,
		FilterSections:   filterSections,
		FilterDueDate:    int(m.projectTree.filter.DueDate),
		FilterActionable: m.projectTree.filter.Actionable,
		SortField:        int(m.projectTree.sortField),
		SortAscending:    m.projectTree.sortAscending,
		SidebarEnabled:   m.projectTree.sidebar.IsEnabled(),
		FilterBarEnabled: m.projectTree.filterBar.IsEnabled(),
		ViewMode:         int(m.projectTree.viewMode),
	}
	m.config.SetProjectViewState(m.currentProject.ID, state)
	_ = m.configManager.Save(m.config)
}

// switchToProject handles switching to a different project, loading it and creating the tree view.
// It saves current state, loads the project from storage, applies colors, and creates the tree.
// Pass restoreState=false to skip restoring saved filter/sort state (e.g., when applying custom filters).
func (m *AppModel) switchToProject(project *models.Project, restoreState bool) {
	m.saveProjectTreeState()
	m.currentProject = project
	_ = m.persister.Load(project)
	m.config.ApplyGlobalColorsToProject(project)
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	if restoreState {
		m.restoreProjectTreeState()
	}
	m.views.Register(ViewProjectTree, m.projectTree)
}

// restoreProjectTreeState applies saved filter/sort state to the current project tree.
func (m *AppModel) restoreProjectTreeState() {
	if m.projectTree == nil || m.currentProject == nil {
		return
	}
	state, exists := m.config.GetProjectViewState(m.currentProject.ID)
	if !exists {
		return
	}

	m.projectTree.filter.Type = models.TaskType(state.FilterType)
	m.projectTree.filter.Status = models.TaskStatus(state.FilterStatus)
	m.projectTree.filter.Priority = state.FilterPriority
	m.projectTree.filter.DueDate = DueDateFilter(state.FilterDueDate)
	m.projectTree.filter.Actionable = state.FilterActionable
	m.projectTree.sortField = SortField(state.SortField)
	m.projectTree.sortAscending = state.SortAscending
	m.projectTree.sidebar.SetEnabled(state.SidebarEnabled)
	m.projectTree.filterBar.SetEnabled(state.FilterBarEnabled)
	m.projectTree.viewMode = ViewMode(state.ViewMode)

	// Restore tags - validate they still exist
	m.projectTree.filter.Tags = make(map[string]bool)
	existingTags := m.projectTree.getAllTags()
	tagSet := make(map[string]bool)
	for _, t := range existingTags {
		tagSet[t] = true
	}
	for _, tag := range state.FilterTags {
		if tagSet[tag] {
			m.projectTree.filter.Tags[tag] = true
		}
	}

	// Restore sections - validate they still exist
	m.projectTree.filter.Sections = make(map[string]bool)
	existingSections := m.currentProject.GetSections()
	sectionSet := make(map[string]bool)
	for _, s := range existingSections {
		sectionSet[s] = true
	}
	for _, section := range state.FilterSections {
		if sectionSet[section] {
			m.projectTree.filter.Sections[section] = true
		}
	}

	m.projectTree.updateVisibility()
	m.projectTree.clampCursor()
}

// errorCmd creates a tea.Cmd that returns an ErrorMsg with a formatted error.
func errorCmd(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: fmt.Errorf(format, args...)}
	}
}
