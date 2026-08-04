package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// handleProjectMessages handles project-related messages (create, edit, archive, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case SelectProjectMsg:
		if msg.Project != nil && msg.Project.Archived {
			return true, m.showProjectArchiveConfirm(msg.Project, false)
		}
		return true, m.selectProjectNow(msg.Project)

	case CreateProjectMsg:
		m.pendingProjectMoveTask = msg.MoveTask
		m.pendingProjectMoveSource = msg.SourceProject
		m.showView(ViewProjectCreator, NewProjectCreatorModel(m.runtime, m.configManager, m.appService))
		return true, nil

	case ProjectCreatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		m.refreshScopeHubIfActive()
		notesCmd := m.refreshActiveNotesManager()
		moveTask := m.pendingProjectMoveTask
		moveSource := m.pendingProjectMoveSource
		m.pendingProjectMoveTask = nil
		m.pendingProjectMoveSource = nil
		reloadCmd := m.requestCrossProjectOutlinerReload()
		if moveTask != nil && moveSource != nil && msg.Project != nil {
			moveCmd := func() tea.Msg {
				return MoveTaskToProjectMsg{
					Task:          moveTask,
					SourceProject: moveSource,
					DestProjectID: msg.Project.ID,
					FromGlobal:    true,
				}
			}
			return true, tea.Batch(popCmd, moveCmd, reloadCmd, notesCmd)
		}
		return true, tea.Batch(popCmd, reloadCmd, notesCmd)

	case ProjectArchiveRequestMsg:
		if msg.Project != nil && m.runtime.IsSystemUnfiledProject(msg.Project.ID) {
			return true, errorCmd("cannot archive the system Unfiled project")
		}
		return true, m.showProjectArchiveConfirm(msg.Project, msg.Archive)

	case projectArchiveConfirmedMsg:
		return true, m.confirmProjectArchive()

	case ProjectArchivedMsg:
		return true, m.commitProjectArchive(msg.Project)

	case ProjectPinRequestMsg:
		if msg.Project == nil {
			return true, nil
		}
		updated := *msg.Project
		if msg.Pinned {
			updated.Pin()
		} else {
			updated.Unpin()
		}
		action := "Pin Project"
		if !msg.Pinned {
			action = "Unpin Project"
		}
		if err := m.commitCatalogProjectMutation(action, []string{updated.ID}, func(catalog *config.WorkspaceCatalog) error {
			return catalog.UpsertProject(&updated)
		}); err != nil {
			m.err = err
			return true, nil
		}
		copyCatalogMetadata(msg.Project, &updated)
		statusMsg := "Pinned"
		if !msg.Pinned {
			statusMsg = "Unpinned"
		}
		m.setSuccessStatus(statusMsg)
		m.refreshScopeHubIfActive()
		return true, tea.Batch(
			m.requestCrossProjectOutlinerReload(),
			m.refreshActiveNotesManager(),
			defaultStatusTimeout(),
		)

	case ProjectRestoredMsg:
		return true, m.commitProjectUnarchive(msg.Project)

	case EditProjectMsg:
		if msg.Project != nil {
			m.showView(ViewProjectEditor, NewProjectEditorModel(
				msg.Project,
				m.configManager,
				m.runtime,
				m.appService,
				m.runtime.IsSystemUnfiledProject(msg.Project.ID),
			))
		}
		return true, nil

	case ProjectUpdatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		// An edit that parked the project cleared its planning memberships in the
		// same transaction, so the shell caches are rederived before any surface
		// reads them again.
		m.rebuildPlanningFromRuntime()
		m.refreshProjectView()
		m.refreshScopeHubIfActive()
		return true, tea.Batch(
			popCmd,
			m.requestCrossProjectOutlinerReload(),
			m.refreshActiveNotesManager(),
		)

	}
	return false, nil
}

func (m *AppModel) selectProjectNow(project *models.Project) tea.Cmd {
	m.switchToProject(project, true)
	_, cmd := m.handleProjectViewMessages(ShowOutlinerMsg{Project: project})
	return cmd
}

func (m *AppModel) showProjectArchiveConfirm(project *models.Project, archive bool) tea.Cmd {
	if project == nil {
		m.clearPendingProjectArchive()
		return nil
	}
	if m.runtime.IsSystemUnfiledProject(project.ID) {
		m.clearPendingProjectArchive()
		return errorCmd("cannot archive the system Unfiled project")
	}

	m.pendingProjectArchive = project
	m.pendingProjectArchiveArchive = archive

	verb := "archive"
	if !archive {
		verb = "unarchive"
	}
	m.projectArchiveConfirm.Show(projectArchiveConfirmAction, fmt.Sprintf("Are you sure you want to %s this project?", verb))
	return nil
}

func (m *AppModel) clearPendingProjectArchive() {
	m.pendingProjectArchive = nil
	m.pendingProjectArchiveArchive = false
	m.projectArchiveConfirm.Cancel()
}

func (m *AppModel) confirmProjectArchive() tea.Cmd {
	project := m.pendingProjectArchive
	archive := m.pendingProjectArchiveArchive
	m.clearPendingProjectArchive()

	if project == nil {
		return nil
	}
	if m.runtime.IsSystemUnfiledProject(project.ID) {
		return errorCmd("cannot archive the system Unfiled project")
	}

	if archive {
		return func() tea.Msg {
			return ProjectArchivedMsg{Project: project}
		}
	}

	return func() tea.Msg {
		return ProjectRestoredMsg{Project: project}
	}
}

// commitProjectArchive applies the confirmed lifecycle mutation to an isolated
// copy, commits every catalog/planning/machine effect, and only then publishes
// the new metadata to the live project pointer. A rejected commit therefore
// cannot leak speculative archive state or clear the current project.
func (m *AppModel) commitProjectArchive(project *models.Project) tea.Cmd {
	if project == nil {
		return nil
	}
	updated := *project
	if updated.Tasks == nil {
		if m.persister == nil {
			return errorCmd("cannot archive project: project storage is unavailable")
		}
		if err := m.persister.Load(&updated); err != nil {
			return errorCmd("cannot archive project: %v", err)
		}
	}
	updated.Archive()

	archivedLocators := make([]config.ProjectTaskLocator, 0, len(updated.Tasks))
	for _, task := range updated.Tasks {
		if task != nil && task.ID != "" {
			archivedLocators = append(archivedLocators, config.ProjectTaskLocator{
				ProjectID: updated.ID,
				TaskID:    task.ID,
			})
		}
	}

	domains := cloneRuntimeDomains(m.runtime)
	if err := domains.Catalog.UpsertProject(&updated); err != nil {
		m.err = err
		return nil
	}
	if len(archivedLocators) > 0 {
		domains.Planning.RemoveDurableTaskLocators(archivedLocators)
	}
	result, err := m.commitRuntimeDomainsWithUndo(domains, "Archive Project", []string{updated.ID})
	if err != nil {
		m.err = err
		return nil
	}

	copyCatalogMetadata(project, &updated)
	if m.currentProject != nil && m.currentProject.ID == updated.ID {
		m.currentProject = nil
	}
	m.pruneDeletedTaskRefs()
	action := fmt.Sprintf("Archived %s", updated.Name)
	if result.MachineErr != nil {
		m.setWarningStatus(machineRepairWarning(action, result.MachineErr))
	} else {
		m.setSuccessStatus(action)
	}
	m.refreshScopeHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		defaultStatusTimeout(),
	)
}

// commitProjectUnarchive mirrors commitProjectArchive for the catalog-only
// recovery path: checked persistence precedes publication to every live cache.
func (m *AppModel) commitProjectUnarchive(project *models.Project) tea.Cmd {
	if project == nil {
		return nil
	}
	updated := *project
	updated.Unarchive()
	if err := m.commitCatalogProjectMutation("Restore Project", []string{updated.ID}, func(catalog *config.WorkspaceCatalog) error {
		return catalog.UpsertProject(&updated)
	}); err != nil {
		m.err = err
		return nil
	}
	copyCatalogMetadata(project, &updated)
	m.setSuccessStatus(fmt.Sprintf("Unarchived %s", updated.Name))
	m.refreshScopeHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		defaultStatusTimeout(),
	)
}
