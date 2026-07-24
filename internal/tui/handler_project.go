package tui

import (
	"encoding/json"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleProjectMessages handles project-related messages (create, edit, archive, delete, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case SelectProjectMsg:
		if msg.Project != nil && msg.Project.Archived {
			return true, m.showProjectArchiveConfirm(msg.Project, false)
		}
		cmd, _ := m.maybePromptPark(TaskIdentity{}, func() tea.Cmd {
			return m.selectProjectNow(msg.Project)
		})
		return true, cmd

	case CreateProjectMsg:
		m.pendingProjectMoveTask = msg.MoveTask
		m.pendingProjectMoveSource = msg.SourceProject
		m.showView(ViewProjectCreator, NewProjectCreatorModel(m.runtime))
		return true, nil

	case ProjectCreatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.ProjectCreated = true })
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
			return true, tea.Batch(popCmd, moveCmd, reloadCmd)
		}
		return true, tea.Batch(popCmd, reloadCmd)

	case ProjectArchiveRequestMsg:
		if msg.Project != nil && m.runtime.IsSystemInboxProject(msg.Project.ID) {
			return true, errorCmd("cannot archive the system Inbox project")
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
		if err := m.commitCatalogMutation(func(catalog *config.WorkspaceCatalog) {
			catalog.UpsertProject(&updated)
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
		return true, tea.Batch(m.requestCrossProjectOutlinerReload(), defaultStatusTimeout())

	case ProjectDeletedMsg:
		var manageFallback *ManageParams
		if _, params, ok := m.manageHubPrimary(); ok && params.Scope.Kind == ScopeProject && params.Scope.ProjectID == msg.Project.ID {
			fallback := params
			fallback.Scope = AppEntityScope()
			manageFallback = &fallback
		}
		domains := cloneRuntimeDomains(m.runtime)
		domains.RemoveProjectReferences(msg.Project.ID, msg.Project.Slug)
		if m.currentProject != nil && m.currentProject.ID == msg.Project.ID {
			m.currentProject = nil
		}
		if err := m.commitRuntimeDomains(domains); err != nil {
			m.err = err
		}
		m.pruneDeletedTaskRefs()
		if manageFallback != nil {
			m.setInfoStatus("Project removed — showing all " + manageFallback.Module.String())
			return true, m.activateManageHub(*manageFallback)
		}
		// Reset to the global Outliner root since the project no longer exists.
		m.exitAskView()
		if m.typedNavigation {
			presentationCmd := m.prepareCrossProjectOutliner()
			m.navigation.ResetRoot()
			return true, tea.Batch(m.activateTaskWorkspaceRoute(outlinerRootRoute()), presentationCmd)
		}
		m.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}
		return true, nil

	case ProjectRestoredMsg:
		return true, m.commitProjectUnarchive(msg.Project)

	case EditProjectMsg:
		if msg.Project != nil {
			if m.runtime.IsSystemInboxProject(msg.Project.ID) {
				return true, errorCmd("cannot rename the system Inbox project")
			}
			m.showView(ViewProjectEditor, NewProjectEditorModel(msg.Project, m.configManager, m.runtime, m.appService))
		}
		return true, nil

	case ProjectUpdatedMsg:
		popCmd := m.popView()
		m.syncProjectCache()
		m.refreshProjectView()
		return true, tea.Batch(popCmd, m.requestCrossProjectOutlinerReload())

	case TutorDismissedMsg:
		// Tutor dismissal is a portable behavior preference (Settings domain), so
		// it commits only the settings domain rather than saving the whole config.
		if err := m.commitSettingsMutation(func(s *config.Settings) {
			s.SetTutorDismissed(true)
		}); err != nil {
			m.err = err
			return true, nil
		}
		m.setSuccessStatus("Reference prompt dismissed")
		return true, defaultStatusTimeout()

	case ShowTrashedProjectsMsg:
		m.showView(ViewTrashedProjects, NewTrashedProjectsModel(m.runtime.DataDir()))
		return true, nil

	case RestoreFromTrashMsg:
		m.isLoading = true
		m.loadingMessage = "Restoring..."
		return true, tea.Batch(
			m.spinner.Tick(),
			m.restoreProjectFromTrash(msg.EntryID),
			tea.Tick(10*time.Second, func(time.Time) tea.Msg { return SaveTimeoutMsg{} }),
		)

	case projectRestoredFromTrashMsg:
		m.isLoading = false
		m.loadingMessage = ""
		if msg.Project == nil {
			m.err = fmt.Errorf("project restore returned no project")
			return true, nil
		}
		var popCmd tea.Cmd
		if m.currentView() == ViewTrashedProjects {
			popCmd = m.popView()
		}
		m.rememberTrashRestoreRecovery(msg)
		if err := m.completeTrashRestoreRecovery(msg.Project.ID); err != nil {
			m.err = trashRestorePartialError(msg.Project.Name, err)
			return true, tea.Batch(
				popCmd,
				m.requestCrossProjectOutlinerReload(),
				retryTrashRestoreMachineRefsCmd(msg.Project.ID),
			)
		}
		m.err = nil
		m.setSuccessStatus(fmt.Sprintf("Restored: %s", msg.Project.Name))
		return true, tea.Batch(popCmd, m.requestCrossProjectOutlinerReload(), defaultStatusTimeout())

	case retryTrashRestoreMachineRefsMsg:
		recovery, ok := m.pendingTrashRestoreRecovery[msg.ProjectID]
		if !ok {
			return true, nil
		}
		if err := m.completeTrashRestoreRecovery(msg.ProjectID); err != nil {
			m.err = trashRestorePartialError(recovery.Project.Name, err)
			return true, tea.Batch(
				m.requestCrossProjectOutlinerReload(),
				retryTrashRestoreMachineRefsCmd(msg.ProjectID),
			)
		}
		m.err = nil
		m.setSuccessStatus(fmt.Sprintf("Restored: %s", recovery.Project.Name))
		return true, tea.Batch(m.requestCrossProjectOutlinerReload(), defaultStatusTimeout())

	case PurgeFromTrashMsg:
		m.isLoading = true
		m.loadingMessage = "Deleting..."
		return true, tea.Batch(
			m.spinner.Tick(),
			m.purgeProjectFromTrash(msg.EntryID, msg.Slug),
			tea.Tick(10*time.Second, func(time.Time) tea.Msg { return SaveTimeoutMsg{} }),
		)

	case projectPurgedFromTrashMsg:
		m.isLoading = false
		m.loadingMessage = ""
		// Refresh the trashed projects list by re-registering the view
		newModel := NewTrashedProjectsModel(m.runtime.DataDir())
		newModel.SetSize(m.width, m.height-2)
		m.views.Register(ViewTrashedProjects, newModel)
		m.setSuccessStatus(fmt.Sprintf("Permanently deleted: %s", msg.Slug))
		return true, defaultStatusTimeout()

	case SandboxResetRequestMsg:
		sandbox := m.runtime.GetSandboxProject()
		if sandbox == nil {
			m.setInfoStatus("Sandbox project not found")
			return true, defaultStatusTimeout()
		}
		if m.appService == nil {
			m.err = fmt.Errorf("sandbox reset service is unavailable")
			return true, nil
		}
		preview, err := m.appService.PreviewSandboxReset(sandbox.ID)
		if err != nil {
			m.err = fmt.Errorf("failed to inspect sandbox: %w", err)
			return true, nil
		}
		if preview.Counts.Empty() {
			m.setInfoStatus("Sandbox is already empty")
			return true, defaultStatusTimeout()
		}

		m.pendingSandboxReset = true
		m.pendingSandboxProjectID = preview.ProjectID
		m.pendingSandboxTaskCount = preview.Counts.Total()
		m.setInfoStatus(fmt.Sprintf("Reset sandbox? Delete %d item(s)?", preview.Counts.Total()))
		return true, nil

	case SandboxResetDoneMsg:
		m.pendingSandboxReset = false
		m.pendingSandboxProjectID = ""
		m.pendingSandboxTaskCount = 0
		m.reloadProjectByID(msg.Result.ProjectID)
		m.reloadCrossProjectOutlinerProject(msg.Result.ProjectID)
		m.setSuccessStatus(fmt.Sprintf("Cleared %d item(s) from sandbox", msg.Result.Cleared.Total()))
		// Refresh views if sandbox is currently displayed
		if m.currentProject != nil && m.currentProject.ID == msg.Result.ProjectID {
			m.refreshProjectView()
		}
		return true, defaultStatusTimeout()

	case SandboxResetFailedMsg:
		m.pendingSandboxReset = false
		m.pendingSandboxProjectID = ""
		m.pendingSandboxTaskCount = 0
		m.err = msg.Err
		return true, statusTimeout(5 * time.Second)
	}
	return false, nil
}

func (m *AppModel) selectProjectNow(project *models.Project) tea.Cmd {
	// Pop overlay views if that's where we came from
	if m.currentView() == ViewCommandPalette {
		m.popView()
	}
	m.switchToProject(project, true)
	if m.typedNavigation {
		_, cmd := m.handleProjectViewMessages(ShowOutlinerMsg{Project: project})
		return cmd
	}
	m.saveOutlinerToggleState()
	m.saveOutlinerFilterState()
	m.outliner = NewOutlinerModel(project, m.configManager, m.runtime)
	m.outliner.queueIDs = m.focusQueueIDs
	m.outliner.dailyPlanIDs = m.dailyPlanIDs
	m.restoreOutlinerToggleState()
	m.restoreOutlinerFilterState()
	m.showView(ViewOutliner, m.outliner)
	return nil
}

func (m *AppModel) showProjectArchiveConfirm(project *models.Project, archive bool) tea.Cmd {
	if project == nil {
		m.clearPendingProjectArchive()
		return nil
	}
	if project.Sandbox {
		m.clearPendingProjectArchive()
		return errorCmd("cannot archive the sandbox project")
	}
	if m.runtime.IsSystemInboxProject(project.ID) {
		m.clearPendingProjectArchive()
		return errorCmd("cannot archive the system Inbox project")
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
	if project.Sandbox {
		return errorCmd("cannot archive the sandbox project")
	}
	if m.runtime.IsSystemInboxProject(project.ID) {
		return errorCmd("cannot archive the system Inbox project")
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
	domains.Catalog.UpsertProject(&updated)
	if domains.Machine.CurrentProject == updated.ID {
		domains.Machine.CurrentProject = ""
	}
	if len(archivedLocators) > 0 {
		domains.Planning.RemoveDurableTaskLocators(archivedLocators)
		domains.Machine.RemoveMachineLocalTaskLocators(archivedLocators)
	}
	if err := m.commitRuntimeDomains(domains); err != nil {
		m.err = err
		return nil
	}

	copyCatalogMetadata(project, &updated)
	if m.currentProject != nil && m.currentProject.ID == updated.ID {
		m.currentProject = nil
	}
	m.pruneDeletedTaskRefs()
	m.setSuccessStatus(fmt.Sprintf("Archived %s", updated.Name))
	return tea.Batch(m.requestCrossProjectOutlinerReload(), defaultStatusTimeout())
}

// commitProjectUnarchive mirrors commitProjectArchive for the catalog-only
// recovery path: checked persistence precedes publication to every live cache.
func (m *AppModel) commitProjectUnarchive(project *models.Project) tea.Cmd {
	if project == nil {
		return nil
	}
	updated := *project
	updated.Unarchive()
	if err := m.commitCatalogMutation(func(catalog *config.WorkspaceCatalog) {
		catalog.UpsertProject(&updated)
	}); err != nil {
		m.err = err
		return nil
	}
	copyCatalogMetadata(project, &updated)
	m.setSuccessStatus(fmt.Sprintf("Unarchived %s", updated.Name))
	return tea.Batch(m.requestCrossProjectOutlinerReload(), defaultStatusTimeout())
}

// restoreProjectFromTrash captures an immutable command input on the Bubble Tea
// update loop. The command commits the directory move, drained trash-entry
// removal, and synced configuration rewrite atomically, then returns the
// restored project and its machine-reference recovery payload. It never reads
// or mutates the live RuntimeState or AppModel from the asynchronous goroutine.
func (m *AppModel) restoreProjectFromTrash(entryID string) tea.Cmd {
	if m == nil || m.runtime == nil {
		return errorCmd("runtime state is unavailable")
	}
	mgr := m.configManager
	svc := m.appService
	if mgr == nil || svc == nil {
		return errorCmd("project restore service is unavailable")
	}
	domains, err := m.runtime.CloneDomains()
	if err != nil {
		return errorCmd("failed to capture restore state: %v", err)
	}
	dataDir := m.runtime.DataDir()
	return func() tea.Msg {
		var refs config.ProjectReferences
		res, err := svc.RestoreProject(app.RestoreProjectRequest{
			DataDir: dataDir,
			EntryID: entryID,
			BuildConfig: func(project *models.Project, tomb *storage.TrashTombstone) ([]storage.ArtifactChange, error) {
				if catalogContainsProject(domains.Catalog, project.ID, project.Slug) {
					return nil, fmt.Errorf("active project '%s' already exists; rename or delete it first", project.Slug)
				}
				refs = restoredProjectReferences(tomb)
				domains.Catalog.UpsertProject(project)
				domains.ApplySyncedProjectReferences(project.ID, refs)
				return syncedDomainArtifacts(
					mgr,
					domains.Catalog,
					domains.Planning,
					domains.Settings,
					domains.Automation,
				)
			},
		})
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to restore project from trash: %w", err)}
		}

		return projectRestoredFromTrashMsg{Project: res.Project, References: refs}
	}
}

func catalogContainsProject(catalog *config.WorkspaceCatalog, projectID, slug string) bool {
	if catalog == nil {
		return false
	}
	for _, project := range catalog.Projects {
		if project.ID == projectID || project.Slug == slug {
			return true
		}
	}
	return false
}

// restoredProjectReferences decodes the caller-owned reference snapshot stored
// in a tombstone. Decode failures retain the historical best-effort behavior:
// the project itself remains restorable, with an empty recovery payload.
func restoredProjectReferences(tomb *storage.TrashTombstone) config.ProjectReferences {
	if tomb == nil || len(tomb.ConfigRefs) == 0 {
		return config.ProjectReferences{}
	}
	var refs config.ProjectReferences
	if err := json.Unmarshal(tomb.ConfigRefs, &refs); err != nil {
		return config.ProjectReferences{}
	}
	return refs
}

// projectRestoredFromTrashMsg is an internal message for trash restoration completion.
type projectRestoredFromTrashMsg struct {
	Project    *models.Project
	References config.ProjectReferences
}

type trashRestoreMachineRecovery struct {
	Project    *models.Project
	References config.ProjectReferences
}

type retryTrashRestoreMachineRefsMsg struct {
	ProjectID string
}

const trashRestoreMachineRetryDelay = time.Second

func retryTrashRestoreMachineRefsCmd(projectID string) tea.Cmd {
	return tea.Tick(trashRestoreMachineRetryDelay, func(time.Time) tea.Msg {
		return retryTrashRestoreMachineRefsMsg{ProjectID: projectID}
	})
}

func (m *AppModel) rememberTrashRestoreRecovery(msg projectRestoredFromTrashMsg) {
	if msg.Project == nil {
		return
	}
	if m.pendingTrashRestoreRecovery == nil {
		m.pendingTrashRestoreRecovery = make(map[string]trashRestoreMachineRecovery)
	}
	m.pendingTrashRestoreRecovery[msg.Project.ID] = trashRestoreMachineRecovery(msg)
}

// completeTrashRestoreRecovery runs only on the Bubble Tea update loop. It
// refreshes the durable domains and project cache before attempting machine
// repair, and removes the captured payload only after machine.json succeeds.
func (m *AppModel) completeTrashRestoreRecovery(projectID string) error {
	recovery, ok := m.pendingTrashRestoreRecovery[projectID]
	if !ok || recovery.Project == nil {
		return nil
	}
	if m.runtime == nil {
		return fmt.Errorf("runtime state is unavailable")
	}
	if err := m.runtime.RefreshAfterCommit(); err != nil {
		return fmt.Errorf("refresh restored workspace: %w", err)
	}
	if m.projects == nil {
		m.projects = make(map[string]*models.Project)
	}
	// Seed the cache with the fully loaded result before syncing catalog
	// metadata, so the restored task tree is not replaced by a metadata-only
	// RuntimeState project.
	m.projects[projectID] = recovery.Project
	m.syncProjectCache()

	machine := cloneMachineState(m.runtime.Machine())
	if machine.ApplyProjectReferences(projectID, recovery.References) {
		if m.runtime.DataDir() == "" {
			m.runtime.SetMachineInMemory(machine)
		} else if err := m.runtime.CommitMachine(machine); err != nil {
			return fmt.Errorf("restore machine-local project references: %w", err)
		}
	}
	m.syncProjectCache()
	if m.runtime.Machine().CurrentProject == projectID {
		m.currentProject = m.projectByID(projectID)
	}
	delete(m.pendingTrashRestoreRecovery, projectID)
	return nil
}

func trashRestorePartialError(projectName string, err error) error {
	return fmt.Errorf("restored %s, but machine-state recovery is incomplete and will retry: %w", projectName, err)
}

// projectPurgedFromTrashMsg is an internal message for trash purge completion.
type projectPurgedFromTrashMsg struct {
	Slug string
}

// purgeProjectFromTrash permanently deletes a single trash entry through the
// application layer's journaled recursive-delete transaction (irreversible, no
// undo), replacing the direct storage purge.
func (m *AppModel) purgeProjectFromTrash(entryID, slug string) tea.Cmd {
	svc := m.appService
	dataDir := m.runtime.DataDir()
	return func() tea.Msg {
		if _, err := svc.PurgeProject(app.PurgeProjectRequest{
			DataDir: dataDir,
			EntryID: entryID,
			Slug:    slug,
			Confirm: true,
		}); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to delete project: %w", err)}
		}
		return projectPurgedFromTrashMsg{Slug: slug}
	}
}

// executeSandboxReset captures only the stable project id and application
// service. The asynchronous command never loads through or mutates AppModel's
// cached sandbox pointer; committed state is reloaded by SandboxResetDoneMsg on
// the Bubble Tea update loop.
func (m *AppModel) executeSandboxReset() tea.Cmd {
	svc := m.appService
	projectID := m.pendingSandboxProjectID
	return func() tea.Msg {
		if svc == nil {
			return SandboxResetFailedMsg{Err: fmt.Errorf("sandbox reset service is unavailable")}
		}
		if projectID == "" {
			return SandboxResetFailedMsg{Err: fmt.Errorf("sandbox project not found")}
		}
		result, err := svc.ResetSandbox(projectID)
		if err != nil {
			return SandboxResetFailedMsg{Err: fmt.Errorf("failed to reset sandbox: %w", err)}
		}
		return SandboxResetDoneMsg{Result: result}
	}
}
