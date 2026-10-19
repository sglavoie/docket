package taskform

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/status"
)

// Recovery contains plain values, never live project/task pointers. An expanded
// form keeps unparsed text too: closing must not validate or normalize a draft.
type composerRecovery struct {
	Version      int
	ProjectID    string
	Text         string
	Line, Column int
	Expanded     *expandedRecovery
	Pending      bool
}

type expandedRecovery struct {
	PresetDue           *time.Time
	PresetDestination   app.CaptureDestination
	Request             app.CaptureRequest
	DueInput            string
	Checklist           []models.ChecklistItem
	Subtasks            []string
	Tab, Focus          int
	Collapsed           bool
	ChecklistExpanded   bool
	SubtasksExpanded    bool
	AttachmentsExpanded bool
	CommentsExpanded    bool
}

const composerAutosaveDelay = 500 * time.Millisecond

type composerAutosaveMsg struct {
	owner    *TaskComposerModel
	sequence uint64
}

func (m *TaskComposerModel) scheduleDraftAutosave() tea.Cmd {
	m.recoveryAutosaveSequence++
	sequence := m.recoveryAutosaveSequence
	return status.TickCmd(composerAutosaveDelay, func(time.Time) tea.Msg {
		return composerAutosaveMsg{owner: m, sequence: sequence}
	})
}

func (m *TaskComposerModel) autosaveBeforeUpdate() (composerRecovery, bool) {
	if !m.preservesDraft || m.SubmissionInFlight {
		return composerRecovery{}, false
	}
	return m.recoverySnapshot(m.recoveryPending), true
}

func (m *TaskComposerModel) autosaveAfterUpdate(before composerRecovery, cmd tea.Cmd) tea.Cmd {
	if m.SubmissionInFlight || (m.Expanded != nil && m.Expanded.submissionInFlight) {
		return cmd
	}
	if !reflect.DeepEqual(before, m.recoverySnapshot(m.recoveryPending)) {
		return tea.Batch(cmd, m.scheduleDraftAutosave())
	}
	return cmd
}

func (m *TaskComposerModel) flushDraftAutosave(msg composerAutosaveMsg) {
	if msg.owner != m || msg.sequence != m.recoveryAutosaveSequence ||
		!m.preservesDraft || m.SubmissionInFlight ||
		(m.Expanded != nil && m.Expanded.submissionInFlight) {
		return
	}
	if m.recoveryPending {
		// An interrupted submission must retain its durable pending marker.
		return
	}
	var err error
	if m.HasDraft() || m.Mode == ComposerExpanded {
		err = m.writeRecovery(false)
	} else if m.recovery != nil && m.recoveryBlocked == nil {
		err = m.recovery.Replace(nil)
	}
	if err != nil {
		m.recoveryIssue(err)
	}
}

// EnableDraftRecovery is called by the shell once, before mounting the composer.
// The key follows the entry context, while the document remembers a project
// selected inside capture. Switching projects cannot file a recovered child in
// a different parent or silently replace another context's draft.
func (m *TaskComposerModel) EnableDraftRecovery() {
	// Unrooted/headless models have no workspace in which to recover a draft.
	if m.runtime.DataDir() == "" {
		return
	}
	m.preservesDraft = true
	defer m.wireExpandedRecovery()
	key, err := json.Marshal(struct {
		Kind    TaskComposerPresetKind
		Project string
		Shared  TaskComposerSharedAttributes
		Initial string
		Seed    *TaskComposerSeed
	}{m.Preset.Kind, composerProjectID(m.Preset.Project), m.Preset.Shared, m.Preset.InitialText, m.Preset.Seed})
	if err == nil {
		m.recovery, err = m.runtime.CaptureDraft(key)
	}
	if err != nil {
		m.recoveryIssue(err)
		return
	}
	data, err := m.recovery.Read()
	if err != nil {
		m.recoveryBlocked = err
		m.recoveryIssue(err)
		return
	}
	if len(data) == 0 {
		m.wireExpandedRecovery()
		return
	}
	var saved composerRecovery
	if err = json.Unmarshal(data, &saved); err == nil && saved.Version != 1 {
		err = errors.New("unsupported capture draft version")
	}
	if err != nil {
		m.recoveryBlocked = err
		m.recoveryIssue(err)
		return
	}
	project := m.projectByID(saved.ProjectID)
	if project == nil {
		m.recoveryBlocked = errors.New("the saved draft's project is unavailable")
		m.recoveryIssue(m.recoveryBlocked)
		return
	}
	m.Preset.Project = project
	m.initCompactInput()
	m.Input.SetValue(saved.Text)
	m.Input.MoveToBegin()
	for m.Input.Line() < min(saved.Line, len(strings.Split(saved.Text, "\n"))-1) {
		m.Input.CursorEnd()
		m.Input.CursorDown()
	}
	m.Input.SetCursorColumn(saved.Column)
	m.UpdatePreview()
	if saved.Expanded != nil {
		form := saved.Expanded
		if m.projectByID(form.Request.ProjectID) == nil {
			m.recoveryBlocked = errors.New("the saved form's project is unavailable")
			m.recoveryIssue(m.recoveryBlocked)
			return
		}
		m.ExpandRequest(form.Request)
		m.Expanded.presetDue = form.PresetDue
		m.Expanded.presetDestination = form.PresetDestination
		m.Expanded.titleInput.SetValue(form.Request.Title)
		m.Expanded.contextInput.SetValue(form.Request.Context)
		m.Expanded.dueTextInput.SetValue(form.DueInput)
		m.Expanded.draft.dueInput = form.DueInput
		m.Expanded.draft.checklist = form.Checklist
		for _, title := range form.Subtasks {
			m.Expanded.draft.subtasks = append(m.Expanded.draft.subtasks, formSubtaskDraft{title: title})
		}
		m.Expanded.checklistExpanded = form.ChecklistExpanded
		m.Expanded.subtasksExpanded = form.SubtasksExpanded
		m.Expanded.attachmentsExpanded = form.AttachmentsExpanded
		m.Expanded.commentsExpanded = form.CommentsExpanded
		m.Expanded.activeTab = max(0, min(form.Tab, formTabCount-1))
		m.Expanded.collapsedBasics = form.Collapsed
		m.Expanded.focusIndex = max(0, min(form.Focus, m.Expanded.fieldCountForActiveTab()-1))
		m.Expanded.syncInputFocus()
	}
	m.recoveryPending = saved.Pending
	m.wireExpandedRecovery()
	if saved.Pending {
		m.pendingRecoveryIssue()
	}
	m.syncInputHeight()
}

func (m *TaskComposerModel) recoverySnapshot(pending bool) composerRecovery {
	saved := composerRecovery{Version: 1, ProjectID: composerProjectID(m.Preset.Project), Text: m.Input.Value(), Line: m.Input.Line(), Column: m.Input.Column(), Pending: pending}
	if m.Mode == ComposerExpanded && m.Expanded != nil {
		f := m.Expanded
		d := f.draft
		req := app.CaptureRequest{ProjectID: f.project.ID, Title: f.titleInput.Value(), Description: d.desc, Context: f.contextInput.Value(), Priority: d.priority, Tags: d.tags, Pinned: d.pinned, Review: d.review, Due: d.dueDate, Defer: d.deferredUntil, ParentID: d.parentID, ChildIDs: d.childIDs, Dependencies: d.dependencies, CheckIn: d.checkIn, Prepend: d.prepend, Destination: app.CaptureDestination(d.state)}
		if f.presetDestination.IsPlanning() && d.state == models.StateBacklog {
			req.Destination = f.presetDestination
		}
		for _, a := range d.attachments {
			req.Attachments = append(req.Attachments, a.value)
		}
		for _, c := range d.comments {
			req.Comments = append(req.Comments, c.text)
		}
		expanded := &expandedRecovery{PresetDue: f.presetDue, PresetDestination: f.presetDestination, Request: req, DueInput: f.dueTextInput.Value(), Checklist: d.checklist, Tab: f.activeTab, Focus: f.focusIndex, Collapsed: f.collapsedBasics}
		expanded.ChecklistExpanded, expanded.SubtasksExpanded = f.checklistExpanded, f.subtasksExpanded
		expanded.AttachmentsExpanded, expanded.CommentsExpanded = f.attachmentsExpanded, f.commentsExpanded
		for _, s := range d.subtasks {
			expanded.Subtasks = append(expanded.Subtasks, s.title)
		}
		saved.Expanded = expanded
	}
	return saved
}

func (m *TaskComposerModel) recoveryIssue(err error) {
	message := fmt.Sprintf("Couldn’t keep a recovery copy. Your draft is still open. %v. Edit or close again after fixing the problem.", err)
	m.showRecoveryIssue(message)
}

func (m *TaskComposerModel) pendingRecoveryIssue() {
	m.showRecoveryIssue("A previous save may have completed. Your draft is available here. Check your tasks, then use Ctrl+X to discard this copy before creating another.")
}

func (m *TaskComposerModel) showRecoveryIssue(message string) {
	if m.Expanded != nil && m.Mode == ComposerExpanded {
		m.Expanded.err = errors.New(message)
	} else {
		m.Errors = []grammar.LineError{{Line: 1, Message: message}}
	}
}

func (m *TaskComposerModel) writeRecovery(pending bool) error {
	if m.recoveryBlocked != nil {
		if m.recovery != nil {
			data, err := m.recovery.Read()
			if err == nil && len(data) == 0 {
				m.recoveryBlocked = nil
			} else {
				return m.recoveryBlocked
			}
		} else {
			return m.recoveryBlocked
		}
	}
	if m.recovery == nil {
		return errors.New("recovery storage is unavailable")
	}
	data, err := json.Marshal(m.recoverySnapshot(pending))
	if err != nil {
		return err
	}
	return m.recovery.Replace(data)
}

func (m *TaskComposerModel) closeKeepingDraft() tea.Cmd {
	if m.HasDraft() || m.Mode == ComposerExpanded {
		if err := m.writeRecovery(m.recoveryPending); err != nil {
			m.recoveryIssue(err)
			return nil
		}
	} else if m.recovery != nil && m.recoveryBlocked == nil {
		if err := m.recovery.Replace(nil); err != nil {
			m.recoveryIssue(err)
			return nil
		}
	}
	return func() tea.Msg { return status.BackMsg{} }
}

func (m *TaskComposerModel) discardRecovery() tea.Cmd {
	// Explicit discard may remove an unreadable-format document. Replace still
	// refuses if another terminal changed it since our successful read.
	if m.recovery != nil {
		if err := m.recovery.Replace(nil); err != nil {
			m.recoveryIssue(err)
			return nil
		}
	}
	return func() tea.Msg { return status.BackMsg{} }
}

func (m *TaskComposerModel) wireExpandedRecovery() {
	if m.preservesDraft && m.Expanded != nil {
		m.Expanded.closeDraft = m.closeKeepingDraft
		m.Expanded.discardDraft = m.discardRecovery
		m.Expanded.guardSubmit = m.guardRecoverySubmit
	}
}

// The pending marker is durable before a command can commit. If the process
// disappears before acknowledging its result, recovery never silently retries
// an ambiguous submission. Successful saves remove the recovery copy before
// returning to the shell; failures retain the draft for editing and retry.
func (m *TaskComposerModel) guardRecoverySubmit(cmd tea.Cmd) tea.Cmd {
	if cmd == nil || !m.preservesDraft {
		return cmd
	}
	if m.recoveryPending {
		m.pendingRecoveryIssue()
		return nil
	}
	saved := m.recoverySnapshot(false)
	if err := m.writeRecovery(true); err != nil {
		m.recoveryIssue(err)
		return nil
	}
	store := m.recovery
	return func() tea.Msg {
		result := cmd()
		if created, ok := result.(coordinator.TaskCreatedMsg); ok {
			if err := store.Replace(nil); err != nil {
				created.RecoveryWarning = "Task saved, but its recovery copy could not be removed. Reopening capture will ask you to check the saved task."
			}
			return created
		}
		data, err := json.Marshal(saved)
		if err == nil {
			err = store.Replace(data)
		}
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("save did not finish and the recovery copy could not be updated; keep this draft open: %w", err)}
		}
		return result
	}
}
