package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

// handleNotesMessages handles note-related view messages.
// Returns true if the message was handled.
func (m *AppModel) handleNotesMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case notes.ProjectNoteMutationResultMsg:
		// A committed note transaction is the primary invalidation: the index
		// holds parsed note bodies, so a create, edit, rename, move, or delete
		// makes it stale immediately.
		return true, tea.Batch(m.reconcileNoteResult(msg), m.requestNoteSearchIndexLoad())

	case notes.ProjectNoteMutationErrorMsg:
		location := notes.NormalizedNoteLocation(msg.Location, msg.ProjectID)
		if nv := m.getActiveNotesView(); notes.NoteViewMatchesLocation(nv, location) {
			nv.SetErrorStatus(msg.Err.Error())
			return true, nil
		}
		return true, func() tea.Msg { return status.ErrorMsg{Err: msg.Err} }

	case notes.ProjectNoteLedgerResultMsg:
		location := notes.NormalizedNoteLocation(msg.Location, msg.ProjectID)
		cmds, refreshErr := m.reconcileNoteLedgerRestore(msg.Entry)
		nv := m.reloadActiveNotesAfterLedgerRestore()
		if notes.NoteViewMatchesLocation(nv, location) {
			verb := "Undid"
			if msg.Redo {
				verb = "Redid"
			}
			if refreshErr != nil {
				nv.SetErrorStatus(fmt.Sprintf("%s completed, refresh failed: %v", ledgerRestoreLabel(msg.Redo), refreshErr))
			} else {
				nv.SetStatus(fmt.Sprintf("%s: %s", verb, msg.Action))
			}
		} else if refreshErr != nil {
			m.feedback.err = fmt.Errorf("%s completed, refresh failed: %w", ledgerRestoreLabel(msg.Redo), refreshErr)
			cmds = append(cmds, status.StatusTimeout(5*time.Second))
		}
		// A ledger restore rewrites note bytes without emitting a mutation
		// result, so it owes the index its own reload.
		cmds = append(cmds, m.requestNoteSearchIndexLoad())
		return true, tea.Batch(cmds...)

	case notes.ProjectNoteLedgerErrorMsg:
		location := notes.NormalizedNoteLocation(msg.Location, msg.ProjectID)
		if nv := m.getActiveNotesView(); notes.NoteViewMatchesLocation(nv, location) {
			nv.SetErrorStatus(msg.Err.Error())
		}
		return true, nil

	case notes.ProjectNoteEditorDoneMsg:
		location := notes.NormalizedNoteLocation(msg.Location, msg.ProjectID)
		if msg.Err != nil {
			return true, func() tea.Msg {
				return notes.ProjectNoteMutationErrorMsg{Location: location, ProjectID: location.ProjectID, Err: msg.Err}
			}
		}
		if msg.Modified == msg.OriginalContent {
			if nv := m.getActiveNotesView(); notes.NoteViewMatchesLocation(nv, location) {
				if msg.Create {
					nv.SetStatus("Note creation canceled")
				} else {
					nv.SetStatus("Note unchanged")
				}
			}
			return true, nil
		}
		updated, err := notes.ParseProjectNoteEditorResult(msg.Original, msg.Modified, msg.Create)
		if err != nil {
			return true, func() tea.Msg {
				return notes.ProjectNoteMutationErrorMsg{Location: location, ProjectID: location.ProjectID, Err: err}
			}
		}
		kind := app.NoteUpdate
		undoAction := "Edit Note"
		if msg.Create {
			kind = app.NoteCreate
			undoAction = "Create Note"
		} else if updated.Title != msg.Original.Title {
			kind = app.NoteRename
		}
		operation := app.NoteMutationRequest{Kind: kind, Location: location, Note: updated}
		if kind == app.NoteRename {
			operation.NewTitle = updated.Title
		}
		return true, notes.NoteMutationCmd(m.domain.service, location, app.NoteBatchRequest{
			Operations:  []app.NoteMutationRequest{operation},
			UndoAction:  undoAction,
			UndoContext: updated.Title,
		}, undoAction+": "+updated.Title)

	case notes.ShowNotesManagerMsg:
		hadNotesManager := false
		if m.manageHub != nil {
			_, hadNotesManager = m.manageHub.ActiveModuleView().(*notes.NotesViewModel)
		}
		cmd := m.activateManageHub(route.ManageParams{Module: route.ManageNotes, NotesScope: msg.Scope})
		if hadNotesManager {
			if nv := m.getActiveNotesView(); nv != nil {
				nv.RefreshWorkspace(m.availableProjects()...)
			}
		}
		m.selectActiveNote(msg.Focus)
		if msg.MoveSource.Valid() {
			if nv := m.getActiveNotesView(); nv != nil {
				return true, tea.Batch(cmd, nv.BeginMovePickerForSource(msg.MoveSource))
			}
		}
		return true, cmd

	}
	return false, nil
}

func (m *AppModel) selectActiveNote(identity notes.NoteRowIdentity) {
	if identity.Filename == "" || identity.Location.Scope == "" || m.manageHub == nil {
		return
	}
	manager, ok := m.manageHub.ActiveModuleView().(*notes.NotesViewModel)
	if !ok {
		return
	}
	manager.FocusNote(identity)
}

func ledgerRestoreLabel(redo bool) string {
	if redo {
		return "redo"
	}
	return "undo"
}

func (m *AppModel) reconcileNoteResult(msg notes.ProjectNoteMutationResultMsg) tea.Cmd {
	location := notes.NormalizedNoteLocation(msg.Location, msg.ProjectID)
	feedbackCmd := m.setRecoveryFeedback(msg.SuccessMessage, msg.Result.TransactionID, false)
	nv := m.getActiveNotesView()
	if !notes.NoteViewMatchesLocation(nv, location) {
		return feedbackCmd
	}
	target := notes.NoteRowIdentity{}
	if len(msg.Result.Results) > 0 {
		result := msg.Result.Results[len(msg.Result.Results)-1]
		target = notes.NoteRowIdentity{
			Location: notes.NormalizedNoteLocation(result.Location, result.ProjectID),
			Filename: result.Filename,
		}
	}
	nv.Reload()
	nv.FocusNote(target)
	nv.SetStatus(msg.SuccessMessage)
	return feedbackCmd
}
