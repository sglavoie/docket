package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
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
		// A bare modifier is not a grammar key on any surface, so it is dropped
		// here rather than reaching a strategy and abandoning a pending chord.
		if isModifierOnlyKeyPress(msg) {
			return true, nil
		}
		return m.handleGlobalKeyPress(msg)
	case tea.KeyboardEnhancementsMsg:
		return true, nil
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
	case status.QuitMsg:
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
		adopted := msg.generation == m.crossOutliner.crossLoadGen
		if adopted {
			m.adoptCrossProjectSnapshot(msg.projects)
		}
		wantedFocus := msg.focusProjectID
		cmd := m.crossOutliner.handleCrossProjectsLoaded(msg)
		// The launch lens is drained here rather than in the launch handler
		// because a lens names a project scope and a set of project facets, and
		// neither can be validated against a population that has not been adopted
		// yet. Draining after the adoption above is what lets a lens naming a
		// deleted project be refused instead of installed over an empty view.
		// A superseded result adopts nothing, so it drains nothing either: the
		// load that is still on its way is the one that owns the launch.
		var lensCmd tea.Cmd
		if adopted {
			lensCmd = m.restorePendingLaunchLens()
		}
		returnCmd := m.finishParkedProjectFocus(wantedFocus)
		// This is the message that publishes a new project population, so it is
		// where the note index learns that an owner appeared, was renamed, was
		// archived, or went away. The refresh is a no-op unless the catalog moved.
		var noteCmd tea.Cmd
		if adopted {
			noteCmd = m.refreshNoteSearchIndexOwners()
		}
		if lensCmd == nil && returnCmd == nil && noteCmd == nil {
			return true, cmd
		}
		return true, tea.Batch(cmd, returnCmd, lensCmd, noteCmd)
	case lensProjectsLoadedMsg:
		// A lens application publishes a whole population or none of it, so the
		// shell cache is adopted only once the lens actually went live.
		if m.crossOutliner == nil {
			return true, nil
		}
		cmd, published := m.crossOutliner.handleLensProjectsLoaded(msg)
		if published {
			m.adoptCrossProjectSnapshot(m.crossOutliner.loadedProjects)
		}
		return true, cmd
	case noteSearchIndexLoadedMsg:
		m.handleNoteSearchIndexLoaded(msg)
		return true, nil
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
	case status.PeriodicSaveMsg:
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		// A consumed wake makes rows visible that ComputeWorkspacePulse excluded
		// while their deferral was pending, so the same update recounts the pulse
		// even within one local day.
		if m.wakeDeferredRows(msg.At) {
			m.refreshWorkspacePulseAt(msg.At)
		} else if !m.completionActivity.IsCurrentLocalDay(msg.At) {
			m.refreshWorkspacePulseAcrossLocalDay(msg.At)
		}
		// Conflict bundles are written by another process on another machine, so
		// no local mutation can announce one; the periodic tick is what makes the
		// indicator appear, and what clears it once the CLI resolves the last one.
		m.refreshOutstandingConflicts()
		return true, status.TickCmd(periodicSaveInterval, func(at time.Time) tea.Msg {
			return status.PeriodicSaveMsg{At: at}
		})
	case status.SaveCompleteMsg:
		return true, m.handleSaveComplete(msg)
	case coordinator.TransactionResultMsg:
		return true, m.handleTransactionResult(msg)
	case coordinator.BatchTransactionResultMsg:
		return true, m.handleBatchTransactionResult(msg)
	case coordinator.TransactionErrorMsg:
		m.err = msg.Err
		return true, status.StatusTimeout(5 * time.Second)
	default:
		return false, nil
	}
}

// wakeDeferredRows returns a lapsed deferral to the outline on the first tick
// past it, reporting whether it consumed the wake. It flips the memoization
// bypass instead of rebuilding, so the render pass Bubble Tea runs right after
// Update stays the single rebuild authority and computeOutlinerViewport keeps
// sole ownership of ViewScrollOffset. A refusal leaves nextDeferWake standing,
// so the next tick retries.
func (m *AppModel) wakeDeferredRows(now time.Time) bool {
	if !m.deferWakeOwedAt(now) || !m.safeToRebuildForWake() {
		return false
	}
	out := m.crossOutliner
	out.nextDeferWake = time.Time{}
	out.crossForceRebuild = true
	return true
}

// deferWakeOwedAt reports whether a lapsed defer wake is still standing. After a
// wake attempt only a refusal leaves one, because wakeDeferredRows zeroes the
// clock it consumes.
func (m *AppModel) deferWakeOwedAt(now time.Time) bool {
	out := m.crossOutliner
	return out != nil && !out.nextDeferWake.IsZero() && !now.Before(out.nextDeferWake)
}

// safeToRebuildForWake refuses while a new row would invalidate what the user
// is already in the middle of: an overlay or value picker holds a pending
// operation's resolved refs, an inline row editor and a visual selection both
// name rows the rebuild would renumber, and the flat ranked list is a settled
// order the woken task was never scored against. A query that is merely
// narrowing the mode's own lanes is not such a state: it is re-answered on
// every rebuild, so the woken task is matched against it like any other row.
func (m *AppModel) safeToRebuildForWake() bool {
	out := m.crossOutliner
	if out == nil {
		return false
	}
	return !m.Overlays().Active() &&
		!out.quickEdit.IsActive() &&
		!out.IsMultiSelectActive() &&
		!out.showsRankedList()
}

// handleTransactionResult routes a committed feature-coordinator mutation back
// into the shell. The application layer committed one atomic ChangeSet against
// its own freshly loaded snapshot, so the AppModel's in-memory copy of the
// affected project is reloaded from disk before the shared post-save refresh
// runs, keeping every open view consistent with the committed transaction.
func (m *AppModel) handleTransactionResult(msg coordinator.TransactionResultMsg) tea.Cmd {
	m.reloadProjectByID(msg.Result.ProjectID)
	if m.crossOutliner != nil {
		m.crossOutliner.armOutlinerRelocation(outlinerRelocationFrom(msg.Relocation))
	}
	if msg.PlanningChanged && m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()
	if m.crossOutliner != nil {
		m.crossOutliner.settleRelocationCursor()
	}
	saveCmd := m.handleSaveComplete(status.SaveCompleteMsg{})
	feedbackCmd := m.setRecoveryFeedback(msg.SuccessMessage(), msg.Result.TransactionID, false)
	return tea.Batch(saveCmd, feedbackCmd)
}

// handleBatchTransactionResult routes a committed project-scoped or multi-task
// coordinator mutation (bulk transition/delete or metadata update) back into the
// shell. The application layer committed one atomic ChangeSet across every touched
// project, so each affected project's in-memory copy is reloaded from disk before
// the shared post-save refresh runs, keeping every open view consistent.
func (m *AppModel) handleBatchTransactionResult(msg coordinator.BatchTransactionResultMsg) tea.Cmd {
	for _, projectID := range msg.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
	}
	if m.crossOutliner != nil {
		m.crossOutliner.armOutlinerRelocation(outlinerRelocationFrom(msg.Relocation))
	}
	m.refreshLoadedTaskWorkspace()
	if m.crossOutliner != nil {
		m.crossOutliner.settleRelocationCursor()
	}
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
	saveCmd := m.handleSaveComplete(status.SaveCompleteMsg{})
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

func (m *AppModel) handleSaveComplete(msg status.SaveCompleteMsg) tea.Cmd {
	m.clearToggleHistory()
	// An empty success message means the mutation carried no status of its own
	// (e.g. a ProjectTree/Outliner reorder committed through CommitProjectWorkflow):
	// leave any status the view set synchronously intact rather than blanking it.
	if msg.SuccessMessage != "" {
		m.setSuccessStatus(msg.SuccessMessage)
	}
	saveCmds := m.refreshTaskViewsAfterSave()
	saveCmds = append(saveCmds, status.DefaultStatusTimeout())
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
		for _, viewType := range []presentation.PresentationID{
			presentation.ViewTaskComments,
			presentation.ViewTaskAttachments,
		} {
			view := m.views.Get(viewType)
			refresher, ok := view.(interface{ RefreshAfterReload() bool })
			if !ok {
				continue
			}
			if !refresher.RefreshAfterReload() && m.currentView() == viewType {
				saveCmds = append(saveCmds, func() tea.Msg { return status.BackMsg{} })
			}
		}
	}
	return saveCmds
}

func (m *AppModel) refreshWorkspacePulseAt(now time.Time) {
	cachedActivityIsCurrent := m.completionActivity.IsCurrentLocalDay(now)
	m.workspacePulse = computeWorkspacePulse(m.runtime, m.todayIDs, now)
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

// refreshWorkspacePulseAcrossLocalDay refreshes a pulse whose cached completion
// activity belongs to an earlier local day. A wake still owed here was refused
// for interaction safety, so its task remains hidden from the outline and only
// Done may move: holding Inbox, Today, and Overdue keeps the counts and the rows
// they describe in step until the rebuild is allowed.
func (m *AppModel) refreshWorkspacePulseAcrossLocalDay(now time.Time) {
	held := m.workspacePulse
	m.refreshWorkspacePulseAt(now)
	if !m.deferWakeOwedAt(now) {
		return
	}
	m.workspacePulse.Inbox = held.Inbox
	m.workspacePulse.Today = held.Today
	m.workspacePulse.Overdue = held.Overdue
}

func (m *AppModel) refreshProjectViewAfterTaskMutation(persistProjectTreeState bool) {
	if persistProjectTreeState {
		m.refreshProjectView()
	} else {
		m.refreshProjectViewInMemory()
	}
}
