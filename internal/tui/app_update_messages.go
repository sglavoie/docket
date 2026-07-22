package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// handleAskMessage owns action-plan completion independently of whichever view
// is currently visible. Transactions cannot be canceled once committing, so a
// covered or replaced Ask still needs exactly-once reference repair, project
// reloads, and cache/view refreshes after its command returns.
func (m *AppModel) handleAskMessage(raw tea.Msg) (bool, tea.Cmd) {
	msg, ok := raw.(AskActionPlanAppliedMsg)
	if !ok {
		return false, nil
	}
	if msg.OperationID == 0 {
		return true, nil
	}
	if m.handledAskApplyIDs == nil {
		m.handledAskApplyIDs = make(map[uint64]struct{})
	}
	if _, handled := m.handledAskApplyIDs[msg.OperationID]; handled {
		return true, nil
	}
	m.handledAskApplyIDs[msg.OperationID] = struct{}{}

	var cmds []tea.Cmd
	if msg.Err == nil {
		msg.RepairWarning = repairAskActionPlanTaskRefs(m.runtime, msg.Result.DeletedTaskIDs, msg.PendingDomains)
		for _, projectID := range msg.Result.TouchedProjects {
			m.reloadProjectByID(projectID)
		}
		m.clearToggleHistory()
		m.observeLoadedProjectMilestones(msg.Result.TouchedProjects...)
		if m.dashboard != nil {
			m.dashboard.Refresh()
		}
		cmds = append(cmds, m.refreshTaskViewsAfterSave()...)
	}

	var matchingAsk *AskViewModel
	if m.views != nil {
		if ask, registered := m.views.Get(ViewAsk).(*AskViewModel); registered &&
			ask.state == askStateApplying && ask.applyID == msg.OperationID {
			matchingAsk = ask
		}
	}
	matchingAskVisible := matchingAsk != nil && m.currentView() == ViewAsk
	if matchingAsk != nil {
		_, cmd := matchingAsk.Update(msg)
		cmds = append(cmds, cmd)
	}

	if !matchingAskVisible {
		if msg.Err != nil {
			failure := fmt.Sprintf("AI action plan apply failed: %s", askErrorText(msg.Err))
			m.setBackgroundErrorStatus(failure)
			cmds = append(cmds, statusTimeout(5*time.Second))
		} else {
			success := fmt.Sprintf("Applied %d AI action(s) · transaction %s · undo %s", len(msg.Result.Actions), msg.Result.TransactionID, msg.Result.UndoID)
			m.setSuccessStatus(success)
			if msg.RepairWarning != nil {
				warning := fmt.Sprintf("%s — warning: local reference repair incomplete: %s", success, askErrorText(msg.RepairWarning))
				m.setWarningStatus(warning)
			}
			cmds = append(cmds, defaultStatusTimeout())
		}
	}

	return true, tea.Batch(cmds...)
}

func (m *AppModel) handleGlobalMessage(msg tea.Msg) (bool, tea.Cmd) {
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
	case tea.PasteMsg:
		if m.quickNoteOverlay != nil {
			var cmd tea.Cmd
			m.quickNoteOverlay.input, cmd = m.quickNoteOverlay.input.Update(msg)
			return true, cmd
		}
		return false, nil
	case tea.KeyPressMsg:
		return m.handleGlobalKeyPress(msg)
	case tea.KeyboardEnhancementsMsg:
		m.keyboardEnhancements = msg
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
	case RecordRepeatableActionMsg:
		m.lastRepeatableAction = msg.Action
		return true, nil
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
	case projectSidebarStateChangedMsg:
		m.setProjectSidebarEnabled(msg.ProjectID, msg.Enabled)
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
	case ConfigTransactionResultMsg:
		// A config-domain coordinator committed the settings/machine or automation
		// domain through a checked save; advance the shared runtime snapshot so views
		// reading it observe the committed preferences without a render-time disk load.
		m.refreshRuntime()
		return true, m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: msg.SuccessMessage()})
	case TransactionErrorMsg:
		m.isLoading = false
		m.loadingMessage = ""
		m.err = msg.Err
		// A rejected transaction must leave the inbox queue untouched: drop the
		// armed post-commit bookkeeping so the same task stays selected and
		// unprocessed, and likewise drop any armed Weekly Review deletion accounting
		// so a failed delete stays uncounted. Correlate on the request token so only
		// THIS delete's rejection clears its own state — an unrelated transaction
		// error (token 0 or a different delete's token) leaves an in-flight Inbox or
		// Weekly Review delete intact.
		m.clearPendingDeleteBookkeeping(msg.DeleteRequestToken)
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
	m.observeLoadedProjectMilestones(msg.Result.ProjectID)
	if m.dashboard != nil {
		m.dashboard.Refresh()
	}
	return m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: msg.SuccessMessage()})
}

// handleBatchTransactionResult routes a committed project-scoped or multi-task
// coordinator mutation (bulk transition/delete, blocker update) back into the
// shell. The application layer committed one atomic ChangeSet across every touched
// project, so each affected project's in-memory copy is reloaded from disk before
// the shared post-save refresh runs, keeping every open view consistent.
func (m *AppModel) handleBatchTransactionResult(msg BatchTransactionResultMsg) tea.Cmd {
	for _, projectID := range msg.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
	}
	m.refreshLoadedTaskWorkspace()
	// Repair the machine-local recent visits for every removed task — a bulk
	// delete folds the DURABLE planning refs (Focus Queue, Daily Plan, dismissals)
	// into the transaction as a config artifact, but recent visits are
	// non-authoritative UI state cleaned separately after the commit, mirroring the
	// CLI delete. A save failure here is a warning, not an error: the delete is
	// already committed.
	if len(msg.RemovedTaskIDs) > 0 {
		removed := taskIDSliceToSet(msg.RemovedTaskIDs)
		if err := m.commitMachineMutation(func(machine *config.MachineState) {
			machine.RemoveMachineLocalTaskRefs(removed)
		}); err != nil {
			logging.Warn("failed to update recent visits after bulk delete: %v", err)
		}
	}
	// Clear multiselect state only now that the batch has committed, so a rejected
	// batch (TransactionErrorMsg, which never reaches this handler) leaves the
	// selection intact for the user to retry.
	if len(msg.ClearSelectionTaskIDs) > 0 {
		m.clearBulkTaskSelectionsForTaskIDs(msg.ClearSelectionTaskIDs)
	}
	// Persist workflow-rule trigger counts fired by a bulk-with-rules mutation.
	// They live in workspace config (not project Markdown), so they ride outside
	// the ChangeSet; doing it here in Update() keeps the shared config write off
	// the async command goroutine.
	if len(msg.RuleTriggerCounts) > 0 {
		if err := m.commitAutomationMutation(func(automation *config.AutomationConfig) {
			automation.IncrementRuleTriggerCounts(msg.RuleTriggerCounts)
		}); err != nil {
			logging.Warn("failed to update workflow rule trigger counts: %v", err)
		}
	}
	// The checked planning artifact folded into this batch is now committed.
	// Rebuild the shared planning caches only after the runtime snapshot advances:
	// registered views refresh below and must never render a known-stale snapshot.
	if m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.observeLoadedProjectMilestones(msg.TouchedProjectIDs...)
	if m.dashboard != nil {
		m.dashboard.Refresh()
	}
	// Credit the Weekly Review completion summary for a committed delete it
	// originated. A no-op for every other origin (the count is armed only for a
	// Weekly Review-origin delete in confirmTaskDelete) and for an unrelated batch
	// whose token does not match the armed request.
	m.applyPendingWeeklyReviewDelete(msg.DeleteRequestToken, msg.RemovedTaskIDs)
	saveCmd := m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: msg.SuccessMessage()})
	// Apply inbox processor bookkeeping only after the committed views have been
	// refreshed above (so the queue rebuild reads committed project data). A
	// no-op for non-inbox origins and for a batch whose token does not match the
	// armed request.
	if inboxCmd := m.applyPendingInboxDelete(msg.DeleteRequestToken, msg.RemovedTaskIDs); inboxCmd != nil {
		return tea.Batch(saveCmd, inboxCmd)
	}
	return saveCmd
}

func (m *AppModel) refreshLoadedTaskWorkspace() {
	if m.taskWorkspace == nil {
		return
	}
	if err := m.taskWorkspace.RefreshIfLoaded(); err != nil {
		logging.Warn("failed to refresh Task Workspace: %v", err)
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
	switch msg := msg.(type) {
	case RefreshOpenLoopsMsg:
		m.openLoops = ComputeOpenLoops(m.runtime, m.dailyPlanIDs)
		return true, nil
	case TodayClearedBannerClearMsg:
		m.todayClearedBanner = ""
		return true, nil
	case NotesRefreshMsg:
		m.refreshNotesSearchState()
		if nv := m.getActiveNotesView(); nv != nil {
			nv.tasksLoaded = false
			nv.loadNotes()
		}
		return true, nil
	case systemAppearanceCheckMsg:
		if !m.runtime.Settings().GetAutoTheme() {
			return true, nil
		}
		detector := m.appearanceDetector
		return true, func() tea.Msg {
			return systemAppearanceResultMsg{Appearance: detector.Detect()}
		}
	case systemAppearanceResultMsg:
		cmd := m.handleAppearanceResult(msg)
		if m.runtime.Settings().GetAutoTheme() {
			return true, tea.Batch(cmd, tea.Tick(appearanceCheckInterval, func(time.Time) tea.Msg {
				return systemAppearanceCheckMsg{}
			}))
		}
		return true, cmd
	default:
		return false, nil
	}
}

func (m *AppModel) persistOutlinerFilterState() {
	// Persist filter state immediately so navigation (e.g. go drill-down
	// from cross-outliner) reads the up-to-date cleared/changed values.
	switch m.currentView() { //nolint:exhaustive
	case ViewCrossProjectOutliner:
		return
	default:
		m.saveOutlinerFilterState()
	}
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
	return m.refreshTaskViews(true)
}

// refreshTaskViewsAfterRestore uses the same cache/view fan-out as a committed
// save. When the runtime reload failed, it rebuilds the current project tree in
// memory without persisting view state from the stale runtime snapshot.
func (m *AppModel) refreshTaskViewsAfterRestore(runtimeReloaded bool) []tea.Cmd {
	return m.refreshTaskViews(runtimeReloaded)
}

func (m *AppModel) refreshTaskViews(persistProjectTreeState bool) []tea.Cmd {
	m.invalidateFilterCaches()
	m.openLoops = ComputeOpenLoops(m.runtime, m.dailyPlanIDs)
	m.refreshNotesSearchState()
	m.refreshProjectViewAfterTaskMutation(persistProjectTreeState)
	if m.allTasksView != nil {
		m.allTasksView.loadAllTasks()
		m.allTasksView.clampCursor()
	}
	if m.reviewHub != nil {
		m.reviewHub.SetPlanningIndicators(m.focusQueueIDs, m.dailyPlanIDs)
		m.reviewHub.Refresh()
	}
	m.reloadRegisteredProjectOutliners(storage.LoadProject)
	m.rebuildRegisteredProjectOutliners()
	var saveCmds []tea.Cmd
	if m.crossOutliner != nil {
		saveCmds = append(saveCmds, m.crossOutliner.reloadCrossProjects())
	}
	if m.views != nil {
		if v := m.views.Get(ViewPriorityView); v != nil {
			if pv, ok := v.(*PriorityViewModel); ok {
				pv.reloadData()
				pv.clampCursor()
			}
		}
		if v := m.views.Get(ViewDailyReview); v != nil {
			if dr, ok := v.(*DailyReviewModel); ok {
				dr.reloadData()
				dr.clampCursor()
			}
		}
		if v := m.views.Get(ViewWeeklyReview); v != nil {
			if wr, ok := v.(*WeeklyReviewModel); ok {
				wr.reloadData()
				wr.clampCursor()
			}
		}
		if v := m.views.Get(ViewFocusBoard); v != nil {
			if fb, ok := v.(*FocusBoardModel); ok {
				fb.refreshData()
			}
		}
		if v := m.views.Get(ViewRecentActivity); v != nil {
			if ra, ok := v.(*RecentActivityModel); ok {
				ra.loadTasks()
			}
		}
		if v := m.views.Get(ViewTaskAttachments); v != nil {
			if av, ok := v.(*TaskAttachmentsModel); ok {
				av.refreshAfterReload()
			}
		}
		if v := m.views.Get(ViewTaskDetail); v != nil {
			if td, ok := v.(*TaskDetailModel); ok {
				if td.task != nil {
					td.SetPlanningMembership(m.dailyPlanIDs[td.task.ID], m.focusQueueIDs[td.task.ID])
				}
				if !td.refreshAfterReload() && m.taskDetail == td && m.activeTaskDetail() {
					saveCmds = append(saveCmds, func() tea.Msg { return BackMsg{} })
				}
			}
		}
		m.refreshLinkedTaskSelectorAfterReload()
		for _, vt := range []PresentationID{ViewKanban, ViewCrossProjectKanban} {
			if v := m.views.Get(vt); v != nil {
				if kb, ok := v.(*KanbanModel); ok {
					kb.reloadData()
					kb.clampCursor()
				}
			}
		}
	}
	if m.globalInboxProcessor != nil {
		m.globalInboxProcessor.loadAllInboxTasks()
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

func (m *AppModel) refreshLinkedTaskSelectorAfterReload() {
	if v := m.views.Get(ViewLinkedTaskSelector); v != nil {
		if ls, ok := v.(*LinkedTaskSelectorModel); ok {
			ls.refreshAfterReload(m.projectByID(tuiUndoProjectID(ls.project)))
		}
	}
}

func (m *AppModel) refreshNotesSearchState() {
	if m.views == nil {
		return
	}
	if v := m.views.Get(ViewNotesSearch); v != nil {
		if ns, ok := v.(*NotesSearchModel); ok {
			ns.MarkStale()
		}
	}
}
