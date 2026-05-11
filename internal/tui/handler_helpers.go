package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
)

// refreshAllViews refreshes the project tree and rebuilds outliner rows.
// Use this when task data has changed and both views need updating.
func (m *AppModel) refreshAllViews() {
	m.refreshProjectTree()
	if m.outliner != nil {
		m.outliner.rebuildRows()
	}
}

// handleWithRefresh consolidates the common handler pattern of:
// pop view → reload project → refresh tree → set status message.
//
// Parameters:
//   - popView: whether to pop the current view from the stack
//   - reload: whether to reload the current project from storage
//   - statusMsg: optional status message to display (empty string skips)
//
// Returns a tea.Cmd for status timeout if statusMsg is provided, nil otherwise.
func (m *AppModel) handleWithRefresh(popView, reload bool, statusMsg string) tea.Cmd {
	if popView {
		m.popView()
	}
	if reload && m.currentProject != nil {
		_ = m.persister.Load(m.currentProject)
		m.invalidateFilterCaches()
	}
	m.refreshAllViews()
	if statusMsg != "" {
		m.setSuccessStatus(statusMsg)
		return defaultStatusTimeout()
	}
	return nil
}

// setSuccessStatus sets a status message with success styling (green checkmark).
func (m *AppModel) setSuccessStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = false
	m.messageLog.Append(msg, MessageLogLevelSuccess)
}

// setInfoStatus sets a status message with informational styling (neutral color, no checkmark).
func (m *AppModel) setInfoStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = true
	m.messageLog.Append(msg, MessageLogLevelInfo)
}

// setUndoableInfoStatus sets an info status message with an appended "(u to undo)" hint.
func (m *AppModel) setUndoableInfoStatus(msg string) {
	m.setInfoStatus(msg + " (u to undo)")
}

// clearStatus clears the status message and resets the info flag.
func (m *AppModel) clearStatus() {
	m.statusMessage = ""
	m.statusIsInfo = false
	m.err = nil
}

// invalidateFilterCaches clears data-dependent filter caches after a project
// reload. Only clears priorityViewFilterCache and allTasksFilterCache, which
// may hold filter criteria that reference tags or sections removed by the
// reload. Spatial and toggle state (UX state) is intentionally preserved.
func (m *AppModel) invalidateFilterCaches() {
	m.priorityViewFilterCache = make(map[string]*PriorityViewCache)
	m.allTasksFilterCache = nil
}

// pruneDeletedTaskRefs removes tasks that no longer exist in any active project
// from the in-memory focusQueue, dailyPlan, and recentTaskVisits.
// It then persists the cleaned lists to config. Call this after task deletion
// or archiving to prevent stale entries from being re-persisted.
func (m *AppModel) pruneDeletedTaskRefs() {
	// Build a set of all valid task IDs across all projects.
	// Projects not yet loaded from disk have nil Tasks; load them
	// temporarily so their task IDs are not incorrectly pruned.
	validTasks := make(map[string]bool)
	for _, p := range m.config.GetAllLoadableProjects() {
		wasUnloaded := p.Tasks == nil
		if wasUnloaded {
			if err := m.persister.Load(p); err != nil {
				logging.Warn("pruneDeletedTaskRefs: failed to load project %s: %v", p.Name, err)
				continue
			}
		}
		for _, t := range p.Tasks {
			validTasks[t.ID] = true
		}
		if wasUnloaded {
			p.Tasks = nil
		}
	}

	dirty := false

	// Prune focus queue
	n := 0
	for _, ft := range m.focusQueue {
		if validTasks[ft.Task.ID] {
			m.focusQueue[n] = ft
			n++
		} else {
			delete(m.focusQueueIDs, ft.Task.ID)
			dirty = true
		}
	}
	m.focusQueue = m.focusQueue[:n]

	// Prune daily plan
	n = 0
	for _, ft := range m.dailyPlan {
		if validTasks[ft.Task.ID] {
			m.dailyPlan[n] = ft
			n++
		} else {
			delete(m.dailyPlanIDs, ft.Task.ID)
			dirty = true
		}
	}
	m.dailyPlan = m.dailyPlan[:n]

	// Prune recent task visits
	n = 0
	for _, rv := range m.recentTaskVisits {
		if validTasks[rv.Task.ID] {
			m.recentTaskVisits[n] = rv
			n++
		} else {
			dirty = true
		}
	}
	m.recentTaskVisits = m.recentTaskVisits[:n]

	if dirty {
		m.persistFocusQueue()
		m.persistDailyPlan()
		m.persistRecentTaskVisits()
	}
}

// saveConfigWarn attempts to save the config and logs a warning on failure.
// Use this in TUI code where a config save failure is non-fatal.
func saveConfigWarn(mgr *config.Manager, cfg *config.Config, context string) {
	if err := mgr.Save(cfg); err != nil {
		logging.Warn("failed to save config (%s): %v", context, err)
	}
}

// syncInboxProcessor updates the typed inboxProcessor pointer after view updates.
// This keeps the pointer in sync when the inbox processor view is active.
func (m *AppModel) syncInboxProcessor() {
	if m.currentView() == ViewInboxProcessor {
		if v := m.views.Get(ViewInboxProcessor); v != nil {
			m.inboxProcessor = v.(*InboxProcessorModel)
		}
	}
}
