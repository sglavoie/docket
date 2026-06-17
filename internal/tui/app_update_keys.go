package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// currentViewAsModel returns the current view as a tea.Model for interface assertions.
func (m *AppModel) currentViewAsModel() tea.Model {
	return m.views.Get(m.currentView())
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

func (m *AppModel) handleGlobalReferenceOpen() (bool, tea.Cmd) {
	if _, ok := m.currentViewAsModel().(taskReferenceProvider); !ok {
		return false, nil
	}

	task, project := m.getCurrentCursorTaskAndProject()
	if task == nil {
		m.setInfoStatus("No task selected for reference open")
		return true, defaultStatusTimeout()
	}

	return true, m.openTaskReferenceSnapshot(task, project)
}

func (m *AppModel) handleGlobalNotesOpen() (bool, tea.Cmd) {
	if _, ok := m.currentViewAsModel().(taskReferenceProvider); !ok {
		return false, nil
	}

	task, project := m.getCurrentCursorTaskAndProject()
	if task == nil || project == nil {
		m.setInfoStatus("No task selected for Notes")
		return true, defaultStatusTimeout()
	}

	return true, func() tea.Msg { return ShowNotesMsg{Project: project} }
}

func (m *AppModel) openTaskReferenceSnapshot(task *models.Task, project *models.Project) tea.Cmd {
	linkedNotes, err := linkedNotesForReferenceTask(task, project)
	if err != nil {
		return errorCmd("failed to load linked notes: %w", err)
	}

	content := external.FormatTaskReferenceMarkdownWithNotes(task, linkedNotes)
	path, err := createReferenceSnapshotFile(content, "tt-task-reference-", ".md")
	if err != nil {
		return errorCmd("failed to create reference snapshot: %w", err)
	}

	externalEditor := ""
	if m.config != nil {
		externalEditor = m.config.GetExternalEditor()
	}
	result, err := openReferenceFile(externalEditor, path)
	if err != nil {
		if result.Status == external.ReferenceOpenStatusUnsupportedEditor {
			m.setInfoStatus(err.Error())
			return defaultStatusTimeout()
		}
		return errorCmd("%s", err)
	}

	m.setSuccessStatus("Opened task reference copy")
	return defaultStatusTimeout()
}

func linkedNotesForReferenceTask(task *models.Task, project *models.Project) ([]*models.Note, error) {
	if task == nil || project == nil {
		return nil, nil
	}

	notes, err := storage.ListNotes(project)
	if err != nil {
		return nil, err
	}

	linked := make([]*models.Note, 0)
	for _, note := range notes {
		if note != nil && note.HasLink(task.ID) {
			linked = append(linked, note)
		}
	}
	return linked, nil
}

func (m *AppModel) handleGlobalKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) { //nolint:gocyclo
	if isCtrlEscKey(msg) {
		return true, m.jumpToDashboard()
	}

	// Handle pending sandbox reset confirmation before clearing status
	if m.pendingSandboxReset {
		switch msg.String() {
		case "y", "Y":
			return true, m.executeSandboxReset()
		case "n", "N", "esc":
			m.pendingSandboxReset = false
			m.pendingSandboxTaskCount = 0
			m.setInfoStatus("Canceled")
			return true, defaultStatusTimeout()
		default:
			// Keep showing the confirmation prompt
			return true, nil
		}
	}

	if handled, cmd := m.handleProjectArchiveConfirmKey(msg.String()); handled {
		return true, cmd
	}

	if handled, cmd := m.handleActiveKeyOverlay(msg); handled {
		return true, cmd
	}

	if handled, cmd := m.handlePendingPrefixes(msg); handled {
		return true, cmd
	}

	// Intercept keys when peek overlay is active: handle scroll, pane focus,
	// and dismiss; block everything else so the underlying view is frozen.
	if m.peekActive {
		switch msg.String() {
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
	if m.currentView() != ViewDashboard && m.validateCurrentProject() {
		return true, defaultStatusTimeout()
	}

	switch msg.String() {
	case "ctrl+c":
		m.saveProjectTreeState()
		m.saveOutlinerFilterState()
		m.saveCrossOutlinerState()
		m.quitting = true
		return true, tea.Quit
	case "alt+o":
		return true, m.navigateHistoryBack()
	case "alt+i":
		return true, m.navigateHistoryForward()
	case "ctrl+6":
		if !m.hasActiveTextInput() && m.lastNavEntry.ViewType != 0 {
			currentEntry := NavEntry{ViewType: m.currentView()}
			if m.currentProject != nil && !isGlobalNavView(m.currentView()) {
				currentEntry.ProjectID = m.currentProject.ID
			}
			if isTaskScopedView(m.currentView()) {
				currentEntry.TaskID = m.lastNavigatedTaskID
			}
			targetEntry := m.lastNavEntry
			m.lastNavEntry = currentEntry
			return true, m.navigateToEntry(targetEntry)
		}
	case "alt+l":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowMessageLogMsg{} }
		}
	case "|":
		if !m.hasActiveTextInput() {
			return true, m.handleQuickSwitchToggle()
		}
	case "#":
		if !m.hasActiveTextInput() && !m.hasActivePendingPrefix() {
			return true, m.handlePinnedPopupToggle()
		}
	case "esc":
		if handled, cmd := m.handleEscKey(); handled {
			return true, cmd
		}
	case "q":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() {
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
		if cmd := m.toggleDailyPlan(); cmd != nil {
			return true, m.batchWithFocusRefresh(cmd)
		}
	case "u":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
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
	case "ctrl+r":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
			break
		}
		if cmd := m.handleRedoToggle(); cmd != nil {
			return true, cmd
		}
	case "ctrl+z":
		if !m.hasActiveTextInput() && m.currentProject != nil && m.currentView() != ViewUndoHistory {
			project := m.currentProject
			return true, func() tea.Msg { return ShowUndoHistoryMsg{Project: project} }
		}
	case "ctrl+x":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowProjectPickerMsg{} }
		}
	case ":":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowCommandPaletteMsg{} }
		}
	case ";":
		if !m.hasActiveTextInput() && !m.hasActivePendingPrefix() {
			m.pendingSemicolonPrefix = true
			timeout := m.config.GetPopupTimeoutSeconds()
			if timeout <= 0 {
				return true, nil
			}
			return true, func() tea.Msg {
				time.Sleep(time.Duration(timeout) * time.Second)
				return ClearSemicolonPrefixMsg{}
			}
		}
	case "ctrl+,":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return SettingsMsg{} }
		}
	case "ctrl+i":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowGlobalInboxProcessorMsg{} }
		}
	case "ctrl+/":
		if !m.hasActiveTextInput() {
			return true, func() tea.Msg { return ShowGlobalSearchMsg{} }
		}
	case "ctrl+o":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() || m.hasActiveMultiSelect() {
			break
		}
		if handled, cmd := m.handleGlobalReferenceOpen(); handled {
			return true, cmd
		}
	case "ctrl+n":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() || m.hasActiveMultiSelect() {
			break
		}
		if handled, cmd := m.handleGlobalNotesOpen(); handled {
			return true, cmd
		}
	case `ctrl+\`:
		if !m.hasActiveTextInput() {
			if m.currentView() == ViewKeybindingExplorer {
				m.popView()
				return true, nil
			}
			return true, func() tea.Msg { return ShowKeybindingExplorerMsg{} }
		}
	case "y", "Y", "ctrl+y":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() || m.hasActiveConfirmDialog() {
			break
		}
		if handled, cmd := m.handleGlobalClipboard(msg.String()); handled {
			return true, cmd
		}
	case "space":
		if m.hasActiveTextInput() || m.hasActivePendingPrefix() {
			break
		}
		if m.hasActiveMultiSelect() {
			return false, nil
		}
		if m.currentView() == ViewTaskDetail {
			return false, nil
		}
		task, project := m.getCurrentCursorTaskAndProject()
		if task == nil {
			return false, nil
		}
		m.activatePeek(task, project)
		return true, nil
	case ".":
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
	if m.showQuickSwitch {
		_, cmd := m.handleQuickSwitchKey(msg)
		return true, cmd
	}
	if m.showPinnedPopup {
		_, cmd := m.handlePinnedPopupKey(msg)
		return true, cmd
	}
	if m.showProjectCyclePicker {
		_, cmd := m.handleProjectCyclePickerKey(msg)
		return true, cmd
	}
	if m.showProjectCycle {
		_, cmd := m.handleProjectCycleKey(msg)
		return true, cmd
	}
	if m.quickNoteOverlay != nil {
		dismiss, cmd := dispatchOverlayKey(m.quickNoteOverlay, msg)
		if dismiss {
			m.quickNoteOverlay = nil
		}
		return true, cmd
	}
	if m.quickCaptureOverlay != nil {
		newModel, cmd := m.quickCaptureOverlay.Update(msg)
		if qc, ok := newModel.(*QuickCaptureModel); ok {
			m.quickCaptureOverlay = qc
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

func (m *AppModel) handlePendingPrefixes(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.pendingSemicolonTPrefix {
		return m.handlePendingSemicolonTPrefix(msg)
	}

	if m.pendingSemicolonPrefix {
		return m.handlePendingSemicolonPrefix(msg)
	}
	return false, nil
}

func (m *AppModel) handlePendingSemicolonTPrefix(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if isModifierOnly(msg) {
		return true, nil
	}
	m.pendingSemicolonTPrefix = false
	switch msg.String() {
	case "m":
		return true, showGlobalProjectPickerCmd(PickerPurposeTaskMoveSource)
	case "N":
		return true, showGlobalProjectPickerCmd(PickerPurposeTemplatePick)
	case "d":
		return true, showGlobalProjectPickerCmd(PickerPurposeDueDate)
	case "s":
		return true, showGlobalProjectPickerCmd(PickerPurposeSectionChange)
	default:
		return true, nil
	}
}

func (m *AppModel) handlePendingSemicolonPrefix(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if isModifierOnly(msg) {
		return true, nil
	}
	m.pendingSemicolonPrefix = false
	switch msg.String() {
	case ";":
		return true, func() tea.Msg { return ShowPipelineHubMsg{} }
	case "n":
		return true, func() tea.Msg { return ShowNotificationCenterMsg{} }
	case "d":
		return true, func() tea.Msg { return ShowDailyReviewMsg{} }
	case "r":
		return true, func() tea.Msg { return ShowWeeklyReviewMsg{} }
	case "N":
		return true, func() tea.Msg { return ShowGlobalNotesMsg{} }
	case "q":
		return true, func() tea.Msg { return ShowFocusViewMsg{} }
	case "w":
		return true, func() tea.Msg { return ShowFocusBoardMsg{} }
	case "l":
		return true, func() tea.Msg { return ShowRecentActivityMsg{} }
	case "B":
		return true, func() tea.Msg { return ShowGlobalBlockersMsg{} }
	case "D":
		return true, func() tea.Msg { return ShowDueDateOverviewMsg{} }
	case "m":
		return true, func() tea.Msg { return ShowMetricsDashboardMsg{} }
	case "a":
		return true, func() tea.Msg { return ShowAllTasksMsg{} }
	case "W":
		return true, func() tea.Msg { return ShowWorkflowRulesMsg{} }
	case "k":
		return true, func() tea.Msg { return ShowCrossProjectKanbanMsg{} }
	case "o":
		return true, func() tea.Msg { return ShowGlobalOutlinerMsg{} }
	case "R":
		return true, func() tea.Msg { return ShowRoadmapMsg{} }
	case "i":
		return true, m.resolveQuickCaptureCmd()
	case "I":
		return true, showGlobalProjectPickerCmd(PickerPurposeBulkCapture)
	case "{":
		return true, showGlobalProjectPickerCmd(PickerPurposeNoteCreate)
	case ",":
		return true, showGlobalProjectPickerCmd(PickerPurposeStatusUpdate)
	case "c":
		return true, showGlobalProjectPickerCmd(PickerPurposeLinkedTask)
	case "t":
		m.pendingSemicolonTPrefix = true
		return true, nil
	case "!":
		return true, showGlobalProjectPickerCmd(PickerPurposePriorityUpdate)
	default:
		return true, nil
	}
}

func showGlobalProjectPickerCmd(purpose PickerPurpose) tea.Cmd {
	return func() tea.Msg {
		return ShowGlobalProjectPickerMsg{Purpose: purpose}
	}
}
