package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
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
	if projectID == "" || m.crossOutliner == nil || m.domain.persister == nil {
		return
	}
	for _, project := range m.crossOutliner.Population.Projects {
		if project == nil || project.ID != projectID {
			continue
		}
		if err := m.domain.persister.Load(project); err != nil {
			logging.Warn("failed to reload cross-project copy %s: %v", project.Name, err)
			return
		}
		if m.domain.runtime != nil {
			coordinator.CopyCatalogMetadata(project, m.domain.runtime.GetProject(projectID))
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
			_ = m.domain.persister.Load(currentProject)
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
// A non-undoable status replaces any pending undo toast: leaving the callback
// armed would let a later `u` keypress silently invoke the previous
// destructive action's undo while a different (non-undoable) message is on
// screen. announce is what releases it.
func (m *AppModel) setSuccessStatus(msg string) {
	m.feedback.announce(msg, false, time.Now())
	m.feedback.recordSeverity(flashSuccess)
	m.refreshWorkspacePulseAt(time.Now())
}

// setInfoStatus sets a status message with informational styling (neutral color, no checkmark).
// Callers that want the message to be undoable must go through
// setUndoableStatus, which re-installs the callback after this clears it.
func (m *AppModel) setInfoStatus(msg string) {
	m.feedback.announce(msg, true, time.Now())
	m.feedback.recordSeverity(flashInfo)
}

// setWarningStatus sets a non-fatal warning while preserving the committed
// operation's success semantics. Warnings use the neutral status surface.
func (m *AppModel) setWarningStatus(msg string) {
	m.feedback.announce(msg, true, time.Now())
	// Severity is a separate axis from the neutral-surface flag: the text keeps
	// the neutral surface while the divider flash still distinguishes a warning.
	m.feedback.recordSeverity(flashWarning)
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
	m.feedback.armUndo(undoCb, time.Now().Add(UndoToastTimeout))
	return status.TickCmd(UndoToastTimeout, func(time.Time) tea.Msg {
		return status.UndoToastTimeoutMsg{}
	})
}

// setRecoveryFeedback installs the canonical recovery feedback only when the
// just-committed transaction is still the newest global ledger entry. Missing,
// unreadable, or superseded correlation degrades to the ordinary two-second
// success status without leaving an undo callback armed.
func (m *AppModel) setRecoveryFeedback(successMessage, transactionID string, originRecorded bool) tea.Cmd {
	return m.setRecoveryFeedbackWithContext(successMessage, transactionID, "", originRecorded)
}

// setRecoveryFeedbackWithContext is that same toast with the committed outcome
// named in place of the ledger entry's stored context. Only a mutation whose
// ledger context describes the gesture rather than the result passes one, which
// today is the deadline stepper alone; an empty override keeps the ledger's own
// words, so every other caller reads exactly as before.
func (m *AppModel) setRecoveryFeedbackWithContext(
	successMessage, transactionID, contextOverride string,
	originRecorded bool,
) tea.Cmd {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		m.setSuccessStatus(successMessage)
		return status.DefaultStatusTimeout()
	}
	entry, ok, err := workspaceLedgerFromConfig(m.domain.runtime).SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || entry.TransactionID != transactionID {
		m.setSuccessStatus(successMessage)
		return status.DefaultStatusTimeout()
	}
	context := entry.Context
	if override := strings.TrimSpace(contextOverride); override != "" {
		context = override
	}
	undoCb := func() tea.Cmd { return m.handleLedgerTaskUndoRedo(false) }
	cmd := m.setUndoableStatus(formatRecoveryFeedback(entry.Action, context, originRecorded), undoCb)
	m.feedback.correlate(transactionID, successMessage, contextOverride)
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

// clearStatus clears the status message, the error beside it, and the info flag.
func (m *AppModel) clearStatus() {
	m.feedback.clear()
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
			if err := m.domain.persister.Load(p); err != nil {
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
		if m.domain.service != nil {
			if changed, err := m.domain.service.ReconcilePlanning(); err != nil {
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
	if m.domain.runtime == nil || m.domain.runtime.DataDir() == "" {
		return false
	}
	if err := m.domain.runtime.RefreshAfterCommit(); err != nil {
		logging.Warn("failed to refresh runtime domain snapshot: %v", err)
		return false
	}
	m.syncProjectCache()
	return true
}

// handleTagMessages handles tag-related messages.
// Returns true if the message was handled.
func (m *AppModel) handleTagMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case listsurface.ShowTagSelectorMsg:
		selector := listsurface.NewTagSelectorModel(msg.Task, msg.Project, msg.CurrentTags)
		m.showView(presentation.ViewTagSelector, selector)
		return true, nil

	case listsurface.TagSelectorDoneMsg:
		m.popView()
		if msg.Canceled {
			return true, nil
		}
		// Check if we came from project tree, outliner, or cross-project outliner - apply tags directly
		if m.currentView() == presentation.ViewCrossProjectOutliner && msg.Task != nil {
			return true, m.applyTagsToTask(msg.Identity, msg.Task, msg.SelectedTags)
		}
		// Otherwise forward to the current view.
		cmd := m.viewNav.views.Update(m.currentView(), msg)
		return true, cmd

	}
	return false, nil
}

// handleClipboardMessages handles clipboard-related messages (copy task title/details).
// Returns true if the message was handled.
func (m *AppModel) handleClipboardMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case coordinator.TaskClipboardProjectedMsg:
		// One write for the whole projection: the application layer has already
		// assembled every task's text, so there is no state in which part of a
		// selection reached the clipboard and part of it did not.
		if err := external.CopyText(msg.Projection.Text); err != nil {
			return true, status.ErrorCmd("copy failed: %w", err)
		}
		copied := TaskCopiedMsg{Count: msg.Projection.TaskCount, Hidden: msg.Hidden}
		switch msg.Projection.Operation {
		case app.TaskOperationCopyTitle:
		case app.TaskOperationCopyDescription:
			copied.FullDetail = true
		case app.TaskOperationCopyTitleDescription:
			copied.TitleAndDesc = true
		default:
			return true, status.ErrorCmd("copy failed: unknown clipboard projection %q", msg.Projection.Operation)
		}
		return true, func() tea.Msg { return copied }

	case TaskCopiedMsg:
		var what string
		if msg.TitleAndDesc {
			what = "title and description"
		} else if msg.FullDetail {
			what = "description"
		} else {
			what = "title"
		}
		summary := fmt.Sprintf("Copied %s to clipboard", what)
		if msg.Count > 1 {
			summary = fmt.Sprintf("Copied %s for %d tasks to clipboard", what, msg.Count)
		}
		if msg.Hidden > 0 {
			summary = fmt.Sprintf("%s · %d hidden", summary, msg.Hidden)
		}
		m.setSuccessStatus(summary)
		return true, status.DefaultStatusTimeout()
	}
	return false, nil
}

// TaskProjectRef is the shell's spelling of a qualified task. The type itself
// lives in taskref so the packages below the shell can name one.
type TaskProjectRef = taskref.TaskProjectRef

// finishTaskBulkSelection clears task multi-select state after a successful bulk action.
