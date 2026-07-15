package tui

import (
	"fmt"
	"os"
	"strings"

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
		m.reconcileNoteResult(msg)
		return true, nil

	case ProjectNoteMutationErrorMsg:
		location := normalizedNoteLocation(msg.Location, msg.ProjectID)
		if nv := m.getActiveNotesView(); noteViewMatchesLocation(nv, location) {
			nv.setErrorStatus(msg.Err.Error())
			return true, nil
		}
		return true, func() tea.Msg { return ErrorMsg{Err: msg.Err} }

	case ProjectNoteLedgerResultMsg:
		location := normalizedNoteLocation(msg.Location, msg.ProjectID)
		if nv := m.getActiveNotesView(); noteViewMatchesLocation(nv, location) {
			nv.loadNotes()
			verb := "Undid"
			if msg.Redo {
				verb = "Redid"
			}
			nv.setStatus(fmt.Sprintf("%s: %s", verb, msg.Action))
		}
		m.refreshNotesSearchState()
		return true, nil

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
		}, undoAction+": "+updated.Title, false)

	case ShowNotesMsg:
		if msg.Project != nil {
			m.showView(ViewNotes, NewNotesViewModelWithService(msg.Project, m.appService, workspaceLedgerFromConfig(m.runtime), m.runtime.Machine()))
		}
		return true, nil

	case ShowGlobalNotesMsg:
		dir := storage.GlobalNotesDir(m.runtime.DataDir())
		m.showView(ViewGlobalNotes, NewGlobalNotesViewModelWithService(dir, m.appService, workspaceLedgerFromConfig(m.runtime), m.runtime.Machine()))
		return true, nil

	case ShowGlobalNoteCreatorMsg:
		return m.handleCreateGlobalNote()

	case ShowGlobalNoteMoveMsg:
		// Open a project picker for moving the note
		projects := m.activeProjects()
		if len(projects) == 0 {
			m.setInfoStatus("No projects available")
			return true, nil
		}
		m.pendingGlobalNoteMove = &pendingNoteMove{Source: msg.Source}
		m.showView(ViewProjectSelector, NewGlobalNoteMoveProjectSelectorModel(projects))
		return true, nil

	case GlobalNoteMoveDoneMsg:
		if msg.ProjectID == "" {
			m.pendingGlobalNoteMove = nil
			m.popView()
			return true, nil
		}
		if m.pendingGlobalNoteMove != nil {
			source := m.pendingGlobalNoteMove.Source
			m.pendingGlobalNoteMove = nil
			m.popView()
			note := source.noteCopy()
			return true, noteMutationCmd(m.appService, app.GlobalNoteLocation(), app.NoteBatchRequest{
				Operations: []app.NoteMutationRequest{{
					Kind:        app.NoteMove,
					Location:    source.Location,
					Destination: app.ProjectNoteLocation(msg.ProjectID),
					Note:        note,
				}},
				UndoAction:  "Move Note",
				UndoContext: note.Title,
			}, fmt.Sprintf("Moved '%s' to %s", note.Title, msg.ProjectName), false)
		}
		return true, nil

	case ShowNotesSearchMsg:
		if msg.Project != nil {
			m.showView(ViewNotesSearch, NewNotesSearchModel(msg.Project))
		}
		return true, nil

	case NotesSearchSelectMsg:
		// Pop search view, then select the note in the notes view
		m.popView()
		// Find the note index in the notes view and set cursor
		if v := m.views.Get(ViewNotes); v != nil {
			if notesView, ok := v.(*NotesViewModel); ok {
				for i, n := range notesView.notes {
					if n.Filename == msg.Note.Filename {
						notesView.cursor = i
						break
					}
				}
			}
		}
		return true, nil

	case BulkNoteLinkMsg:
		if msg.Project != nil && len(msg.Notes) > 0 {
			// Toggle a copied preview; the result is applied to every selected
			// note as one application-layer batch when the overlay closes.
			m.pendingBulkNotes = newNoteMutationRefs(app.ProjectNoteLocation(msg.Project.ID), msg.Notes)
			m.showView(ViewNoteTaskLinker, NewNoteTaskLinkerModelWithService(msg.Project, msg.Notes[0], m.appService, true))
		}
		return true, nil

	case ShowNoteTaskLinkerMsg:
		if msg.Project != nil && msg.Note != nil {
			m.showView(ViewNoteTaskLinker, NewNoteTaskLinkerModelWithService(msg.Project, msg.Note, m.appService, false))
		}
		return true, nil

	case CloseNoteTaskLinkerMsg:
		m.popView()
		m.pendingBulkNotes = nil
		return true, nil

	case NoteTaskLinkedMsg:
		m.popView()
		// Forward to notes view to refresh
		cmd := m.views.Update(ViewNotes, msg)
		return true, cmd

	case BulkNoteLinkDoneMsg:
		m.popView()
		if msg.Canceled || len(m.pendingBulkNotes) == 0 || len(msg.LinkChanges) == 0 {
			m.pendingBulkNotes = nil
			return true, nil
		}
		projectID := msg.ProjectID
		location := app.ProjectNoteLocation(projectID)
		operations := make([]app.NoteMutationRequest, 0, len(m.pendingBulkNotes))
		for _, ref := range m.pendingBulkNotes {
			updated := ref.noteCopy()
			for taskID, linked := range msg.LinkChanges {
				if linked {
					updated.AddLink(taskID)
				} else {
					updated.RemoveLink(taskID)
				}
			}
			operations = append(operations, app.NoteMutationRequest{Kind: app.NoteUpdate, Location: location, Note: updated})
		}
		return true, projectNoteMutationCmd(m.appService, projectID, app.NoteBatchRequest{
			Operations:  operations,
			UndoAction:  "Link Notes to Tasks",
			UndoContext: fmt.Sprintf("%d notes", len(operations)),
		}, fmt.Sprintf("Updated links on %d notes", len(operations)), true)

	case ShowNoteCreatorMsg:
		if msg.Project != nil {
			// Edit a temporary note and create it only after the application
			// transaction accepts the final content.
			return m.handleCreateNote(msg.Project)
		}
		return true, nil
	}
	return false, nil
}

func (m *AppModel) reconcileNoteResult(msg ProjectNoteMutationResultMsg) {
	location := normalizedNoteLocation(msg.Location, msg.ProjectID)
	nv := m.getActiveNotesView()
	if !noteViewMatchesLocation(nv, location) {
		m.setSuccessStatus(msg.SuccessMessage)
		m.refreshNotesSearchState()
		return
	}
	preferred := ""
	if len(msg.Result.Results) > 0 {
		preferred = msg.Result.Results[len(msg.Result.Results)-1].Filename
	}
	nv.loadNotes()
	if msg.ClearSelection {
		nv.selectedNotes = make(map[string]bool)
		nv.multiSelectMode = false
		m.pendingBulkNotes = nil
	}
	if preferred != "" {
		for i, note := range nv.notes {
			if note.Filename == preferred {
				nv.cursor = i
				break
			}
		}
	}
	nv.setStatus(msg.SuccessMessage)
	m.refreshNotesSearchState()
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
		return view.isGlobal
	}
	return !view.isGlobal && view.project != nil && view.project.ID == location.ProjectID
}

// handleCreateGlobalNote creates a new global note and opens it in the editor.
func (m *AppModel) handleCreateGlobalNote() (bool, tea.Cmd) {
	note := models.NewNote("Untitled Note")
	editor, editorErr := external.ResolveEditor(m.runtime.Machine().GetEditor())
	if editorErr != nil {
		return true, errorCmd("%s", editorErr)
	}
	return true, startNoteEditor(editor, app.GlobalNoteLocation(), note, true)
}

// handleCreateNote creates a new note and opens it in the editor.
func (m *AppModel) handleCreateNote(project *models.Project) (bool, tea.Cmd) {
	note := models.NewNote("Untitled Note")
	editor, editorErr := external.ResolveEditor(m.runtime.Machine().GetEditor())
	if editorErr != nil {
		return true, errorCmd("%s", editorErr)
	}
	return true, startProjectNoteEditor(editor, project.ID, note, true)
}

// startProjectNoteEditor opens a serialized copy in a temporary file. The
// callback reads and removes only that temporary file, then returns copied data
// for Update to validate and commit through the application layer.
func startProjectNoteEditor(editor, projectID string, note *models.Note, create bool) tea.Cmd {
	return startNoteEditor(editor, app.ProjectNoteLocation(projectID), note, create)
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
		updated.Filename = original.Filename
		updated.RawRevision = original.RawRevision
	}
	return updated, nil
}
