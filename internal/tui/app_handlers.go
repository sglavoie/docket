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

// handleProjectMessages handles project-related messages (create, edit, archive, delete, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case SelectProjectMsg:
		m.currentProject = msg.Project
		m.config.MigrateProjectSectionColors(msg.Project.SectionColors)
		m.config.ApplyGlobalColorsToProject(msg.Project)
		m.configManager.Save(m.config)
		m.projectTree = NewProjectTreeModel(msg.Project, m.config, m.configManager)
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil

	case CreateProjectMsg:
		m.showView(ViewProjectCreator, NewProjectCreatorModel(m.config, m.configManager))
		return true, nil

	case ProjectCreatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectArchivedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectPinnedToggledMsg:
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		statusMsg := "Pinned"
		if !msg.Project.Pinned {
			statusMsg = "Unpinned"
		}
		m.statusMessage = statusMsg
		return true, statusTimeout(2 * time.Second)

	case ProjectDeletedMsg:
		m.config.RemoveProject(msg.Project.ID)
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ProjectRestoredMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case EditProjectMsg:
		if msg.Project != nil {
			m.showView(ViewProjectEditor, NewProjectEditorModel(msg.Project, m.config, m.configManager))
		}
		return true, nil

	case ProjectUpdatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case SetCurrentProjectMsg:
		m.config.CurrentProject = msg.Project.ID
		if err := m.configManager.Save(m.config); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		m.statusMessage = fmt.Sprintf("Set current: %s", msg.Project.Name)
		return true, statusTimeout(2 * time.Second)
	}
	return false, nil
}

// handleTaskMessages handles task-related messages (create, edit, delete, status changes).
// Returns true if the message was handled.
func (m *AppModel) handleTaskMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CreateTaskMsg:
		if m.currentProject != nil {
			m.showView(ViewTaskCreator, NewTaskCreatorModel(m.currentProject, m.config, m.configManager, msg.ParentTask))
		}
		return true, nil

	case EditTaskMsg:
		if m.currentProject != nil && msg.Task != nil {
			m.showView(ViewTaskEditor, NewTaskEditorModel(m.currentProject, msg.Task, m.config, m.configManager))
		}
		return true, nil

	case TaskCreatedMsg, TaskUpdatedMsg:
		m.popView()
		m.refreshProjectTree()
		return true, nil

	case TaskDeletedMsg:
		m.refreshProjectTree()
		return true, nil

	case TaskStatusChangedMsg:
		m.refreshProjectTree()
		return true, nil

	case EditTaskContentMsg:
		return m.handleEditTaskContent(msg)

	case TaskContentEditedMsg:
		return m.handleTaskContentEdited(msg)

	case SetDueDateMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDateInput, NewDateInputModel(msg.Task, msg.Project))
		}
		return true, nil

	case DueDateChangedMsg:
		m.popView()
		m.refreshProjectTree()
		return true, nil

	case ManageDependenciesMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewDependencySelector, NewDependencySelectorModel(msg.Task, msg.Project))
		}
		return true, nil

	case DependencySelectorDoneMsg:
		m.popView()
		if msg.HasChanges && msg.Task != nil && msg.Project != nil {
			msg.Task.DependsOn = msg.NewDepsOn
			msg.Task.UpdatedAt = time.Now()
			if err := storage.BackupProject(msg.Project); err != nil {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
				}
			}
			if err := storage.SaveProject(msg.Project); err != nil {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
				}
			}
			storage.LoadProject(msg.Project)
			m.refreshProjectTree()
		}
		return true, nil

	case UndoCompletedMsg:
		m.refreshProjectTree()
		return true, nil
	}
	return false, nil
}

// handleEditTaskContent launches external editor for task title and description.
func (m *AppModel) handleEditTaskContent(msg EditTaskContentMsg) (bool, tea.Cmd) {
	if msg.Task == nil {
		return true, nil
	}

	tmpFile, err := os.CreateTemp("", "tt-edit-*.md")
	if err != nil {
		return true, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create temp file: %w", err)}
		}
	}
	tmpPath := tmpFile.Name()
	content := fmt.Sprintf("# %s\n\n<!-- Edit the title above and description below. -->\n\n%s", msg.Task.Title, msg.Task.Description)
	tmpFile.WriteString(content)
	tmpFile.Close()

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
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
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
}

// handleTaskContentEdited handles result from external editor.
func (m *AppModel) handleTaskContentEdited(msg TaskContentEditedMsg) (bool, tea.Cmd) {
	if msg.Err != nil {
		return true, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", msg.Err)}
		}
	}

	if msg.Task != nil {
		titleChanged := msg.NewTitle != msg.Task.Title
		descChanged := msg.NewDescription != msg.Task.Description
		if titleChanged || descChanged {
			msg.Task.Title = msg.NewTitle
			msg.Task.Description = msg.NewDescription
			msg.Task.UpdatedAt = time.Now()

			if msg.Project != nil {
				if err := storage.BackupProject(msg.Project); err != nil {
					return true, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
					}
				}
				if err := storage.SaveProject(msg.Project); err != nil {
					return true, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
					}
				}
			}
		}
	}

	if m.currentView() == ViewTaskEditor && msg.Task != nil {
		taskEditor := NewTaskEditorModel(msg.Project, msg.Task, m.config, m.configManager)
		taskEditor.SetSize(m.width, m.height-2)
		m.views.Register(ViewTaskEditor, taskEditor)
	}
	return true, nil
}

// handleViewMessages handles view navigation and display messages.
// Returns true if the message was handled.
func (m *AppModel) handleViewMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case BackMsg:
		m.popView()
		return true, nil

	case RefreshMsg:
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ErrorMsg:
		m.err = msg.Err
		return true, nil

	case SettingsMsg:
		m.showView(ViewSettings, NewSettingsModel(m.config, m.configManager))
		return true, nil

	case SettingsSavedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case ShowHelpMsg:
		m.helpView = NewHelpViewModel(msg.Items, msg.SourceView, msg.ViewName)
		m.showView(ViewHelp, m.helpView)
		return true, nil

	case CloseHelpMsg:
		m.popView()
		return true, nil

	case ShowFilterMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewFilter, NewFilterViewModel(msg.ProjectTree))
		}
		return true, nil

	case CloseFilterMsg:
		m.popView()
		return true, nil

	case ShowTaskJumperMsg:
		if msg.Project != nil {
			m.taskJumper = NewTaskJumperModel(msg.Project)
			m.showView(ViewTaskJumper, m.taskJumper)
			return true, m.taskJumper.Init()
		}
		return true, nil

	case TaskJumperSelectMsg:
		m.popView()
		if msg.Task != nil && m.projectTree != nil {
			m.projectTree.NavigateToTask(msg.Task)
		}
		return true, nil

	case CloseTaskJumperMsg:
		m.popView()
		return true, nil

	case ShowProjectPreviewMsg:
		if msg.Project != nil {
			m.showView(ViewProjectPreview, NewProjectPreviewModel(msg.Project))
		}
		return true, nil

	case CloseProjectPreviewMsg:
		m.popView()
		return true, nil

	case ShowDueDateOverviewMsg:
		m.showView(ViewDueDateOverview, NewDueDateOverviewModel(m.config))
		return true, nil

	case CloseDueDateOverviewMsg:
		m.popView()
		return true, nil

	case DueDateOverviewSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.currentProject = msg.Project
			return true, func() tea.Msg {
				return GlowViewMsg{Task: msg.Task}
			}
		}
		return true, nil

	case GlowViewMsg:
		return m.handleGlowView(msg)

	case ShowContextPickerMsg:
		if msg.Dashboard != nil {
			m.showView(ViewContextPicker, NewContextPickerModel(msg.Dashboard))
		}
		return true, nil

	case ContextPickerDoneMsg:
		m.popView()
		if !msg.Cancelled && m.dashboard != nil {
			m.dashboard.filterContext = msg.SelectedContext
			visible := m.dashboard.visibleProjects()
			if m.dashboard.cursor >= len(visible) {
				if len(visible) > 0 {
					m.dashboard.cursor = len(visible) - 1
				} else {
					m.dashboard.cursor = 0
				}
			}
		}
		return true, nil

	case StatusClearMsg:
		m.statusMessage = ""
		return true, nil

	case MoveBlockedMsg:
		m.statusMessage = msg.Reason
		return true, statusTimeout(2 * time.Second)

	case ShowGlobalSearchMsg:
		m.globalSearch = NewGlobalSearchModel(m.config)
		m.showView(ViewGlobalSearch, m.globalSearch)
		return true, m.globalSearch.Init()

	case CloseGlobalSearchMsg:
		m.popView()
		return true, nil

	case GlobalSearchSelectMsg:
		m.popView()
		if msg.Task != nil && msg.Project != nil {
			m.currentProject = msg.Project
			storage.LoadProject(msg.Project)
			m.config.ApplyGlobalColorsToProject(msg.Project)
			m.projectTree = NewProjectTreeModel(msg.Project, m.config, m.configManager)
			m.projectTree.NavigateToTask(msg.Task)
			m.showView(ViewProjectTree, m.projectTree)
		}
		return true, nil
	}
	return false, nil
}

// handleGlowView opens task in glow pager.
func (m *AppModel) handleGlowView(msg GlowViewMsg) (bool, tea.Cmd) {
	if msg.Task == nil || !external.GlowAvailable() {
		return true, nil
	}

	content := external.FormatTaskAsMarkdown(msg.Task)
	tmpFile, err := os.CreateTemp("", "tt-glow-*.md")
	if err != nil {
		return true, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create temp file: %w", err)}
		}
	}
	tmpPath := tmpFile.Name()
	tmpFile.WriteString(content)
	tmpFile.Close()

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("glow failed: %w", err)}
		}
		return nil
	})
}

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowTagManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewTagManager, NewTagManagerModel(msg.ProjectTree))
		}
		return true, nil

	case CloseTagManagerMsg:
		m.popView()
		return true, nil

	case ShowTagFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewTagFilterSelector, NewTagFilterSelectorModel(msg.ProjectTree))
		}
		return true, nil

	case TagFilterSelectorDoneMsg:
		m.popView()
		if !msg.Cancelled && m.projectTree != nil {
			m.projectTree.filterTags = make(map[string]bool)
			for _, tag := range msg.SelectedTags {
				m.projectTree.filterTags[tag] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return true, nil

	case TagRenamedMsg:
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil

	case ShowGlobalTagManagerMsg:
		m.showView(ViewGlobalTagManager, NewGlobalTagManagerModel(m.config))
		return true, nil

	case CloseGlobalTagManagerMsg:
		m.popView()
		return true, nil

	case GlobalTagRenamedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh(m.config)
		}
		return true, nil

	case GlobalTagNavigateMsg:
		m.popView()
		m.currentProject = msg.Project
		storage.LoadProject(msg.Project)
		m.config.ApplyGlobalColorsToProject(msg.Project)
		m.projectTree = NewProjectTreeModel(msg.Project, m.config, m.configManager)
		m.projectTree.filterTags = map[string]bool{msg.Tag: true}
		m.projectTree.updateVisibility()
		m.showView(ViewProjectTree, m.projectTree)
		return true, nil
	}
	return false, nil
}

// handleSectionMessages handles section-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleSectionMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowSectionFilterSelectorMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewSectionFilterSelector, NewSectionFilterSelectorModel(msg.ProjectTree))
		}
		return true, nil

	case SectionFilterSelectorDoneMsg:
		m.popView()
		if !msg.Cancelled && m.projectTree != nil {
			m.projectTree.filterSections = make(map[string]bool)
			for _, section := range msg.SelectedSections {
				m.projectTree.filterSections[section] = true
			}
			m.projectTree.updateVisibility()
			m.projectTree.clampCursor()
		}
		return true, nil

	case ShowSectionPickerMsg:
		if msg.Task != nil && msg.Project != nil {
			m.showView(ViewSectionPicker, NewSectionPickerModel(msg.Task, msg.Project))
		}
		return true, nil

	case SectionPickerDoneMsg:
		m.popView()
		if !msg.Cancelled && msg.Task != nil && msg.Project != nil {
			msg.Task.Section = msg.Section
			msg.Task.UpdatedAt = time.Now()

			if msg.Section != "" {
				msg.Project.EnsureSection(msg.Section)
				if globalColor := m.config.GetGlobalSectionColor(msg.Section); globalColor != "gray" {
					msg.Project.SetSectionColor(msg.Section, globalColor)
				}
			}

			if err := storage.BackupProject(msg.Project); err != nil {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
				}
			}
			if err := storage.SaveProject(msg.Project); err != nil {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
				}
			}

			storage.LoadProject(msg.Project)
			m.config.ApplyGlobalColorsToProject(msg.Project)
			m.refreshProjectTree()
		}
		return true, nil

	case ShowSectionManagerMsg:
		if msg.ProjectTree != nil {
			m.showView(ViewSectionManager, NewSectionManagerModel(msg.ProjectTree, m.config, m.configManager))
		}
		return true, nil

	case CloseSectionManagerMsg:
		m.popView()
		return true, nil

	case SectionRenamedMsg, SectionAddedMsg, SectionRemovedMsg:
		m.popView()
		m.reloadAndRefreshProjectTree()
		return true, nil
	}
	return false, nil
}

// handleClipboardMessages handles clipboard-related messages (copy task title/details).
// Returns true if the message was handled.
func (m *AppModel) handleClipboardMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case CopyTaskTitleMsg:
		if msg.Task == nil {
			return true, nil
		}
		_, err := external.CopyTaskTitle(msg.Task)
		if err != nil {
			return true, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, FullDetail: false}
		}

	case CopyTaskDetailsMsg:
		if msg.Task == nil {
			return true, nil
		}
		_, err := external.CopyTaskDetails(msg.Task)
		if err != nil {
			return true, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: 1, FullDetail: true}
		}

	case CopyTasksTitleMsg:
		if len(msg.Tasks) == 0 {
			return true, nil
		}
		_, err := external.CopyTaskTitles(msg.Tasks)
		if err != nil {
			return true, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		count := len(msg.Tasks)
		return true, func() tea.Msg {
			return TaskCopiedMsg{Count: count, FullDetail: false}
		}

	case CopyTasksDetailsMsg:
		if len(msg.Tasks) == 0 {
			return true, nil
		}
		_, err := external.CopyTasksDetails(msg.Tasks)
		if err != nil {
			return true, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("copy failed: %w", err)}
			}
		}
		count := len(msg.Tasks)
		return true, func() tea.Msg {
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
		return true, statusTimeout(2 * time.Second)
	}
	return false, nil
}

// reloadAndRefreshProjectTree reloads project from storage and refreshes the tree.
func (m *AppModel) reloadAndRefreshProjectTree() {
	if m.projectTree != nil && m.currentProject != nil {
		storage.LoadProject(m.currentProject)
		m.config.ApplyGlobalColorsToProject(m.currentProject)
		m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
		m.projectTree.SetSize(m.width, m.height-2)
	}
}
