package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *NotesViewModel) viewCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]

	if !external.GlowAvailable() {
		m.statusMessage = "glow not available for viewing"
		return nil
	}

	content := fmt.Sprintf("# %s\n\n", note.Title)
	if len(note.Tags) > 0 {
		content += fmt.Sprintf("Tags: %s\n\n", strings.Join(note.Tags, ", "))
	}
	content += note.Body

	tmpFile, err := os.CreateTemp("", "tt-note-*.md")
	if err != nil {
		return nil
	}
	tmpPath := tmpFile.Name()
	_, writeErr := tmpFile.WriteString(content)
	closeErr := tmpFile.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil
	}

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		return NotesRefreshMsg{}
	})
}

func (m *NotesViewModel) editCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]

	configEditor := ""
	if m.config != nil {
		configEditor = m.config.GetEditor()
	}
	editor := external.ResolveEditor(configEditor)
	path := storage.NoteFilePathIn(m.effectiveDir(), note.Filename)
	c := external.EditorCommand(editor, path)

	oldFilename := note.Filename
	dir := m.effectiveDir()
	return tea.ExecProcess(c, func(err error) tea.Msg {
		reloaded, loadErr := storage.LoadNoteFrom(dir, oldFilename)
		if loadErr == nil && reloaded.Title != note.Title {
			_, _ = storage.RenameNoteIn(dir, oldFilename, reloaded.Title)
		}
		return NotesRefreshMsg{}
	})
}

func (m *NotesViewModel) createNote() tea.Cmd {
	if m.isGlobal {
		return func() tea.Msg {
			return ShowGlobalNoteCreatorMsg{NotesDir: m.notesDir}
		}
	}
	return func() tea.Msg {
		return ShowNoteCreatorMsg{Project: m.project}
	}
}

func (m *NotesViewModel) deleteCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}

	note := m.notes[m.cursor]
	m.confirm.Show("delete", fmt.Sprintf("Delete '%s'? (y/n)", note.Title))
	m.statusMessage = m.confirm.Prompt()
	m.pendingDeleteIdx = m.cursor
	return nil
}

func (m *NotesViewModel) performDelete() tea.Cmd {
	if m.pendingDeleteIdx >= len(m.notes) {
		return nil
	}
	note := m.notes[m.pendingDeleteIdx]

	if err := storage.DeleteNoteFrom(m.effectiveDir(), note.Filename); err != nil {
		m.statusMessage = "Failed to delete: " + err.Error()
		return nil
	}

	m.lastDeleted = note
	m.lastBulkDeleted = nil
	m.statusMessage = "Deleted: " + note.Title
	m.loadNotes()
	return func() tea.Msg { return NotesRefreshMsg{} }
}

// batchSaveNotes saves a slice of notes and returns the count of failures.
func (m *NotesViewModel) batchSaveNotes(notes []*models.Note) int {
	dir := m.effectiveDir()
	var failed int
	for _, note := range notes {
		if err := storage.SaveNoteTo(dir, note); err != nil {
			failed++
		}
	}
	return failed
}

func (m *NotesViewModel) undoDelete() tea.Cmd {
	// Priority: delete > tag > link
	if m.lastBulkDeleted != nil {
		count := len(m.lastBulkDeleted)
		failed := m.batchSaveNotes(m.lastBulkDeleted)
		m.lastBulkDeleted = nil
		if failed > 0 {
			m.statusMessage = fmt.Sprintf("Restored %d notes (%d failed)", count-failed, failed)
		} else {
			m.statusMessage = fmt.Sprintf("Restored %d notes", count)
		}
		m.loadNotes()
		return nil
	}

	if m.lastDeleted != nil {
		if err := storage.SaveNoteTo(m.effectiveDir(), m.lastDeleted); err != nil {
			m.statusMessage = "Failed to restore: " + err.Error()
			return nil
		}
		m.statusMessage = "Restored: " + m.lastDeleted.Title
		m.lastDeleted = nil
		m.loadNotes()
		return nil
	}

	if len(m.lastTagUndo) > 0 {
		notes := make([]*models.Note, len(m.lastTagUndo))
		for i, entry := range m.lastTagUndo {
			entry.Note.Tags = entry.OldTags
			entry.Note.ModifiedAt = time.Now()
			notes[i] = entry.Note
		}
		count := len(m.lastTagUndo)
		failed := m.batchSaveNotes(notes)
		m.lastTagUndo = nil
		if failed > 0 {
			m.statusMessage = fmt.Sprintf("Undid tags on %d notes (%d failed)", count, failed)
		} else {
			m.statusMessage = fmt.Sprintf("Undid tags on %d notes", count)
		}
		m.loadNotes()
		return nil
	}

	if len(m.lastLinkUndo) > 0 {
		notes := make([]*models.Note, len(m.lastLinkUndo))
		for i, entry := range m.lastLinkUndo {
			entry.Note.LinkedTasks = entry.OldLinks
			entry.Note.ModifiedAt = time.Now()
			notes[i] = entry.Note
		}
		count := len(m.lastLinkUndo)
		failed := m.batchSaveNotes(notes)
		m.lastLinkUndo = nil
		if failed > 0 {
			m.statusMessage = fmt.Sprintf("Undid links on %d notes (%d failed)", count, failed)
		} else {
			m.statusMessage = fmt.Sprintf("Undid links on %d notes", count)
		}
		m.loadNotes()
		return nil
	}

	m.statusMessage = "Nothing to undo"
	return nil
}

func (m *NotesViewModel) linkCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	return func() tea.Msg {
		return ShowNoteTaskLinkerMsg{Project: m.project, Note: note}
	}
}

func (m *NotesViewModel) showTagSelector() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	return func() tea.Msg {
		return ShowNoteTagSelectorMsg{
			Note:        note,
			Project:     m.project,
			NotesDir:    m.notesDir,
			IsGlobal:    m.isGlobal,
			CurrentTags: note.Tags,
		}
	}
}

func (m *NotesViewModel) togglePin() {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return
	}
	note := m.notes[m.cursor]
	note.Pinned = !note.Pinned
	note.ModifiedAt = time.Now()
	if err := storage.SaveNoteTo(m.effectiveDir(), note); err != nil {
		m.statusMessage = "Failed to save: " + err.Error()
		return
	}
	if note.Pinned {
		m.statusMessage = "Pinned: " + note.Title
	} else {
		m.statusMessage = "Unpinned: " + note.Title
	}
	m.loadNotes()
}

func (m *NotesViewModel) performBulkDelete() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}

	dir := m.effectiveDir()
	var deleted []*models.Note
	var errors int
	for _, note := range selected {
		if err := storage.DeleteNoteFrom(dir, note.Filename); err == nil {
			deleted = append(deleted, note)
		} else {
			errors++
		}
	}

	m.selectedNotes = make(map[string]bool)
	m.multiSelectMode = false

	if errors > 0 {
		m.statusMessage = fmt.Sprintf("Deleted %d/%d notes (%d failed)", len(deleted), len(selected), errors)
	} else {
		m.statusMessage = fmt.Sprintf("Deleted %d notes", len(deleted))
	}

	m.lastBulkDeleted = deleted
	m.lastDeleted = nil
	m.loadNotes()
	return func() tea.Msg { return NotesRefreshMsg{} }
}

func (m *NotesViewModel) bulkTagNotes() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}
	return func() tea.Msg {
		return BulkNoteTagMsg{
			Notes:    selected,
			Project:  m.project,
			NotesDir: m.notesDir,
			IsGlobal: m.isGlobal,
		}
	}
}

func (m *NotesViewModel) bulkLinkNotes() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}
	return func() tea.Msg {
		return BulkNoteLinkMsg{
			Notes:   selected,
			Project: m.project,
		}
	}
}

func (m *NotesViewModel) cycleSortMode() {
	m.sortMode = (m.sortMode + 1) % 4
	m.loadNotes()
	m.statusMessage = fmt.Sprintf("Sort: %s", m.sortMode)
}

// copyNoteContent is a generic helper for clipboard copy operations. It handles
// both single-note and bulk (multi-select) copying, reducing duplication across
// the three copy commands.
func (m *NotesViewModel) copyNoteContent(
	singleFn func(*models.Note) (string, error),
	bulkFn func([]*models.Note) (string, error),
	singleMsg, bulkMsg string,
) tea.Cmd {
	if m.multiSelectMode && m.selectedNoteCount() > 0 {
		selected := m.getSelectedNotesList()
		_, err := bulkFn(selected)
		if err != nil {
			m.statusMessage = "Copy failed: " + err.Error()
			m.statusIsError = true
		} else {
			m.statusMessage = fmt.Sprintf(bulkMsg, len(selected))
			m.statusIsError = false
		}
		return nil
	}
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	_, err := singleFn(m.notes[m.cursor])
	if err != nil {
		m.statusMessage = "Copy failed: " + err.Error()
		m.statusIsError = true
	} else {
		m.statusMessage = singleMsg
		m.statusIsError = false
	}
	return nil
}

func (m *NotesViewModel) copyNoteTitle() tea.Cmd {
	return m.copyNoteContent(
		external.CopyNoteTitle, external.CopyNoteTitles,
		"Copied title to clipboard", "Copied %d titles to clipboard",
	)
}

func (m *NotesViewModel) copyNoteBody() tea.Cmd {
	return m.copyNoteContent(
		external.CopyNoteBody, external.CopyNoteBodies,
		"Copied body to clipboard", "Copied %d note bodies to clipboard",
	)
}

func (m *NotesViewModel) copyNoteTitleAndBody() tea.Cmd {
	return m.copyNoteContent(
		external.CopyNoteTitleAndBody, external.CopyNoteTitlesAndBodies,
		"Copied title and body to clipboard", "Copied %d notes to clipboard",
	)
}

func (m *NotesViewModel) moveNoteToProject() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	return func() tea.Msg {
		return ShowGlobalNoteMoveMsg{Note: note, SourceDir: m.notesDir}
	}
}
