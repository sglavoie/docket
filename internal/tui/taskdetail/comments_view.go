package taskdetail

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func (m *CommentsModel) View() tea.View {
	if m.owner == nil {
		return tea.NewView("No comments to show")
	}

	// The owner splits its own title the way the task-scoped renderer needs it,
	// which is the only place the shared presentation still mentions a task: an
	// owner that is not one names itself entirely in the prefix.
	title := m.owner.Title()
	var b strings.Builder
	b.WriteString(chrome.RenderTaskScopedFullscreenTitleWithPulse(
		title.Prefix,
		title.Task,
		"",
		m.CurrentWorkspacePulse(),
		m.Width(),
		m.AccentColor()))
	b.WriteString("\n\n")

	if m.ExpandedView && m.HasSelection() {
		return m.renderExpandedView(&b)
	}
	if m.EditMode {
		return m.renderEditMode(&b)
	}
	if m.AddMode {
		return m.renderAddMode(&b)
	}

	m.renderCommentsSummary(&b)
	m.renderMessages(&b)
	spec, ok := m.guidanceSpec()
	footer := renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor())
	var tail strings.Builder
	m.renderStatusMessage(&tail)
	tail.WriteString(footer)
	m.renderCommentsList(&b, taskToolAvailableRows(b.String(), tail.String(), m.Height()))
	b.WriteString(tail.String())

	return tea.NewView(b.String())
}

func (m *CommentsModel) renderExpandedView(b *strings.Builder) tea.View {
	entry := m.entries[m.Cursor]
	timestamp := chrome.FormatTimestampBracket(entry.CreatedAt)
	headerLabel := fmt.Sprintf("Comment [%d]", m.Cursor)

	b.WriteString(theme.FieldLabelFocusedStyle.Render(headerLabel))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render(timestamp))
	b.WriteString("\n\n")
	b.WriteString(textfit.RenderMarkdown(entry.Text, max(m.Width()-4, 1)))
	b.WriteString("\n\n")
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

// orphanedEditDraftNotice heads a composer whose comment left the list. It says
// both things a reader needs at once — the comment is gone, and the text below is
// still theirs — and it says them without repeating whichever refusal renderMessages
// is about to print, so a save, an editor launch, and a bare reload each read
// honestly under the same heading.
const orphanedEditDraftNotice = "This comment is no longer in the list. Your unsaved text is kept below; Esc discards it."

// recoveredDraftNotice heads the block that shows a draft the composer above it
// cannot hold in full. It says the two things a reader needs before they decide
// what to do: the box is not the whole of their text, and the whole of it is
// still there and is what a save will write.
const recoveredDraftNotice = "The box above cannot show all of this draft. The complete text is kept and is what a save writes:"

// recoveredDraftExcerptRows bounds that block, because a recovered draft is
// unbounded by construction and the panel below it still has to be readable.
const recoveredDraftExcerptRows = 6

// recoveredDraftReadOnlyHint opens the line that closes that block: what the box
// above it is, and the one way out of it that depends on nothing. The excerpt
// proves the draft survived; it cannot show the middle of a long one, and a
// draft whose comment or project is gone cannot be saved at all — so the block
// that admits the box is not showing everything is exactly where the keys that
// hand the whole of it back have to be named, and where the box has to say that
// it is a preview rather than a buffer.
//
// Writing a file is the half that is true in every state, because it asks the
// workspace for nothing. The editor half is not, so it is appended from
// draftEditorGuidance rather than stated here.
const recoveredDraftReadOnlyHint = "The box is a read-only preview; Ctrl+X writes the whole draft to a file."

// renderRecoveredDraft draws the part of a retained draft the composer is not
// showing. It is the only place the text beyond the textarea's own limits
// reaches the screen, so a reader can confirm their work survived rather than
// take the panel's word for it.
func (m *CommentsModel) renderRecoveredDraft(b *strings.Builder) {
	text, ok := m.lossyRetainedDraft()
	if !ok {
		return
	}
	width := max(m.Width()-4, 1)
	b.WriteString(theme.ValidationErrorStyle.Render(recoveredDraftNotice))
	b.WriteString("\n")
	for _, row := range recoveredDraftExcerpt(text, width) {
		b.WriteString(theme.MutedStyle.Render(textfit.TruncateIntentional(row, width, "…")))
		b.WriteString("\n")
	}
	// Wrapped, unlike the notice above it: this line is the only one in the block
	// whose length depends on the panel's state, so the width it is drawn at is
	// the one thing that cannot be assumed about it.
	for _, row := range textfit.WrapText(recoveredDraftReadOnlyHint+m.draftEditorGuidance(m.openComposerKind()), width) {
		b.WriteString(theme.MutedStyle.Render(row))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// recoveredDraftExcerpt bounds the retained text to a fixed number of rows while
// keeping its end on screen. What the composer dropped is the tail, so an
// excerpt that elided the tail would prove nothing to the reader it exists for;
// the beginning is kept too, so the excerpt is recognizable as their own text.
func recoveredDraftExcerpt(text string, width int) []string {
	rows := textfit.WrapText(text, width)
	if len(rows) <= recoveredDraftExcerptRows {
		return rows
	}
	const headRows = 2
	tail := rows[len(rows)-(recoveredDraftExcerptRows-headRows-1):]
	excerpt := append([]string{}, rows[:headRows]...)
	excerpt = append(excerpt, fmt.Sprintf("... %d more line(s) ...", len(rows)-headRows-len(tail)))
	return append(excerpt, tail...)
}

func (m *CommentsModel) renderEditMode(b *strings.Builder) tea.View {
	index := m.indexOfRef(m.editRef)
	if index < 0 {
		return m.renderOrphanedEditMode(b)
	}
	entry := m.entries[index]
	b.WriteString(theme.FieldLabelFocusedStyle.Render(fmt.Sprintf("Edit Comment [%d]:", index)))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render(chrome.FormatTimestampLabeled("Created", entry.CreatedAt)))
	b.WriteString("\n\n")
	theme.ApplyThemedTextarea(&m.EditTextInput)
	b.WriteString(m.EditTextInput.View())
	b.WriteString("\n\n")
	m.renderMessages(b)
	m.renderRecoveredDraft(b)
	m.renderStatusMessage(b)
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

// renderOrphanedEditMode draws the composer over a comment the list no longer
// holds. The draft is the whole point of the branch: the comment it was opened on
// is gone, so this buffer is the only copy of the user's replacement, and hiding
// the textarea to report that would take the work away at the moment it became
// irreplaceable. There is no timestamp and no row number to head it with — both
// belonged to the entry that left — so the notice takes their place and the
// discard grammar stays exactly the one the ordinary composer offers.
func (m *CommentsModel) renderOrphanedEditMode(b *strings.Builder) tea.View {
	b.WriteString(theme.FieldLabelFocusedStyle.Render("Edit Comment:"))
	b.WriteString("\n")
	b.WriteString(theme.ValidationErrorStyle.Render(orphanedEditDraftNotice))
	b.WriteString("\n\n")
	theme.ApplyThemedTextarea(&m.EditTextInput)
	b.WriteString(m.EditTextInput.View())
	b.WriteString("\n\n")
	m.renderMessages(b)
	m.renderRecoveredDraft(b)
	m.renderStatusMessage(b)
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *CommentsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(theme.FieldLabelFocusedStyle.Render("Add Comment:"))
	b.WriteString("\n")
	theme.ApplyThemedTextarea(&m.TextInput)
	b.WriteString(m.TextInput.View())
	b.WriteString("\n\n")
	m.renderMessages(b)
	m.renderRecoveredDraft(b)
	m.renderStatusMessage(b)
	spec, ok := m.guidanceSpec()
	b.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))
	return tea.NewView(b.String())
}

func (m *CommentsModel) renderCommentsSummary(b *strings.Builder) {
	b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("%d comment(s)", len(m.entries))))
	b.WriteString("\n\n")
}

func (m *CommentsModel) renderMessages(b *strings.Builder) {
	if m.Err == nil {
		return
	}
	b.WriteString(theme.ValidationErrorStyle.Render(m.Err.Error()))
	b.WriteString("\n\n")
}

func (m *CommentsModel) renderCommentsList(b *strings.Builder, availableRows int) {
	if len(m.entries) == 0 {
		b.WriteString(theme.MutedStyle.Render("No comments yet."))
		b.WriteString("\n")
		return
	}

	maxVisible := taskToolMaxVisible(len(m.entries), availableRows, 4)
	if maxVisible == 0 {
		return
	}
	scrollOffset := chrome.CalcScrollOffset(m.Cursor, maxVisible)
	maxLen := max(m.Width()-12, 1)

	for i := scrollOffset; i < len(m.entries) && i < scrollOffset+maxVisible; i++ {
		b.WriteString(m.renderCommentEntry(i, m.entries[i], maxLen))
	}

	if len(m.entries) > maxVisible {
		remaining := len(m.entries) - scrollOffset - maxVisible
		if remaining > 0 {
			b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

}

func (m *CommentsModel) renderStatusMessage(b *strings.Builder) {
	if m.StatusMessage == "" {
		return
	}
	b.WriteString(theme.MutedStyle.Render(m.StatusMessage + m.statusEditorGuidance()))
	b.WriteString("\n")
}

// statusEditorGuidance is the state-dependent half of the read-only refusal, and
// is empty for every other status line.
//
// The refusal outlives the state it was written in. A draft the box turned a
// keystroke away from may still have had its comment and its project at that
// moment and lose either a moment later — to a delete on another surface, to the
// project going away — and a sticky line written before that would go on naming
// an editor the key has since stopped opening, beside a recovery block that had
// already recomputed and said the opposite. So the workspace-dependent sentence
// is not stored at all: it is asked again here, from the same
// draftEditorGuidance the block above the composer is closed with, every time the
// line is drawn.
//
// Two things narrow it, and both are about not speaking out of turn. The status
// generation says this line is still the refusal rather than something written
// after it, which is what leaves an exported draft's path — the one status a user
// has to be able to read character for character — exactly as it was written. And
// the composer must still be previewing a kept draft, because a sentence about
// what to do with that draft means nothing once it has been committed or
// discarded.
func (m *CommentsModel) statusEditorGuidance() string {
	if m.previewRefusalStamp != m.StatusStamp() || !m.showsDraftPreview() {
		return ""
	}
	return m.draftEditorGuidance(m.openComposerKind())
}

func (m *CommentsModel) renderCommentEntry(index int, entry CommentEntry, maxLen int) string {
	var row strings.Builder
	row.WriteString("  ")

	timestamp := chrome.FormatTimestampBracket(entry.CreatedAt)

	if index == m.Cursor {
		row.WriteString(theme.BoldStyle.Render(fmt.Sprintf("[%d] ", index)))
		row.WriteString(theme.FieldLabelFocusedStyle.Render(timestamp))
	} else {
		row.WriteString(theme.MutedStyle.Render(fmt.Sprintf("[%d] ", index)))
		row.WriteString(theme.MutedStyle.Render(timestamp))
	}
	row.WriteString("\n")

	renderCommentPreview(&row, entry.Text, maxLen)

	return row.String()
}

// renderCommentPreview shows the first two rendered lines of a comment and says
// how many remain, so a long comment costs the list a fixed three rows.
func renderCommentPreview(entry *strings.Builder, text string, maxLen int) {
	renderedLines := strings.Split(textfit.RenderMarkdown(text, maxLen), "\n")
	for j, line := range renderedLines {
		if j > 1 {
			fmt.Fprintf(entry, "      ... (%d more lines, press Enter to expand)\n", len(renderedLines)-2)
			break
		}

		entry.WriteString("      ")
		entry.WriteString(textfit.TruncateIntentional(line, maxLen, ""))
		entry.WriteString("\n")
	}
}
