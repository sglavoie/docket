package tui

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
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

	case ProjectPinnedToggledMsg:
		// Save config with updated pinned status
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return m, nil
		}
		// Refresh dashboard to reflect pinned status change
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		statusMsg := "Pinned"
		if !msg.Project.Pinned {
			statusMsg = "Unpinned"
		}
		m.statusMessage = statusMsg
		return m, statusTimeout(2 * time.Second)

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

	case EditTaskContentMsg:
		// Launch external editor for task title and description
		if msg.Task == nil {
			return m, nil
		}

		// Create temp file with title and description content
		tmpFile, err := os.CreateTemp("", "tt-edit-*.md")
		if err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("failed to create temp file: %w", err)}
			}
		}
		tmpPath := tmpFile.Name()
		content := fmt.Sprintf("# %s\n\n<!-- Edit the title above and description below. -->\n\n%s", msg.Task.Title, msg.Task.Description)
		tmpFile.WriteString(content)
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
				return TaskContentEditedMsg{Task: task, Project: project, Err: err}
			}
			content, readErr := os.ReadFile(tmpPath)
			os.Remove(tmpPath)
			if readErr != nil {
				return TaskContentEditedMsg{Task: task, Project: project, Err: readErr}
			}
			newTitle, newDesc := parseTaskContent(string(content), task)
			return TaskContentEditedMsg{
				Task:           task,
				Project:        project,
				NewTitle:       newTitle,
				NewDescription: newDesc,
			}
		})

	case TaskContentEditedMsg:
		// Handle result from external editor
		if msg.Err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("editor failed: %w", msg.Err)}
			}
		}

		// Update task title and description if changed
		if msg.Task != nil {
			titleChanged := msg.NewTitle != msg.Task.Title
			descChanged := msg.NewDescription != msg.Task.Description
			if titleChanged || descChanged {
				msg.Task.Title = msg.NewTitle
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

	case ShowTagManagerMsg:
		if msg.ProjectTree != nil {
			m.tagManager = NewTagManagerModel(msg.ProjectTree)
			m.tagManager.SetSize(m.width, m.height-2)
			m.pushView(ViewTagManager)
		}
		return m, nil

	case CloseTagManagerMsg:
		m.popView()
		return m, nil

	case ShowTagFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.tagFilterSelector = NewTagFilterSelectorModel(msg.ProjectTree)
			m.tagFilterSelector.SetSize(m.width, m.height-2)
			m.pushView(ViewTagFilterSelector)
		}
		return m, nil

	case TagFilterSelectorDoneMsg:
		m.popView()
		if !msg.Cancelled && m.projectTree != nil {
			// Apply selected tags filter
			m.projectTree.filterTags = make(map[string]bool)
			for _, tag := range msg.SelectedTags {
				m.projectTree.filterTags[tag] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return m, nil

	case ShowSectionFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.sectionFilterSelector = NewSectionFilterSelectorModel(msg.ProjectTree)
			m.sectionFilterSelector.SetSize(m.width, m.height-2)
			m.pushView(ViewSectionFilterSelector)
		}
		return m, nil

	case SectionFilterSelectorDoneMsg:
		m.popView()
		if !msg.Cancelled && m.projectTree != nil {
			// Apply selected sections filter
			m.projectTree.filterSections = make(map[string]bool)
			for _, section := range msg.SelectedSections {
				m.projectTree.filterSections[section] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return m, nil

	case ShowSectionPickerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.sectionPicker = NewSectionPickerModel(msg.Task, msg.Project)
			m.sectionPicker.SetSize(m.width, m.height-2)
			m.pushView(ViewSectionPicker)
		}
		return m, nil

	case SectionPickerDoneMsg:
		m.popView()
		if !msg.Cancelled && msg.Task != nil && msg.Project != nil {
			// Update task section
			msg.Task.Section = msg.Section
			msg.Task.UpdatedAt = time.Now()

			// Ensure section exists in project
			if msg.Section != "" {
				msg.Project.EnsureSection(msg.Section)
			}

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

	case ShowSectionManagerMsg:
		if msg.ProjectTree != nil {
			m.sectionManager = NewSectionManagerModel(msg.ProjectTree)
			m.sectionManager.SetSize(m.width, m.height-2)
			m.pushView(ViewSectionManager)
		}
		return m, nil

	case CloseSectionManagerMsg:
		m.popView()
		return m, nil

	case SectionRenamedMsg, SectionAddedMsg, SectionRemovedMsg:
		// Close the section manager and refresh project tree
		m.popView()
		if m.projectTree != nil && m.currentProject != nil {
			storage.LoadProject(m.currentProject)
			m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
			m.projectTree.SetSize(m.width, m.height-2)
		}
		return m, nil

	case ShowContextPickerMsg:
		if msg.Dashboard != nil {
			m.contextPicker = NewContextPickerModel(msg.Dashboard)
			m.contextPicker.SetSize(m.width, m.height-2)
			m.pushView(ViewContextPicker)
		}
		return m, nil

	case ContextPickerDoneMsg:
		m.popView()
		if !msg.Cancelled && m.dashboard != nil {
			m.dashboard.filterContext = msg.SelectedContext
			// Reset cursor if out of bounds after filter change
			visible := m.dashboard.visibleProjects()
			if m.dashboard.cursor >= len(visible) {
				if len(visible) > 0 {
					m.dashboard.cursor = len(visible) - 1
				} else {
					m.dashboard.cursor = 0
				}
			}
		}
		return m, nil

	case TagRenamedMsg:
		// Close the tag manager and refresh project tree
		m.popView()
		if m.projectTree != nil && m.currentProject != nil {
			storage.LoadProject(m.currentProject)
			m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
			m.projectTree.SetSize(m.width, m.height-2)
		}
		return m, nil

	case ShowGlobalTagManagerMsg:
		m.globalTagManager = NewGlobalTagManagerModel(m.config)
		m.globalTagManager.SetSize(m.width, m.height-2)
		m.pushView(ViewGlobalTagManager)
		return m, nil

	case CloseGlobalTagManagerMsg:
		m.popView()
		return m, nil

	case GlobalTagRenamedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return m, nil

	case GlobalTagNavigateMsg:
		m.popView()
		m.currentProject = msg.Project
		storage.LoadProject(msg.Project)
		m.projectTree = NewProjectTreeModel(msg.Project, m.config, m.configManager)
		m.projectTree.filterTags = map[string]bool{msg.Tag: true}
		m.projectTree.updateVisibility()
		m.projectTree.SetSize(m.width, m.height-2)
		m.pushView(ViewProjectTree)
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

	case CopyTaskTitleMsg:
		if msg.Task == nil {
			return m, nil
		}
		_, err := external.CopyTaskTitle(msg.Task)
		if err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		return m, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, FullDetail: false}
		}

	case CopyTaskDetailsMsg:
		if msg.Task == nil {
			return m, nil
		}
		_, err := external.CopyTaskDetails(msg.Task)
		if err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		return m, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, FullDetail: true}
		}

	case CopyTasksTitleMsg:
		if len(msg.Tasks) == 0 {
			return m, nil
		}
		_, err := external.CopyTaskTitles(msg.Tasks)
		if err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		count := len(msg.Tasks)
		return m, func() tea.Msg {
			return TaskCopiedMsg{Count: count, FullDetail: false}
		}

	case CopyTasksDetailsMsg:
		if len(msg.Tasks) == 0 {
			return m, nil
		}
		_, err := external.CopyTasksDetails(msg.Tasks)
		if err != nil {
			return m, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		count := len(msg.Tasks)
		return m, func() tea.Msg {
			return TaskCopiedMsg{Count: count, FullDetail: true}
		}

	case TaskCopiedMsg:
		var what string
		if msg.FullDetail {
			what = "description"
		} else {
			what = "title"
		}
		if msg.Count == 1 {
			m.statusMessage = fmt.Sprintf("Copied %s to clipboard", what)
		} else {
			m.statusMessage = fmt.Sprintf("Copied %s for %d tasks to clipboard", what, msg.Count)
		}
		return m, statusTimeout(2 * time.Second)

	case StatusClearMsg:
		m.statusMessage = ""
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

	case ViewTagManager:
		if m.tagManager != nil {
			newModel, cmd := m.tagManager.Update(msg)
			m.tagManager = newModel.(*TagManagerModel)
			cmds = append(cmds, cmd)
		}

	case ViewGlobalTagManager:
		if m.globalTagManager != nil {
			newModel, cmd := m.globalTagManager.Update(msg)
			m.globalTagManager = newModel.(*GlobalTagManagerModel)
			cmds = append(cmds, cmd)
		}

	case ViewTagFilterSelector:
		if m.tagFilterSelector != nil {
			newModel, cmd := m.tagFilterSelector.Update(msg)
			m.tagFilterSelector = newModel.(*TagFilterSelectorModel)
			cmds = append(cmds, cmd)
		}

	case ViewSectionFilterSelector:
		if m.sectionFilterSelector != nil {
			newModel, cmd := m.sectionFilterSelector.Update(msg)
			m.sectionFilterSelector = newModel.(*SectionFilterSelectorModel)
			cmds = append(cmds, cmd)
		}

	case ViewSectionPicker:
		if m.sectionPicker != nil {
			newModel, cmd := m.sectionPicker.Update(msg)
			m.sectionPicker = newModel.(*SectionPickerModel)
			cmds = append(cmds, cmd)
		}

	case ViewSectionManager:
		if m.sectionManager != nil {
			newModel, cmd := m.sectionManager.Update(msg)
			m.sectionManager = newModel.(*SectionManagerModel)
			cmds = append(cmds, cmd)
		}

	case ViewContextPicker:
		if m.contextPicker != nil {
			newModel, cmd := m.contextPicker.Update(msg)
			m.contextPicker = newModel.(*ContextPickerModel)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}
