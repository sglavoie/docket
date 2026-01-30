package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *AppModel) notesViewCommands() []Command {
	return []Command{
		{
			Name:        "edit note",
			Description: "Edit note in external editor",
			Shortcut:    "e",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"open note editor", "modify note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
			},
		},
		{
			Name:        "delete note",
			Description: "Delete selected note",
			Shortcut:    "d",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"remove note", "trash note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "undo delete note",
			Description: "Undo last note deletion",
			Shortcut:    "u",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"restore note", "undelete note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
		{
			Name:        "link note to task",
			Description: "Link note to a task",
			Shortcut:    "l",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"attach note", "note task link"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
			},
		},
		{
			Name:        "toggle note pin",
			Description: "Toggle pin status on note",
			Shortcut:    "p",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"pin note", "unpin note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
			},
		},
		{
			Name:        "edit note tags",
			Description: "Add or remove tags on note",
			Shortcut:    "t",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"tag note", "note tags"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
			},
		},
		{
			Name:        "cycle note tag filter",
			Description: "Open note tag filter selector",
			Shortcut:    "T",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"filter notes by tag"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}}
			},
		},
		{
			Name:        "search notes",
			Description: "Search notes by title or content",
			Shortcut:    "/",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes},
			Aliases:     []string{"find note", "note search"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
			},
		},
		{
			Name:        "create note",
			Description: "Create a new note",
			Shortcut:    "n",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"new note", "add note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "rename note",
			Description: "Rename selected note inline",
			Shortcut:    "r",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"note rename", "change note title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "cycle note sort",
			Description: "Cycle sort mode (date/title/pinned)",
			Shortcut:    "s",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"sort notes", "note sort mode"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "clear tag filter",
			Description: "Clear all note tag filters",
			Shortcut:    "C",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"reset note filters", "clear note tags"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}}
			},
		},
		{
			Name:        "copy note title",
			Description: "Copy note title to clipboard",
			Shortcut:    "y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"yank note title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy note body",
			Description: "Copy note body to clipboard",
			Shortcut:    "Y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"yank note body"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy note title and body",
			Description: "Copy title and body to clipboard",
			Shortcut:    "ctrl+y",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"yank note full"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
		{
			Name:        "toggle multi-select",
			Description: "Toggle note multi-select mode",
			Shortcut:    "V",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"multi select notes"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'V'}}
			},
		},
		{
			Name:        "view note",
			Description: "View selected note",
			Shortcut:    "enter",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewNotes, ViewGlobalNotes},
			Aliases:     []string{"open note", "read note"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEnter}
			},
		},
	}
}

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

func (m *AppModel) blockersCommands() []Command {
	return []Command{
		{Name: "add blocker", Description: "Add a new blocker", Shortcut: "a", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"new blocker", "n"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "edit blocker", Description: "Edit selected blocker", Shortcut: "e", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"modify blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "edit in editor", Description: "Edit blocker in external editor", Shortcut: "E", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "toggle resolve", Description: "Toggle resolve/unresolve blocker", Shortcut: "R", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"resolve blocker", "unresolve"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}} }},
		{Name: "delete blocker", Description: "Delete selected blocker", Shortcut: "d", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"remove blocker", "x"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "toggle show resolved", Description: "Toggle showing resolved blockers", Shortcut: "tab", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"show resolved"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyTab} }},
		{Name: "expand blocker", Description: "Expand/view blocker details", Shortcut: "enter", Category: CategoryBlockers, Views: []ViewType{ViewBlockers}, Aliases: []string{"view blocker"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
	}
}

func (m *AppModel) taskCommentsCommands() []Command {
	return []Command{
		{Name: "add comment", Description: "Add a new comment", Shortcut: "a", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"new comment", "n"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}} }},
		{Name: "edit comment", Description: "Edit selected comment", Shortcut: "e", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"modify comment"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}} }},
		{Name: "edit in editor", Description: "Edit comment in external editor", Shortcut: "E", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"external editor"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}} }},
		{Name: "delete comment", Description: "Delete selected comment", Shortcut: "d", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"remove comment", "x"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}} }},
		{Name: "expand comment", Description: "Expand/view comment details", Shortcut: "enter", Category: CategoryComments, Views: []ViewType{ViewTaskComments}, Aliases: []string{"view comment"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
	}
}

func (m *AppModel) undoHistoryCommands() []Command {
	return []Command{
		{Name: "perform undo", Description: "Undo selected entry or view detail", Shortcut: "enter", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"undo entry", "view detail"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "switch pane", Description: "Switch between entry list and diff pane", Shortcut: "tab", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"toggle pane"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyTab} }},
		{Name: "scroll diff left", Description: "Scroll diff pane left", Shortcut: "h", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"diff left"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}} }},
		{Name: "scroll diff right", Description: "Scroll diff pane right", Shortcut: "l", Category: CategoryUndoHistory, Views: []ViewType{ViewUndoHistory}, Aliases: []string{"diff right"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}} }},
	}
}

func (m *AppModel) globalTagManagerCommands() []Command {
	return []Command{
		{Name: "navigate to tag project", Description: "Navigate to tag's project", Shortcut: "enter", Category: CategoryActions, Views: []ViewType{ViewGlobalTagManager}, Aliases: []string{"open tag project"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }},
		{Name: "rename tag", Description: "Rename selected tag", Shortcut: "r", Category: CategoryActions, Views: []ViewType{ViewGlobalTagManager}, Aliases: []string{"edit tag name"}, Handler: func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}} }},
	}
}

func (m *AppModel) attachmentCommands() []Command {
	return []Command{
		{
			Name:        "add attachment",
			Description: "Add a new attachment to task",
			Shortcut:    "a",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"attach file", "new attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
			},
		},
		{
			Name:        "open attachment",
			Description: "Open selected attachment",
			Shortcut:    "o",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"view attachment", "launch attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}
			},
		},
		{
			Name:        "open attachments directory",
			Description: "Open directory containing attachments",
			Shortcut:    "O",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"reveal attachments", "attachments folder"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}}
			},
		},
		{
			Name:        "open original path",
			Description: "Open attachment at its original path",
			Shortcut:    "ctrl+o",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"original location", "source path"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlO}
			},
		},
		{
			Name:        "delete attachment",
			Description: "Delete selected attachment",
			Shortcut:    "d",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"remove attachment"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "unlink attachment",
			Description: "Unlink attachment but keep file",
			Shortcut:    "u",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"detach file", "keep and unlink"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
		{
			Name:        "copy attachment path",
			Description: "Copy attachment path to clipboard",
			Shortcut:    "y",
			Category:    CategoryAttachments,
			Views:       []ViewType{ViewTaskAttachments},
			Aliases:     []string{"yank path", "clipboard path"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
	}
}

func (m *AppModel) focusViewCommands() []Command {
	return []Command{
		{
			Name:        "remove from focus",
			Description: "Remove active flag from task",
			Shortcut:    "*",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"unflag task", "unstar task", "deactivate"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'*'}}
			},
		},
		{
			Name:        "go to task in project",
			Description: "Navigate to task in project tree",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"open in project", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewFocusView, ViewProjectFocusView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}

func (m *AppModel) dailyReviewCommands() []Command {
	return []Command{
		{
			Name:        "focus on due today",
			Description: "Focus on due today section",
			Shortcut:    "sd",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"due today section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on top priority",
			Description: "Focus on top priority section",
			Shortcut:    "st",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"top priority section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on stale tasks",
			Description: "Focus on stale tasks section",
			Shortcut:    "ss",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"stale section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on recently logged",
			Description: "Focus on recently logged section",
			Shortcut:    "sl",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"recently logged section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on flagged tasks",
			Description: "Focus on flagged/active tasks section",
			Shortcut:    "sf",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"flagged section", "active section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "focus on overdue tasks",
			Description: "Focus on overdue tasks section",
			Shortcut:    "so",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"overdue section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "collapse/expand section",
			Description: "Toggle section collapse/expand",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"fold section", "toggle section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyTab}
			},
		},
		{
			Name:        "next section",
			Description: "Jump to next section",
			Shortcut:    "n",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"section forward"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "previous section",
			Description: "Jump to previous section",
			Shortcut:    "N",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"section backward"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDailyReview},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}

func (m *AppModel) priorityViewCommands() []Command {
	return []Command{
		{
			Name:        "start suggested task",
			Description: "Start the suggested task",
			Shortcut:    "s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"begin suggestion", "accept suggestion"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "dismiss suggestion",
			Description: "Dismiss the current suggestion",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"skip suggestion", "reject suggestion"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
			},
		},
		{
			Name:        "go to task in tree",
			Description: "Navigate to task in project tree",
			Shortcut:    "gt",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"open in tree", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "toggle waiting section",
			Description: "Toggle waiting-on section visibility",
			Shortcut:    "tab",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"show waiting", "hide waiting"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyTab}
			},
		},
		{
			Name:        "refresh priority view",
			Description: "Refresh priority data",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"reload priorities"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewPriorityView},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}

func (m *AppModel) inboxProcessorCommands() []Command {
	return []Command{
		{
			Name:        "skip task",
			Description: "Skip current inbox task",
			Shortcut:    "n",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"next inbox task", "pass"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "assign section",
			Description: "Assign task to a section",
			Shortcut:    "s",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"pick section", "move to section"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
			},
		},
		{
			Name:        "send to waiting",
			Description: "Send task to Waiting section",
			Shortcut:    "w",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"waiting", "defer task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
			},
		},
		{
			Name:        "manage inbox task tags",
			Description: "Add or remove tags on task",
			Shortcut:    "t",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"tag inbox task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
			},
		},
		{
			Name:        "create subtask from inbox",
			Description: "Create a subtask for current task",
			Shortcut:    "c",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"add subtask inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
			},
		},
		{
			Name:        "set inbox task due date",
			Description: "Set due date for current task",
			Shortcut:    "D",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox due date", "deadline"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
			},
		},
		{
			Name:        "manage inbox dependencies",
			Description: "Manage task dependencies",
			Shortcut:    "b",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox dependencies", "add dependency"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
			},
		},
		{
			Name:        "delete inbox task",
			Description: "Delete current inbox task",
			Shortcut:    "x",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"remove inbox task"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
			},
		},
		{
			Name:        "undo inbox action",
			Description: "Undo last inbox processing action",
			Shortcut:    "u",
			Category:    CategoryActions,
			Views:       []ViewType{ViewInboxProcessor},
			Aliases:     []string{"inbox undo", "revert inbox"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}
			},
		},
	}
}

func (m *AppModel) dueDateOverviewCommands() []Command {
	return []Command{
		{
			Name:        "refresh due dates",
			Description: "Refresh due date overview data",
			Shortcut:    "r",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"reload due dates"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
			},
		},
		{
			Name:        "go to task project",
			Description: "Navigate to task in project tree",
			Shortcut:    "gp",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"open in project", "show in tree"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
			},
		},
		{
			Name:        "copy task title",
			Description: "Copy task title to clipboard",
			Shortcut:    "y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"yank title"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
			},
		},
		{
			Name:        "copy task details",
			Description: "Copy task with details to clipboard",
			Shortcut:    "Y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"yank details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
			},
		},
		{
			Name:        "copy task with description",
			Description: "Copy task title and description",
			Shortcut:    "ctrl+y",
			Category:    CategoryActions,
			Views:       []ViewType{ViewDueDateOverview},
			Aliases:     []string{"yank description"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyCtrlY}
			},
		},
	}
}

func (m *AppModel) workflowRulesCommands() []Command {
	return []Command{
		{
			Name:        "create workflow rule",
			Description: "Create a new workflow rule",
			Shortcut:    "n",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"new rule", "add rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
			},
		},
		{
			Name:        "edit workflow rule",
			Description: "Edit selected workflow rule",
			Shortcut:    "e",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"modify rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
			},
		},
		{
			Name:        "delete workflow rule",
			Description: "Delete selected workflow rule",
			Shortcut:    "d",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"remove rule"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
			},
		},
		{
			Name:        "view workflow rule",
			Description: "View rule details",
			Shortcut:    "enter",
			Category:    CategoryWorkflowRules,
			Views:       []ViewType{ViewWorkflowRules},
			Aliases:     []string{"open rule", "rule details"},
			Handler: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEnter}
			},
		},
	}
}

func (m *AppModel) sidebarCommands() []Command {
	return []Command{
		{
			Name:        "enable sidebar for all projects",
			Description: "Turn on sidebar for every active project",
			Category:    CategoryView,
			Views:       []ViewType{},
			Aliases:     []string{"sidebar on all", "show sidebar all"},
			Handler: func() tea.Msg {
				for _, p := range m.config.GetActiveProjects() {
					state, _ := m.config.GetProjectViewState(p.ID)
					state.SidebarEnabled = true
					m.config.SetProjectViewState(p.ID, state)
				}
				_ = m.configManager.Save(m.config)
				if m.projectTree != nil {
					m.projectTree.sidebar.SetEnabled(true)
				}
				return nil
			},
		},
		{
			Name:        "disable sidebar for all projects",
			Description: "Turn off sidebar for every active project",
			Category:    CategoryView,
			Views:       []ViewType{},
			Aliases:     []string{"sidebar off all", "hide sidebar all"},
			Handler: func() tea.Msg {
				for _, p := range m.config.GetActiveProjects() {
					state, _ := m.config.GetProjectViewState(p.ID)
					state.SidebarEnabled = false
					m.config.SetProjectViewState(p.ID, state)
				}
				_ = m.configManager.Save(m.config)
				if m.projectTree != nil {
					m.projectTree.sidebar.SetEnabled(false)
				}
				return nil
			},
		},
	}
}
