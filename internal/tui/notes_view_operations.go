package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *NotesViewModel) viewCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	cmd, err := notePreviewCmd(m.notes[m.cursor])
	if err != nil {
		m.setErrorStatus(err.Error())
		return nil
	}
	if cmd == nil {
		m.setStatus("glow not available for viewing")
		return nil
	}
	m.setStatus("Opening in glow...")
	return cmd
}

// notePreviewCmd renders one note into a temporary file and pages it through
// glow. A nil command with a nil error means glow is not installed.
func notePreviewCmd(note *models.Note) (tea.Cmd, error) {
	if note == nil {
		return nil, fmt.Errorf("note is unavailable")
	}
	if note.ParseError != "" {
		return nil, fmt.Errorf("cannot preview — note has a parse error; press e to edit")
	}
	if !external.GlowAvailable() {
		return nil, nil
	}

	content := fmt.Sprintf("# %s\n\n", note.Title)
	content += note.Body

	tmpFile, err := os.CreateTemp("", "tt-note-*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create note preview: %w", err)
	}
	tmpPath := tmpFile.Name()
	_, writeErr := tmpFile.WriteString(content)
	closeErr := tmpFile.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to write note preview")
	}

	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		return NotesRefreshMsg{}
	}), nil
}

func (m *NotesViewModel) openCurrentNoteReference() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]

	status, err := openNoteReference(note, m.effectiveDir(), "")
	if err != nil {
		return errorCmd("%s", err)
	}

	m.setStatus(status)
	return defaultStatusTimeout()
}

// openNoteReference writes a read-only snapshot of one note and hands it to the
// external editor, returning the status the caller should surface. An editor
// that cannot open a detached snapshot is reported as a status, not an error,
// so the note view keeps its existing non-fatal message.
func openNoteReference(note *models.Note, dir, externalEditor string) (string, error) {
	if note == nil {
		return "", fmt.Errorf("note is unavailable")
	}
	path, err := storage.NoteFilePathIn(dir, note.Filename)
	if err != nil {
		return "", fmt.Errorf("invalid note path: %w", err)
	}
	content, err := noteReferenceMarkdown(note, path)
	if err != nil {
		return "", fmt.Errorf("failed to prepare note reference: %w", err)
	}
	snapshotPath, err := createReferenceSnapshotFile(content, "tt-note-reference-", ".md")
	if err != nil {
		return "", fmt.Errorf("failed to create note reference: %w", err)
	}
	result, err := openReferenceFile(externalEditor, snapshotPath)
	if err != nil {
		if result.Status == external.ReferenceOpenStatusUnsupportedEditor {
			return err.Error(), nil
		}
		return "", err
	}
	return "Opened note reference copy", nil
}

func noteReferenceMarkdown(note *models.Note, path string) (string, error) {
	if note.ParseError != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return formatRawNoteReferenceMarkdown(note, string(raw)), nil
	}

	return external.FormatNoteReferenceMarkdownWithTasks(note, nil), nil
}

func formatRawNoteReferenceMarkdown(note *models.Note, raw string) string {
	title := note.Title
	if title == "" {
		title = models.TitleFromFilename(note.Filename)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Note: %s\n\n", title)
	fmt.Fprintf(&b, "> **%s**\n\n", external.ReferenceSnapshotWarning)
	b.WriteString("This note could not be parsed. Raw file content is shown below for reference.\n\n")
	b.WriteString("## Parse Error\n\n")
	b.WriteString(note.ParseError)
	b.WriteString("\n\n")
	b.WriteString("## Raw File Content\n\n")
	b.WriteString(raw)
	if !strings.HasSuffix(raw, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func (m *NotesViewModel) editCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]

	var (
		editor    string
		editorErr error
	)
	if m.combined {
		editor, editorErr = external.ResolveNoteEditor()
	} else {
		editor, editorErr = external.ResolveEditor("")
	}
	if editorErr != nil {
		return errorCmd("%s", editorErr)
	}
	location := m.noteLocationFor(note)
	if location.Scope == "" {
		return errorCmd("cannot edit note without a location")
	}
	return noteEditCmd(note, location, m.effectiveDir(), editor)
}

// noteEditCmd opens one note in the editor. A malformed note is edited as raw
// file content so its frontmatter can be repaired by hand.
func noteEditCmd(note *models.Note, location app.NoteLocation, dir, editor string) tea.Cmd {
	if note.ParseError != "" {
		path, pathErr := storage.NoteFilePathIn(dir, note.Filename)
		if pathErr != nil {
			return errorCmd("invalid note path: %w", pathErr)
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return errorCmd("failed to read malformed note: %w", readErr)
		}
		return startNoteEditorWithContent(editor, location, note, string(raw), false)
	}
	return startNoteEditor(editor, location, note, false)
}

func (m *NotesViewModel) createNote() tea.Cmd {
	if m.isGlobal {
		return func() tea.Msg {
			return ShowGlobalNoteCreatorMsg{}
		}
	}
	return func() tea.Msg {
		return ShowNoteCreatorMsg{Project: m.project}
	}
}

func (m *NotesViewModel) beginDestinationPicker() {
	m.destinations = []app.NoteLocation{app.GlobalNoteLocation()}
	for _, project := range m.creationProjects {
		m.destinations = append(m.destinations, app.ProjectNoteLocation(project.ID))
	}
	m.destinationCursor = 0
	m.destinationScroll = 0
	for i, location := range m.destinations {
		if location == m.preferred {
			m.destinationCursor = i
			break
		}
	}
	m.destinationPicker = true
}

func (m *NotesViewModel) createNoteAt(location app.NoteLocation) tea.Cmd {
	editor, err := external.ResolveNoteEditor()
	if err != nil {
		m.setErrorStatus(err.Error())
		return nil
	}
	return startNoteEditor(editor, location, models.NewNote("Untitled Note"), true)
}

func (m *NotesViewModel) deleteCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}

	note := m.notes[m.cursor]
	m.confirm.Show("delete", fmt.Sprintf("Delete '%s'?", note.Title))
	m.pendingDeleteIdx = m.cursor
	return nil
}

func (m *NotesViewModel) performDelete() tea.Cmd {
	if m.pendingDeleteIdx >= len(m.notes) {
		return nil
	}
	note := m.notes[m.pendingDeleteIdx]
	location := m.noteLocationFor(note)
	return noteMutationCmd(m.noteService, location, app.NoteBatchRequest{
		Operations:  []app.NoteMutationRequest{{Kind: app.NoteDelete, Location: location, Note: note}},
		UndoAction:  "Delete Note",
		UndoContext: note.Title,
	}, "Deleted: "+note.Title, false)
}

func (m *NotesViewModel) undoDelete() tea.Cmd {
	location := m.noteLocation()
	if m.combined {
		location = app.GlobalNoteLocation()
	} else if location.Scope == "" {
		return nil
	}
	return noteLedgerRestoreCmd(m.noteLedger, location, m.noteLedgerScope(), false)
}

func (m *NotesViewModel) redoNoteMutation() tea.Cmd {
	location := m.noteLocation()
	if m.combined {
		location = app.GlobalNoteLocation()
	} else if location.Scope == "" {
		return nil
	}
	return noteLedgerRestoreCmd(m.noteLedger, location, m.noteLedgerScope(), true)
}

func (m *NotesViewModel) performBulkDelete() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}

	location := m.noteLocation()
	operations := make([]app.NoteMutationRequest, len(selected))
	for i, note := range selected {
		operations[i] = app.NoteMutationRequest{Kind: app.NoteDelete, Location: location, Note: note}
	}
	return noteMutationCmd(m.noteService, location, app.NoteBatchRequest{
		Operations:  operations,
		UndoAction:  "Delete Notes",
		UndoContext: fmt.Sprintf("%d notes", len(selected)),
	}, fmt.Sprintf("Deleted %d notes", len(selected)), true)
}

func (m *NotesViewModel) cycleSortMode() {
	m.sortMode = (m.sortMode + 1) % 4
	m.loadNotes()
	m.setStatus(fmt.Sprintf("Sort: %s", m.sortMode))
}

// copyNoteContent is a generic helper for clipboard copy operations. It handles
// both single-note and bulk (multi-select) copying, reducing duplication across
// the three copy commands.
func (m *NotesViewModel) copyNoteContent(
	singleFn func(*models.Note) (string, error),
	bulkFn func([]*models.Note) (string, error),
	singleMsg, bulkMsg string,
) tea.Cmd {
	if !m.multiSelectMode && len(m.notes) > 0 && m.cursor < len(m.notes) && m.notes[m.cursor].ParseError != "" {
		m.setErrorStatus("cannot copy — note has parse error. Press e to edit.")
		return nil
	}
	if m.multiSelectMode && m.selectedNoteCount() > 0 {
		selected := m.getSelectedNotesList()
		_, err := bulkFn(selected)
		if err != nil {
			m.setErrorStatus("Copy failed: " + err.Error())
		} else {
			m.setStatus(fmt.Sprintf(bulkMsg, len(selected)))
		}
		return nil
	}
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	_, err := singleFn(m.notes[m.cursor])
	if err != nil {
		m.setErrorStatus("Copy failed: " + err.Error())
	} else {
		m.setStatus(singleMsg)
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
