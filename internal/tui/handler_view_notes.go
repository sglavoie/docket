package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
		return true, func() tea.Msg { return ErrorMsg{Err: msg.Err} }

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
			cmds = append(cmds, statusTimeout(5*time.Second))
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
		cmd := m.activateManageHub(ManageParams{Module: ManageNotes, NotesScope: msg.Scope})
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

func normalizedNoteLocation(location app.NoteLocation, projectID string) app.NoteLocation {
	if location.Scope != "" {
		return location
	}
	if projectID != "" {
		return app.ProjectNoteLocation(projectID)
	}
	return app.GlobalNoteLocation()
}

func noteViewMatchesLocation(view *NotesViewModel, location app.NoteLocation) bool {
	if view == nil {
		return false
	}
	if location.Scope == app.NoteScopeGlobal {
		_, ok := view.owners[app.GlobalNoteLocation()]
		return ok
	}
	_, ok := view.owners[location]
	return ok
}

func startNoteEditor(editor string, location app.NoteLocation, note *models.Note, create bool) tea.Cmd {
	original := cloneNoteForCommand(note)
	content, err := storage.RenderNoteContent(original)
	if err != nil {
		return errorCmd("failed to render note: %w", err)
	}
	return startNoteEditorWithContent(editor, location, original, content, create)
}

func startNoteEditorWithContent(editor string, location app.NoteLocation, note *models.Note, content string, create bool) tea.Cmd {
	original := cloneNoteForCommand(note)
	tmpPath, err := external.CreateTempFileWithContent(content, "tt-note-edit-", ".md")
	if err != nil {
		return errorCmd("failed to create temporary note: %w", err)
	}
	command := external.EditorCommand(editor, tmpPath)
	return tea.ExecProcess(command, func(execErr error) tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()
		if execErr != nil {
			return ProjectNoteEditorDoneMsg{Location: location, ProjectID: location.ProjectID, Original: original, OriginalContent: content, Create: create, Err: fmt.Errorf("editor failed: %w", execErr)}
		}
		modified, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return ProjectNoteEditorDoneMsg{Location: location, ProjectID: location.ProjectID, Original: original, OriginalContent: content, Create: create, Err: fmt.Errorf("failed to read temporary note: %w", readErr)}
		}
		return ProjectNoteEditorDoneMsg{Location: location, ProjectID: location.ProjectID, Original: original, OriginalContent: content, Modified: string(modified), Create: create}
	})
}

func parseProjectNoteEditorResult(original *models.Note, modified string, create bool) (*models.Note, error) {
	if original == nil {
		return nil, fmt.Errorf("edited note has no source")
	}
	updated, err := storage.ParseNoteContent(original.Filename, modified)
	if err != nil {
		return nil, fmt.Errorf("failed to parse edited note: %w", err)
	}
	if updated.ParseError != "" {
		return nil, fmt.Errorf("edited note has invalid frontmatter: %s (changes not saved)", updated.ParseError)
	}
	if strings.TrimSpace(updated.Title) == "" {
		return nil, fmt.Errorf("edited note has empty title (changes not saved)")
	}
	substituted := strings.TrimSuffix(original.Filename, ".md")
	if original.Title != "" && original.Title != substituted && updated.Title == substituted {
		return nil, fmt.Errorf("edited note has empty title (changes not saved)")
	}
	if create {
		updated.Filename = ""
		updated.RawRevision = ""
	} else {
		updated.CreatedAt = original.CreatedAt
		updated.ModifiedAt = time.Now()
		updated.Filename = original.Filename
		updated.RawRevision = original.RawRevision
	}
	return updated, nil
}
