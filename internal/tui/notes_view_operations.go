package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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
	note := m.notes[m.cursor]

	if note.ParseError != "" {
		m.setErrorStatus("cannot preview — note has parse error. Press e to edit.")
		return nil
	}

	if !external.GlowAvailable() {
		m.setStatus("glow not available for viewing")
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

	m.setStatus("Opening in glow...")
	c := exec.Command("sh", "-c", fmt.Sprintf("glow --style=dark '%s' | less -R", tmpPath))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		_ = os.Remove(tmpPath)
		return NotesRefreshMsg{}
	})
}

func (m *NotesViewModel) openCurrentNoteReference() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]

	path, err := storage.NoteFilePathIn(m.effectiveDir(), note.Filename)
	if err != nil {
		return errorCmd("invalid note path: %w", err)
	}

	content, err := m.formatNoteReferenceMarkdown(note, path)
	if err != nil {
		return errorCmd("failed to prepare note reference: %w", err)
	}

	snapshotPath, err := createReferenceSnapshotFile(content, "tt-note-reference-", ".md")
	if err != nil {
		return errorCmd("failed to create note reference: %w", err)
	}

	externalEditor := ""
	if m.config != nil {
		externalEditor = m.config.GetExternalEditor()
	}
	result, err := openReferenceFile(externalEditor, snapshotPath)
	if err != nil {
		if result.Status == external.ReferenceOpenStatusUnsupportedEditor {
			m.setStatus(err.Error())
			return defaultStatusTimeout()
		}
		return errorCmd("%s", err)
	}

	m.setStatus("Opened note reference copy")
	return defaultStatusTimeout()
}

func (m *NotesViewModel) formatNoteReferenceMarkdown(note *models.Note, path string) (string, error) {
	if note.ParseError != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return formatRawNoteReferenceMarkdown(note, string(raw)), nil
	}

	return external.FormatNoteReferenceMarkdownWithTasks(note, m.linkedTasksForReferenceNote(note)), nil
}

func (m *NotesViewModel) linkedTasksForReferenceNote(note *models.Note) []*models.Task {
	if note == nil || len(note.LinkedTasks) == 0 {
		return nil
	}

	tasks := m.cachedTasks
	if len(tasks) == 0 && m.project != nil {
		tasks = m.project.Tasks
	}

	taskByID := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		if task != nil && task.ID != "" {
			taskByID[task.ID] = task
		}
	}

	linked := make([]*models.Task, 0, len(note.LinkedTasks))
	seen := make(map[string]bool, len(note.LinkedTasks))
	for _, id := range note.LinkedTasks {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if task := taskByID[id]; task != nil {
			linked = append(linked, task)
		}
	}
	return linked
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

	configEditor := ""
	if m.config != nil {
		configEditor = m.config.GetEditor()
	}
	editor, editorErr := external.ResolveEditor(configEditor)
	if editorErr != nil {
		return errorCmd("%s", editorErr)
	}
	location := m.noteLocation()
	if location.Scope == "" {
		return errorCmd("cannot edit note without a location")
	}
	if note.ParseError != "" {
		path, pathErr := storage.NoteFilePathIn(m.effectiveDir(), note.Filename)
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
	location := m.noteLocation()
	return noteMutationCmd(m.noteService, location, app.NoteBatchRequest{
		Operations:  []app.NoteMutationRequest{{Kind: app.NoteDelete, Location: location, Note: note}},
		UndoAction:  "Delete Note",
		UndoContext: note.Title,
	}, "Deleted: "+note.Title, false)
}

func (m *NotesViewModel) undoDelete() tea.Cmd {
	location := m.noteLocation()
	if location.Scope == "" {
		return nil
	}
	return noteLedgerRestoreCmd(m.noteLedger, location, m.noteLedgerScope(), false)
}

func (m *NotesViewModel) redoNoteMutation() tea.Cmd {
	location := m.noteLocation()
	if location.Scope == "" {
		return nil
	}
	return noteLedgerRestoreCmd(m.noteLedger, location, m.noteLedgerScope(), true)
}

func (m *NotesViewModel) linkCurrentNote() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	if note.ParseError != "" {
		m.setErrorStatus("cannot link — note has parse error. Press e to edit.")
		return nil
	}
	return func() tea.Msg {
		return ShowNoteTaskLinkerMsg{Project: m.project, Note: note}
	}
}

func (m *NotesViewModel) showTagSelector() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	if note.ParseError != "" {
		m.setErrorStatus("cannot edit tags — note has parse error. Press e to edit.")
		return nil
	}
	return func() tea.Msg {
		return ShowNoteTagSelectorMsg{
			Source:        newNoteMutationRef(m.noteLocation(), note),
			AvailableTags: m.noteTagOptions(),
			CurrentTags:   append([]string(nil), note.Tags...),
		}
	}
}

func (m *NotesViewModel) togglePin() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	if note.ParseError != "" {
		m.setErrorStatus("cannot pin — note has parse error. Press e to edit.")
		return nil
	}
	updated := cloneNoteForCommand(note)
	updated.Pinned = !updated.Pinned
	updated.ModifiedAt = time.Now()
	status := "Pinned: " + note.Title
	if note.Pinned {
		status = "Unpinned: " + note.Title
	}
	location := m.noteLocation()
	return noteMutationCmd(m.noteService, location, app.NoteBatchRequest{
		Operations:  []app.NoteMutationRequest{{Kind: app.NoteUpdate, Location: location, Note: updated}},
		UndoAction:  "Toggle Note Pin",
		UndoContext: note.Title,
	}, status, false)
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

func (m *NotesViewModel) bulkTagNotes() tea.Cmd {
	selected := m.getSelectedNotesList()
	if len(selected) == 0 {
		return nil
	}
	return func() tea.Msg {
		return BulkNoteTagMsg{
			Sources:       newNoteMutationRefs(m.noteLocation(), selected),
			AvailableTags: m.noteTagOptions(),
		}
	}
}

func (m *NotesViewModel) noteTagOptions() []TagInfo {
	notes := m.notes
	if m.noteService != nil {
		if loaded, err := m.noteService.NotesAt(m.noteLocation()); err == nil {
			notes = loaded
		}
	}
	return buildNoteTagList(notes, m.project)
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

func (m *NotesViewModel) moveNoteToProject() tea.Cmd {
	if len(m.notes) == 0 || m.cursor >= len(m.notes) {
		return nil
	}
	note := m.notes[m.cursor]
	if note.ParseError != "" {
		m.setErrorStatus("cannot move — note has parse error. Press e to edit.")
		return nil
	}
	return func() tea.Msg {
		return ShowGlobalNoteMoveMsg{Source: newNoteMutationRef(app.GlobalNoteLocation(), note)}
	}
}
