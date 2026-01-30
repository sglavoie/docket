package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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

func (m *AppModel) statusCommands() []Command {
	return []Command{
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
}

func (m *AppModel) dueDateCommands() []Command {
	return []Command{
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
}

func (m *AppModel) notesCommands() []Command {
	return []Command{
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
}

func (m *AppModel) otherCommands() []Command {
	return []Command{
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
}

func (m *AppModel) priorityCommands() []Command {
	return []Command{
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
}

func (m *AppModel) structureCommands() []Command {
	return []Command{
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
}

func (m *AppModel) sectionSendCommands() []Command {
	return []Command{
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
}

func (m *AppModel) linkedTaskCommands() []Command {
	return []Command{
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
}

func (m *AppModel) multiSelectCommands() []Command {
	return []Command{
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
}

func (m *AppModel) foldCommands() []Command {
	return []Command{
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
}

func (m *AppModel) filterBarCommands() []Command {
	return []Command{
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
}
