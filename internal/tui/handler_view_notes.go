package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleNotesMessages handles note-related view messages.
// Returns true if the message was handled.
func (m *AppModel) handleNotesMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowNotesMsg:
		if msg.Project != nil {
			m.showView(ViewNotes, NewNotesViewModel(msg.Project, m.config))
		}
		return true, nil

	case CloseNotesMsg:
		m.popView()
		return true, nil

	case ShowGlobalNotesMsg:
		dir := storage.GlobalNotesDir(m.config.DataDir)
		m.showView(ViewGlobalNotes, NewGlobalNotesViewModel(dir, m.config))
		return true, nil

	case CloseGlobalNotesMsg:
		m.popToAnchorView()
		return true, nil

	case ShowGlobalNoteCreatorMsg:
		return m.handleCreateGlobalNote(msg.NotesDir)

	case ShowGlobalNoteMoveMsg:
		// Open a project picker for moving the note
		projects := m.config.GetActiveProjects()
		if len(projects) == 0 {
			m.statusMessage = "No projects available"
			return true, nil
		}
		m.pendingGlobalNoteMove = &pendingNoteMove{Note: msg.Note, SourceDir: msg.SourceDir}
		m.showView(ViewProjectSelector, NewGlobalNoteMoveProjectSelectorModel(projects))
		return true, nil

	case GlobalNoteMoveDoneMsg:
		if msg.Project == nil {
			m.pendingGlobalNoteMove = nil
			m.popView()
			return true, nil
		}
		if m.pendingGlobalNoteMove != nil {
			note := m.pendingGlobalNoteMove.Note
			sourceDir := m.pendingGlobalNoteMove.SourceDir
			m.pendingGlobalNoteMove = nil
			m.popView()
			// Copy note to project, delete from global
			if err := storage.SaveNote(msg.Project, note); err != nil {
				m.statusMessage = "Failed to move note: " + err.Error()
				return true, nil
			}
			if err := storage.DeleteNoteFrom(sourceDir, note.Filename); err != nil {
				m.statusMessage = "Moved but failed to delete original: " + err.Error()
				return true, nil
			}
			m.statusMessage = fmt.Sprintf("Moved '%s' to %s", note.Title, msg.Project.Name)
			// Refresh global notes view
			if nv := m.getActiveNotesView(); nv != nil {
				nv.loadNotes()
			}
		}
		return true, nil

	case ShowNotesSearchMsg:
		if msg.Project != nil {
			m.showView(ViewNotesSearch, NewNotesSearchModel(msg.Project))
		}
		return true, nil

	case CloseNotesSearchMsg:
		m.popView()
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
			// Open linker for first note; result applied to all via pendingBulkNotes
			m.pendingBulkNotes = msg.Notes
			m.showView(ViewNoteTaskLinker, NewNoteTaskLinkerModel(msg.Project, msg.Notes[0]))
		}
		return true, nil

	case ShowNoteTaskLinkerMsg:
		if msg.Project != nil && msg.Note != nil {
			m.showView(ViewNoteTaskLinker, NewNoteTaskLinkerModel(msg.Project, msg.Note))
		}
		return true, nil

	case CloseNoteTaskLinkerMsg:
		m.popView()
		m.pendingBulkNotes = nil
		return true, nil

	case NoteTaskLinkedMsg:
		m.popView()
		// If in bulk mode, link all pending notes to the same task
		if len(m.pendingBulkNotes) > 0 && msg.TaskID != "" {
			if v := m.views.Get(ViewNotes); v != nil {
				if notesView, ok := v.(*NotesViewModel); ok {
					// Snapshot for undo (all notes including first)
					var undoEntries []noteLinkUndoEntry
					for _, note := range m.pendingBulkNotes {
						oldLinks := make([]string, len(note.LinkedTasks))
						copy(oldLinks, note.LinkedTasks)
						undoEntries = append(undoEntries, noteLinkUndoEntry{Note: note, OldLinks: oldLinks})
					}
					var failed int
					for i := 1; i < len(m.pendingBulkNotes); i++ {
						note := m.pendingBulkNotes[i]
						note.AddLink(msg.TaskID)
						if err := storage.SaveNote(notesView.project, note); err != nil {
							failed++
						}
					}
					total := len(m.pendingBulkNotes)
					if failed > 0 {
						m.statusMessage = fmt.Sprintf("Linked %d notes (%d failed)", total, failed)
					} else {
						m.statusMessage = fmt.Sprintf("Linked %d notes", total)
					}
					notesView.lastLinkUndo = undoEntries
					notesView.lastTagUndo = nil
				}
			}
			m.pendingBulkNotes = nil
		}
		// Forward to notes view to refresh
		cmd := m.views.Update(ViewNotes, msg)
		return true, cmd

	case ShowNoteCreatorMsg:
		if msg.Project != nil {
			// Create a new note with a default title and open in editor
			return m.handleCreateNote(msg.Project)
		}
		return true, nil
	}
	return false, nil
}

// handleCreateGlobalNote creates a new global note and opens it in the editor.
func (m *AppModel) handleCreateGlobalNote(notesDir string) (bool, tea.Cmd) {
	note := models.NewNote("Untitled Note")
	if err := storage.SaveNoteTo(notesDir, note); err != nil {
		return true, errorCmd("failed to create note: %w", err)
	}

	editor := external.ResolveEditor(m.config.Editor)
	path := storage.NoteFilePathIn(notesDir, note.Filename)
	c := external.EditorCommand(editor, path)

	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		reloaded, loadErr := storage.LoadNoteFrom(notesDir, note.Filename)
		if loadErr == nil && reloaded.Title == "Untitled Note" && strings.TrimSpace(reloaded.Body) == "" {
			_ = storage.DeleteNoteFrom(notesDir, note.Filename)
		} else if loadErr == nil && reloaded.Title != "Untitled Note" {
			_, _ = storage.RenameNoteIn(notesDir, note.Filename, reloaded.Title)
		}
		return NotesRefreshMsg{}
	})
}

// handleCreateNote creates a new note and opens it in the editor.
func (m *AppModel) handleCreateNote(project *models.Project) (bool, tea.Cmd) {
	note := models.NewNote("Untitled Note")
	if err := storage.SaveNote(project, note); err != nil {
		return true, errorCmd("failed to create note: %w", err)
	}

	editor := external.ResolveEditor(m.config.Editor)
	path := storage.NoteFilePath(project, note.Filename)
	c := external.EditorCommand(editor, path)

	return true, tea.ExecProcess(c, func(err error) tea.Msg {
		reloaded, loadErr := storage.LoadNote(project, note.Filename)
		if loadErr == nil && reloaded.Title == "Untitled Note" && strings.TrimSpace(reloaded.Body) == "" {
			_ = storage.DeleteNote(project, note.Filename)
		} else if loadErr == nil && reloaded.Title != "Untitled Note" {
			_, _ = storage.RenameNoteIn(storage.NotesDir(project), note.Filename, reloaded.Title)
		}
		return NotesRefreshMsg{}
	})
}
