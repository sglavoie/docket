package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

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
