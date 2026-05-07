package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) outlinerCommands() []Command { //nolint:gocyclo
	// Helper to get the task under the outliner cursor.
	cursorTask := func() *models.Task {
		if m.outliner == nil || len(m.outliner.rows) == 0 || m.outliner.cursor >= len(m.outliner.rows) {
			return nil
		}
		return m.outliner.rows[m.outliner.cursor].task
	}

	// outlinerStatusHandler builds a handler that applies a status change to the cursor task.
	outlinerStatusHandler := func(status models.TaskStatus) func() tea.Msg {
		return func() tea.Msg {
			task := cursorTask()
			if task == nil || m.outliner == nil {
				return nil
			}
			pf := m.outliner.workflowPersistFn()
			result := applyStatusChange(pf, m.outliner.project, task.ID, status)
			if result.Err != nil {
				return ErrorMsg{Err: result.Err}
			}
			m.outliner.statusMessage = result.StatusMessage
			m.outliner.rebuildRows()
			return nil
		}
	}

	cmds := []Command{
		{Name: "start task", Description: "Set task status to in progress", Shortcut: ",s", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"begin task", "in progress"}, Handler: outlinerStatusHandler(models.StatusInProgress)},
		{Name: "complete task", Description: "Set task status to done", Shortcut: ",d", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"done task", "finish task", "mark done"}, Handler: outlinerStatusHandler(models.StatusDone)},
		{Name: "block task", Description: "Set task status to blocked", Shortcut: ",b", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"blocked task", "mark blocked"}, Handler: outlinerStatusHandler(models.StatusBlocked)},
		{Name: "standby task", Description: "Set task status to standby", Shortcut: ",S", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"standby", "pause task"}, Handler: outlinerStatusHandler(models.StatusStandby)},
		{Name: "cancel task", Description: "Set task status to canceled", Shortcut: ",x", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"canceled task", "mark canceled"}, Handler: outlinerStatusHandler(models.StatusCanceled)},
		{Name: "reopen task", Description: "Reopen a completed or canceled task", Shortcut: ",o", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"reopen", "undo complete", "mark todo"}, Handler: outlinerStatusHandler(models.StatusTodo)},
		{Name: "review task", Description: "Set task status to review", Shortcut: ",r", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"mark review", "in review"}, Handler: outlinerStatusHandler(models.StatusReview)},
		{Name: "maybe task", Description: "Set task status to maybe", Shortcut: ",m", Category: CategoryStatus, Views: []ViewType{ViewOutliner}, Aliases: []string{"maybe", "someday task"}, Handler: outlinerStatusHandler(models.StatusMaybe)},
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
			Name:        "today",
			Description: "Switch to today view",
			Shortcut:    "ctrl+f",
			Category:    CategoryViews,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"focus board", "focus view", "focus board view"},
			Handler: func() tea.Msg {
				return ShowFocusBoardMsg{}
			},
		},
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
			Name:        "dim/undim task",
			Description: "Toggle dim state of task",
			Shortcut:    "",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewOutliner},
			Aliases:     []string{"dim task", "undim task", "mute task"},
			Handler: func() tea.Msg {
				task := cursorTask()
				if task == nil || m.outliner == nil {
					return nil
				}
				pf := simplePersistWithUndo(NewProjectService(), m.config, "Toggle Dim")
				result := applyToggleDimmed(pf, m.outliner.project, task.ID)
				if result.Err != nil {
					return ErrorMsg{Err: result.Err}
				}
				m.outliner.rebuildRows()
				return StatusMsg(result.StatusMessage)
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
