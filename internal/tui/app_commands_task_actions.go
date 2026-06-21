package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// selectedTreeTask returns the task under the project tree cursor,
// or nil when the project tree is not active.
func (m *AppModel) selectedTreeTask() *models.Task {
	if m.projectTree == nil {
		return nil
	}
	return m.projectTree.getSelectedTask()
}

func (m *AppModel) taskActionCommands() []Command { //nolint:gocyclo
	return []Command{
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
				if task := m.selectedTreeTask(); task != nil {
					return EditTaskMsg{Task: task}
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
				if task := m.selectedTreeTask(); task != nil {
					m.projectTree.enterQuickEditMode(task, false)
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
				if task := m.selectedTreeTask(); task != nil {
					m.projectTree.enterQuickEditMode(task, true)
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
					m.projectTree.showBulkDeleteConfirm()
					return nil
				}
				if task := m.selectedTreeTask(); task != nil {
					m.projectTree.showDeleteConfirm(task)
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
				if m.projectTreeHasBulkSelection() {
					return m.projectTreeClipboardMsg("y")
				}
				if task := m.selectedTreeTask(); task != nil {
					return CopyTaskTitleMsg{Task: task}
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
				if m.projectTreeHasBulkSelection() {
					return m.projectTreeClipboardMsg("Y")
				}
				if task := m.selectedTreeTask(); task != nil {
					return CopyTaskDetailsMsg{Task: task}
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
				if m.projectTreeHasBulkSelection() {
					return m.projectTreeClipboardMsg("ctrl+y")
				}
				if task := m.selectedTreeTask(); task != nil {
					return CopyTaskTitleAndDescMsg{Task: task}
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
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return ShowProjectSelectorForMoveMsg{
						Task:          task,
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
				if m.projectTreeHasBulkSelection() && m.projectTree != nil {
					cmd, _ := handleToggleActiveKey(m.projectTree)
					return m.runProjectTreeCmd(cmd)
				}
				if task := m.selectedTreeTask(); task != nil {
					task.ToggleActive()
					return taskPropertyChangedWithUndo(task, "Toggle Active")
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
				if task := m.selectedTreeTask(); task != nil {
					task.TogglePinned()
					return taskPropertyChangedWithUndo(task, "Toggle Pin")
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
				if refs := m.projectTreeBulkTaskRefs(); len(refs) > 0 {
					return SetDueDateMsg{TaskRefs: refs}
				}
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return SetDueDateMsg{
						Task:    task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "move to section",
			Description: "Pick section for task",
			Shortcut:    "ts",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"change section", "assign section"},
			Handler: func() tea.Msg {
				if tasks := m.projectTreeBulkSelectedTasks(); len(tasks) > 0 {
					return ShowBulkSectionPickerMsg{
						Tasks:   tasks,
						Project: m.projectTreeProject(),
					}
				}
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return ShowSectionPickerMsg{
						Task:    task,
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
				if refs := m.projectTreeBulkTaskRefs(); len(refs) > 0 {
					return ShowBulkTagSelectorMsg{TaskRefs: refs}
				}
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
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
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return ManageDependenciesMsg{
						Task:    task,
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
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return ShowDependencyViewerMsg{
						Task:    task,
						Project: m.currentProject,
					}
				}
				return StatusMsg("No task selected")
			},
		},
		{
			Name:        "edit content",
			Description: "Edit task description in external editor",
			Shortcut:    "ctrl+e",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"edit description", "open editor"},
			Handler: func() tea.Msg {
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return EditTaskContentMsg{
						Task:    task,
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
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return ShowTaskCommentsMsg{
						Task:    task,
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
				if task := m.selectedTreeTask(); task != nil && m.currentProject != nil {
					return ShowTaskAttachmentsMsg{
						Task:    task,
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
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"activity log", "task history"},
			Handler: func() tea.Msg {
				task, project := m.paletteTaskTarget()
				if task == nil || project == nil {
					return StatusMsg(statusNoTaskSelected)
				}
				return OpenTaskActivityMsg{
					Task:    task,
					Project: project,
				}
			},
		},
		{
			Name:        "project recent activity",
			Description: "Show recent activity for the current project",
			Shortcut:    "gl",
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"recent activity", "project history"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return StatusMsg("No project selected")
				}
				return ShowProjectRecentActivityMsg{Project: project}
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
}
