package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/status"
)

// handleProjectMessages handles project-related messages (create, edit, archive, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case projecthub.CreateProjectMsg:
		m.showView(presentation.ViewProjectCreator, projecthub.NewProjectCreatorModel(m.domain.runtime, m.domain.manager, m.domain.service))
		return true, nil

	case projecthub.ProjectCreatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		m.refreshProjectHubIfActive()
		notesCmd := m.refreshActiveNotesManager()
		reloadCmd := m.requestCrossProjectOutlinerReload()
		projectName := "project"
		if msg.Project != nil {
			projectName = msg.Project.Name
			m.project.SeatProjectHubCursor(m, msg.Project.ID)
		}
		feedbackCmd := m.setRecoveryFeedback("Created "+projectName, msg.TransactionID, false)
		return true, tea.Batch(popCmd, reloadCmd, notesCmd, feedbackCmd)

	case projecthub.ProjectArchiveRequestMsg:
		if msg.Project != nil && m.domain.runtime.IsSystemUnfiledProject(msg.Project.ID) {
			return true, status.ErrorCmd("cannot archive the system Unfiled project")
		}
		return true, m.showProjectArchiveConfirm(msg.Project, msg.Archive)

	case projecthub.ProjectArchiveConfirmedMsg:
		return true, m.confirmProjectArchive()

	case projecthub.ProjectArchivedMsg:
		return true, m.commitProjectArchive(msg.Project)

	case projecthub.ProjectDeleteRequestMsg:
		if msg.Project != nil && m.domain.runtime.IsSystemUnfiledProject(msg.Project.ID) {
			return true, status.ErrorCmd("cannot delete the system Unfiled project")
		}
		return true, m.showProjectDeleteConfirm(msg.Project)

	case projecthub.ProjectDeleteConfirmedMsg:
		return true, m.confirmProjectDelete()

	case projecthub.ProjectDeletedMsg:
		return true, m.commitProjectDelete(msg.Project)

	case projecthub.ProjectPinRequestMsg:
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
			m.feedback.err = err
			return true, nil
		}
		coordinator.CopyCatalogMetadata(msg.Project, &updated)
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

	case projecthub.SetProjectTagsMsg:
		return true, m.requestProjectTags(msg.Project, msg.Tags)

	case projecthub.AddProjectCommentMsg:
		return true, m.addProjectCommentCmd(msg)

	case projecthub.ProjectCommentRejectedMsg:
		return true, m.reopenProjectCommentPrompt(msg)

	case projecthub.ProjectBulkTagsMsg:
		return true, m.requestProjectBulkTags(msg)

	case projecthub.ProjectEditorParkRequestMsg:
		return true, m.requestProjectEditorPark(msg.Save)

	case projecthub.ProjectParkConfirmedMsg:
		pending := m.project.TakePark()
		if save := pending.EditorSave; save != nil {
			return true, save.Commit
		}
		if edit := pending.Bulk; edit != nil {
			return true, m.commitProjectBulkTags(*edit)
		}
		return true, m.commitProjectTags(pending.Project, pending.Tags)

	case projecthub.ProjectRestoredMsg:
		return true, m.commitProjectUnarchive(msg.Project)

	case projecthub.EditProjectMsg:
		if msg.Project != nil {
			m.showView(presentation.ViewProjectEditor, projecthub.NewProjectEditorModel(
				msg.Project,
				m.domain.manager,
				m.domain.runtime,
				m.domain.service,
				m.domain.runtime.IsSystemUnfiledProject(msg.Project.ID),
			))
		}
		return true, nil

	case projecthub.ProjectUpdatedMsg:
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
			m.feedback.appendDetail(projectTaskCountPhrase(msg.Project) + " hidden · Fp to show")
		}
		cmds = append(cmds, feedbackCmd)
		return true, tea.Batch(cmds...)

	}
	return false, nil
}

func (m *AppModel) showProjectArchiveConfirm(project *models.Project, archive bool) tea.Cmd {
	if project == nil {
		m.project.ClearArchive()
		return nil
	}
	if m.domain.runtime.IsSystemUnfiledProject(project.ID) {
		m.project.ClearArchive()
		return status.ErrorCmd("cannot archive the system Unfiled project")
	}

	m.project.ArmArchive(project, archive)
	return nil
}

func (m *AppModel) confirmProjectArchive() tea.Cmd {
	project, archive := m.project.TakeArchive()

	if project == nil {
		return nil
	}
	if m.domain.runtime.IsSystemUnfiledProject(project.ID) {
		return status.ErrorCmd("cannot archive the system Unfiled project")
	}

	if archive {
		return func() tea.Msg {
			return projecthub.ProjectArchivedMsg{Project: project}
		}
	}

	return func() tea.Msg {
		return projecthub.ProjectRestoredMsg{Project: project}
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
		if m.domain.persister == nil {
			return status.ErrorCmd("cannot archive project: project storage is unavailable")
		}
		if err := m.domain.persister.Load(&updated); err != nil {
			return status.ErrorCmd("cannot archive project: %v", err)
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

	domains := cloneRuntimeDomains(m.domain.runtime)
	if err := domains.Catalog.UpsertProject(&updated); err != nil {
		m.feedback.err = err
		return nil
	}
	if len(archivedLocators) > 0 {
		domains.Planning.RemoveDurableTaskLocators(archivedLocators)
	}
	result, err := m.commitRuntimeDomainsWithUndo(domains, "Archive Project", updated.Name, []string{updated.ID})
	if err != nil {
		m.feedback.err = err
		return nil
	}

	coordinator.CopyCatalogMetadata(project, &updated)
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
		m.feedback.info = true
		m.feedback.appendDetail("machine-state cleanup incomplete; retrying")
	}
	m.refreshProjectHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		feedbackCmd,
	)
}

// showProjectDeleteConfirm raises the shell's own delete prompt. The project's
// tasks are read here rather than at commit time because the count is what makes
// the prompt honest, and the loaded copy is what the commit then reuses: a
// refused or abandoned confirmation leaves the live project pointer untouched.
func (m *AppModel) showProjectDeleteConfirm(project *models.Project) tea.Cmd {
	if project == nil {
		m.project.ClearDelete()
		return nil
	}
	if m.domain.runtime.IsSystemUnfiledProject(project.ID) {
		m.project.ClearDelete()
		return status.ErrorCmd("cannot delete the system Unfiled project")
	}

	loaded := *project
	if loaded.Tasks == nil && m.domain.persister != nil {
		if err := m.domain.persister.Load(&loaded); err != nil {
			m.project.ClearDelete()
			return status.ErrorCmd("cannot delete project: %v", err)
		}
	}

	m.project.ArmDelete(
		&loaded,
		fmt.Sprintf("Delete %s and everything it owns?", loaded.Name),
		&chrome.ConfirmDetail{
			CascadeNote: m.projectDeleteCascadeNote(&loaded),
			UndoHint:    true,
		},
	)
	return nil
}

// projectDeleteCascadeNote names what leaves the workspace with the project. A
// population the shell could not read is omitted rather than reported as zero,
// so the sentence never understates the delete.
func (m *AppModel) projectDeleteCascadeNote(project *models.Project) string {
	parts := []string{projectTaskCountPhrase(project)}
	attachments := 0
	for _, task := range project.Tasks {
		if task != nil {
			attachments += len(task.Attachments)
		}
	}
	if m.domain.service != nil {
		if notes, err := m.domain.service.Notes(project.ID); err == nil && len(notes) > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", len(notes), listsurface.Pluralize(len(notes), "note", "notes")))
		}
	}
	if attachments > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", attachments, listsurface.Pluralize(attachments, "attachment", "attachments")))
	}
	return "Deletes " + joinWithAnd(parts) + ". This cannot be reopened, only undone."
}

// joinWithAnd renders a list as an English series so the cascade note reads as a
// sentence rather than as comma-separated fields.
func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
	}
}

func (m *AppModel) confirmProjectDelete() tea.Cmd {
	project := m.project.TakeDelete()

	if project == nil {
		return nil
	}
	if m.domain.runtime.IsSystemUnfiledProject(project.ID) {
		return status.ErrorCmd("cannot delete the system Unfiled project")
	}
	return func() tea.Msg {
		return projecthub.ProjectDeletedMsg{Project: project}
	}
}

// commitProjectDelete removes the project's data directory and its catalog and
// planning entries as one journaled transaction, so the workspace undo ledger
// holds a single entry that restores the tasks, notes, and attachments together.
// Nothing is published to a live cache until that commit returns.
func (m *AppModel) commitProjectDelete(project *models.Project) tea.Cmd {
	if project == nil {
		return nil
	}
	if m.domain.runtime == nil || m.domain.service == nil {
		return status.ErrorCmd("cannot delete project: project storage is unavailable")
	}
	updated := *project
	if updated.Tasks == nil {
		if m.domain.persister == nil {
			return status.ErrorCmd("cannot delete project: project storage is unavailable")
		}
		if err := m.domain.persister.Load(&updated); err != nil {
			return status.ErrorCmd("cannot delete project: %v", err)
		}
	}

	// The anchor reads the rows the delete is about to remove, so it runs before
	// any cache the reload rebuilds from.
	m.anchorOutlinerCursorAfterProjectPark(updated.ID)

	domains := cloneRuntimeDomains(m.domain.runtime)
	if _, err := domains.RemoveProject(updated.ID, updated.Slug); err != nil {
		m.feedback.err = err
		return nil
	}
	artifacts, err := syncedDomainArtifacts(m.domain.runtime, domains.Catalog, domains.Planning)
	if err != nil {
		m.feedback.err = err
		return nil
	}
	transactionID, err := m.domain.service.DeleteProjectResult(app.DeleteProjectRequest{
		Project:                &updated,
		SystemUnfiledProjectID: m.domain.runtime.Catalog().SystemUnfiledProjectID,
		DataDir:                m.domain.runtime.DataDir(),
		Config:                 artifacts,
	})
	if err != nil {
		m.feedback.err = err
		return nil
	}
	if err := m.domain.runtime.RefreshAfterCommit(); err != nil {
		m.feedback.err = err
		return nil
	}
	m.syncProjectCache()

	if m.currentProject != nil && m.currentProject.ID == updated.ID {
		m.currentProject = nil
	}
	m.rebuildPlanningFromRuntime()
	m.pruneDeletedTaskRefs()
	m.refreshWorkspacePulseAt(time.Now())
	feedbackCmd := m.setRecoveryFeedback(fmt.Sprintf("Deleted %s", updated.Name), transactionID, false)
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
		m.feedback.err = err
		return nil
	}
	coordinator.CopyCatalogMetadata(project, &updated)
	feedbackCmd := m.setRecoveryFeedback(fmt.Sprintf("Unarchived %s", updated.Name), transactionID, false)
	m.refreshProjectHubIfActive()
	return tea.Batch(
		m.requestCrossProjectOutlinerReload(),
		m.refreshActiveNotesManager(),
		feedbackCmd,
	)
}
