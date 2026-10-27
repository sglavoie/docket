package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/conflict"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/entityops"
	"github.com/sglavoie/docket/internal/tui/lens"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/notesops"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/operationhost"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/valuepicker"
)

func (m *AppModel) handleGlobalMessage(msg tea.Msg) (bool, tea.Cmd) {
	// A host's request answers for itself whatever overlay is up: it refuses
	// rather than reach an overlay that would mistake it for input.
	if handled, cmd := m.handleControlMessage(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleOperationOverlayMessage(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleGlobalInputMessage(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleFocusMessage(msg); handled {
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
		// A keystroke is the user taking the cursor. The launch rule is a starting
		// position rather than a correction, so a population still on its way may
		// no longer move what the first press has begun steering. A bare modifier
		// is not a keystroke, which is why this sits under that drop.
		m.lifecycle.dropLaunchFocus()
		return m.handleGlobalKeyPress(msg)
	case tea.PasteMsg:
		return m.handleActivePasteOverlay(msg)
	case tea.KeyboardEnhancementsMsg:
		m.lifecycle.keyDisambiguation = msg.SupportsKeyDisambiguation()
		return true, nil
	case tea.KeyReleaseMsg:
		return true, nil
	case tea.MouseClickMsg, tea.MouseWheelMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg:
		// Motion and release fold to nothing inside the seam, so every mouse
		// message is consumed here and none reaches a view's own Update.
		return m.handlePointerMessage(msg)
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
		// After the adoption and after the lens drain, because both decide what
		// the first visible task is. A lens that started a read of its own leaves
		// the population loading, so nothing is observed here and the intent is
		// spent when that read settles instead.
		m.settleLaunchFocus()
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
		// The other half of the launch rule: a saved lens that had to read its own
		// projects settles here, and it settles whether it published or was
		// refused — a refusal leaves the base workspace standing, which is the
		// projection the launch rule then answers for.
		m.settleLaunchFocus()
		return true, cmd
	case notesops.NoteSearchIndexLoadedMsg:
		m.handleNoteSearchIndexLoaded(msg)
		return true, nil
	case outliner.OutlinerOriginRecordedMsg:
		if !m.feedback.correlates(msg.TransactionID, time.Now()) {
			return true, nil
		}
		return true, m.setRecoveryFeedbackWithContext(
			m.feedback.recoveryFallback, msg.TransactionID, m.feedback.recoveryWords, true)
	default:
		return false, nil
	}
}

// settleLaunchFocus spends the one-shot launch intent once the population it
// must read is the accepted one, seating the cursor on the first visible task —
// or on the first heading when the projection genuinely draws no task.
//
// It is idempotent by construction: the intent is dropped in the same act it is
// honored, so a second accepted load, a later reload, and a duplicate delivery
// all do nothing. An unsettled population is left owed rather than answered, so
// a startup that completes after the user has already pressed a key finds the
// intent gone instead of overriding them.
func (m *AppModel) settleLaunchFocus() {
	if m == nil || !m.lifecycle.launchFocusArmed() || m.crossOutliner == nil {
		return
	}
	if !m.crossOutliner.LaunchFocusSettled() {
		return
	}
	m.lifecycle.dropLaunchFocus()
	m.crossOutliner.FocusFirstLaunchRow()
}

func (m *AppModel) handleGlobalSaveMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case status.PeriodicSaveMsg:
		m.saveProjectViewState()
		m.saveOutlinerFilterState()
		// A consumed wake makes rows visible that ComputeWorkspaceReadout excluded
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
		// A session exchanging on its own timer leaves the sync half to its
		// passes, which read it again whenever one could have changed it.
		if m.automaticSync.exchange != nil {
			m.recountOutstandingConflicts()
		} else {
			m.refreshOutstandingConflicts()
		}
		// A change adopted on focus but refused mid-interaction is retried here,
		// so putting the picker down is enough to see it.
		return true, tea.Batch(m.lifecycle.periodicSaveCmd(), m.adoptExternalWorkspaceChange(), m.notifyArrivals(msg.At))
	case status.SaveCompleteMsg:
		return true, m.handleSaveComplete(msg)
	case coordinator.TasksCapturedMsg:
		if m.taskComposer != nil {
			m.taskComposer.Update(msg)
		}
		planningRefreshed := msg.PlanningChanged && m.refreshRuntime()
		for _, result := range msg.Results {
			m.reloadProjectByID(result.ProjectID)
		}
		if planningRefreshed {
			m.rebuildPlanningFromRuntime()
		}
		return true, m.handleSaveComplete(status.SaveCompleteMsg{})
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
	case conflict.ResolveMsg:
		return true, m.resolveBrowserConflict(typed)
	case conflict.ConflictBrowserClosedMsg:
		m.conflictBrowser = nil
		return true, nil
	case operation.OperationConfirmationRequestMsg:
		m.operations.ShowConfirmation(typed)
		return true, nil
	case operation.OperationValueRequestMsg:
		// A value step may itself offer a typed route to a different editor (the
		// combined scheduling picker does this for Check-in). Replace the mounted
		// value overlay while retaining the request's frozen target identities.
		m.operations.ClearValueStep()
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
	case operationhost.CheckInRejectedMsg:
		return true, m.operations.ReopenCheckInEditor(m, typed, operationRefusalReason(typed.Err))
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
