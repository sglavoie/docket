package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) handleGlobalMessage(msg tea.Msg) (bool, tea.Cmd) {
	if handled, cmd := m.handleOperationOverlayMessage(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleGlobalInputMessage(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleGlobalLifecycleMessage(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleGlobalSaveMessage(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleGlobalRefreshMessage(msg); handled {
		return true, cmd
	}
	return false, nil
}

func (m *AppModel) handleGlobalInputMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleGlobalKeyPress(msg)
	case tea.KeyboardEnhancementsMsg:
		return true, upgradeKittyKeyboard()
	case tea.KeyReleaseMsg:
		return true, nil
	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
		return true, nil
	default:
		return false, nil
	}
}

func (m *AppModel) handleGlobalLifecycleMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case QuitMsg:
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		m.quitting = true
		return true, tea.Quit
	case spinner.TickMsg:
		if !m.isLoading {
			return false, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return true, cmd
	case crossProjectsLoadedMsg:
		// Cross-project loads belong to the sole authoritative Outliner, not to
		// whichever child presentation happens to be visible when the async
		// command completes. Routing the result globally guarantees loading state
		// settles even if navigation covered the Outliner after the load started.
		// Adopt accepted snapshots into the shell cache first so operation
		// resolution sees the same disk-loaded task trees as the rendered rows.
		if m.crossOutliner == nil {
			return true, nil
		}
		if msg.generation == m.crossOutliner.crossLoadGen {
			m.adoptCrossProjectSnapshot(msg.projects)
		}
		return true, m.crossOutliner.handleCrossProjectsLoaded(msg)
	default:
		return false, nil
	}
}

func (m *AppModel) handleGlobalSaveMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case periodicSaveMsg:
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		return true, tea.Tick(periodicSaveInterval, func(time.Time) tea.Msg {
			return periodicSaveMsg{}
		})
	case outlinerFilterStateChangedMsg:
		m.persistOutlinerFilterState()
		return true, nil
	case SaveStartMsg:
		m.isLoading = true
		m.loadingMessage = msg.Message
		m.err = nil
		return true, tea.Batch(
			m.spinner.Tick(),
			tea.Tick(10*time.Second, func(time.Time) tea.Msg { return SaveTimeoutMsg{} }),
		)
	case SaveTimeoutMsg:
		return true, m.handleSaveTimeout()
	case SaveCompleteMsg:
		return true, m.handleSaveComplete(msg)
	case SaveErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		return true, statusTimeout(5 * time.Second)
	case TransactionResultMsg:
		return true, m.handleTransactionResult(msg)
	case BatchTransactionResultMsg:
		return true, m.handleBatchTransactionResult(msg)
	case TransactionErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		return true, statusTimeout(5 * time.Second)
	default:
		return false, nil
	}
}

// handleTransactionResult routes a committed feature-coordinator mutation back
// into the shell. The application layer committed one atomic ChangeSet against
// its own freshly loaded snapshot, so the AppModel's in-memory copy of the
// affected project is reloaded from disk before the shared post-save refresh
// runs, keeping every open view consistent with the committed transaction.
func (m *AppModel) handleTransactionResult(msg TransactionResultMsg) tea.Cmd {
	m.reloadProjectByID(msg.Result.ProjectID)
	if msg.PlanningChanged && m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()
	return m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: msg.SuccessMessage()})
}

// handleBatchTransactionResult routes a committed project-scoped or multi-task
// coordinator mutation (bulk transition/delete or metadata update) back into the
// shell. The application layer committed one atomic ChangeSet across every touched
// project, so each affected project's in-memory copy is reloaded from disk before
// the shared post-save refresh runs, keeping every open view consistent.
func (m *AppModel) handleBatchTransactionResult(msg BatchTransactionResultMsg) tea.Cmd {
	for _, projectID := range msg.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
	}
	m.refreshLoadedTaskWorkspace()
	// Clear multiselect state only now that the batch has committed, so a rejected
	// batch (TransactionErrorMsg, which never reaches this handler) leaves the
	// selection intact for the user to retry.
	if len(msg.ClearSelectionTaskLocators) > 0 {
		m.clearBulkTaskSelectionsForLocators(msg.ClearSelectionTaskLocators)
	} else if len(msg.ClearSelectionTaskIDs) > 0 {
		m.clearBulkTaskSelectionsForTaskIDs(msg.ClearSelectionTaskIDs)
	}
	// The checked planning artifact folded into this batch is now committed.
	// Rebuild the shared planning caches only after the runtime snapshot advances:
	// registered views refresh below and must never render a known-stale snapshot.
	if m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	saveCmd := m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: msg.SuccessMessage()})
	return saveCmd
}

func (m *AppModel) refreshLoadedTaskWorkspace() {
	if m.crossOutliner != nil {
		rebuildOutlinerPreservingCursor(m.crossOutliner)
	}
}

// reloadProjectByID reloads a project's tasks from disk into the AppModel's
// in-memory copy so views bound to it reflect a mutation committed through the
// application layer. A blank id, unknown project, or load error is a no-op — the
// shared post-save refresh reloads the current view's projects regardless.
func (m *AppModel) reloadProjectByID(projectID string) {
	if projectID == "" {
		return
	}
	project := m.projectByID(projectID)
	if project == nil {
		return
	}
	if err := storage.LoadProject(project); err != nil {
		logging.Warn("failed to reload project %s after transaction: %v", project.Name, err)
	}
}

func (m *AppModel) handleGlobalRefreshMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg.(type) {
	case RefreshOpenLoopsMsg:
		m.openLoops = ComputeOpenLoops(m.runtime, m.todayIDs)
		return true, nil
	case TodayClearedBannerClearMsg:
		m.todayClearedBanner = ""
		return true, nil
	case NotesRefreshMsg:
		return true, m.refreshActiveNotesManager()
	default:
		return false, nil
	}
}

func (m *AppModel) persistOutlinerFilterState() {
	// Persist filter state immediately so navigation (e.g. go drill-down
	// from cross-outliner) reads the up-to-date cleared/changed values.
	m.saveOutlinerFilterState()
}

func (m *AppModel) handleSaveTimeout() tea.Cmd {
	if !m.isLoading {
		return nil
	}
	loadMsg := m.loadingMessage
	m.isLoading = false
	m.loadingMessage = ""
	if loadMsg != "" {
		m.err = fmt.Errorf("save timed out — %s", loadMsg)
	} else {
		m.err = fmt.Errorf("save timed out")
	}
	return statusTimeout(5 * time.Second)
}

func (m *AppModel) handleSaveComplete(msg SaveCompleteMsg) tea.Cmd {
	m.isLoading = false
	m.loadingMessage = ""
	m.clearToggleHistory()
	// An empty success message means the mutation carried no status of its own
	// (e.g. a ProjectTree/Outliner reorder committed through CommitProjectWorkflow):
	// leave any status the view set synchronously intact rather than blanking it,
	// matching the legacy treeRefreshMsg/outlinerRefreshMsg path that never touched
	// the status line.
	if msg.SuccessMessage != "" {
		m.setSuccessStatus(msg.SuccessMessage)
	}
	saveCmds := m.refreshTaskViewsAfterSave()
	saveCmds = append(saveCmds, defaultStatusTimeout())
	return tea.Batch(saveCmds...)
}

func (m *AppModel) refreshTaskViewsAfterSave() []tea.Cmd {
	return m.refreshTaskViews(true, true)
}

// refreshTaskViewsAfterPlanningMutation fans planning changes out through the
// already-loaded task views. The application service has synchronously reloaded
// every touched project and planning snapshot, so starting an asynchronous
// cross-project reload here would blank live rows until its command is run.
func (m *AppModel) refreshTaskViewsAfterPlanningMutation() []tea.Cmd {
	return m.refreshTaskViews(true, false)
}

// refreshTaskViewsAfterRestore uses the same cache/view fan-out as a committed
// save. When the runtime reload failed, it rebuilds the current project tree in
// memory without persisting view state from the stale runtime snapshot.
func (m *AppModel) refreshTaskViewsAfterRestore(runtimeReloaded bool) []tea.Cmd {
	return m.refreshTaskViews(runtimeReloaded, true)
}

func (m *AppModel) refreshTaskViews(persistProjectTreeState, reloadCrossProject bool) []tea.Cmd {
	m.openLoops = ComputeOpenLoops(m.runtime, m.todayIDs)
	m.refreshProjectViewAfterTaskMutation(persistProjectTreeState)
	var saveCmds []tea.Cmd
	if reloadCrossProject {
		if reloadCmd := m.requestCrossProjectOutlinerReload(); reloadCmd != nil {
			saveCmds = append(saveCmds, reloadCmd)
		}
	}
	if m.views != nil {
		for _, viewType := range []PresentationID{
			ViewTaskChecklist,
			ViewTaskComments,
			ViewTaskAttachments,
		} {
			view := m.views.Get(viewType)
			refresher, ok := view.(interface{ refreshAfterReload() bool })
			if !ok {
				continue
			}
			if !refresher.refreshAfterReload() && m.currentView() == viewType {
				saveCmds = append(saveCmds, func() tea.Msg { return BackMsg{} })
			}
		}
	}
	return saveCmds
}

func (m *AppModel) refreshProjectViewAfterTaskMutation(persistProjectTreeState bool) {
	if persistProjectTreeState {
		m.refreshProjectView()
	} else {
		m.refreshProjectViewInMemory()
	}
}
