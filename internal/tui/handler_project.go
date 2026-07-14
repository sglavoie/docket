package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleProjectMessages handles project-related messages (create, edit, archive, delete, restore).
// Returns true if the message was handled.
func (m *AppModel) handleProjectMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case SelectProjectMsg:
		cmd, _ := m.maybePromptPark("", func() tea.Cmd {
			return m.selectProjectNow(msg.Project)
		})
		return true, cmd

	case CreateProjectMsg:
		m.pendingProjectMoveTask = msg.MoveTask
		m.pendingProjectMoveSource = msg.SourceProject
		m.showView(ViewProjectCreator, NewProjectCreatorModel(m.runtime))
		return true, nil

	case ProjectCreatedMsg:
		m.popView()
		m.syncProjectCache()
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		m.advanceOnboarding(func(progress *config.OnboardingProgress) { progress.ProjectCreated = true })
		moveTask := m.pendingProjectMoveTask
		moveSource := m.pendingProjectMoveSource
		m.pendingProjectMoveTask = nil
		m.pendingProjectMoveSource = nil
		if moveTask != nil && moveSource != nil && msg.Project != nil {
			return true, func() tea.Msg {
				return MoveTaskToProjectMsg{
					Task:          moveTask,
					SourceProject: moveSource,
					DestProjectID: msg.Project.ID,
				}
			}
		}
		return true, nil

	case ProjectArchiveRequestMsg:
		if msg.Project != nil && m.runtime.IsSystemInboxProject(msg.Project.ID) {
			return true, errorCmd("cannot archive the system Inbox project")
		}
		return true, m.showProjectArchiveConfirm(msg.Project, msg.Archive)

	case projectArchiveConfirmedMsg:
		return true, m.confirmProjectArchive()

	case ProjectArchivedMsg:
		popCmd := m.popView()
		domains := cloneRuntimeDomains(m.runtime)
		if msg.Project != nil {
			domains.Catalog.UpsertProject(msg.Project)
		}
		// Clear current project if archived
		if msg.Project != nil && msg.Project.Archived && domains.Machine.CurrentProject == msg.Project.ID {
			domains.Machine.CurrentProject = ""
		}
		if msg.Project != nil && msg.Project.Archived && m.currentProject != nil && m.currentProject.ID == msg.Project.ID {
			m.currentProject = nil
		}
		// Clean up config references for tasks in the archived project
		if msg.Project != nil {
			taskIDs := make(map[string]bool, len(msg.Project.Tasks))
			for _, t := range msg.Project.Tasks {
				taskIDs[t.ID] = true
			}
			domains.Planning.RemoveDurableTaskRefs(taskIDs)
			domains.Machine.RemoveMachineLocalTaskRefs(taskIDs)
		}
		if err := m.commitRuntimeDomains(domains); err != nil {
			m.err = err
			return true, nil
		}
		m.pruneDeletedTaskRefs()
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		if msg.Project != nil {
			m.setSuccessStatus(fmt.Sprintf("Archived %s", msg.Project.Name))
			return true, tea.Batch(popCmd, defaultStatusTimeout())
		}
		return true, popCmd

	case ProjectPinnedToggledMsg:
		if err := m.commitCatalogMutation(func(catalog *config.WorkspaceCatalog) {
			catalog.UpsertProject(msg.Project)
		}); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		statusMsg := "Pinned"
		if !msg.Project.Pinned {
			statusMsg = "Unpinned"
		}
		m.setSuccessStatus(statusMsg)
		return true, defaultStatusTimeout()

	case ProjectDeletedMsg:
		domains := cloneRuntimeDomains(m.runtime)
		domains.RemoveProjectReferences(msg.Project.ID, msg.Project.Slug)
		if m.currentProject != nil && m.currentProject.ID == msg.Project.ID {
			m.currentProject = nil
		}
		if err := m.commitRuntimeDomains(domains); err != nil {
			m.err = err
		}
		m.pruneDeletedTaskRefs()
		// Pop back to dashboard since the project no longer exists
		m.viewStack = m.viewStack[:1]
		if m.dashboard != nil {
			m.dashboard.Refresh()
			m.dashboard.ResetCursor()
		}
		return true, nil

	case ProjectRestoredMsg:
		if err := m.commitCatalogMutation(func(catalog *config.WorkspaceCatalog) {
			catalog.UpsertProject(msg.Project)
		}); err != nil {
			m.err = err
			return true, nil
		}
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		if msg.Project != nil {
			m.setSuccessStatus(fmt.Sprintf("Unarchived %s", msg.Project.Name))
			return true, defaultStatusTimeout()
		}
		return true, nil

	case EditProjectMsg:
		if msg.Project != nil {
			if m.runtime.IsSystemInboxProject(msg.Project.ID) {
				return true, errorCmd("cannot rename the system Inbox project")
			}
			m.showView(ViewProjectEditor, NewProjectEditorModel(msg.Project, m.configManager, m.runtime, m.appService))
		}
		return true, nil

	case ProjectUpdatedMsg:
		m.popView()
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		m.refreshProjectTree()
		return true, nil

	case ToggleInfoPanelMsg:
		var mode config.InfoPanelMode
		if err := m.commitMachineMutation(func(machine *config.MachineState) {
			mode = machine.CycleInfoPanelMode()
		}); err != nil {
			m.err = err
			return true, nil
		}
		switch mode {
		case config.InfoPanelOff:
			m.setInfoStatus("Info panel hidden")
		case config.InfoPanelInfoCards:
			m.setInfoStatus("Info panel: info cards")
		case config.InfoPanelProjectSummary:
			m.setInfoStatus("Info panel: project summary")
		}
		return true, defaultStatusTimeout()

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
		if m.currentView() == ViewTrashedProjects {
			m.popView()
		}
		m.rememberTrashRestoreRecovery(msg)
		if err := m.completeTrashRestoreRecovery(msg.Project.ID); err != nil {
			m.err = trashRestorePartialError(msg.Project.Name, err)
			if m.dashboard != nil {
				m.dashboard.Refresh()
			}
			return true, retryTrashRestoreMachineRefsCmd(msg.Project.ID)
		}
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		m.err = nil
		m.setSuccessStatus(fmt.Sprintf("Restored: %s", msg.Project.Name))
		return true, defaultStatusTimeout()

	case retryTrashRestoreMachineRefsMsg:
		recovery, ok := m.pendingTrashRestoreRecovery[msg.ProjectID]
		if !ok {
			return true, nil
		}
		if err := m.completeTrashRestoreRecovery(msg.ProjectID); err != nil {
			m.err = trashRestorePartialError(recovery.Project.Name, err)
			return true, retryTrashRestoreMachineRefsCmd(msg.ProjectID)
		}
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		m.err = nil
		m.setSuccessStatus(fmt.Sprintf("Restored: %s", recovery.Project.Name))
		return true, defaultStatusTimeout()

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

	case ShowProjectPickerMsg:
		projects := projectDiscoveryProjects(m.runtime)
		for _, p := range projects {
			if err := storage.LoadProject(p); err != nil {
				logging.Warn("failed to load project %s: %v", p.Name, err)
			}
		}
		return m.showViewInit(ViewProjectPicker, NewProjectPickerModel(projects))

	case ProjectPickerDoneMsg:
		m.popView() // Close picker
		if !msg.Canceled && msg.Project != nil {
			// Save current project's active view before switching away
			m.saveCurrentProjectView()
			// Save current project ID so we can restore it when navigating back
			if m.currentProject != nil {
				m.projectStack = append(m.projectStack, m.currentProject.ID)
			}
			m.switchToProject(msg.Project, true)
			targetView := m.getProjectLastView(msg.Project.ID)
			switch targetView {
			case ViewOutliner:
				m.saveOutlinerToggleState()
				m.saveOutlinerFilterState()
				m.outliner = NewOutlinerModel(msg.Project, m.configManager, m.runtime)
				m.outliner.queueIDs = m.focusQueueIDs
				m.outliner.dailyPlanIDs = m.dailyPlanIDs
				m.restoreOutlinerToggleState()
				m.restoreOutlinerFilterState()
				m.showView(ViewOutliner, m.outliner)
			default:
				m.pushView(ViewProjectTree)
			}
		}
		return true, nil

	case SandboxResetRequestMsg:
		sandbox := m.runtime.GetSandboxProject()
		if sandbox == nil {
			m.setInfoStatus("Sandbox project not found")
			return true, defaultStatusTimeout()
		}
		if err := storage.LoadProject(sandbox); err != nil {
			m.err = fmt.Errorf("failed to load sandbox: %w", err)
			return true, nil
		}
		taskCount := len(sandbox.Tasks)
		notes, _ := storage.ListNotes(sandbox)
		noteCount := len(notes)
		tagCount := len(sandbox.AllTags())
		pinnedTagCount := len(sandbox.PinnedTagList())

		if taskCount == 0 && noteCount == 0 && tagCount == 0 && pinnedTagCount == 0 {
			m.setInfoStatus("Sandbox is already empty")
			return true, defaultStatusTimeout()
		}

		// Build item count for confirmation
		totalItems := taskCount + noteCount + tagCount + pinnedTagCount
		m.pendingSandboxReset = true
		m.pendingSandboxTaskCount = totalItems
		m.setInfoStatus(fmt.Sprintf("Reset sandbox? Delete %d item(s)?", totalItems))
		return true, nil

	case SandboxResetDoneMsg:
		m.pendingSandboxReset = false
		m.pendingSandboxTaskCount = 0
		m.setSuccessStatus(fmt.Sprintf("Cleared %d item(s) from sandbox", msg.TaskCount))
		// Refresh views if sandbox is currently displayed
		if m.currentProject != nil && m.currentProject.Sandbox {
			m.refreshProjectTree()
		}
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		return true, defaultStatusTimeout()
	}
	return false, nil
}

func (m *AppModel) selectProjectNow(project *models.Project) tea.Cmd {
	// Pop overlay views if that's where we came from
	if m.currentView() == ViewCommandPalette {
		m.popView()
	}
	m.saveCurrentProjectView()
	m.switchToProject(project, true)
	targetView := m.getProjectLastView(project.ID)
	switch targetView {
	case ViewProjectTree:
		m.showView(ViewProjectTree, m.projectTree)
	default:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.outliner = NewOutlinerModel(project, m.configManager, m.runtime)
		m.outliner.queueIDs = m.focusQueueIDs
		m.outliner.dailyPlanIDs = m.dailyPlanIDs
		m.restoreOutlinerToggleState()
		m.restoreOutlinerFilterState()
		m.showView(ViewOutliner, m.outliner)
	}
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
		project.Archive()
		return func() tea.Msg {
			return ProjectArchivedMsg{Project: project}
		}
	}

	project.Unarchive()
	return func() tea.Msg {
		return ProjectRestoredMsg{Project: project}
	}
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
	m.pendingTrashRestoreRecovery[msg.Project.ID] = trashRestoreMachineRecovery{
		Project: msg.Project, References: msg.References,
	}
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

// executeSandboxReset performs the actual sandbox reset operation. The task-level
// clear commits through the application-layer transaction boundary
// (Service.ResetSandbox), so the reset is atomic and restores through the
// workspace undo ledger; notes live outside the project document and are cleared
// as a filesystem side effect, matching the pre-transaction undo scope.
func (m *AppModel) executeSandboxReset() tea.Cmd {
	svc := m.appService
	sandbox := m.runtime.GetSandboxProject()
	return func() tea.Msg {
		if sandbox == nil {
			return ErrorMsg{Err: fmt.Errorf("sandbox project not found")}
		}

		if err := storage.LoadProject(sandbox); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to load sandbox: %w", err)}
		}

		taskCount := len(sandbox.Tasks)
		notes, _ := storage.ListNotes(sandbox)
		noteCount := len(notes)
		tagCount := len(sandbox.AllTags())
		pinnedTagCount := len(sandbox.PinnedTagList())
		totalItems := taskCount + noteCount + tagCount + pinnedTagCount

		if totalItems == 0 {
			return SandboxResetDoneMsg{TaskCount: 0}
		}

		// Clear tasks, sections, and tags through the app transaction (undoable).
		if _, err := svc.ResetSandbox(sandbox.ID); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reset sandbox: %w", err)}
		}

		// Mirror the committed clear onto the in-memory project so any shared
		// pointer (e.g. the current project) reflects the reset immediately.
		sandbox.Tasks = nil
		sandbox.NextShortID = 1
		sandbox.Sections = []string{models.SectionNext}
		sandbox.SectionColors = make(map[string]string)
		sandbox.Tags = []string{}
		sandbox.PinnedTags = []string{}

		// Clear notes directory (not part of the project document).
		if noteCount > 0 {
			notesDir := storage.NotesDir(sandbox)
			if err := os.RemoveAll(notesDir); err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to clear notes: %w", err)}
			}
		}

		// The sandbox reset's durable effect (the cleared tasks.md) already
		// committed through Service.ResetSandbox; sandbox catalog metadata is
		// unchanged by a reset, so no config write is needed.
		return SandboxResetDoneMsg{TaskCount: totalItems}
	}
}
