package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/conflict"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/entityops"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/notesops"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
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
	case tea.PasteMsg:
		return m.handleActivePasteOverlay(msg)
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
		m.lifecycle.quitting = true
		return true, tea.Quit
	case outliner.CrossProjectsLoadedMsg:
		// Cross-project loads belong to the sole authoritative Outliner, not to
		// whichever child presentation happens to be visible when the async
		// command completes. Routing the result globally guarantees loading state
		// settles even if navigation covered the Outliner after the load started.
		// Adopt accepted snapshots into the shell cache first so operation
		// resolution sees the same disk-loaded task trees as the rendered rows.
		if m.crossOutliner == nil {
			return true, nil
		}
		adopted := msg.Generation == m.crossOutliner.Population.LoadGen
		if adopted {
			m.adoptCrossProjectSnapshot(msg.Projects)
		}
		wantedFocus := msg.FocusProjectID
		cmd := m.crossOutliner.HandleCrossProjectsLoaded(msg)
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
	case outliner.LensProjectsLoadedMsg:
		// A lens application publishes a whole population or none of it, so the
		// shell cache is adopted only once the lens actually went live.
		if m.crossOutliner == nil {
			return true, nil
		}
		cmd, published := m.crossOutliner.HandleLensProjectsLoaded(msg)
		if published {
			m.adoptCrossProjectSnapshot(m.crossOutliner.Population.Projects)
		}
		return true, cmd
	case notesops.NoteSearchIndexLoadedMsg:
		m.handleNoteSearchIndexLoaded(msg)
		return true, nil
	case outliner.OutlinerOriginRecordedMsg:
		if !m.feedback.correlates(msg.TransactionID, time.Now()) {
			return true, nil
		}
		return true, m.setRecoveryFeedbackWithContext(
			m.feedback.recoveryFallback, msg.TransactionID, m.feedback.recoveryContext, true)
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
		rowsInvalidated := m.wakeDeferredRows(msg.At)
		if m.refreshAttentionDay(msg.At) {
			rowsInvalidated = true
		}
		if rowsInvalidated {
			m.refreshWorkspacePulseAt(msg.At)
		} else if !m.completionActivity.IsCurrentLocalDay(msg.At) {
			m.refreshWorkspacePulseAcrossLocalDay(msg.At)
		}
		// Conflict bundles are written by another process on another machine, so
		// no local mutation can announce one; the periodic tick is what makes the
		// indicator appear, and what clears it once the CLI resolves the last one.
		m.refreshOutstandingConflicts()
		return true, m.lifecycle.periodicSaveCmd()
	case status.SaveCompleteMsg:
		return true, m.handleSaveComplete(msg)
	case coordinator.TransactionResultMsg:
		return true, m.handleTransactionResult(msg)
	case coordinator.BatchTransactionResultMsg:
		return true, m.handleBatchTransactionResult(msg)
	case coordinator.DueStepResultMsg:
		return true, m.handleDueStepResult(msg)
	case coordinator.TransactionErrorMsg:
		m.feedback.err = msg.Err
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
	if !m.deferWakeOwedAt(now) || !m.safeToInvalidateTimeDrivenRows() {
		return false
	}
	out := m.crossOutliner
	out.NextDeferWake = time.Time{}
	out.InvalidateTimeDrivenRows()
	return true
}

// refreshAttentionDay invalidates calendar-derived rows on the first safe tick
// of a new local day. The projection day is advanced only by the rebuild itself,
// so an overlay, picker, editor, visual selection, or ranked result leaves the
// refresh owed for the next tick instead of silently consuming it.
func (m *AppModel) refreshAttentionDay(now time.Time) bool {
	out := m.crossOutliner
	if out == nil || !out.TimeDrivenRefreshOwedAt(now) || !m.safeToInvalidateTimeDrivenRows() {
		return false
	}
	out.InvalidateTimeDrivenRows()
	return true
}

// deferWakeOwedAt reports whether a lapsed defer wake is still standing. After a
// wake attempt only a refusal leaves one, because wakeDeferredRows zeroes the
// clock it consumes.
func (m *AppModel) deferWakeOwedAt(now time.Time) bool {
	out := m.crossOutliner
	return out != nil && !out.NextDeferWake.IsZero() && !now.Before(out.NextDeferWake)
}

// safeToInvalidateTimeDrivenRows refuses while a new row would invalidate what the user
// is already in the middle of: an overlay or value picker holds a pending
// operation's resolved refs, an inline row editor and a visual selection both
// name rows the rebuild would renumber, and the flat ranked list is a settled
// order the woken task was never scored against. A query that is merely
// narrowing the mode's own lanes is not such a state: it is re-answered on
// every rebuild, so the woken task is matched against it like any other row.
func (m *AppModel) safeToInvalidateTimeDrivenRows() bool {
	out := m.crossOutliner
	if out == nil {
		return false
	}
	return !m.Overlays().Active() &&
		!out.QuickEdit.IsActive() &&
		!out.IsMultiSelectActive() &&
		!out.ShowsRankedList()
}

// handleTransactionResult routes a committed feature-coordinator mutation back
// into the shell. The application layer committed one atomic ChangeSet against
// its own freshly loaded snapshot, so the AppModel's in-memory copy of the
// affected project is reloaded from disk before the shared post-save refresh
// runs, keeping every open view consistent with the committed transaction.
func (m *AppModel) handleTransactionResult(msg coordinator.TransactionResultMsg) tea.Cmd {
	m.reloadProjectByID(msg.Result.ProjectID)
	if m.crossOutliner != nil {
		m.crossOutliner.ArmOutlinerRelocation(outliner.OutlinerRelocationFrom(msg.Relocation))
	}
	if msg.PlanningChanged && m.refreshRuntime() {
		m.rebuildPlanningFromRuntime()
	}
	m.refreshLoadedTaskWorkspace()
	if m.crossOutliner != nil {
		m.crossOutliner.SettleRelocationCursor()
	}
	m.observeCommittedDueEffect(msg.Result.TransactionID, msg.DueEffect)
	m.revealCommittedChecklistItemAt(msg.RevealChecklistFor, msg.RevealChecklistIndex)
	m.revealCommittedTask(msg.RevealTaskFor, taskref.TaskIdentity{
		ProjectID: msg.Result.ProjectID,
		TaskID:    msg.Result.TaskID,
	})
	saveCmd := m.handleSaveComplete(status.SaveCompleteMsg{})
	feedbackCmd := m.setRecoveryFeedback(msg.SuccessMessage(), msg.Result.TransactionID, false)
	return tea.Batch(saveCmd, feedbackCmd)
}

// revealCommittedChecklistItemAt opens the owner's checklist and seats the
// cursor on the item the transaction appended. It runs from the success message
// alone and after the workspace refresh, so the fold and the cursor move only
// once the committed item actually exists in a rebuilt projection; a refused
// append never reaches here and leaves the outline exactly as it was. A nil
// inserted index means the append landed at the end.
func (m *AppModel) revealCommittedChecklistItemAt(identity taskref.TaskIdentity, inserted *int) {
	if m == nil || m.crossOutliner == nil || !identity.Valid() {
		return
	}
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil || len(task.Checklist) == 0 {
		return
	}
	m.crossOutliner.Folds.SetChecklist(identity, true)
	m.crossOutliner.RebuildRows()
	index := len(task.Checklist) - 1
	if inserted != nil {
		index = *inserted
	}
	if !m.crossOutliner.MoveCursorToChecklistItem(identity, index) && inserted != nil {
		m.crossOutliner.MoveCursorToIdentity(identity)
		m.crossOutliner.SetTransientStatus("Checklist item was created but is hidden by this view")
	}
}

// revealCommittedTask seats the cursor on a task this transaction minted, named
// by qualified identity because the row the gesture was made on no longer
// exists: checklist promotion consumes the item it converts, so the child is the
// only thing left to point at and only the commit knows what it is.
//
// It runs from the success message alone and after the reload, so the cursor
// moves only once the child is really in a rebuilt projection. It opens the
// branch that owns the child and nothing else — scope, mode, query, sections and
// every unrelated fold are left as the user had them — which means the current
// view may legitimately refuse to show the child. That is reported rather than
// worked around: the cursor falls back to the owner, or to whatever the clamp
// leaves under it, and the status names the child the user cannot see.
func (m *AppModel) revealCommittedTask(identity, owner taskref.TaskIdentity) {
	if m == nil || m.crossOutliner == nil || !identity.Valid() {
		return
	}
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil {
		return
	}
	out := m.crossOutliner
	if out.ExpandAncestorsOfTask(task, identity.ProjectID) {
		out.RebuildRows()
	}
	if out.MoveCursorToIdentity(identity) {
		return
	}
	if owner.Valid() && owner != identity {
		out.MoveCursorToIdentity(owner)
	}
	out.SetTransientStatus(hiddenCommittedTaskStatus(task))
}

// hiddenCommittedTaskStatus names a committed task the current view does not
// admit. A promoted item that was already ticked becomes a done subtask, and
// done work is hidden by default, so that case says so outright instead of
// leaving the user to wonder whether anything was written at all.
func hiddenCommittedTaskStatus(task *models.Task) string {
	title := strings.TrimSpace(task.Title)
	if task.IsTerminalState() {
		return fmt.Sprintf("Subtask %q was created %s and is hidden by this view", title, task.State())
	}
	return fmt.Sprintf("Subtask %q was created and is hidden by this view", title)
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
		m.crossOutliner.ArmOutlinerRelocation(outliner.OutlinerRelocationFrom(msg.Relocation))
	}
	m.refreshLoadedTaskWorkspace()
	if m.crossOutliner != nil {
		m.crossOutliner.SettleRelocationCursor()
	}
	m.observeCommittedDueEffect(msg.TransactionID, msg.DueEffect)
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
		successMessage = m.feedback.message
	}
	saveCmd := m.handleSaveComplete(status.SaveCompleteMsg{})
	feedbackCmd := m.setRecoveryFeedbackWithContext(successMessage, msg.TransactionID, msg.RecoveryContext, false)
	return tea.Batch(saveCmd, feedbackCmd)
}

func (m *AppModel) refreshLoadedTaskWorkspace() {
	if m.crossOutliner != nil {
		outliner.RebuildOutlinerPreservingCursor(m.crossOutliner)
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
	if m.viewNav.views != nil {
		for _, viewType := range []presentation.PresentationID{
			presentation.ViewTaskComments,
			presentation.ViewTaskAttachments,
		} {
			view := m.viewNav.views.Get(viewType)
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
	m.workspacePulse = outliner.ComputeWorkspacePulse(m.domain.runtime, m.todayIDs, now)
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
func (m *AppModel) handleOperationOverlayMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch typed := msg.(type) {
	case listsurface.ContextActionSheetResultMsg:
		return true, m.operations.ApplySheetResult(m, typed)
	case listsurface.ContextActionSheetClosedMsg:
		m.operations.ActionSheet = nil
		return true, nil
	case lens.NamedLensPickerClosedMsg:
		m.lenses.Close()
		return true, nil
	case lens.NamedLensPickerResultMsg:
		return true, m.applyNamedLensPickerResult(typed)
	case lens.NamedLensActionsRequestMsg:
		return true, m.openNamedLensActions(typed)
	case outliner.SaveViewAsLensRequestMsg:
		return true, m.saveViewAsLens()
	case outliner.MenuBarActivateMsg:
		// The menu named an operation and closed; running it is the shell's because
		// the executor is.
		return true, m.operations.ActivateMenuBarEntry(m, typed.OperationID)
	case outliner.QueryReadoutReloadRequestMsg:
		// The peel already wrote the population fields; only the reload the facet
		// owes is left, and it is the shell's because the projects are.
		return true, m.filters.ProjectFilterReloadCmd(m, typed.Status)
	case conflict.ConflictBrowserClosedMsg:
		m.conflictBrowser = nil
		return true, nil
	case operation.OperationConfirmationRequestMsg:
		m.operations.ShowConfirmation(typed)
		return true, nil
	case operation.OperationValueRequestMsg:
		return true, m.operations.OpenValueStep(m, typed)
	case operation.OperationValueSelectedMsg:
		m.operations.ClearValueStep()
		request := typed.Request
		request.Value = typed.Value
		request.HasValue = true
		return true, m.operations.ResumeRequest(m, request)
	case operation.OperationValueCanceledMsg:
		m.operations.ClearValueStep()
		return true, nil
	case ResumeNoteRejectedMsg:
		return true, m.reopenResumeNotePrompt(typed)
	case CommentRejectedMsg:
		return true, m.reopenCommentPrompt(typed)
	case entityops.ChecklistItemRejectedMsg:
		return true, m.reopenChecklistItemPrompt(typed)
	case valuepicker.ProjectTagVocabularyActionsMsg:
		m.operations.ClearValueStep()
		return true, m.openProjectTagVocabularyActions(typed.Tag)
	case valuepicker.VocabularyRenameRequestedMsg:
		// The three rename messages deliberately leave the value step mounted: the
		// workflow is a stage of that picker, and its query, cursor, and pending
		// choices are preserved by never being torn down in the first place.
		m.handleVocabularyRenameRequested(typed)
		return true, nil
	case valuepicker.VocabularyRenameConfirmedMsg:
		return true, m.handleVocabularyRenameConfirmed(typed)
	case valuepicker.VocabularyRenameAbandonedMsg:
		m.handleVocabularyRenameAbandoned()
		return true, nil
	default:
		return false, nil
	}
}

// =============================================================================
// Outliner Messages (Show/Close Pattern)
// =============================================================================

// launchOutlinerMsg is emitted exactly once by AppModel.Init.
type launchOutlinerMsg struct{}

// FooterFlashTickMsg advances the footer divider flash one frame. Seq
// identifies the flash generation so a tick from a superseded flash is ignored.
type FooterFlashTickMsg struct{ Seq uint64 }

// footerFlashTick schedules the next frame of the footer divider flash.
func footerFlashTick(seq uint64) tea.Cmd {
	return status.TickCmd(footerFlashFrameInterval, func(time.Time) tea.Msg {
		return FooterFlashTickMsg{Seq: seq}
	})
}

// =============================================================================
// Clipboard Messages
// =============================================================================

// TaskCopiedMsg confirms task(s) were copied to clipboard. Hidden is the part
// of Count that had no visible row when the selection was frozen.
type TaskCopiedMsg struct {
	Count        int
	Hidden       int
	FullDetail   bool
	TitleAndDesc bool
}

// =============================================================================
// External Editor Messages
// =============================================================================

// EditTaskContentMsg launches external editor for task title/description. An
// empty Path opens a fresh buffer; a reopened one reuses the file the user just
// saved so a refused title keeps its description. RejectedTitle is the title the
// capture/edit grammar refused, which is both the baseline the returned title is
// compared against and the marker that this open is a retry. Notice is that
// refusal, rendered into the buffer's instruction comment, and Description is
// the uncommitted body a retry must carry back into the buffer.
type EditTaskContentMsg struct {
	Task          *models.Task
	Project       *models.Project
	Path          string
	RejectedTitle string
	Notice        string
	Description   string
}

// TaskContentEditedMsg returns results from external editor. Path, RejectedTitle
// and Notice ride back so a second refusal can reopen the same buffer and an
// abandoned retry can report the refusal that started it.
type TaskContentEditedMsg struct {
	Task           *models.Task
	Project        *models.Project
	Path           string
	RejectedTitle  string
	Notice         string
	NewTitle       string
	NewDescription string
	Err            error
}

// OpenOutlineEditorMsg dumps one project's task tree to an external editor for
// bulk restructuring. The Outliner emits it; the shell owns the round trip.
// Project names which project to outline: the shell reloads it from an
// application snapshot before dumping, so the cursor's own copy is never what
// reaches the buffer.
type OpenOutlineEditorMsg struct {
	Project *models.Project
}

// OutlineEditedMsg returns one edited outline buffer. Project and Revision are
// the snapshot pair the buffer was dumped from, and the revision travels into
// the apply so a workspace that moved while the editor was open is refused
// instead of merged. ScopeTaskIDs are the tasks the dump actually printed, which
// is what makes absence readable as deletion. Path is kept so a refused buffer
// can be named in the status rather than silently discarded.
type OutlineEditedMsg struct {
	Project      *models.Project
	Revision     string
	ScopeTaskIDs []string
	Path         string
	Buffer       string
	Err          error
}
