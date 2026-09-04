package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// activeCrossProjectOutliner returns the sole Outliner model.
func (m *AppModel) activeCrossProjectOutliner() *outliner.OutlinerModel {
	return m.crossOutliner
}

// requestCrossProjectOutlinerReload starts a disk-backed reload only when its
// completion message can be routed to the cross-project Outliner. Full-screen
// child views defer the reload until popView emits ViewDidAppearMsg after the
// Outliner becomes active again.
func (m *AppModel) requestCrossProjectOutlinerReload() tea.Cmd {
	cross := m.crossOutliner
	if cross == nil {
		return nil
	}
	if !m.activeOutlinerRoute() {
		return nil
	}
	return cross.ReloadCrossProjects()
}

// reloadCrossProjectOutlinerProject refreshes one project object already owned
// by the sole cross-project Outliner.
func (m *AppModel) reloadCrossProjectOutlinerProject(projectID string) {
	if projectID == "" || m.crossOutliner == nil || m.persister == nil {
		return
	}
	for _, project := range m.crossOutliner.Population.Projects {
		if project == nil || project.ID != projectID {
			continue
		}
		if err := m.persister.Load(project); err != nil {
			logging.Warn("failed to reload cross-project copy %s: %v", project.Name, err)
			return
		}
		if m.runtime != nil {
			coordinator.CopyCatalogMetadata(project, m.runtime.GetProject(projectID))
		}
		m.crossOutliner.Population.Invalidate()
		m.crossOutliner.CrossProjectRebuildRows()
		m.crossOutliner.ClampCursor()
		return
	}
}

// refreshAllViews rebuilds in-memory rows for the project tree and Outliner.
// It never reloads from disk;
// disk-backed reloads belong in refreshTaskViewsAfterSave and handleWithRefresh.
func (m *AppModel) refreshAllViews() {
	m.refreshProjectView()
	if cross := m.activeCrossProjectOutliner(); cross != nil {
		outliner.RebuildOutlinerPreservingCursor(cross)
	}
}

// handleWithRefresh consolidates the common handler pattern of:
// pop view → reload project → refresh tree → set status message.
//
// Parameters:
//   - popView: whether to pop the current view from the stack
//   - reload: whether to reload the current project from storage
func (m *AppModel) handleWithRefresh(popView, reload bool) tea.Cmd {
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
	}
	// Cross-project outliner has its own population cache that bypasses
	// m.currentProject, so a disk-backed reload is needed for mutations to
	// any loaded project to become visible without leaving the view. The
	// cursor is saved before reload and restored after refreshAllViews
	// (which would otherwise clamp it to 0 while the population is briefly
	// nil); the final clamp to the rebuilt row count happens later in
	// handleCrossProjectsLoaded via crossProjectRebuildRows.
	var crossReloadCmd tea.Cmd
	var crossSavedCursor int
	crossInstance := m.crossOutliner
	crossReload := reload && crossInstance != nil
	if crossReload {
		crossSavedCursor = crossInstance.Cursor
		crossReloadCmd = m.requestCrossProjectOutlinerReload()
	}
	m.refreshAllViews()
	m.refreshLoadedTaskWorkspace()
	if crossReloadCmd != nil {
		crossInstance.Cursor = crossSavedCursor
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
	m.statusTransactionID = ""
	m.statusRecoveryFallback = ""
	m.recordStatusSeverity(flashSuccess)
	m.refreshWorkspacePulseAt(time.Now())
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
	m.statusTransactionID = ""
	m.statusRecoveryFallback = ""
	m.recordStatusSeverity(flashInfo)
}

// setWarningStatus sets a non-fatal warning while preserving the committed
// operation's success semantics. Warnings use the neutral status surface.
func (m *AppModel) setWarningStatus(msg string) {
	m.statusMessage = msg
	m.statusIsInfo = true
	m.statusUpdatedAt = time.Now()
	m.statusUndoCb = nil
	m.statusExpiresAt = time.Time{}
	m.statusTransactionID = ""
	m.statusRecoveryFallback = ""
	// Severity is a separate axis from statusIsInfo: the text keeps the neutral
	// surface while the divider flash still distinguishes a warning.
	m.recordStatusSeverity(flashWarning)
}

// recordStatusSeverity advances the status generation the footer flash arms on.
// Only the setters call it, so a cleared status never flashes.
func (m *AppModel) recordStatusSeverity(severity footerFlashSeverity) {
	m.statusSeverity = severity
	m.statusSeq++
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
		return status.DefaultStatusTimeout()
	}
	m.setInfoStatus(msg)
	m.statusUndoCb = undoCb
	m.statusExpiresAt = time.Now().Add(UndoToastTimeout)
	return status.TickCmd(UndoToastTimeout, func(time.Time) tea.Msg {
		return status.UndoToastTimeoutMsg{}
	})
}

// setRecoveryFeedback installs the canonical recovery feedback only when the
// just-committed transaction is still the newest global ledger entry. Missing,
// unreadable, or superseded correlation degrades to the ordinary two-second
// success status without leaving an undo callback armed.
func (m *AppModel) setRecoveryFeedback(successMessage, transactionID string, originRecorded bool) tea.Cmd {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		m.setSuccessStatus(successMessage)
		return status.DefaultStatusTimeout()
	}
	entry, ok, err := workspaceLedgerFromConfig(m.runtime).SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || entry.TransactionID != transactionID {
		m.setSuccessStatus(successMessage)
		return status.DefaultStatusTimeout()
	}
	undoCb := func() tea.Cmd { return m.handleLedgerTaskUndoRedo(false) }
	cmd := m.setUndoableStatus(formatRecoveryFeedback(entry.Action, entry.Context, originRecorded), undoCb)
	m.statusTransactionID = transactionID
	m.statusRecoveryFallback = successMessage
	return cmd
}

// formatRecoveryFeedback renders the canonical recovery grammar. Context is
// omitted when absent; origin is advertised only after jump history records it.
func formatRecoveryFeedback(action, context string, originRecorded bool) string {
	message := strings.TrimSpace(action)
	if context = strings.TrimSpace(context); context != "" {
		message += ": " + context
	}
	message += " · u undo"
	if originRecorded {
		message += " · Ctrl+O origin"
	}
	return message
}

// appendRecoveryFeedbackDetail adds domain-specific consequence text while
// keeping recovery controls in their canonical trailing order. It is a no-op
// for ordinary fallback statuses because unverified feedback must not resemble
// a correlated recovery toast.
func (m *AppModel) appendRecoveryFeedbackDetail(detail string) {
	detail = strings.TrimSpace(detail)
	if detail == "" || m.statusUndoCb == nil {
		return
	}
	origin := strings.HasSuffix(m.statusMessage, " · Ctrl+O origin")
	m.statusMessage = strings.TrimSuffix(m.statusMessage, " · Ctrl+O origin")
	m.statusMessage = strings.TrimSuffix(m.statusMessage, " · u undo")
	m.statusMessage += " · " + detail + " · u undo"
	if origin {
		m.statusMessage += " · Ctrl+O origin"
	}
}

// clearStatus clears the status message and resets the info flag.
func (m *AppModel) clearStatus() {
	m.statusMessage = ""
	m.statusIsInfo = false
	m.statusUpdatedAt = time.Time{}
	m.statusUndoCb = nil
	m.statusExpiresAt = time.Time{}
	m.statusTransactionID = ""
	m.statusRecoveryFallback = ""
	m.statusSeverity = flashNone
	m.err = nil
}

// pruneDeletedTaskRefs removes tasks that no longer exist in any active project
// from the in-memory planning collections and reconciles durable planning.
func (m *AppModel) pruneDeletedTaskRefs() {
	// Build a set of all valid project-qualified task identities.
	// Projects not yet loaded from disk have nil Tasks; load them
	// temporarily so their task IDs are not incorrectly pruned.
	validTasks := make(map[taskref.TaskIdentity]bool)
	for _, p := range m.availableProjects() {
		wasUnloaded := p.Tasks == nil
		if wasUnloaded {
			if err := m.persister.Load(p); err != nil {
				logging.Warn("pruneDeletedTaskRefs: failed to load project %s: %v", p.Name, err)
				// A transient load failure must not be treated as
				// "project has zero tasks" — that would prune every
				// planning entry
				// belonging to this project. Abort the prune entirely
				// so stale-but-recoverable references survive until the
				// next successful load.
				return
			}
		}
		for _, t := range p.Tasks {
			validTasks[taskref.For(t, p)] = true
		}
		if wasUnloaded {
			p.Tasks = nil
		}
	}

	dirty := false

	// Prune focus queue
	n := 0
	for _, ft := range m.nextOrder {
		identity := taskref.For(ft.Task, ft.Project)
		if validTasks[identity] {
			m.nextOrder[n] = ft
			n++
		} else {
			delete(m.nextOrderIDs, identity)
			dirty = true
		}
	}
	m.nextOrder = m.nextOrder[:n]

	// Prune daily plan
	n = 0
	for _, ft := range m.today {
		identity := taskref.For(ft.Task, ft.Project)
		if validTasks[identity] {
			m.today[n] = ft
			n++
		} else {
			delete(m.todayIDs, identity)
			dirty = true
		}
	}
	m.today = m.today[:n]

	if dirty {
		if m.appService != nil {
			if changed, err := m.appService.ReconcilePlanning(); err != nil {
				logging.Warn("failed to reconcile deleted planning references: %v", err)
			} else if changed {
				m.refreshRuntime()
				m.rebuildPlanningFromRuntime()
			}
		}
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
