package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// reconcileNoteLedgerRestore advances every shell cache affected by the exact
// workspace transaction restored from a notes view. The ledger operation has
// already completed before this runs, so failures are accumulated while safe
// refresh work continues and are reported separately from restore failures.
func (m *AppModel) reconcileNoteLedgerRestore(entry storage.LedgerEntry) ([]tea.Cmd, error) {
	var refreshErrs []error
	runtimeReloaded := true
	if m.runtime != nil && m.runtime.DataDir() != "" {
		if err := m.runtime.RefreshAfterCommit(); err != nil {
			runtimeReloaded = false
			refreshErrs = append(refreshErrs, fmt.Errorf("reload runtime configuration: %w", err))
		} else {
			m.syncProjectCache()
		}
	} else {
		m.syncProjectCache()
	}

	reloadedProjects, projectErrs := m.reloadLedgerProjects(entry.ProjectIDs)
	refreshErrs = append(refreshErrs, projectErrs...)
	if runtimeReloaded {
		if err := m.repairTaskProjectRefsAfterRestore(reloadedProjects); err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("repair task references: %w", err))
		}
	}

	cmds := m.refreshTaskViewsAfterRestore(runtimeReloaded)
	// A few established view refreshers reload their own project pointer from
	// disk. Rebind the shell-owned planning/visit pointers once more after that
	// fan-out without repeating any persistent runtime repair.
	if runtimeReloaded {
		m.rewireTaskProjectRefsAfterRestore(reloadedProjects)
	}
	return cmds, errors.Join(refreshErrs...)
}

// reloadLedgerProjects reloads only the projects named by the restored entry
// and returns the successful set. Reference repair must never use a project
// whose disk reload failed, because its task pointers still describe the state
// from before the ledger restore.
func (m *AppModel) reloadLedgerProjects(projectIDs []string) ([]*models.Project, []error) {
	projects := make([]*models.Project, 0, len(projectIDs))
	var refreshErrs []error
	seen := make(map[string]bool, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID == "" || seen[projectID] {
			continue
		}
		seen[projectID] = true
		project := m.ledgerProjectByID(projectID)
		if project == nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("reload project %q: project is not in the runtime catalog", projectID))
			continue
		}
		if err := storage.LoadProject(project); err != nil {
			refreshErrs = append(refreshErrs, fmt.Errorf("reload project %q: %w", projectID, err))
			continue
		}
		projects = append(projects, project)
	}
	return projects, refreshErrs
}

func (m *AppModel) ledgerProjectByID(projectID string) *models.Project {
	if project := m.projectByID(projectID); project != nil {
		return project
	}
	if m.currentProject != nil && tuiUndoProjectID(m.currentProject) == projectID {
		return m.currentProject
	}
	if m.views != nil {
		if view, ok := m.views.Get(ViewNotes).(*NotesViewModel); ok {
			if project := view.projectByID(projectID); project != nil {
				return project
			}
		}
	}
	return nil
}

func (m *AppModel) reloadActiveNotesAfterLedgerRestore() *NotesViewModel {
	view := m.getActiveNotesView()
	if view == nil {
		return nil
	}
	view.loadNotes()
	return view
}
