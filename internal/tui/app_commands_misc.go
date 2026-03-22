package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) viewModeCommands() []Command {
	type viewModeEntry struct {
		name, desc, shortcut string
		aliases              []string
		mode                 ViewMode
	}
	entries := []viewModeEntry{
		{"active view", "Switch to Active view mode", "ga", []string{"view active", "active tasks"}, ViewModeActive},
		{"next view", "Switch to Next view mode", "gn", []string{"view next", "next actions"}, ViewModeNext},
		{"inbox view", "Switch to Inbox view mode", "gi", []string{"view inbox"}, ViewModeInbox},
		{"waiting view", "Switch to Waiting view mode", "gw", []string{"view waiting", "waiting for"}, ViewModeWaiting},
		{"someday view", "Switch to Someday view mode", "gs", []string{"view someday", "someday maybe"}, ViewModeSomeday},
	}

	cmds := make([]Command, 0, len(entries)+1)
	for _, e := range entries {
		mode := e.mode
		cmds = append(cmds, Command{
			Name:        e.name,
			Description: e.desc,
			Shortcut:    e.shortcut,
			Category:    CategoryViews,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     e.aliases,
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.viewMode = mode
					m.projectTree.updateVisibility()
				}
				return nil
			},
		})
	}

	cmds = append(cmds, Command{
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
	})

	return cmds
}

func (m *AppModel) notesCommands() []Command {
	return []Command{
		{
			Name:        "show notes",
			Description: "Open notes view",
			Shortcut:    "[",
			Category:    CategoryNotes,
			Views:       projectTreeAndOutlinerViews(),
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
			Views:       projectTreeAndOutlinerViews(),
			Aliases:     []string{"new note", "add note"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowNoteCreatorMsg{Project: m.currentProject}
				}
				return nil
			},
		},
	}
}

func (m *AppModel) otherCommands() []Command {
	return []Command{
		{
			Name:        "project focus",
			Description: "Project-scoped focus view",
			Shortcut:    "f",
			Category:    CategoryViews,
			Views:       projectTreeAndOutlinerViews(),
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
			Views:       projectTreeAndOutlinerViews(),
			Aliases:     []string{"blockers", "blocked tasks"},
			Handler: func() tea.Msg {
				if m.currentProject != nil {
					return ShowBlockersMsg{Project: m.currentProject}
				}
				return nil
			},
		},
		{
			Name:        "all blockers",
			Description: "Show blockers across all projects",
			Shortcut:    "b",
			Category:    CategoryViews,
			Views:       []ViewType{ViewDashboard},
			Aliases:     []string{"blockers overview", "global blockers"},
			Handler: func() tea.Msg {
				return ShowBlockersMsg{}
			},
		},
		{
			Name:        "reload project",
			Description: "Reload project data from storage",
			Shortcut:    "ctrl+r",
			Category:    CategoryActions,
			Views:       projectTreeAndOutlinerViews(),
			Aliases:     []string{"refresh", "reload data", "sync"},
			Handler: func() tea.Msg {
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					err := storage.LoadProject(m.outliner.project)
					if err != nil {
						logging.Warn("failed to load project %s: %v", m.outliner.project.Name, err)
					}
					m.outliner.rebuildRows()
					if err != nil {
						return ErrorMsg{Err: err}
					}
					return nil
				}
				if m.projectTree != nil {
					m.projectTree.clearSelection()
					err := storage.LoadProject(m.projectTree.project)
					if err != nil {
						logging.Warn("failed to load project %s: %v", m.projectTree.project.Name, err)
					}
					m.projectTree.buildTree()
					if err != nil {
						return ErrorMsg{Err: err}
					}
				}
				return nil
			},
		},
		{
			Name:        "archive project",
			Description: "Archive or unarchive the current project",
			Shortcut:    "ctrl+a",
			Category:    CategoryActions,
			Views:       projectTreeAndOutlinerViews(),
			Aliases:     []string{"unarchive project"},
			Handler: func() tea.Msg {
				if m.currentProject == nil {
					return nil
				}
				if m.currentProject.Sandbox {
					return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
				}
				if m.baseProjectView() == ViewOutliner && m.outliner != nil {
					return m.outliner.archiveProject()()
				}
				if m.projectTree == nil {
					return nil
				}
				if m.currentProject.Archived {
					m.projectTree.confirm.Show("unarchive", "Are you sure you want to unarchive this project?")
				} else {
					m.projectTree.confirm.Show("archive", "Are you sure you want to archive this project?")
				}
				return nil
			},
		},
	}
}

func (m *AppModel) outlinerCommands() []Command {
	// Helper to get the task under the outliner cursor.
	cursorTask := func() *models.Task {
		if m.outliner == nil || len(m.outliner.rows) == 0 || m.outliner.cursor >= len(m.outliner.rows) {
			return nil
		}
		return m.outliner.rows[m.outliner.cursor].task
	}

	cmds := []Command{
		// --- Status changes (,s ,d ,b ,x ,o ,r) ---
		{
			Name:        "start task",
			Description: "Set task status to in progress",
			Shortcut:    ",s",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"begin task", "in progress"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task == nil || m.outliner == nil {
					return nil
				}
				pf := simplePersist(NewProjectService())
				result := applyStatusChange(pf, m.outliner.project, task.ID, models.StatusInProgress)
				if result.Err != nil {
					return ErrorMsg{Err: result.Err}
				}
				m.outliner.statusMessage = result.StatusMessage
				m.outliner.rebuildRows()
				return nil
			},
		},
		{
			Name:        "complete task",
			Description: "Set task status to done",
			Shortcut:    ",d",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"done task", "finish task", "mark done"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task == nil || m.outliner == nil {
					return nil
				}
				pf := simplePersist(NewProjectService())
				result := applyStatusChange(pf, m.outliner.project, task.ID, models.StatusDone)
				if result.Err != nil {
					return ErrorMsg{Err: result.Err}
				}
				m.outliner.statusMessage = result.StatusMessage
				m.outliner.rebuildRows()
				return nil
			},
		},
		{
			Name:        "block task",
			Description: "Set task status to blocked",
			Shortcut:    ",b",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"blocked task", "mark blocked"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task == nil || m.outliner == nil {
					return nil
				}
				pf := simplePersist(NewProjectService())
				result := applyStatusChange(pf, m.outliner.project, task.ID, models.StatusBlocked)
				if result.Err != nil {
					return ErrorMsg{Err: result.Err}
				}
				m.outliner.statusMessage = result.StatusMessage
				m.outliner.rebuildRows()
				return nil
			},
		},
		{
			Name:        "cancel task",
			Description: "Set task status to canceled",
			Shortcut:    ",x",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"canceled task", "mark canceled"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task == nil || m.outliner == nil {
					return nil
				}
				pf := simplePersist(NewProjectService())
				result := applyStatusChange(pf, m.outliner.project, task.ID, models.StatusCanceled)
				if result.Err != nil {
					return ErrorMsg{Err: result.Err}
				}
				m.outliner.statusMessage = result.StatusMessage
				m.outliner.rebuildRows()
				return nil
			},
		},
		{
			Name:        "reopen task",
			Description: "Reopen a completed or canceled task",
			Shortcut:    ",o",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"reopen", "undo complete", "mark todo"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task == nil || m.outliner == nil {
					return nil
				}
				pf := simplePersist(NewProjectService())
				result := applyStatusChange(pf, m.outliner.project, task.ID, models.StatusTodo)
				if result.Err != nil {
					return ErrorMsg{Err: result.Err}
				}
				m.outliner.statusMessage = result.StatusMessage
				m.outliner.rebuildRows()
				return nil
			},
		},
		{
			Name:        "review task",
			Description: "Set task status to review",
			Shortcut:    ",r",
			Category:    CategoryStatus,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"mark review", "in review"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task == nil || m.outliner == nil {
					return nil
				}
				pf := simplePersist(NewProjectService())
				result := applyStatusChange(pf, m.outliner.project, task.ID, models.StatusReview)
				if result.Err != nil {
					return ErrorMsg{Err: result.Err}
				}
				m.outliner.statusMessage = result.StatusMessage
				m.outliner.rebuildRows()
				return nil
			},
		},

		// --- Sort (s1-s5, s0) ---
		{
			Name:        "outliner sort by priority",
			Description: "Sort tasks by priority",
			Shortcut:    "s1",
			Category:    CategorySort,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"outliner priority sort"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					m.outliner.toggleSort(SortByPriority)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by status",
			Description: "Sort tasks by status",
			Shortcut:    "s2",
			Category:    CategorySort,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"outliner status sort"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					m.outliner.toggleSort(SortByStatus)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by due date",
			Description: "Sort tasks by due date",
			Shortcut:    "s3",
			Category:    CategorySort,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"outliner due date sort", "outliner deadline sort"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					m.outliner.toggleSort(SortByDueDate)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by created",
			Description: "Sort tasks by creation date",
			Shortcut:    "s4",
			Category:    CategorySort,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"outliner created sort", "outliner creation date sort"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					m.outliner.toggleSort(SortByCreatedAt)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by score",
			Description: "Sort tasks by composite score",
			Shortcut:    "s5",
			Category:    CategorySort,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"outliner score sort", "outliner scoring sort"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					m.outliner.toggleSort(SortByScore)
				}
				return nil
			},
		},
		{
			Name:        "outliner reset sort",
			Description: "Reset sort to default order",
			Shortcut:    "s0",
			Category:    CategorySort,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"outliner default sort", "outliner clear sort"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					m.outliner.sort.Reset()
					m.outliner.rebuildRows()
				}
				return nil
			},
		},

		// --- Edit operations ---
		{
			Name:        "view task details",
			Description: "Open task detail view",
			Shortcut:    "o",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"open task", "show details"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return ShowTaskDetailMsg{Task: task, Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "edit task",
			Description: "Edit selected task in external editor",
			Shortcut:    "e",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"open editor", "modify task"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil {
					return EditTaskMsg{Task: task}
				}
				return nil
			},
		},
		{
			Name:        "rename task",
			Description: "Inline rename task title",
			Shortcut:    "tr",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"rename", "change title"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return m.outliner.quickEdit.Enter(task, true)()
				}
				return nil
			},
		},

		// --- Create operations (t-prefix) ---
		{
			Name:        "create subtask",
			Description: "Create a new child task under cursor",
			Shortcut:    "tn",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"new subtask", "add child"},
			Handler: func() tea.Msg {
				if m.outliner == nil {
					return nil
				}
				task := cursorTask()
				var focusedTask *models.Task
				if len(m.outliner.zoomStack) > 0 {
					topID := m.outliner.zoomStack[len(m.outliner.zoomStack)-1].TaskID
					focusedTask = m.outliner.project.GetTaskByID(topID)
				}
				parentTask := focusedTask
				if parentTask == nil {
					parentTask = task
				}
				if parentTask != nil {
					return CreateLinkedTaskMsg{SourceTask: parentTask, Project: m.outliner.project, LinkType: LinkTypeSubtask}
				}
				return nil
			},
		},
		{
			Name:        "create sibling task",
			Description: "Create a new sibling task at same level",
			Shortcut:    "tN",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"new sibling", "add sibling"},
			Handler: func() tea.Msg {
				if m.outliner == nil {
					return nil
				}
				return CreateTaskMsg{}
			},
		},

		// --- Delete ---
		{
			Name:        "delete task",
			Description: "Delete selected task",
			Shortcut:    "D",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"remove task", "trash task"},
			Handler: func() tea.Msg {
				if m.outliner == nil {
					return nil
				}
				task := cursorTask()
				if task != nil {
					m.outliner.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", task.ShortID))
					m.outliner.deleteTarget = task
				}
				return nil
			},
		},

		// --- Clipboard ---
		{
			Name:        "copy title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"yank title", "clipboard title"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil {
					return CopyTaskTitleMsg{Task: task}
				}
				return nil
			},
		},
		{
			Name:        "copy details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"yank details", "clipboard details"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil {
					return CopyTaskDetailsMsg{Task: task}
				}
				return nil
			},
		},
		{
			Name:        "copy with description",
			Description: "Copy task title and description to clipboard",
			Shortcut:    "ctrl+y",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"yank description", "clipboard description"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil {
					return CopyTaskTitleAndDescMsg{Task: task}
				}
				return nil
			},
		},

		// --- Score editor ---
		{
			Name:        "score editor",
			Description: "Open score editor for task",
			Shortcut:    "S",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"edit score", "scoring", "set score"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return ShowScoreEditorMsg{Task: task, Project: m.outliner.project}
				}
				return nil
			},
		},

		// --- View switching ---
		{
			Name:        "project tree",
			Description: "Switch to project tree view",
			Shortcut:    "ctrl+p",
			Category:    CategoryViews,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"tree view", "project tree view"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					return ShowProjectTreeMsg{Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "focus board",
			Description: "Switch to focus board view",
			Shortcut:    "ctrl+f",
			Category:    CategoryViews,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"focus view", "focus board view"},
			Handler: func() tea.Msg {
				return ShowFocusBoardMsg{}
			},
		},

		// --- Multi-select ---
		{
			Name:        "multi-select mode",
			Description: "Toggle multi-select mode",
			Shortcut:    "v",
			Category:    CategoryMultiSelect,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"visual mode", "select multiple"},
			Handler: func() tea.Msg {
				if m.outliner != nil {
					m.outliner.toggleMultiSelectMode()
				}
				return nil
			},
		},

		// --- Linked task operations (t-prefix message keys) ---
		{
			Name:        "edit content",
			Description: "Edit task description in external editor",
			Shortcut:    "to",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"edit description", "open content editor"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return EditTaskContentMsg{Task: task, Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "task comments",
			Description: "View and add task comments",
			Shortcut:    "tc",
			Category:    CategoryComments,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"comments", "add comment"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return ShowTaskCommentsMsg{Task: task, Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "task attachments",
			Description: "View and manage task attachments",
			Shortcut:    "ta",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"attachments", "add attachment"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return ShowTaskAttachmentsMsg{Task: task, Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "navigate linked tasks",
			Description: "Navigate to linked tasks",
			Shortcut:    "tl",
			Category:    CategoryLinkedTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"show linked", "go to linked"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return ShowLinkedTaskSelectorMsg{Task: task, Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "manage dependencies",
			Description: "Manage task dependencies",
			Shortcut:    "td",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"add dependency", "link dependency"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return ManageDependenciesMsg{Task: task, Project: m.outliner.project}
				}
				return nil
			},
		},
		{
			Name:        "move to project",
			Description: "Move task to another project",
			Shortcut:    "tm",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"transfer task", "relocate task"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task != nil && m.outliner != nil {
					return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: m.outliner.project}
				}
				return nil
			},
		},
	}

	return cmds
}

func (m *AppModel) sortCommands() []Command {
	return []Command{
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
			Name:        "sort by score",
			Description: "Sort tasks by composite score",
			Shortcut:    "5",
			Category:    CategorySort,
			Views:       []ViewType{ViewProjectTree},
			Aliases:     []string{"score sort", "scoring sort"},
			Handler: func() tea.Msg {
				if m.projectTree != nil {
					m.projectTree.toggleSort(SortByScore)
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
					m.projectTree.sort.Reset()
					m.projectTree.buildTree()
				}
				return nil
			},
		},
	}
}
