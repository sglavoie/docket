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
			Views:       projectNotesViews(),
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

func projectNotesViews() []ViewType {
	return []ViewType{
		ViewProjectTree,
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
		ViewFocusBoard,
		ViewAllTasks,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewFocusView,
		ViewProjectFocusView,
		ViewPriorityView,
		ViewDueDateOverview,
		ViewKanban,
		ViewCrossProjectKanban,
		ViewRecentActivity,
		ViewInboxProcessor,
		ViewGlobalInboxProcessor,
		ViewTaskDetail,
	}
}

func (m *AppModel) notesTargetProject() *models.Project {
	if project := m.selectedTaskProjectFromOrigin(); project != nil {
		return project
	}
	return m.currentProject
}

func (m *AppModel) selectedTaskProjectFromOrigin() *models.Project {
	origin := m.nonPaletteOrigin()

	switch origin {
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
