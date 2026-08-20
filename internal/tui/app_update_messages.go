package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
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
		wantedFocus := msg.focusProjectID
		cmd := m.crossOutliner.handleCrossProjectsLoaded(msg)
		if returnCmd := m.finishParkedProjectFocus(wantedFocus); returnCmd != nil {
			return true, tea.Batch(cmd, returnCmd)
		}
		return true, cmd
	case outlinerOriginRecordedMsg:
		if msg.TransactionID == "" || msg.TransactionID != m.statusTransactionID || m.statusUndoCb == nil || !time.Now().Before(m.statusExpiresAt) {
			return true, nil
		}
		return true, m.setRecoveryFeedback(m.statusRecoveryFallback, msg.TransactionID, true)
	default:
		return false, nil
	}
}

func (m *AppModel) handleGlobalSaveMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case periodicSaveMsg:
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		m.wakeDeferredRows(msg.at)
		if !m.completionActivity.IsCurrentLocalDay(msg.at) {
			m.refreshWorkspacePulseAt(msg.at)
		}
		return true, tickCmd(periodicSaveInterval, func(at time.Time) tea.Msg {
			return periodicSaveMsg{at: at}
		})
	case SaveCompleteMsg:
		return true, m.handleSaveComplete(msg)
	case TransactionResultMsg:
		return true, m.handleTransactionResult(msg)
	case BatchTransactionResultMsg:
		return true, m.handleBatchTransactionResult(msg)
	case TransactionErrorMsg:
		m.err = msg.Err
		return true, statusTimeout(5 * time.Second)
	default:
		return false, nil
	}
}

// wakeDeferredRows returns a lapsed deferral to the outline on the first tick
// past it. It flips the memoization bypass instead of rebuilding, so the render
// pass Bubble Tea runs right after Update stays the single rebuild authority
// and computeOutlinerViewport keeps sole ownership of viewScrollOffset. A
// refusal leaves nextDeferWake standing, so the next tick retries.
func (m *AppModel) wakeDeferredRows(now time.Time) {
	out := m.crossOutliner
	if out == nil || out.nextDeferWake.IsZero() || now.Before(out.nextDeferWake) {
		return
	}
	if !m.safeToRebuildForWake() {
		return
	}
	out.nextDeferWake = time.Time{}
	out.crossForceRebuild = true
}

// safeToRebuildForWake refuses while a new row would invalidate what the user
// is already in the middle of: an overlay or value picker holds a pending
// operation's resolved refs, an inline row editor and a visual selection both
// name rows the rebuild would renumber, and a search projection is a frozen
// answer to a query the woken task was never matched against.
func (m *AppModel) safeToRebuildForWake() bool {
	out := m.crossOutliner
	if out == nil {
		return false
	}
	return !m.Overlays().Active() &&
		!out.quickEdit.IsActive() &&
		!out.IsMultiSelectActive() &&
		!out.hasSearchProjection()
}

// handleTransactionResult routes a committed feature-coordinator mutation back
// into the shell. The application layer committed one atomic ChangeSet against
// its own freshly loaded snapshot, so the AppModel's in-memory copy of the
// affected project is reloaded from disk before the shared post-save refresh
// runs, keeping every open view consistent with the committed transaction.
func (m *AppModel) handleTransactionResult(msg TransactionResultMsg) tea.Cmd {
	m.reloadProjectByID(msg.Result.ProjectID)
	if m.crossOutliner != nil {
		m.crossOutliner.armOutlinerRelocation(msg.outlinerRelocation)
	}
	if msg.PlanningChanged && m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()
	saveCmd := m.handleSaveComplete(SaveCompleteMsg{})
	feedbackCmd := m.setRecoveryFeedback(msg.SuccessMessage(), msg.Result.TransactionID, false)
	return tea.Batch(saveCmd, feedbackCmd)
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
	if m.crossOutliner != nil {
		m.crossOutliner.armOutlinerRelocation(msg.outlinerRelocation)
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
	successMessage := msg.SuccessMessage()
	if successMessage == "" {
		successMessage = m.statusMessage
	}
	saveCmd := m.handleSaveComplete(SaveCompleteMsg{})
	feedbackCmd := m.setRecoveryFeedback(successMessage, msg.TransactionID, false)
	return tea.Batch(saveCmd, feedbackCmd)
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

func (m *AppModel) handleGlobalRefreshMessage(tea.Msg) (bool, tea.Cmd) { return false, nil }

func (m *AppModel) handleSaveComplete(msg SaveCompleteMsg) tea.Cmd {
	m.clearToggleHistory()
	// An empty success message means the mutation carried no status of its own
	// (e.g. a ProjectTree/Outliner reorder committed through CommitProjectWorkflow):
	// leave any status the view set synchronously intact rather than blanking it.
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
	m.refreshWorkspacePulseAt(time.Now())
	m.refreshProjectViewAfterTaskMutation(persistProjectTreeState)
	var saveCmds []tea.Cmd
	if reloadCrossProject {
		if reloadCmd := m.requestCrossProjectOutlinerReload(); reloadCmd != nil {
			saveCmds = append(saveCmds, reloadCmd)
		}
	}
	if m.views != nil {
		for _, viewType := range []PresentationID{
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

func (m *AppModel) refreshWorkspacePulseAt(now time.Time) {
	cachedActivityIsCurrent := m.completionActivity.IsCurrentLocalDay(now)
	m.workspacePulse = ComputeWorkspacePulse(m.runtime, m.todayIDs, now)
	activity, err := m.features.Activity.CompletionActivity()
	if err != nil {
		logging.Warn("refresh completion activity: %v", err)
		if cachedActivityIsCurrent {
			m.workspacePulse.Done = m.completionActivity.CompletedToday()
		} else {
			m.completionActivity = app.CompletionActivity{}
			m.workspacePulse.Done = 0
		}
		return
	}
	m.completionActivity = activity
	m.workspacePulse.Done = activity.CompletedToday()
}

func (m *AppModel) refreshProjectViewAfterTaskMutation(persistProjectTreeState bool) {
	if persistProjectTreeState {
		m.refreshProjectView()
	} else {
		m.refreshProjectViewInMemory()
	}
}
