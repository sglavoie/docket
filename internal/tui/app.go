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
