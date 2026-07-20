package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AppModel) outlinerCommands() []Command { //nolint:gocyclo
	// Origin-aware: returns the cursor task on whichever outliner is the
	// non-palette origin (single-project or cross-project).
	cursorTask := func() *models.Task {
		task, _ := m.outlinerTarget()
		return task
	}

	// outlinerStatusHandler builds a handler that applies a status change to the
	// cursor task through the single-task coordinator (Transition), invoking the
	// command inline so the resulting TransactionResultMsg flows back to the shell
	// (reload + toast) while the palette handler keeps its synchronous
	// tea.Msg-returning shape. Single-project and cross-project modes share the
	// one coordinator path.
	outlinerStatusHandler := func(status models.TaskStatus) func() tea.Msg {
		return func() tea.Msg {
			out := m.activeOutliner()
			task, project := m.outlinerTarget()
			if out == nil || task == nil || project == nil {
				return nil
			}
			return out.taskItem.Transition(task.ID, models.TargetStateForStatus(status))()
		}
	}

	outlinerViews := []ViewType{ViewOutliner, ViewCrossProjectOutliner}

	cmds := []Command{
		{Name: "start task", Description: "Set task status to in progress", Shortcut: ",s", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"begin task", "in progress"}, Handler: outlinerStatusHandler(models.StatusInProgress)},
		{Name: "complete task", Description: "Set task status to done", Shortcut: ",d", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"done task", "finish task", "mark done"}, Handler: outlinerStatusHandler(models.StatusDone)},
		{Name: "block task", Description: "Set task status to blocked", Shortcut: ",b", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"blocked task", "mark blocked"}, Handler: outlinerStatusHandler(models.StatusBlocked)},
		{Name: "standby task", Description: "Set task status to standby", Shortcut: ",S", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"standby", "pause task"}, Handler: outlinerStatusHandler(models.StatusStandby)},
		{Name: "cancel task", Description: "Set task status to canceled", Shortcut: ",x", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"canceled task", "mark canceled"}, Handler: outlinerStatusHandler(models.StatusCanceled)},
		{Name: "reopen task", Description: "Reopen a completed or canceled task", Shortcut: ",o", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"reopen", "undo complete", "mark todo"}, Handler: outlinerStatusHandler(models.StatusTodo)},
		{Name: "review task", Description: "Set task status to review", Shortcut: ",r", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"mark review", "in review"}, Handler: outlinerStatusHandler(models.StatusReview)},
		{Name: "maybe task", Description: "Set task status to maybe", Shortcut: ",m", Category: CategoryStatus, Views: outlinerViews, Aliases: []string{"maybe", "someday task"}, Handler: outlinerStatusHandler(models.StatusMaybe)},
		{
			Name:        "outliner sort by priority",
			Description: "Sort tasks by priority",
			Shortcut:    "1",
			Category:    CategorySort,
			Views:       outlinerViews,
			Aliases:     []string{"outliner priority sort"},
			Handler: func() tea.Msg {
				if out := m.activeOutliner(); out != nil {
					out.toggleSort(SortByPriority)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by status",
			Description: "Sort tasks by status",
			Shortcut:    "2",
			Category:    CategorySort,
			Views:       outlinerViews,
			Aliases:     []string{"outliner status sort"},
			Handler: func() tea.Msg {
				if out := m.activeOutliner(); out != nil {
					out.toggleSort(SortByStatus)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by due date",
			Description: "Sort tasks by due date",
			Shortcut:    "3",
			Category:    CategorySort,
			Views:       outlinerViews,
			Aliases:     []string{"outliner due date sort", "outliner deadline sort"},
			Handler: func() tea.Msg {
				if out := m.activeOutliner(); out != nil {
					out.toggleSort(SortByDueDate)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by created",
			Description: "Sort tasks by creation date",
			Shortcut:    "4",
			Category:    CategorySort,
			Views:       outlinerViews,
			Aliases:     []string{"outliner created sort", "outliner creation date sort"},
			Handler: func() tea.Msg {
				if out := m.activeOutliner(); out != nil {
					out.toggleSort(SortByCreatedAt)
				}
				return nil
			},
		},
		{
			Name:        "outliner sort by score",
			Description: "Sort tasks by composite score",
			Shortcut:    "5",
			Category:    CategorySort,
			Views:       outlinerViews,
			Aliases:     []string{"outliner score sort", "outliner scoring sort"},
			Handler: func() tea.Msg {
				if out := m.activeOutliner(); out != nil {
					out.toggleSort(SortByScore)
				}
				return nil
			},
		},
		{
			Name:        "outliner reset sort",
			Description: "Reset sort to default order",
			Shortcut:    "0",
			Category:    CategorySort,
			Views:       outlinerViews,
			Aliases:     []string{"outliner default sort", "outliner clear sort"},
			Handler: func() tea.Msg {
				if out := m.activeOutliner(); out != nil {
					out.sort.Reset()
					out.rebuildRows()
				}
				return nil
			},
		},
		{
			Name:        "view task details",
			Description: "Open task detail view",
			Shortcut:    "o",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"open task", "show details"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task == nil || project == nil {
					return nil
				}
				return ShowTaskDetailMsg{Task: task, Project: project}
			},
		},
		{
			Name:        "edit task",
			Description: "Edit selected task in external editor",
			Shortcut:    "e",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"open editor", "modify task"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task == nil {
					return nil
				}
				return EditTaskMsg{Task: task, Project: project}
			},
		},
		{
			Name:        "rename task",
			Description: "Inline rename task title",
			Shortcut:    "tr",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"rename", "change title"},
			Handler: func() tea.Msg {
				out := m.activeOutliner()
				task, project := m.outlinerTarget()
				if out == nil || task == nil {
					return nil
				}
				if out.project == nil {
					out.crossQuickEditProject = project
				}
				return out.quickEdit.Enter(task, true)()
			},
		},
		{
			Name:        "new task (no template)",
			Description: "Create a new child task under cursor without template picker",
			Shortcut:    "tn",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"create child task", "new child task", "add child"},
			Handler: func() tea.Msg {
				parentTask, project := m.outlinerCreateParent()
				if parentTask == nil || project == nil {
					return nil
				}
				return CreateTaskMsg{ParentTask: parentTask, Project: project, SkipTemplatePicker: true}
			},
		},
		{
			Name:        "new task (with template picker)",
			Description: "Create a new child task under cursor with template picker",
			Shortcut:    "tN",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"create child task with template", "new child task with template", "add child with template"},
			Handler: func() tea.Msg {
				parentTask, project := m.outlinerCreateParent()
				if parentTask == nil || project == nil {
					return nil
				}
				return CreateTaskMsg{ParentTask: parentTask, Project: project, SkipTemplatePicker: false}
			},
		},
		{
			Name:        "create linked subtask",
			Description: "Create a new subtask linked task",
			Shortcut:    "cs",
			Category:    CategoryLinkedTasks,
			Views:       outlinerViews,
			Aliases:     []string{"new linked subtask", "add linked subtask", "linked subtask"},
			Handler: func() tea.Msg {
				parentTask, project := m.outlinerCreateParent()
				if parentTask == nil || project == nil {
					return nil
				}
				return CreateLinkedTaskMsg{SourceTask: parentTask, Project: project, LinkType: LinkTypeSubtask}
			},
		},
		{
			Name:        "delete task",
			Description: "Delete selected task",
			Shortcut:    "tD",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"remove task", "trash task"},
			Handler: func() tea.Msg {
				out := m.activeOutliner()
				task, _ := m.outlinerTarget()
				if out == nil || task == nil {
					return nil
				}
				out.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", task.ShortID))
				out.deleteTarget = task
				return nil
			},
		},
		{
			Name:        "copy title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryTasks,
			Views:       outlinerViews,
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
			Views:       outlinerViews,
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
			Views:       outlinerViews,
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
			Views:       outlinerViews,
			Aliases:     []string{"edit score", "scoring", "set score"},
			Handler: func() tea.Msg {
				task, project := m.outlinerScoreTarget()
				if task != nil && project != nil {
					return ShowScoreEditorMsg{Task: task, Project: project}
				}
				return nil
			},
		},
		{
			Name:        "multi-select mode",
			Description: "Toggle multi-select mode",
			Shortcut:    "v",
			Category:    CategoryMultiSelect,
			Views:       outlinerViews,
			Aliases:     []string{"visual mode", "select multiple"},
			Handler: func() tea.Msg {
				if out := m.activeOutliner(); out != nil {
					out.toggleMultiSelectMode()
				}
				return nil
			},
		},
		{
			Name:        "edit content",
			Description: "Edit task description in external editor",
			Shortcut:    "to",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"edit description", "open content editor"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task != nil && project != nil {
					return EditTaskContentMsg{Task: task, Project: project}
				}
				return nil
			},
		},
		{
			Name:        "task comments",
			Description: "View and add task comments",
			Shortcut:    "tc",
			Category:    CategoryComments,
			Views:       outlinerViews,
			Aliases:     []string{"comments", "add comment"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task != nil && project != nil {
					return ShowTaskCommentsMsg{Task: task, Project: project}
				}
				return nil
			},
		},
		{
			Name:        "task attachments",
			Description: "View and manage task attachments",
			Shortcut:    "ta",
			Category:    CategoryAttachments,
			Views:       outlinerViews,
			Aliases:     []string{"attachments", "add attachment"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task != nil && project != nil {
					return ShowTaskAttachmentsMsg{Task: task, Project: project}
				}
				return nil
			},
		},
		{
			Name:        "navigate linked tasks",
			Description: "Navigate to linked tasks",
			Shortcut:    "tl",
			Category:    CategoryLinkedTasks,
			Views:       outlinerViews,
			Aliases:     []string{"show linked", "go to linked"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task != nil && project != nil {
					return ShowLinkedTaskSelectorMsg{Task: task, Project: project}
				}
				return nil
			},
		},
		{
			Name:        "manage dependencies",
			Description: "Manage task dependencies",
			Shortcut:    "bm",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"add dependency", "link dependency"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task != nil && project != nil {
					return ManageDependenciesMsg{Task: task, Project: project}
				}
				return nil
			},
		},
		{
			Name:        "manage decisions",
			Description: "View and manage design/architecture decisions for this task",
			Shortcut:    "td",
			Category:    CategoryDecisions,
			Views:       outlinerViews,
			Aliases:     []string{"decisions", "design decisions", "architecture decisions"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task != nil && project != nil {
					return ShowTaskDecisionsMsg{Task: task, Project: project}
				}
				return nil
			},
		},
		{
			Name:        "dim/undim task",
			Description: "Toggle dim state of task",
			Shortcut:    "th",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"dim task", "undim task", "mute task"},
			Handler: func() tea.Msg {
				out := m.activeOutliner()
				task, project := m.outlinerTarget()
				if out == nil || task == nil || project == nil {
					return nil
				}
				return out.taskItem.ToggleDimmed(task.ID)()
			},
		},
		{
			Name:        "move to project",
			Description: "Move task to another project",
			Shortcut:    "tm",
			Category:    CategoryTasks,
			Views:       outlinerViews,
			Aliases:     []string{"transfer task", "relocate task"},
			Handler: func() tea.Msg {
				task, project := m.outlinerTarget()
				if task != nil && project != nil {
					return ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
				}
				return nil
			},
		},
	}

	return cmds
}

// activeOutliner returns the OutlinerModel matching the non-palette origin so
// command-palette handlers operate on the outliner the user actually launched
// from. Falls back to whichever outliner is non-nil when the origin does not
// point at an outliner view (defensive — palette commands scope themselves to
// outliner views, so this fallback is rarely hit).
func (m *AppModel) activeOutliner() *OutlinerModel {
	switch m.nonPaletteOrigin() {
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			return m.crossOutliner
		}
	case ViewOutliner:
		if m.outliner != nil {
			return m.outliner
		}
	default:
	}
	if m.outliner != nil {
		return m.outliner
	}
	return m.crossOutliner
}

// outlinerTarget returns the cursor (task, project) on the active outliner,
// routing through getTaskRef so cross-project rows resolve their owning project
// via loadedProjects.
func (m *AppModel) outlinerTarget() (*models.Task, *models.Project) {
	out := m.activeOutliner()
	if out == nil {
		return nil, nil
	}
	return out.getTaskRef()
}

func (m *AppModel) outlinerScoreTarget() (*models.Task, *models.Project) {
	switch m.nonPaletteOrigin() {
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			return m.crossOutliner.getTaskRef()
		}
	case ViewOutliner:
		if m.outliner != nil {
			return m.outliner.getTaskRef()
		}
	default:
	}
	if m.outliner != nil {
		return m.outliner.getTaskRef()
	}
	return nil, nil
}

// outlinerCreateParent resolves the parent task and target project for palette
// task creation handlers. For single-project outliners with an active zoom
// stack it prefers the focused (zoomed) task as the parent so creation matches
// the in-focus subtree; otherwise it falls back to the cursor task. Cross
// project outliners always use the cursor row's task and owning project.
// Returns (nil, nil) when the cursor row has no resolvable task/project.
func (m *AppModel) outlinerCreateParent() (*models.Task, *models.Project) {
	out := m.activeOutliner()
	if out == nil {
		return nil, nil
	}
	task, project := m.outlinerTarget()
	var focusedTask *models.Task
	if out.project != nil && len(out.zoomStack) > 0 {
		topID := out.zoomStack[len(out.zoomStack)-1].TaskID
		focusedTask = out.project.GetTaskByID(topID)
	}
	parentTask := focusedTask
	if parentTask == nil {
		parentTask = task
	}
	if parentTask == nil || project == nil {
		return nil, nil
	}
	return parentTask, project
}

// outlinerCursorProject returns the project owning the cursor row on the active
// outliner. Returns nil when no outliner is active, the cursor is on a header
// or separator, or a cross-project row points at a stale project ID.
func (m *AppModel) outlinerCursorProject() *models.Project {
	out := m.activeOutliner()
	if out == nil {
		return nil
	}
	return out.cursorProject()
}

// paletteCursorProject returns the project that project-scoped palette commands
// should target based on the non-palette origin. Cross-project outliner origin
// uses the cursor row's project so palette handlers mirror the direct-key
// routing; every other origin falls back to m.currentProject.
func (m *AppModel) paletteCursorProject() *models.Project {
	if m.nonPaletteOrigin() == ViewCrossProjectOutliner && m.crossOutliner != nil {
		return m.crossOutliner.cursorProject()
	}
	return m.currentProject
}

// paletteTaskTarget returns the (task, project) pair that task-scoped palette
// commands should operate on, mirroring the direct-key behavior of each
// origin. Outliner origins use the active outliner's cursor row; project-tree
// origin uses the selected tree task and current project. Returns (nil, nil)
// when no task/project pair can be resolved.
func (m *AppModel) paletteTaskTarget() (*models.Task, *models.Project) {
	switch m.nonPaletteOrigin() {
	case ViewCrossProjectOutliner, ViewOutliner:
		return m.outlinerTarget()
	default:
	}
	return nil, nil
}

// runOutlinerCmd executes an outliner-targeted tea.Cmd and unwraps the result so
// command-palette handlers can return the resulting message directly. It mirrors
// runProjectTreeCmd: nil commands return nil, outlinerRefreshMsg is dispatched
// to the active outliner's Update so rows rebuild in place, and tea.BatchMsg
// values are walked recursively. All other messages pass through unchanged.
func (m *AppModel) runOutlinerCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()
	switch msg := msg.(type) {
	case outlinerRefreshMsg:
		if out := m.activeOutliner(); out != nil {
			_, _ = out.Update(msg)
		}
		return nil
	case tea.BatchMsg:
		for _, batchCmd := range msg {
			if nested := m.runOutlinerCmd(batchCmd); nested != nil {
				return nested
			}
		}
		return nil
	default:
		return msg
	}
}
