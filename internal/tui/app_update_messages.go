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
		m.saveProjectTreeState()
		m.saveOutlinerFilterState()
		m.saveCrossOutlinerState()
		m.quitting = true
		return true, tea.Quit
	case spinner.TickMsg:
		if !m.isLoading {
			return false, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return true, cmd
	case ClearSemicolonPrefixMsg:
		if !m.pendingSemicolonPrefix && !m.pendingSemicolonTPrefix {
			return true, nil
		}
		m.pendingSemicolonPrefix = false
		m.pendingSemicolonTPrefix = false
		m.setInfoStatus("prefix cleared")
		return true, defaultStatusTimeout()
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
		m.saveProjectTreeState()
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
	case ConfigTransactionResultMsg:
		return true, m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: msg.SuccessMessage()})
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
	if m.dashboard != nil {
		m.dashboard.Refresh(m.config)
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
	// Repair the machine-local recent visits for every removed task — a bulk
	// delete folds the DURABLE planning refs (Focus Queue, Daily Plan, dismissals)
	// into the transaction as a config artifact, but recent visits are
	// non-authoritative UI state cleaned separately after the commit, mirroring the
	// CLI delete. A save failure here is a warning, not an error: the delete is
	// already committed.
	if len(msg.RemovedTaskIDs) > 0 && m.config != nil && m.configManager != nil {
		if m.config.RemoveMachineLocalTaskRefs(taskIDSliceToSet(msg.RemovedTaskIDs)) > 0 {
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to update recent visits after bulk delete: %v", err)
			}
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
	if len(msg.RuleTriggerCounts) > 0 && m.config != nil && m.configManager != nil {
		m.config.IncrementRuleTriggerCounts(msg.RuleTriggerCounts)
		_ = m.configManager.Save(m.config)
	}
	if m.dashboard != nil {
		m.dashboard.Refresh(m.config)
	}
	return m.handleSaveComplete(SaveCompleteMsg{SuccessMessage: msg.SuccessMessage()})
}

// reloadProjectByID reloads a project's tasks from disk into the AppModel's
// in-memory copy so views bound to it reflect a mutation committed through the
// application layer. A blank id, unknown project, or load error is a no-op — the
// shared post-save refresh reloads the current view's projects regardless.
func (m *AppModel) reloadProjectByID(projectID string) {
	if projectID == "" {
		return
	}
	project := m.config.GetProject(projectID)
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
		m.openLoops = ComputeOpenLoops(m.config, m.dailyPlanIDs)
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
		if !m.config.GetAutoTheme() {
			return true, nil
		}
		detector := m.appearanceDetector
		return true, func() tea.Msg {
			return systemAppearanceResultMsg{Appearance: detector.Detect()}
		}
	case systemAppearanceResultMsg:
		cmd := m.handleAppearanceResult(msg)
		if m.config.GetAutoTheme() {
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
		m.saveCrossOutlinerState()
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
	m.invalidateFilterCaches()
	m.openLoops = ComputeOpenLoops(m.config, m.dailyPlanIDs)
	m.refreshNotesSearchState()
	m.refreshProjectTree()
	if m.allTasksView != nil {
		m.allTasksView.loadAllTasks()
		m.allTasksView.clampCursor()
	}
	if m.outliner != nil && m.outliner.project != nil {
		if err := storage.LoadProject(m.outliner.project); err != nil {
			logging.Warn("failed to load project %s: %v", m.outliner.project.Name, err)
		}
		m.outliner.rebuildRows()
		m.outliner.clampCursor()
	}
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
		if v := m.views.Get(ViewDueDateOverview); v != nil {
			if dv, ok := v.(*DueDateOverviewModel); ok {
				dv.reloadData()
				dv.clampCursor()
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
		if v := m.views.Get(ViewFocusView); v != nil {
			if fv, ok := v.(*FocusViewModel); ok {
				fv.loadTasks()
				fv.clampCursor()
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
				td.refreshAfterReload()
			}
		}
		for _, vt := range []ViewType{ViewKanban, ViewCrossProjectKanban} {
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

func (m *AppModel) refreshNotesSearchState() {
	if m.globalSearch != nil {
		m.globalSearch.Rebuild()
	}
	if m.views == nil {
		return
	}
	if v := m.views.Get(ViewNotesSearch); v != nil {
		if ns, ok := v.(*NotesSearchModel); ok {
			ns.MarkStale()
		}
	}
}
