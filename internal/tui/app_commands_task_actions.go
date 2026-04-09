package tui

import tea "charm.land/bubbletea/v2"

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
					m.projectTree.showBulkDeleteConfirm()
					return nil
				}
				visible := m.projectTree.visibleNodes()
				if CursorInBounds(m.projectTree.cursor, len(visible)) {
					task := visible[m.projectTree.cursor].Task
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
					return TaskPropertyChangedMsg{Task: task, NeedsSave: true}
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
					return TaskPropertyChangedMsg{Task: task, NeedsSave: true}
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
			Shortcut:    "ctrl+e",
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
}
