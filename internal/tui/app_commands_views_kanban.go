package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) kanbanCommands() []Command {
	return []Command{
		{
			Name:        "move task left column",
			Description: "Move task to left column",
			Shortcut:    "{",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"column left", "move left"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'{'}}
			},
		},
		{
			Name:        "move task right column",
			Description: "Move task to right column",
			Shortcut:    "}",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"column right", "move right"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'}'}}
			},
		},
		{
			Name:        "swap task up in column",
			Description: "Swap task with one above in column",
			Shortcut:    "K",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"reorder up column", "move up column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
			},
		},
		{
			Name:        "swap task down in column",
			Description: "Swap task with one below in column",
			Shortcut:    "J",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"reorder down column", "move down column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
			},
		},
		{
			Name:        "go to active column",
			Description: "Jump to Active column",
			Shortcut:    "ga",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"active column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to next column",
			Description: "Jump to Next column",
			Shortcut:    "gn",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"next column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to inbox column",
			Description: "Jump to Inbox column",
			Shortcut:    "gi",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"inbox column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to waiting column",
			Description: "Jump to Waiting column",
			Shortcut:    "gw",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"waiting column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "go to someday column",
			Description: "Jump to Someday column",
			Shortcut:    "gs",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"someday column"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to active section",
			Description: "Move task to Active section",
			Shortcut:    "gA",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send active"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to next section",
			Description: "Move task to Next section",
			Shortcut:    "gN",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send next"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to inbox section",
			Description: "Move task to Inbox section",
			Shortcut:    "gI",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to waiting section",
			Description: "Move task to Waiting section",
			Shortcut:    "gW",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send waiting"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "send to someday section",
			Description: "Move task to Someday section",
			Shortcut:    "gS",
			Category:    CategoryKanban,
			Views:       []ViewType{ViewKanban},
			Aliases:     []string{"kanban send someday"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
	}
}

func (m *AppModel) kanbanExtCommands() []Command {
	return []Command{
		// Task operations
		{Name: "edit task", Description: "Edit selected task", Shortcut: "e", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"open editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "quick edit", Description: "Inline quick edit with modifier syntax", Shortcut: "E", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"inline edit"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "rename task", Description: "Inline rename task title", Shortcut: "r", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"rename"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}} }},
		{Name: "delete task", Description: "Delete selected task", Shortcut: "D", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"remove task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}} }},
		{Name: "new task", Description: "Create a new task", Shortcut: "n", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"create task", "add task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}} }},
		{Name: "quick capture", Description: "Quick capture a single task", Shortcut: "i", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"capture", "inbox"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}} }},
		{Name: "bulk capture", Description: "Bulk capture multiple tasks", Shortcut: "I", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"multi capture"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'I'}} }},
		{Name: "view task detail", Description: "Open task detail view", Shortcut: "enter", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"open task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "task comments", Description: "View and add task comments", Shortcut: "C", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"comments"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}} }},
		{Name: "task attachments", Description: "View and manage task attachments", Shortcut: "A", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"attachments"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}} }},
		{Name: "dependency viewer", Description: "View task dependency graph", Shortcut: "R", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"dependency graph"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}} }},
		{Name: "edit content", Description: "Edit task description in external editor", Shortcut: "ctrl+o", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"edit description"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlO} }},
		{Name: "move to project", Description: "Move task to another project", Shortcut: "M", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"transfer task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}} }},
		{Name: "undo", Description: "Undo last change", Shortcut: "u", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"undo change", "revert"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}} }},
		{Name: "undo history", Description: "View undo history", Shortcut: "ctrl+z", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"undo list"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlZ} }},
		// Status (comma prefix)
		{Name: "start task", Description: "Set task status to in progress", Shortcut: ",s", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"begin task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "complete task", Description: "Set task status to done", Shortcut: ",d", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"done task", "finish task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "block task", Description: "Set task status to blocked", Shortcut: ",b", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"blocked task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "cancel task", Description: "Set task status to canceled", Shortcut: ",x", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"canceled task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		{Name: "reopen task", Description: "Reopen a completed or canceled task", Shortcut: ",o", Category: CategoryStatus, Views: []ViewType{ViewKanban}, Aliases: []string{"reopen"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}} }},
		// Priority (! prefix)
		{Name: "priority critical", Description: "Set priority to Critical (1)", Shortcut: "!1", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p1"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority high", Description: "Set priority to High (2)", Shortcut: "!2", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p2"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority normal", Description: "Set priority to Normal (3)", Shortcut: "!3", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p3"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority low", Description: "Set priority to Low (4)", Shortcut: "!4", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p4"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		{Name: "priority someday", Description: "Set priority to Someday (5)", Shortcut: "!5", Category: CategoryPriority, Views: []ViewType{ViewKanban}, Aliases: []string{"p5"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}} }},
		// Due date (d prefix)
		{Name: "due today", Description: "Set due date to today", Shortcut: "dt", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"today"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "due tomorrow", Description: "Set due date to tomorrow", Shortcut: "dm", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"tomorrow"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "due next week", Description: "Set due date to +7 days", Shortcut: "dw", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"next week"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "due next month", Description: "Set due date to +30 days", Shortcut: "dn", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"next month"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "clear due date", Description: "Remove due date from task", Shortcut: "d0", Category: CategoryDueDate, Views: []ViewType{ViewKanban}, Aliases: []string{"remove due date"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		// Linked tasks (c prefix)
		{Name: "create blocker", Description: "Create blocker linked task", Shortcut: "cb", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "create dependent", Description: "Create dependent linked task", Shortcut: "cd", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add dependent"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "create subtask", Description: "Create subtask linked task", Shortcut: "cs", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add subtask"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "create follow-up", Description: "Create follow-up linked task", Shortcut: "cf", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"add follow-up"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "navigate linked tasks", Description: "Navigate to linked tasks", Shortcut: "cn", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"show linked"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "unlink task", Description: "Remove linked task relationship", Shortcut: "cx", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"remove link"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		{Name: "cross-project link", Description: "Link task across projects", Shortcut: "cp", Category: CategoryLinkedTasks, Views: []ViewType{ViewKanban}, Aliases: []string{"cross link"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}} }},
		// Copy
		{Name: "copy title", Description: "Copy task title to clipboard", Shortcut: "y", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"yank title"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}} }},
		{Name: "copy details", Description: "Copy task with details to clipboard", Shortcut: "Y", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"yank details"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}} }},
		{Name: "copy with description", Description: "Copy task title and description", Shortcut: "ctrl+y", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"yank description"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlY} }},
		// Flags
		{Name: "toggle active", Description: "Toggle active flag on task", Shortcut: "*", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"star task", "flag task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'*'}} }},
		{Name: "toggle pin", Description: "Toggle pinned status on task", Shortcut: "ctrl+p", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"pin task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlP} }},
		// Tags/sections
		{Name: "manage tags", Description: "Add or remove tags on task", Shortcut: "ctrl+g", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"tag task", "edit tags"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlG} }},
		{Name: "section picker", Description: "Pick section for task", Shortcut: "ctrl+n", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"change section"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlN} }},
		{Name: "tag manager", Description: "Manage project tags", Shortcut: "ctrl+t", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"manage project tags"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlT} }},
		{Name: "section manager", Description: "Manage project sections", Shortcut: "ctrl+s", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"manage sections"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlS} }},
		{Name: "tag filter", Description: "Filter tasks by tag", Shortcut: "T", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"filter tag"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}} }},
		{Name: "manage dependencies", Description: "Manage task dependencies", Shortcut: "ctrl+b", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"add dependency"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlB} }},
		// Sort
		{Name: "sort by priority", Description: "Sort tasks by priority", Shortcut: "1", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"priority sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}} }},
		{Name: "sort by due date", Description: "Sort tasks by due date", Shortcut: "2", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"due date sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}} }},
		{Name: "sort by created", Description: "Sort tasks by creation date", Shortcut: "3", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"created sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}} }},
		{Name: "sort by status", Description: "Sort tasks by status", Shortcut: "4", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"status sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}} }},
		{Name: "reset sort", Description: "Reset sort to default order", Shortcut: "0", Category: CategorySort, Views: []ViewType{ViewKanban}, Aliases: []string{"default sort"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}} }},
		// Filters
		{Name: "toggle filter bar", Description: "Show/hide the filter bar", Shortcut: "F", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"filter bar"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}} }},
		{Name: "filter overlay", Description: "Open active filters overlay", Shortcut: "ctrl+f", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"advanced filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlF} }},
		{Name: "completed view toggle", Description: "Toggle completed view (F1)", Shortcut: "F1", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"show completed"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF1} }},
		{Name: "high priority filter", Description: "Toggle high priority filter (F2)", Shortcut: "F2", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"priority filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF2} }},
		{Name: "type filter cycle", Description: "Cycle type filter (F3)", Shortcut: "F3", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f3 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF3} }},
		{Name: "status filter cycle", Description: "Cycle status filter (F4)", Shortcut: "F4", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f4 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF4} }},
		{Name: "tag filter selector", Description: "Open tag filter (F5)", Shortcut: "F5", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f5 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF5} }},
		{Name: "due date filter cycle", Description: "Cycle due date filter (F7)", Shortcut: "F7", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f7 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF7} }},
		{Name: "actionable filter", Description: "Toggle actionable filter (F8)", Shortcut: "F8", Category: CategoryFilters, Views: []ViewType{ViewKanban}, Aliases: []string{"f8 filter"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyF8} }},
		// Fold (z prefix)
		{Name: "toggle fold", Description: "Toggle expand/collapse current node", Shortcut: "za", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"toggle expand"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		{Name: "open fold", Description: "Expand current node", Shortcut: "zo", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"expand node"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		{Name: "close fold", Description: "Collapse current node", Shortcut: "zc", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"collapse node"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		{Name: "focus subtree", Description: "Focus on current subtree only", Shortcut: "zv", Category: CategoryFold, Views: []ViewType{ViewKanban}, Aliases: []string{"zoom subtree"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}} }},
		// Multi-select
		{Name: "toggle multi-select", Description: "Toggle multi-select mode", Shortcut: "v", Category: CategoryMultiSelect, Views: []ViewType{ViewKanban}, Aliases: []string{"multi select"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}} }},
		{Name: "select all in column", Description: "Select all tasks in column", Shortcut: "a", Category: CategoryMultiSelect, Views: []ViewType{ViewKanban}, Aliases: []string{"select all column"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "toggle selection", Description: "Toggle selection of current task", Shortcut: "space", Category: CategoryMultiSelect, Views: []ViewType{ViewKanban}, Aliases: []string{"select task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeySpace} }},
		// Type management
		{Name: "promote type", Description: "Promote task type (task → story → epic)", Shortcut: "<", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"promote", "type up"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}} }},
		{Name: "demote type", Description: "Demote task type (epic → story → task)", Shortcut: ">", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"demote", "type down"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}} }},
		{Name: "cycle leaf type left", Description: "Cycle leaf type left", Shortcut: "H", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"type left"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}} }},
		{Name: "cycle leaf type right", Description: "Cycle leaf type right", Shortcut: "L", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"type right"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}} }},
		// Structure
		{Name: "outdent task", Description: "Outdent task (move to parent's level)", Shortcut: "←", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"unnest task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyLeft} }},
		{Name: "indent task", Description: "Indent task (nest under task above)", Shortcut: "→", Category: CategoryStructure, Views: []ViewType{ViewKanban}, Aliases: []string{"nest task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRight} }},
		// Views/nav
		{Name: "previous tab", Description: "Switch to previous tab", Shortcut: "[", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"prev tab"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}} }},
		{Name: "next tab", Description: "Switch to next tab", Shortcut: "]", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"next tab"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}} }},
		{Name: "blockers view", Description: "Show blockers and blocked tasks", Shortcut: "b", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"blockers"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}} }},
		{Name: "priority view", Description: "View tasks by priority", Shortcut: "p", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"priority"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}} }},
		{Name: "inbox processor", Description: "Process inbox items", Shortcut: "w", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"inbox"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}} }},
		{Name: "focus subtree view", Description: "Focus on current subtree", Shortcut: "f", Category: CategoryViews, Views: []ViewType{ViewKanban}, Aliases: []string{"focus mode"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}} }},
		{Name: "task jumper", Description: "Jump to task by ID or name", Shortcut: "\\", Category: CategorySearch, Views: []ViewType{ViewKanban}, Aliases: []string{"find task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\\'}} }},
		{Name: "cycle forward project", Description: "Switch to next project in cycle", Shortcut: "`", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"next project"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}} }},
		{Name: "cycle backward project", Description: "Switch to previous project in cycle", Shortcut: "~", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"previous project"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'~'}} }},
		{Name: "project cycle manager", Description: "Configure project cycle", Shortcut: "@", Category: CategoryNavigation, Views: []ViewType{ViewKanban}, Aliases: []string{"cycle manager"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}} }},
		{Name: "archive completed", Description: "Move completed tasks to archive", Shortcut: "ctrl+a", Category: CategoryActions, Views: []ViewType{ViewKanban}, Aliases: []string{"archive"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlA} }},
		{Name: "edit last modified", Description: "Edit the most recently modified task", Shortcut: "ctrl+l", Category: CategoryKanban, Views: []ViewType{ViewKanban}, Aliases: []string{"recent task"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlL} }},
		{Name: "reload project", Description: "Reload project data from storage", Shortcut: "ctrl+r", Category: CategoryActions, Views: []ViewType{ViewKanban}, Aliases: []string{"refresh"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyCtrlR} }},
	}
}
