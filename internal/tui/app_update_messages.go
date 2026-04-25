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
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleGlobalKeyPress(msg)
	case tea.KeyboardEnhancementsMsg:
		m.keyboardEnhancements = msg
		return true, upgradeKittyKeyboard()
	case tea.KeyReleaseMsg:
		if msg.String() == "shift+space" && m.peekActive {
			m.peekTask = nil
			m.peekActive = false
		}
		return true, nil
	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
		return true, nil
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
		if !(m.pendingSemicolonPrefix || m.pendingSemicolonTPrefix) {
			return true, nil
		}
		m.pendingSemicolonPrefix = false
		m.pendingSemicolonTPrefix = false
		m.setInfoStatus("prefix cleared")
		return true, defaultStatusTimeout()
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
	case NotesRefreshMsg:
		m.refreshNotesSearchState()
		return true, nil
	}

	return false, nil
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
	m.setSuccessStatus(msg.SuccessMessage)
	m.invalidateFilterCaches()
	m.refreshProjectTree()
	if m.allTasksView != nil {
		m.allTasksView.loadAllTasks()
	}
	if m.outliner != nil {
		if err := storage.LoadProject(m.outliner.project); err != nil {
			logging.Warn("failed to load project %s: %v", m.outliner.project.Name, err)
		}
		m.outliner.rebuildRows()
	}
	var saveCmds []tea.Cmd
	saveCmds = append(saveCmds, defaultStatusTimeout())
	if m.crossOutliner != nil {
		saveCmds = append(saveCmds, m.crossOutliner.reloadCrossProjects())
	}
	if v := m.views.Get(ViewFocusView); v != nil {
		if fv, ok := v.(*FocusViewModel); ok {
			fv.loadTasks()
		}
	}
	if v := m.views.Get(ViewFocusBoard); v != nil {
		if fb, ok := v.(*FocusBoardModel); ok {
			fb.loadItems()
			fb.clampCursor()
		}
	}
	if v := m.views.Get(ViewRecentActivity); v != nil {
		if ra, ok := v.(*RecentActivityModel); ok {
			ra.loadTasks()
		}
	}
	if m.globalInboxProcessor != nil {
		m.globalInboxProcessor.loadAllInboxTasks()
	}
	for _, vt := range []ViewType{ViewKanban, ViewCrossProjectKanban} {
		if v := m.views.Get(vt); v != nil {
			if kb, ok := v.(*KanbanModel); ok {
				kb.reloadData()
				kb.clampCursor()
			}
		}
	}
	return tea.Batch(saveCmds...)
}

func (m *AppModel) refreshNotesSearchState() {
	if m.globalSearch != nil {
		m.globalSearch.Rebuild()
	}
	if v := m.views.Get(ViewNotesSearch); v != nil {
		if ns, ok := v.(*NotesSearchModel); ok {
			ns.MarkStale()
		}
	}
}
