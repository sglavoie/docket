package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// activeCrossProjectOutliner returns the OutlinerModel currently displaying
// cross-project rows, preferring m.crossOutliner (the dedicated instance for
// ViewCrossProjectOutliner) and falling back to m.outliner only when it is
// itself in cross-project mode (project == nil). Returns nil when no
// cross-project outliner is active.
func (m *AppModel) activeCrossProjectOutliner() *OutlinerModel {
	if m.crossOutliner != nil {
		return m.crossOutliner
	}
	if m.outliner != nil && m.outliner.project == nil {
		return m.outliner
	}
	return nil
}

// registeredProjectOutliners returns every distinct single-project outliner
// owned by the app. Temporary drill-down outliners live only in the view
// registry, so consulting m.outliner alone leaves their project copy and rows
// stale after mutations committed through the application layer.
func (m *AppModel) registeredProjectOutliners() []*OutlinerModel {
	if m == nil {
		return nil
	}

	candidates := []*OutlinerModel{m.outliner}
	if m.views != nil {
		if temporary, ok := m.views.Get(ViewTemporaryOutliner).(*OutlinerModel); ok {
			candidates = append(candidates, temporary)
		}
	}

	outliners := make([]*OutlinerModel, 0, len(candidates))
	seen := make(map[*OutlinerModel]struct{}, len(candidates))
	for _, outliner := range candidates {
		if outliner == nil || outliner.project == nil {
			continue
		}
		if _, exists := seen[outliner]; exists {
			continue
		}
		seen[outliner] = struct{}{}
		outliners = append(outliners, outliner)
	}
	return outliners
}

// reloadRegisteredProjectOutliners reloads the project object owned by each
// single-project outliner. Project pointers, rather than project IDs, are
// deduplicated because a temporary drill-down intentionally owns a different
// project object from the app's canonical project cache.
func (m *AppModel) reloadRegisteredProjectOutliners(
	loader func(*models.Project) error,
	alreadyLoaded ...*models.Project,
) {
	if loader == nil {
		return
	}

	loaded := make(map[*models.Project]struct{}, len(alreadyLoaded))
	for _, project := range alreadyLoaded {
		if project != nil {
			loaded[project] = struct{}{}
		}
	}
	for _, outliner := range m.registeredProjectOutliners() {
		project := outliner.project
		if _, exists := loaded[project]; exists {
			continue
		}
		loaded[project] = struct{}{}
		if err := loader(project); err != nil {
			logging.Warn("failed to load project %s: %v", project.Name, err)
		}
	}
}

func (m *AppModel) rebuildRegisteredProjectOutliners() {
	for _, outliner := range m.registeredProjectOutliners() {
		outliner.rebuildRows()
		outliner.clampCursor()
	}
}

// refreshAllViews rebuilds in-memory rows for the project tree and any active
// outliner (single-project and cross-project). It never reloads from disk;
// disk-backed reloads belong in refreshTaskViewsAfterSave and handleWithRefresh.
func (m *AppModel) refreshAllViews() {
	m.refreshProjectView()
	m.rebuildRegisteredProjectOutliners()
	if cross := m.activeCrossProjectOutliner(); cross != nil {
		cross.rebuildRows()
		cross.clampCursor()
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
	var popCmd tea.Cmd
	if popView {
		popCmd = m.popView()
	}
	if reload {
		var currentProject *models.Project
		if m.currentProject != nil {
			currentProject = m.currentProject
			_ = m.persister.Load(currentProject)
		}
		m.reloadRegisteredProjectOutliners(m.persister.Load, currentProject)
		m.invalidateFilterCaches()
	}
	// Cross-project outliner has its own loadedProjects cache that bypasses
	// m.currentProject, so a disk-backed reload is needed for mutations to
	// any loaded project to become visible without leaving the view. The
	// cursor is saved before reload and restored after refreshAllViews
	// (which would otherwise clamp it to 0 while loadedProjects is briefly
	// nil); the final clamp to the rebuilt row count happens later in
	// handleCrossProjectsLoaded via crossProjectRebuildRows.
	var crossReloadCmd tea.Cmd
	var crossSavedCursor int
	crossInstance := m.activeCrossProjectOutliner()
	crossReload := reload && crossInstance != nil
	if crossReload {
		crossSavedCursor = crossInstance.cursor
		crossReloadCmd = crossInstance.reloadCrossProjects()
	}
	m.refreshAllViews()
	m.refreshLoadedTaskWorkspace()
	if crossReload {
		crossInstance.cursor = crossSavedCursor
	}
	if statusMsg != "" {
		m.setSuccessStatus(statusMsg)
		return tea.Batch(popCmd, crossReloadCmd, defaultStatusTimeout())
	}
	if crossReloadCmd != nil {
		return tea.Batch(popCmd, crossReloadCmd)
	}
	return popCmd
}

// setSuccessStatus sets a status message with success styling (green checkmark).
// A non-undoable status replaces any pending undo toast: leaving statusUndoCb
// armed would let a later `u` keypress silently invoke the previous
// destructive action's undo while a different (non-undoable) message is on
// screen.
func (m *AppModel) setSuccessStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = false
	m.statusUpdatedAt = time.Now()
	m.statusUndoCb = nil
	m.statusExpiresAt = time.Time{}
	m.messageLog.Append(msg, MessageLogLevelSuccess)
	m.openLoops = ComputeOpenLoops(m.runtime, m.dailyPlanIDs)
}

// setInfoStatus sets a status message with informational styling (neutral color, no checkmark).
// Callers that want the message to be undoable must go through
// setUndoableStatus, which re-installs the callback after this clears it.
func (m *AppModel) setInfoStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = true
	m.statusUpdatedAt = time.Now()
	m.statusUndoCb = nil
	m.statusExpiresAt = time.Time{}
	m.messageLog.Append(msg, MessageLogLevelInfo)
}

// setWarningStatus sets a non-fatal warning while preserving the committed
// operation's success semantics. Warnings use the neutral status surface and
// retain their own severity in the message log.
func (m *AppModel) setWarningStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = true
	m.statusUpdatedAt = time.Now()
	m.statusUndoCb = nil
	m.statusExpiresAt = time.Time{}
	m.messageLog.Append(msg, MessageLogLevelWarning)
}

// setBackgroundErrorStatus reports a background failure through the transient
// status and message-log surfaces without replacing m.err, which may belong to
// an unrelated operation that is still in flight.
func (m *AppModel) setBackgroundErrorStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = true
	m.statusUpdatedAt = time.Now()
	m.statusUndoCb = nil
	m.statusExpiresAt = time.Time{}
	m.messageLog.Append(msg, MessageLogLevelError)
}

// UndoToastTimeout is how long a transient undo-available toast remains
// dismissible by the inline `u` key before it auto-clears.
const UndoToastTimeout = 6 * time.Second

// setUndoableStatus sets an info-styled status message that the user can
// undo via the inline `u` key while the toast is still on screen. The
// undoCb is invoked when `u` is pressed within UndoToastTimeout; once the
// timeout elapses (or the toast is replaced), the callback is dropped.
// Returns a tea.Cmd that clears the toast after the timeout.
//
// A nil undoCb is treated as a programming error and degrades to a plain
// info status with the default 2s timeout — otherwise the "u to undo"
// affordance on screen would be a lie (the `u` key handler short-circuits
// on a nil callback) and the 6s window would mis-suppress an unrelated
// StatusClearMsg that arrives in the meantime.
func (m *AppModel) setUndoableStatus(msg string, undoCb func() tea.Cmd) tea.Cmd {
	if undoCb == nil {
		logging.Warn("setUndoableStatus called with nil undoCb; degrading to plain info status: %q", msg)
		m.setInfoStatus(msg)
		return defaultStatusTimeout()
	}
	m.setInfoStatus(msg)
	m.statusUndoCb = undoCb
	m.statusExpiresAt = time.Now().Add(UndoToastTimeout)
	return tea.Tick(UndoToastTimeout, func(time.Time) tea.Msg {
		return UndoToastTimeoutMsg{}
	})
}

// formatUndoToast renders the canonical "<action> '<context>' · u to undo"
// toast string. The context is omitted if empty, producing "<action> · u to undo".
func formatUndoToast(action, context string) string {
	if context == "" {
		return fmt.Sprintf("%s · u to undo", action)
	}
	return fmt.Sprintf("%s '%s' · u to undo", action, context)
}

// formatSuccessWithNext renders "<action> — <nextHint>" for status messages that guide the
// user to their next step. Returns action unchanged when nextHint is empty.
func formatSuccessWithNext(action, nextHint string) string {
	if nextHint == "" {
		return action
	}
	return action + " — " + nextHint
}

// clearStatus clears the status message and resets the info flag.
func (m *AppModel) clearStatus() {
	m.statusMessage = ""
	m.statusIsInfo = false
	m.statusUpdatedAt = time.Time{}
	m.statusUndoCb = nil
	m.statusExpiresAt = time.Time{}
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
	for _, p := range m.allLoadableProjects() {
		wasUnloaded := p.Tasks == nil
		if wasUnloaded {
			if err := m.persister.Load(p); err != nil {
				logging.Warn("pruneDeletedTaskRefs: failed to load project %s: %v", p.Name, err)
				// A transient load failure must not be treated as
				// "project has zero tasks" — that would prune every
				// focusQueue / dailyPlan / recentTaskVisits entry
				// belonging to this project. Abort the prune entirely
				// so stale-but-recoverable references survive until the
				// next successful load.
				return
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

// refreshRuntime advances the shell's domain snapshot to the state a just-landed
// config-bearing commit (or config save) wrote, so views reading the snapshot's
// narrow accessors observe the committed domains without a render-time disk load.
// It is the sole runtime-state refresh path and is deliberately tolerant of a
// reload failure: a stale-but-usable snapshot is preferable to failing an already
// committed mutation. It reports whether the snapshot advanced so callers that
// derive caches from it can avoid rebuilding from known-stale state. A nil
// snapshot (never loaded) is a no-op.
func (m *AppModel) refreshRuntime() bool {
	if m.runtime == nil || m.runtime.DataDir() == "" {
		return false
	}
	if err := m.runtime.RefreshAfterCommit(); err != nil {
		logging.Warn("failed to refresh runtime domain snapshot: %v", err)
		return false
	}
	m.syncProjectCache()
	return true
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
