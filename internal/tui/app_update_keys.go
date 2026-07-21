package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// currentViewAsModel returns the current view as a tea.Model for interface assertions.
func (m *AppModel) currentViewAsModel() tea.Model {
	return m.activeView()
}

// handleGlobalClipboard handles y/Y/ctrl+y clipboard operations for the current view.
// Returns (handled, cmd). If the view doesn't support clipboard, returns (false, nil)
// so the key falls through to the view's own Update().
func (m *AppModel) handleGlobalClipboard(key string) (bool, tea.Cmd) {
	type multiSelector interface {
		getCursorTask() *models.Task
		getMultiSelectedTasks() []*models.Task
		clearMultiSelect()
	}
	if mv, ok := m.currentViewAsModel().(multiSelector); ok {
		return handleClipboardKey(key,
			func() *models.Task { return mv.getCursorTask() },
			func() []*models.Task { return mv.getMultiSelectedTasks() },
			mv.clearMultiSelect,
		)
	}

	type taskRefProvider interface {
		getTaskRef() (*models.Task, *models.Project)
	}
	if tv, ok := m.currentViewAsModel().(taskRefProvider); ok {
		task, _ := tv.getTaskRef()
		return handleClipboardKey(key, func() *models.Task { return task }, nil, nil)
	}

	return false, nil
}

type taskReferenceProvider interface {
	getTaskRef() (*models.Task, *models.Project)
}

var (
	createReferenceSnapshotFile = external.CreateReferenceSnapshotFile
	openReferenceFile           = external.OpenReferenceFile
)

func (m *AppModel) handleGlobalKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) { //nolint:gocyclo
	key := msg.String()
	if key == "ctrl+c" {
		m.exitAskView()
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		m.quitting = true
		return true, tea.Quit
	}
	if isCtrlEscKey(msg) {
		return true, m.jumpToDashboard()
	}
	// Esc is the canonical close/back operation. App-owned overlays and
	// confirmations receive it before routed controls; a routed control may
	// still capture Esc for its own inline state when no shell overlay is open.
	if key == "esc" {
		if m.pendingSandboxReset {
			m.pendingSandboxReset = false
			m.pendingSandboxProjectID = ""
			m.pendingSandboxTaskCount = 0
			m.setInfoStatus("Canceled")
			return true, defaultStatusTimeout()
		}
		if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
			return true, cmd
		}
		if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
			return true, cmd
		}
		if handled, cmd := m.handleEscKey(); handled {
			return true, cmd
		}
	}

	// Handle pending sandbox reset confirmation before clearing status
	if m.pendingSandboxReset {
		switch key {
		case "y", "Y":
			return true, m.executeSandboxReset()
		case "n", "N", "esc":
			m.pendingSandboxReset = false
			m.pendingSandboxProjectID = ""
			m.pendingSandboxTaskCount = 0
			m.setInfoStatus("Canceled")
			return true, defaultStatusTimeout()
		default:
			// Keep showing the confirmation prompt
			return true, nil
		}
	}

	if handled, cmd := m.handleProjectArchiveConfirmKey(key); handled {
		return true, cmd
	}

	if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
		return true, cmd
	}

	// Intercept keys when peek overlay is active: handle scroll, pane focus,
	// and dismiss; block everything else so the underlying view is frozen.
	if m.peekActive {
		switch key {
		case "esc", "space":
			m.dismissPeek()
		case "tab", "shift+tab":
			m.peekFocusRight = !m.peekFocusRight
		case "up", "k":
			if m.peekFocusRight {
				m.peekRightOffset = clampPeekOffset(m.peekRightOffset-1, m.peekRightLines, m.peekPaneHeight)
			} else {
				m.peekLeftOffset = clampPeekOffset(m.peekLeftOffset-1, m.peekLeftLines, m.peekPaneHeight)
			}
		case "down", "j":
			if m.peekFocusRight {
				m.peekRightOffset = clampPeekOffset(m.peekRightOffset+1, m.peekRightLines, m.peekPaneHeight)
			} else {
				m.peekLeftOffset = clampPeekOffset(m.peekLeftOffset+1, m.peekLeftLines, m.peekPaneHeight)
			}
		}
		return true, nil
	}

	// Guard against stale currentProject (e.g., deleted by external CLI command)
	if handled, cmd := m.handleInvalidManageProjectScope(); handled {
		return true, cmd
	}
	if (m.currentView() != ViewDashboard || m.activeTaskWorkspace()) && m.validateCurrentProject() {
		return true, defaultStatusTimeout()
	}
	// The exact non-preemptive global map is suppressed while the active
	// control captures text. Ctrl+C, Ctrl+Esc, and Esc were handled above.
	switch key {
	case "ctrl+g":
		if !m.hasActiveTextInput() {
			m.toggleDestinationSwitcher()
			return true, nil
		}
	case "ctrl+n":
		if !m.typedNavigation {
			break
		}
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowQuickCaptureMsg{} }
		}
	case "ctrl+x":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowProjectPickerMsg{} }
		}
	case ":":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowCommandPaletteMsg{} }
		}
	case "?":
		if !m.hasActiveTextInput() {
			view := m.currentView()
			items := GetKeybindingRegistry().GetForView(view)
			viewName := view.String()
			if workspace, ok := m.activeView().(*TaskWorkspaceView); ok {
				items = workspace.GetHelpItems()
				viewName = "Task Workspace"
			} else if planner, ok := m.activeView().(*FocusBoardModel); ok {
				items = planner.GetHelpItems()
				viewName = "Planner"
			} else if review, ok := m.activeView().(*ReviewHubModel); ok {
				items = review.GetHelpItems()
				viewName = review.StatusBarLabel()
			} else if manage, ok := m.activeView().(*ManageHubModel); ok {
				items = manage.GetHelpItems()
				viewName = manage.StatusBarLabel()
			} else if detail, ok := m.activeView().(*TaskDetailModel); ok {
				items = detail.GetHelpItems()
				viewName = detail.StatusBarLabel()
			}
			return true, func() tea.Msg {
				return ShowHelpMsg{SourceView: view, ViewName: viewName, Items: items}
			}
		}
	case "|":
		if !m.hasActiveTextInput() {
			return true, m.handleQuickSwitchToggle()
		}
	case "#":
		if !m.hasActiveTextInput() && !m.hasActivePendingPrefix() {
			return true, m.handlePinnedPopupToggle()
		}
	case "q":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() {
			break
		}
		if m.activeTaskWorkspace() || m.currentView() == ViewFocusBoard {
			break
		}
		if cmd := m.toggleQueue(); cmd != nil {
			return true, m.batchWithFocusRefresh(cmd)
		}
		return true, nil
	case "w":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
			break
		}
		if m.activeTaskWorkspace() || m.currentView() == ViewFocusBoard {
			break
		}
		if cmd := m.toggleDailyPlan(); cmd != nil {
			return true, m.batchWithFocusRefresh(cmd)
		}
	case "u":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() {
			break
		}
		// Inline undo toast: when an undo-available toast is on screen,
		// `u` triggers its callback rather than the view-local undo
		// (queue/daily-plan toggle). The toast wins because it is the
		// most recent destructive action the user just took.
		if m.statusUndoCb != nil && time.Now().Before(m.statusExpiresAt) {
			cb := m.statusUndoCb
			m.clearStatus()
			return true, cb()
		}
		if cmd := m.handleUndoToggle(); cmd != nil {
			return true, cmd
		}
		if cmd := m.handleGlobalTaskUndo(); cmd != nil {
			return true, cmd
		}
		if m.currentViewUsesTodaySimpleGrammar() {
			return true, nil
		}
	case "ctrl+r":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() {
			break
		}
		if cmd := m.handleRedoToggle(); cmd != nil {
			return true, cmd
		}
		if cmd := m.handleGlobalTaskRedo(); cmd != nil {
			return true, cmd
		}
	case "y", "Y", "ctrl+y":
		if m.currentViewUsesTodaySimpleGrammar() {
			break
		}
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() {
			break
		}
		if m.activeTaskDetail() && m.taskDetail.Tab() != TaskOverview {
			break
		}
		if m.activeTaskWorkspace() && key != "y" {
			break
		}
		if handled, cmd := m.handleGlobalClipboard(msg.String()); handled {
			return true, cmd
		}
	case "enter", "shift+enter":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() || m.hasActiveMultiSelect() {
			break
		}
		if m.activeTaskDetail() {
			break
		}
		if m.activeTaskWorkspace() && m.taskWorkspaceView != nil && m.taskWorkspaceView.menuOpen {
			break
		}
		task, project := m.getCurrentCursorTaskAndProject()
		if task == nil {
			return false, nil
		}
		if key == "shift+enter" {
			m.activatePeek(task, project)
			return true, nil
		}
		return true, func() tea.Msg { return ShowTaskDetailMsg{Task: task, Project: project} }
	case ".":
		if m.activeTaskWorkspace() {
			break
		}
		if !m.hasActiveTextInput() && m.lastRepeatableAction != nil {
			if pv, ok := m.currentViewAsModel().(prefixableView); ok {
				if pv.getCursorTask() != nil {
					return true, m.lastRepeatableAction.apply(pv)
				}
			}
		}
	}

	return false, nil
}

func (m *AppModel) currentViewUsesTodaySimpleGrammar() bool {
	return m.currentView() == ViewFocusBoard
}

func (m *AppModel) handleGlobalTaskUndo() tea.Cmd {
	return m.handleGlobalTaskUndoRedo(false)
}

func (m *AppModel) handleGlobalTaskRedo() tea.Cmd {
	return m.handleGlobalTaskUndoRedo(true)
}

func (m *AppModel) handleGlobalTaskUndoRedo(redo bool) tea.Cmd {
	if !m.currentViewSupportsGlobalTaskUndoRedo() {
		return nil
	}
	_, project := m.getCurrentCursorTaskAndProject()
	if project == nil {
		return nil
	}
	// Application-service mutations commit to the durable workspace ledger, while a
	// few remaining paths (task archive) still record legacy per-project snapshots.
	// Restore through whichever backend holds the newer transaction so a committed
	// application mutation is always reachable and a stale legacy entry never
	// shadows it.
	if m.workspaceLedgerRestoreIsNewest(project, redo) {
		return m.handleLedgerTaskUndoRedo(redo)
	}
	if redo {
		return performRedoForProjectsCmd(m.persister, project, activeUndoProjectsFromRuntime(m.runtime), m.repairTaskProjectRefsAfterRestore)
	}
	return performUndoForProjectsCmd(m.persister, project, activeUndoProjectsFromRuntime(m.runtime), m.repairTaskProjectRefsAfterRestore)
}

func (m *AppModel) currentViewSupportsGlobalTaskUndoRedo() bool {
	if m.activeTaskWorkspace() {
		return true
	}
	if m.activeTaskDetail() {
		return true
	}
	if m.activeReviewHub() {
		return m.reviewHub.SupportsTaskUndoRedo()
	}
	switch m.currentView() { //nolint:exhaustive
	case ViewPriorityView,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewProjectFocusView,
		ViewAllTasks,
		ViewRecentActivity,
		ViewFocusBoard,
		ViewKanban,
		ViewCrossProjectKanban,
		// The single-project Outliner and Task Detail commit their mutations
		// through the same workspace ledger as the views above, so their u/ctrl+r
		// must select and restore that durable transaction (with the legacy
		// per-project fallback retained only when it is genuinely newer). The
		// cross-project Outliner keeps its own crossHandleUndo path and is
		// intentionally excluded here.
		ViewOutliner,
		ViewTaskDetail:
		return true
	default:
		return false
	}
}

func (m *AppModel) handleProjectArchiveConfirmKey(key string) (bool, tea.Cmd) {
	if !m.projectArchiveConfirm.IsActive() {
		return false, nil
	}

	confirmed, handled := m.projectArchiveConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		return true, func() tea.Msg { return projectArchiveConfirmedMsg{} }
	}
	switch key {
	case "n", "N", "esc":
		m.clearPendingProjectArchive()
	}
	return true, nil
}

func (m *AppModel) handleActiveKeyOverlay(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.destinationSwitcher != nil {
		return true, m.handleDestinationSwitcherKey(msg)
	}
	if m.showQuickSwitch {
		_, cmd := m.handleQuickSwitchKey(msg)
		return true, cmd
	}
	if m.showPinnedPopup {
		_, cmd := m.handlePinnedPopupKey(msg)
		return true, cmd
	}
	if m.quickNoteOverlay != nil {
		dismiss, cmd := dispatchOverlayKey(m.quickNoteOverlay, msg)
		if dismiss {
			m.quickNoteOverlay = nil
		}
		return true, cmd
	}
	if m.globalProjectPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.globalProjectPicker, msg)
		if dismiss {
			m.globalProjectPicker = nil
		}
		return true, cmd
	}
	if m.globalTaskPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.globalTaskPicker, msg)
		if dismiss {
			m.globalTaskPicker = nil
		}
		return true, cmd
	}
	if m.statusQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.statusQuickPicker, msg)
		if dismiss {
			m.statusQuickPicker = nil
		}
		return true, cmd
	}
	if m.sectionQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.sectionQuickPicker, msg)
		if dismiss {
			m.sectionQuickPicker = nil
		}
		return true, cmd
	}
	if m.linkTypeQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.linkTypeQuickPicker, msg)
		if dismiss {
			m.linkTypeQuickPicker = nil
		}
		return true, cmd
	}
	if m.linkModeQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.linkModeQuickPicker, msg)
		if dismiss {
			m.linkModeQuickPicker = nil
		}
		return true, cmd
	}
	if m.priorityQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.priorityQuickPicker, msg)
		if dismiss {
			m.priorityQuickPicker = nil
		}
		return true, cmd
	}
	if m.dueDateQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.dueDateQuickPicker, msg)
		if dismiss {
			m.dueDateQuickPicker = nil
		}
		return true, cmd
	}
	if m.sectionChangeQuickPicker != nil {
		dismiss, cmd := dispatchOverlayKey(m.sectionChangeQuickPicker, msg)
		if dismiss {
			m.sectionChangeQuickPicker = nil
		}
		return true, cmd
	}

	return false, nil
}
