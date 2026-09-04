package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

// handleNotesMessages handles note-related view messages.
// Returns true if the message was handled.
func (m *AppModel) handleNotesMessages(msg tea.Msg) (bool, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case ProjectNoteMutationResultMsg:
		// A committed note transaction is the primary invalidation: the index
		// holds parsed note bodies, so a create, edit, rename, move, or delete
		// makes it stale immediately.
		return true, tea.Batch(m.reconcileNoteResult(msg), m.requestNoteSearchIndexLoad())

	case ProjectNoteMutationErrorMsg:
		location := normalizedNoteLocation(msg.Location, msg.ProjectID)
		if nv := m.getActiveNotesView(); noteViewMatchesLocation(nv, location) {
			nv.setErrorStatus(msg.Err.Error())
			return true, nil
		}
		return true, func() tea.Msg { return status.ErrorMsg{Err: msg.Err} }

	case ProjectNoteLedgerResultMsg:
		location := normalizedNoteLocation(msg.Location, msg.ProjectID)
		cmds, refreshErr := m.reconcileNoteLedgerRestore(msg.Entry)
		nv := m.reloadActiveNotesAfterLedgerRestore()
		if noteViewMatchesLocation(nv, location) {
			verb := "Undid"
			if msg.Redo {
				verb = "Redid"
			}
			if refreshErr != nil {
				nv.setErrorStatus(fmt.Sprintf("%s completed, refresh failed: %v", ledgerRestoreLabel(msg.Redo), refreshErr))
			} else {
				nv.setStatus(fmt.Sprintf("%s: %s", verb, msg.Action))
			}
		} else if refreshErr != nil {
			m.err = fmt.Errorf("%s completed, refresh failed: %w", ledgerRestoreLabel(msg.Redo), refreshErr)
			cmds = append(cmds, status.StatusTimeout(5*time.Second))
		}
		// A ledger restore rewrites note bytes without emitting a mutation
		// result, so it owes the index its own reload.
		cmds = append(cmds, m.requestNoteSearchIndexLoad())
		return true, tea.Batch(cmds...)

	case ProjectNoteLedgerErrorMsg:
		location := normalizedNoteLocation(msg.Location, msg.ProjectID)
		if nv := m.getActiveNotesView(); noteViewMatchesLocation(nv, location) {
			nv.setErrorStatus(msg.Err.Error())
		}
		return true, nil

	case ProjectNoteEditorDoneMsg:
		location := normalizedNoteLocation(msg.Location, msg.ProjectID)
		if msg.Err != nil {
			return true, func() tea.Msg {
				return ProjectNoteMutationErrorMsg{Location: location, ProjectID: location.ProjectID, Err: msg.Err}
			}
		}
		if msg.Modified == msg.OriginalContent {
			if nv := m.getActiveNotesView(); noteViewMatchesLocation(nv, location) {
				if msg.Create {
					nv.setStatus("Note creation canceled")
				} else {
					nv.setStatus("No changes made")
				}
			}
			return true, nil
		}
		updated, err := parseProjectNoteEditorResult(msg.Original, msg.Modified, msg.Create)
		if err != nil {
			return true, func() tea.Msg {
				return ProjectNoteMutationErrorMsg{Location: location, ProjectID: location.ProjectID, Err: err}
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
		return true, noteMutationCmd(m.appService, location, app.NoteBatchRequest{
			Operations:  []app.NoteMutationRequest{operation},
			UndoAction:  undoAction,
			UndoContext: updated.Title,
		}, undoAction+": "+updated.Title)

	case ShowNotesManagerMsg:
		hadNotesManager := false
		if m.manageHub != nil {
			_, hadNotesManager = m.manageHub.ActiveModuleView().(*NotesViewModel)
		}
		cmd := m.activateManageHub(route.ManageParams{Module: route.ManageNotes, NotesScope: msg.Scope})
		if hadNotesManager {
			if nv := m.getActiveNotesView(); nv != nil {
				nv.RefreshWorkspace(m.availableProjects()...)
			}
		}
		m.selectActiveNote(msg.Focus)
		if msg.MoveSource.valid() {
			if nv := m.getActiveNotesView(); nv != nil {
				return true, tea.Batch(cmd, nv.beginMovePickerForSource(msg.MoveSource))
			}
		}
		return true, cmd

	}
	return false, nil
}

func (m *AppModel) selectActiveNote(identity NoteRowIdentity) {
	if identity.Filename == "" || identity.Location.Scope == "" || m.manageHub == nil {
		return
	}
	notes, ok := m.manageHub.ActiveModuleView().(*NotesViewModel)
	if !ok {
		return
	}
	for index, row := range notes.visibleRows {
		if row.Identity() == identity {
			notes.cursor = index
			return
		}
	}
}

func ledgerRestoreLabel(redo bool) string {
	if redo {
		return "redo"
	}
	return "undo"
}

func (m *AppModel) reconcileNoteResult(msg ProjectNoteMutationResultMsg) tea.Cmd {
	location := normalizedNoteLocation(msg.Location, msg.ProjectID)
	feedbackCmd := m.setRecoveryFeedback(msg.SuccessMessage, msg.Result.TransactionID, false)
	nv := m.getActiveNotesView()
	if !noteViewMatchesLocation(nv, location) {
		return feedbackCmd
	}
	target := NoteRowIdentity{}
	if len(msg.Result.Results) > 0 {
		result := msg.Result.Results[len(msg.Result.Results)-1]
		target = NoteRowIdentity{
			Location: normalizedNoteLocation(result.Location, result.ProjectID),
			Filename: result.Filename,
		}
	}
	nv.loadNotes()
	if target.Filename != "" {
		for i, row := range nv.visibleRows {
			if row.Identity() == target {
				nv.cursor = i
				break
			}
		}
	}
	nv.setStatus(msg.SuccessMessage)
	return feedbackCmd
}
