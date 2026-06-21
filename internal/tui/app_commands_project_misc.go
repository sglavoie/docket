package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

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
				m.projectTree.viewMode = (m.projectTree.viewMode + 1) % (ViewModeAll + 1)
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
			Description: "Open project Notes",
			Shortcut:    "ctrl+n",
			Category:    CategoryNotes,
			Views:       projectNotesHelpViews(),
			Aliases:     []string{"notes", "view notes", "open notes"},
			Handler: func() tea.Msg {
				if project := m.notesTargetProject(); project != nil {
					return ShowNotesMsg{Project: project}
				}
				return nil
			},
		},
		{
			Name:        "create note",
			Description: "Create a new note",
			Shortcut:    "}",
			Category:    CategoryNotes,
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"new note", "add note"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return ShowNoteCreatorMsg{Project: project}
			},
		},
	}
}

func projectNotesViews() []ViewType {
	return projectNotesHelpViews()
}

func (m *AppModel) notesTargetProject() *models.Project {
	if project := m.selectedTaskProjectFromOrigin(); project != nil {
		return project
	}
	return m.currentProject
}

func (m *AppModel) selectedTaskProjectFromOrigin() *models.Project {
	origin := m.nonPaletteOrigin()

	switch origin { //nolint:exhaustive // only task-origin views need special handling before the generic provider fallback.
	case ViewProjectTree:
		if m.projectTree != nil {
			if task, project := m.projectTree.getTaskRef(); task != nil && project != nil {
				return project
			}
		}
	case ViewOutliner:
		if m.outliner != nil {
			if task, project := m.outliner.getTaskRef(); task != nil && project != nil {
				return project
			}
		}
	case ViewCrossProjectOutliner:
		if m.crossOutliner != nil {
			if task, project := m.crossOutliner.getTaskRef(); task != nil && project != nil {
				return project
			}
		}
	}

	if v := m.views.Get(origin); v != nil {
		if trp, ok := v.(taskReferenceProvider); ok {
			if task, project := trp.getTaskRef(); task != nil && project != nil {
				return project
			}
		}
	}
	return nil
}

func (m *AppModel) otherCommands() []Command {
	return []Command{
		{
			Name:        "project focus",
			Description: "Project-scoped focus view",
			Shortcut:    "alt+f",
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
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"blockers", "blocked tasks"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				return ShowBlockersMsg{Project: project}
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
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"unarchive project"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				if project.Sandbox {
					return ErrorMsg{Err: fmt.Errorf("cannot archive the sandbox project")}
				}
				return ProjectArchiveRequestMsg{Project: project, Archive: !project.Archived}
			},
		},
		{
			Name:        "paste clipboard",
			Description: "Create a task in the project Inbox from clipboard text",
			Shortcut:    "ctrl+v",
			Category:    CategoryTasks,
			Views:       []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner},
			Aliases:     []string{"paste task", "clipboard paste"},
			Handler: func() tea.Msg {
				project := m.paletteCursorProject()
				if project == nil {
					if m.nonPaletteOrigin() == ViewCrossProjectOutliner {
						return StatusMsg(statusNoProjectUnderCursor)
					}
					return nil
				}
				switch m.nonPaletteOrigin() {
				case ViewOutliner, ViewCrossProjectOutliner:
					if out := m.activeOutliner(); out != nil {
						return m.runOutlinerCmd(out.pasteFromClipboardInto(project))
					}
				case ViewProjectTree:
					if m.projectTree != nil {
						return m.runProjectTreeCmd(m.projectTree.pasteFromClipboard())
					}
				}
				return nil
			},
		},
	}
}
