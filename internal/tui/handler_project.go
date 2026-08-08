package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// handleProjectMessages handles project-related messages (create, edit, archive, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case CreateProjectMsg:
		m.pendingProjectMoveTask = msg.MoveTask
		m.pendingProjectMoveSource = msg.SourceProject
		m.showView(ViewProjectCreator, NewProjectCreatorModel(m.runtime, m.configManager, m.appService))
		return true, nil

	case ProjectCreatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		m.refreshProjectHubIfActive()
		notesCmd := m.refreshActiveNotesManager()
		moveTask := m.pendingProjectMoveTask
		moveSource := m.pendingProjectMoveSource
		m.pendingProjectMoveTask = nil
		m.pendingProjectMoveSource = nil
		reloadCmd := m.requestCrossProjectOutlinerReload()
		projectName := "project"
		if msg.Project != nil {
			projectName = msg.Project.Name
		}
		feedbackCmd := m.setRecoveryFeedback("Created "+projectName, msg.TransactionID, false)
		if moveTask != nil && moveSource != nil && msg.Project != nil {
			moveCmd := func() tea.Msg {
				return MoveTaskToProjectMsg{
					Task:          moveTask,
					SourceProject: moveSource,
					DestProjectID: msg.Project.ID,
				}
			}
			return true, tea.Batch(popCmd, moveCmd, reloadCmd, notesCmd, feedbackCmd)
		}
		return true, tea.Batch(popCmd, reloadCmd, notesCmd, feedbackCmd)

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
		transactionID, err := m.commitCatalogProjectMutation(action, updated.Name, []string{updated.ID}, func(catalog *config.WorkspaceCatalog) error {
			return catalog.UpsertProject(&updated)
		})
		if err != nil {
			m.err = err
			return true, nil
		}
		copyCatalogMetadata(msg.Project, &updated)
		statusMsg := "Pinned"
		if !msg.Pinned {
			statusMsg = "Unpinned"
		}
		feedbackCmd := m.setRecoveryFeedback(statusMsg+" "+updated.Name, transactionID, false)
		m.refreshProjectHubIfActive()
		return true, tea.Batch(
			m.requestCrossProjectOutlinerReload(),
			m.refreshActiveNotesManager(),
			feedbackCmd,
		)

	case SetProjectTagsMsg:
		return true, m.requestProjectTags(msg.Project, msg.Tags)

	case ProjectBulkTagsMsg:
		return true, m.requestProjectBulkTags(msg)

	case projectEditorParkRequestMsg:
		return true, m.requestProjectEditorPark(msg.save)

	case projectParkConfirmedMsg:
		if save := m.pendingProjectEditorSave; save != nil {
			m.clearPendingProjectTags()
			return true, save.commit
		}
		if edit := m.pendingProjectBulkTags; edit != nil {
			m.clearPendingProjectTags()
			return true, m.commitProjectBulkTags(*edit)
		}
		project, tags := m.pendingProjectTags, m.pendingProjectTagsSet
		m.clearPendingProjectTags()
		return true, m.commitProjectTags(project, tags)

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
		if msg.Parked && msg.Project != nil {
			// The anchor reads the rows the park is about to remove, so it runs
			// before any cache the reload rebuilds from.
			m.anchorOutlinerCursorAfterProjectPark(msg.Project.ID)
		}
		popCmd := m.popView()
		m.syncProjectCache()
		// A parked project keeps its Today and Next references and they are
		// filtered at read time, so the shell caches are rederived before any
		// surface reads them again.
		m.rebuildPlanningFromRuntime()
		m.refreshWorkspacePulseAt(time.Now())
		m.refreshProjectView()
		m.refreshProjectHubIfActive()
		cmds := []tea.Cmd{
			popCmd,
			m.requestCrossProjectOutlinerReload(),
			m.refreshActiveNotesManager(),
		}
		status := "Updated project"
		if msg.Project != nil {
			status = "Updated " + msg.Project.Name
		}
		if msg.Parked && msg.Project != nil {
			status = projectTagStatus(msg.Project, true, false)
		}
		feedbackCmd := m.setRecoveryFeedback(status, msg.TransactionID, false)
		if msg.Parked && msg.Project != nil {
			m.appendRecoveryFeedbackDetail(projectTaskCountPhrase(msg.Project) + " hidden · Fp to show")
		}
		cmds = append(cmds, feedbackCmd)
		return true, tea.Batch(cmds...)

	}
	return false, nil
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
	result, err := m.commitRuntimeDomainsWithUndo(domains, "Archive Project", updated.Name, []string{updated.ID})
	if err != nil {
		m.err = err
		return nil
	}

	copyCatalogMetadata(project, &updated)
	if m.currentProject != nil && m.currentProject.ID == updated.ID {
		m.currentProject = nil
	}
	m.pruneDeletedTaskRefs()
	m.refreshWorkspacePulseAt(time.Now())
	action := fmt.Sprintf("Archived %s", updated.Name)
	feedbackMessage := action
	if result.MachineErr != nil {
		feedbackMessage = machineRepairWarning(action, result.MachineErr)
	}
	feedbackCmd := m.setRecoveryFeedback(feedbackMessage, result.TransactionID, false)
	if result.MachineErr != nil {
		m.statusIsInfo = true
		m.appendRecoveryFeedbackDetail("machine-state cleanup incomplete; retrying")
	}
	m.refreshProjectHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		feedbackCmd,
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
	transactionID, err := m.commitCatalogProjectMutation("Restore Project", updated.Name, []string{updated.ID}, func(catalog *config.WorkspaceCatalog) error {
		return catalog.UpsertProject(&updated)
	})
	if err != nil {
		m.err = err
		return nil
	}
	copyCatalogMetadata(project, &updated)
	feedbackCmd := m.setRecoveryFeedback(fmt.Sprintf("Unarchived %s", updated.Name), transactionID, false)
	m.refreshProjectHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		feedbackCmd,
	)
}
