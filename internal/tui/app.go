package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ViewType represents the different views in the TUI.
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewProjectList
	ViewProjectTree
	ViewTaskDetail
	ViewProjectCreator
	ViewProjectEditor
	ViewTaskCreator
	ViewTaskEditor
	ViewSettings
	ViewHelp
	ViewDateInput
	ViewDependencySelector
	ViewFilter
	ViewTaskJumper
	ViewProjectPreview
	ViewDueDateOverview
)

// AppModel is the main TUI model with a view stack.
type AppModel struct {
	config        *config.Config
	configManager *config.Manager

	// View stack for navigation
	viewStack []ViewType
	width     int
	height    int

	// View models
	dashboard          *DashboardModel
	projectList        *ProjectListModel
	projectTree        *ProjectTreeModel
	taskDetail         *TaskDetailModel
	projectCreator     *ProjectCreatorModel
	projectEditor      *ProjectEditorModel
	taskCreator        *TaskCreatorModel
	taskEditor         *TaskEditorModel
	settings           *SettingsModel
	helpView           *HelpViewModel
	dateInput          *DateInputModel
	dependencySelector *DependencySelectorModel
	filterView         *FilterViewModel
	taskJumper         *TaskJumperModel
	projectPreview     *ProjectPreviewModel
	dueDateOverview    *DueDateOverviewModel

	// Current project context
	currentProject *models.Project

	// State
	err      error
	quitting bool
}

// NewAppModel creates a new TUI application model.
func NewAppModel(cfg *config.Config, manager *config.Manager) *AppModel {
	app := &AppModel{
		config:        cfg,
		configManager: manager,
		viewStack:     []ViewType{ViewDashboard},
		width:         80,
		height:        24,
	}

	// Initialize dashboard
	app.dashboard = NewDashboardModel(cfg)

	// Load current project if set
	if current := cfg.GetCurrentProject(); current != nil {
		app.currentProject = current
	}

	return app
}

func (m *AppModel) Init() tea.Cmd {
	return nil
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

// Messages for navigation
type (
	SelectProjectMsg   struct{ Project *models.Project }
	SelectTaskMsg      struct{ Task *models.Task }
	BackMsg            struct{}
	RefreshMsg         struct{}
	ErrorMsg           struct{ Err error }
	CreateProjectMsg   struct{}
	ProjectCreatedMsg  struct{ Project *models.Project }
	ProjectArchivedMsg struct{ Project *models.Project }
	ProjectDeletedMsg  struct{ Project *models.Project }
	ProjectRestoredMsg struct{ Project *models.Project }
	EditProjectMsg     struct{ Project *models.Project }
	ProjectUpdatedMsg  struct{ Project *models.Project }

	// Task management messages
	CreateTaskMsg        struct{ ParentTask *models.Task }
	EditTaskMsg          struct{ Task *models.Task }
	TaskCreatedMsg       struct{ Task *models.Task }
	TaskUpdatedMsg       struct{ Task *models.Task }
	TaskDeletedMsg       struct{ Task *models.Task }
	TaskStatusChangedMsg struct{ Task *models.Task }

	// External editor messages
	EditDescriptionMsg struct {
		Task    *models.Task
		Project *models.Project
	}
	DescriptionEditedMsg struct {
		Task           *models.Task
		Project        *models.Project
		NewDescription string
		Err            error
	}

	// Glow view message
	GlowViewMsg struct{ Task *models.Task }

	// Settings messages
	SettingsMsg      struct{}
	SettingsSavedMsg struct{}

	// Due date messages
	SetDueDateMsg struct {
		Task    *models.Task
		Project *models.Project
	}

	// Dependency management messages
	ManageDependenciesMsg struct {
		Task    *models.Task
		Project *models.Project
	}

	// Undo message
	UndoCompletedMsg struct {
		TasksBefore int
		TasksAfter  int
	}
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "q":
			// Quit only from dashboard, go back from other views
			if m.currentView() == ViewDashboard {
				m.quitting = true
				return m, tea.Quit
			}
			m.popView()
			return m, nil
		case "esc":
			// Let views handle esc if they have an active search
			if m.currentView() == ViewProjectTree && m.projectTree != nil && m.projectTree.HasActiveSearch() {
				// Don't intercept - let it fall through to view delegation
			} else if m.currentView() == ViewDashboard && m.dashboard != nil && m.dashboard.HasActiveSearch() {
				// Don't intercept - let dashboard handle its search
			} else if m.currentView() == ViewTaskDetail && m.taskDetail != nil && m.taskDetail.HasActiveSearch() {
				// Don't intercept - let task detail handle its search
			} else if m.currentView() == ViewHelp && m.helpView != nil && m.helpView.HasActiveSearch() {
				// Don't intercept - let help view handle its search
			} else if m.currentView() == ViewTaskJumper && m.taskJumper != nil {
				// Don't intercept - task jumper handles its own closing
			} else if m.currentView() != ViewDashboard {
				m.popView()
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update all view dimensions
		if m.dashboard != nil {
			m.dashboard.SetSize(msg.Width, msg.Height-2)
		}
		if m.projectList != nil {
			m.projectList.SetSize(msg.Width, msg.Height-2)
		}
		if m.projectTree != nil {
			m.projectTree.SetSize(msg.Width, msg.Height-2)
		}
		if m.taskDetail != nil {
			m.taskDetail.SetSize(msg.Width, msg.Height-2)
		}
		if m.settings != nil {
			m.settings.SetSize(msg.Width, msg.Height-2)
		}

	case SelectProjectMsg:
		m.currentProject = msg.Project
		m.projectTree = NewProjectTreeModel(msg.Project, m.config, m.configManager)
		m.projectTree.SetSize(m.width, m.height-2)
		m.pushView(ViewProjectTree)
		return m, nil

	case SelectTaskMsg:
		m.taskDetail = NewTaskDetailModel(msg.Task, m.currentProject, m.config, m.configManager)
		m.taskDetail.SetSize(m.width, m.height-2)
		m.pushView(ViewTaskDetail)
		return m, nil

	case BackMsg:
		m.popView()
		return m, nil

	case RefreshMsg:
		// Reload data
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		return m, nil

	case CreateProjectMsg:
		m.projectCreator = NewProjectCreatorModel(m.config, m.configManager)
		m.projectCreator.SetSize(m.width, m.height-2)
		m.pushView(ViewProjectCreator)
		return m, nil

	case ProjectCreatedMsg:
		m.popView()
		// Refresh dashboard with updated config
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case ProjectArchivedMsg:
		// Return to dashboard after archiving/unarchiving
		m.popView()
		// Refresh dashboard to reflect changes
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case ProjectDeletedMsg:
		// Remove project from config and refresh dashboard
		m.config.RemoveProject(msg.Project.ID)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
		}
		// Refresh dashboard to reflect deletion
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case ProjectRestoredMsg:
		// Project was restored, refresh dashboard
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case EditProjectMsg:
		if msg.Project != nil {
			m.projectEditor = NewProjectEditorModel(msg.Project, m.config, m.configManager)
			m.projectEditor.SetSize(m.width, m.height-2)
			m.pushView(ViewProjectEditor)
		}
		return m, nil

	case ProjectUpdatedMsg:
		m.popView()
		// Refresh dashboard with updated project
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case CreateTaskMsg:
		if m.currentProject != nil {
			m.taskCreator = NewTaskCreatorModel(m.currentProject, m.config, m.configManager, msg.ParentTask)
			m.taskCreator.SetSize(m.width, m.height-2)
			m.pushView(ViewTaskCreator)
		}
		return m, nil

	case EditTaskMsg:
		if m.currentProject != nil && msg.Task != nil {
			m.taskEditor = NewTaskEditorModel(m.currentProject, msg.Task, m.config, m.configManager)
			m.taskEditor.SetSize(m.width, m.height-2)
			m.pushView(ViewTaskEditor)
		}
		return m, nil

	case TaskCreatedMsg, TaskUpdatedMsg:
		m.popView()
		// Refresh project tree
		if m.projectTree != nil && m.currentProject != nil {
			m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
			m.projectTree.SetSize(m.width, m.height-2)
		}
		return m, nil

	case TaskDeletedMsg:
		// Refresh project tree after deletion
		if m.projectTree != nil && m.currentProject != nil {
			m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
			m.projectTree.SetSize(m.width, m.height-2)
		}
		return m, nil

	case TaskStatusChangedMsg:
		// Refresh project tree after status change
		if m.projectTree != nil && m.currentProject != nil {
			m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
			m.projectTree.SetSize(m.width, m.height-2)
		}
		return m, nil

	case EditDescriptionMsg:
		// Launch external editor for task description
		if msg.Task == nil {
			return m, nil
		}

		// Create temp file with description content
		tmpFile, err := os.CreateTemp("", "tt-desc-*.md")
		if err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("failed to create temp file: %w", err)}
			}
		}
		tmpPath := tmpFile.Name()
		tmpFile.WriteString(msg.Task.Description)
		tmpFile.Close()

		// Build editor command
		editor := m.config.Editor
		if editor == "" {
			editor = "code"
		}

		var c *exec.Cmd
		if editor == "code" {
			c = exec.Command(editor, "--wait", tmpPath)
		} else {
			c = exec.Command(editor, tmpPath)
		}

		task := msg.Task
		project := msg.Project
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				os.Remove(tmpPath)
				return DescriptionEditedMsg{Task: task, Project: project, Err: err}
			}
			content, readErr := os.ReadFile(tmpPath)
			os.Remove(tmpPath)
			if readErr != nil {
				return DescriptionEditedMsg{Task: task, Project: project, Err: readErr}
			}
			return DescriptionEditedMsg{
				Task:           task,
				Project:        project,
				NewDescription: strings.TrimSpace(string(content)),
			}
		})

	case DescriptionEditedMsg:
		// Handle result from external editor
		if msg.Err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("editor failed: %w", msg.Err)}
			}
		}

		// Update task description if changed
		if msg.Task != nil && msg.NewDescription != msg.Task.Description {
			msg.Task.Description = msg.NewDescription
			msg.Task.UpdatedAt = time.Now()

			// Save project
			if msg.Project != nil {
				// Create backup before saving for undo support
				if err := storage.BackupProject(msg.Project); err != nil {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
					}
				}
				if err := storage.SaveProject(msg.Project); err != nil {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
					}
				}
			}
		}

		// Refresh task editor if active
		if m.taskEditor != nil && msg.Task != nil {
			m.taskEditor = NewTaskEditorModel(msg.Project, msg.Task, m.config, m.configManager)
			m.taskEditor.SetSize(m.width, m.height-2)
		}
		return m, nil

	case GlowViewMsg:
		// View task with glow pager
		if msg.Task == nil || !external.GlowAvailable() {
			return m, nil
		}

		// Format task as markdown
		content := external.FormatTaskAsMarkdown(msg.Task)

		// Create temp file for glow input
		tmpFile, err := os.CreateTemp("", "tt-glow-*.md")
		if err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("failed to create temp file: %w", err)}
			}
		}
		tmpPath := tmpFile.Name()
		tmpFile.WriteString(content)
		tmpFile.Close()

		// Pipe glow output through less for proper paging
		c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))

		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			os.Remove(tmpPath)
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
			}
			return nil
		})

	case SettingsMsg:
		m.settings = NewSettingsModel(m.config, m.configManager)
		m.settings.SetSize(m.width, m.height-2)
		m.pushView(ViewSettings)
		return m, nil

	case SettingsSavedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case ShowHelpMsg:
		m.helpView = NewHelpViewModel(msg.Items, msg.SourceView, msg.ViewName)
		m.helpView.SetSize(m.width, m.height-2)
		m.pushView(ViewHelp)
		return m, nil

	case CloseHelpMsg:
		m.popView()
		return m, nil

	case ShowFilterMsg:
		if msg.ProjectTree != nil {
			m.filterView = NewFilterViewModel(msg.ProjectTree)
			m.filterView.SetSize(m.width, m.height-2)
			m.pushView(ViewFilter)
		}
		return m, nil

	case CloseFilterMsg:
		m.popView()
		return m, nil

	case ShowTaskJumperMsg:
		if msg.Project != nil {
			m.taskJumper = NewTaskJumperModel(msg.Project)
			m.taskJumper.SetSize(m.width, m.height-2)
			m.pushView(ViewTaskJumper)
			return m, m.taskJumper.Init()
		}
		return m, nil

	case TaskJumperSelectMsg:
		m.popView()
		if msg.Task != nil && m.projectTree != nil {
			m.projectTree.NavigateToTask(msg.Task)
		}
		return m, nil

	case CloseTaskJumperMsg:
		m.popView()
		return m, nil

	case ShowProjectPreviewMsg:
		if msg.Project != nil {
			m.projectPreview = NewProjectPreviewModel(msg.Project)
			m.projectPreview.SetSize(m.width, m.height-2)
			m.pushView(ViewProjectPreview)
		}
		return m, nil

	case CloseProjectPreviewMsg:
		m.popView()
		return m, nil

	case ShowDueDateOverviewMsg:
		m.dueDateOverview = NewDueDateOverviewModel(m.config)
		m.dueDateOverview.SetSize(m.width, m.height-2)
		m.pushView(ViewDueDateOverview)
		return m, nil

	case CloseDueDateOverviewMsg:
		m.popView()
		return m, nil

	case DueDateOverviewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.currentProject = msg.Project
			m.taskDetail = NewTaskDetailModel(msg.Task, msg.Project, m.config, m.configManager)
			m.taskDetail.SetSize(m.width, m.height-2)
			m.pushView(ViewTaskDetail)
		}
		return m, nil

	case SetDueDateMsg:
		if msg.Task != nil && msg.Project != nil {
			m.dateInput = NewDateInputModel(msg.Task, msg.Project)
			m.dateInput.SetSize(m.width, m.height-2)
			m.pushView(ViewDateInput)
		}
		return m, nil

	case DueDateChangedMsg:
		m.popView()
		// Refresh project tree after due date change
		if m.projectTree != nil && m.currentProject != nil {
			m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
			m.projectTree.SetSize(m.width, m.height-2)
		}
		return m, nil

	case ManageDependenciesMsg:
		if msg.Task != nil && msg.Project != nil {
			m.dependencySelector = NewDependencySelectorModel(msg.Task, msg.Project)
			m.dependencySelector.width = m.width
			m.dependencySelector.height = m.height - 2
			m.pushView(ViewDependencySelector)
		}
		return m, nil

	case DependencySelectorDoneMsg:
		m.popView()
		if msg.HasChanges && msg.Task != nil && msg.Project != nil {
			// Update task dependencies
			msg.Task.DependsOn = msg.NewDepsOn
			msg.Task.UpdatedAt = time.Now()

			// Create backup before saving for undo support
			if err := storage.BackupProject(msg.Project); err != nil {
				return m, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
				}
			}

			// Save project
			if err := storage.SaveProject(msg.Project); err != nil {
				return m, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
				}
			}

			// Reload and refresh project tree
			storage.LoadProject(msg.Project)
			if m.projectTree != nil && m.currentProject != nil {
				m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
				m.projectTree.SetSize(m.width, m.height-2)
			}
		}
		return m, nil

	case UndoCompletedMsg:
		// After undo, the current task might no longer exist, so pop back to project tree
		if m.currentView() == ViewTaskDetail {
			m.popView()
		}
		// Refresh project tree after undo
		if m.projectTree != nil && m.currentProject != nil {
			m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
			m.projectTree.SetSize(m.width, m.height-2)
		}
		return m, nil
	}

	// Delegate to current view
	switch m.currentView() {
	case ViewDashboard:
		if m.dashboard != nil {
			newModel, cmd := m.dashboard.Update(msg)
			m.dashboard = newModel.(*DashboardModel)
			cmds = append(cmds, cmd)
		}

	case ViewProjectList:
		if m.projectList != nil {
			newModel, cmd := m.projectList.Update(msg)
			m.projectList = newModel.(*ProjectListModel)
			cmds = append(cmds, cmd)
		}

	case ViewProjectTree:
		if m.projectTree != nil {
			newModel, cmd := m.projectTree.Update(msg)
			m.projectTree = newModel.(*ProjectTreeModel)
			cmds = append(cmds, cmd)
		}

	case ViewTaskDetail:
		if m.taskDetail != nil {
			newModel, cmd := m.taskDetail.Update(msg)
			m.taskDetail = newModel.(*TaskDetailModel)
			cmds = append(cmds, cmd)
		}

	case ViewProjectCreator:
		if m.projectCreator != nil {
			newModel, cmd := m.projectCreator.Update(msg)
			m.projectCreator = newModel.(*ProjectCreatorModel)
			cmds = append(cmds, cmd)
		}

	case ViewProjectEditor:
		if m.projectEditor != nil {
			newModel, cmd := m.projectEditor.Update(msg)
			m.projectEditor = newModel.(*ProjectEditorModel)
			cmds = append(cmds, cmd)
		}

	case ViewTaskCreator:
		if m.taskCreator != nil {
			newModel, cmd := m.taskCreator.Update(msg)
			m.taskCreator = newModel.(*TaskCreatorModel)
			cmds = append(cmds, cmd)
		}

	case ViewTaskEditor:
		if m.taskEditor != nil {
			newModel, cmd := m.taskEditor.Update(msg)
			m.taskEditor = newModel.(*TaskEditorModel)
			cmds = append(cmds, cmd)
		}

	case ViewSettings:
		if m.settings != nil {
			newModel, cmd := m.settings.Update(msg)
			m.settings = newModel.(*SettingsModel)
			cmds = append(cmds, cmd)
		}

	case ViewHelp:
		if m.helpView != nil {
			newModel, cmd := m.helpView.Update(msg)
			m.helpView = newModel.(*HelpViewModel)
			cmds = append(cmds, cmd)
		}

	case ViewDateInput:
		if m.dateInput != nil {
			newModel, cmd := m.dateInput.Update(msg)
			m.dateInput = newModel.(*DateInputModel)
			cmds = append(cmds, cmd)
		}

	case ViewDependencySelector:
		if m.dependencySelector != nil {
			newModel, cmd := m.dependencySelector.Update(msg)
			m.dependencySelector = newModel.(*DependencySelectorModel)
			cmds = append(cmds, cmd)
		}

	case ViewFilter:
		if m.filterView != nil {
			newModel, cmd := m.filterView.Update(msg)
			m.filterView = newModel.(*FilterViewModel)
			cmds = append(cmds, cmd)
		}

	case ViewTaskJumper:
		if m.taskJumper != nil {
			newModel, cmd := m.taskJumper.Update(msg)
			m.taskJumper = newModel.(*TaskJumperModel)
			cmds = append(cmds, cmd)
		}

	case ViewProjectPreview:
		if m.projectPreview != nil {
			newModel, cmd := m.projectPreview.Update(msg)
			m.projectPreview = newModel.(*ProjectPreviewModel)
			cmds = append(cmds, cmd)
		}

	case ViewDueDateOverview:
		if m.dueDateOverview != nil {
			newModel, cmd := m.dueDateOverview.Update(msg)
			m.dueDateOverview = newModel.(*DueDateOverviewModel)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	var content string

	switch m.currentView() {
	case ViewDashboard:
		if m.dashboard != nil {
			content = m.dashboard.View()
		}
	case ViewProjectList:
		if m.projectList != nil {
			content = m.projectList.View()
		}
	case ViewProjectTree:
		if m.projectTree != nil {
			content = m.projectTree.View()
		}
	case ViewTaskDetail:
		if m.taskDetail != nil {
			content = m.taskDetail.View()
		}
	case ViewProjectCreator:
		if m.projectCreator != nil {
			content = m.projectCreator.View()
		}
	case ViewProjectEditor:
		if m.projectEditor != nil {
			content = m.projectEditor.View()
		}
	case ViewTaskCreator:
		if m.taskCreator != nil {
			content = m.taskCreator.View()
		}
	case ViewTaskEditor:
		if m.taskEditor != nil {
			content = m.taskEditor.View()
		}
	case ViewSettings:
		if m.settings != nil {
			content = m.settings.View()
		}
	case ViewHelp:
		if m.helpView != nil {
			content = m.helpView.View()
		}
	case ViewDateInput:
		if m.dateInput != nil {
			content = m.dateInput.View()
		}
	case ViewDependencySelector:
		if m.dependencySelector != nil {
			content = m.dependencySelector.View()
		}
	case ViewFilter:
		if m.filterView != nil {
			content = m.filterView.View()
		}
	case ViewTaskJumper:
		if m.taskJumper != nil {
			content = m.taskJumper.View()
		}
	case ViewProjectPreview:
		if m.projectPreview != nil {
			content = m.projectPreview.View()
		}
	case ViewDueDateOverview:
		if m.dueDateOverview != nil {
			content = m.dueDateOverview.View()
		}
	default:
		content = "Unknown view"
	}

	// Add status bar
	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

func (m *AppModel) renderStatusBar() string {
	var status string

	switch m.currentView() {
	case ViewDashboard:
		status = "Dashboard"
	case ViewProjectList:
		status = "Projects"
	case ViewProjectTree:
		if m.currentProject != nil {
			status = fmt.Sprintf("Project: %s", m.currentProject.Name)
		} else {
			status = "Project"
		}
	case ViewTaskDetail:
		status = "Task Details"
	case ViewProjectCreator:
		status = "New Project"
	case ViewProjectEditor:
		status = "Edit Project"
	case ViewTaskCreator:
		status = "New Task"
	case ViewTaskEditor:
		status = "Edit Task"
	case ViewSettings:
		status = "Settings"
	case ViewHelp:
		status = "Help"
	case ViewDateInput:
		status = "Set Due Date"
	case ViewFilter:
		status = "Filters"
	case ViewTaskJumper:
		status = "Jump to Task"
	case ViewProjectPreview:
		status = "Project Preview"
	case ViewDueDateOverview:
		status = "Due Dates"
	}

	help := "q: quit | esc: back"

	left := StatusBarStyle.Render(status)
	right := HelpStyle.Render(help)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	spacer := lipgloss.NewStyle().Width(gap).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}

// RunApp starts the TUI application.
func RunApp(cfg *config.Config, manager *config.Manager) error {
	model := NewAppModel(cfg, manager)
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
