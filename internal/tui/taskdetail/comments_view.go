package taskdetail

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

func (m *CommentsModel) View() tea.View {
	if m.owner == nil {
		return tea.NewView("No comments to show")
	}

	var b strings.Builder
	b.WriteString(m.panelTitle())

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
	m.renderMessages(&b, true)
	var tail strings.Builder
	m.renderStatusMessage(&tail)
	m.renderCommentsList(&b, taskToolAvailableRows(b.String()+tail.String(), m.FrameHeight()))
	b.WriteString(tail.String())

	return tea.NewView(b.String())
}

// panelTitle is the one row every sub-state of this panel is drawn under, and it
// is a function rather than an inline write because the expanded reader measures
// the rows it has left against it. A header measured from a second spelling of
// itself is a window that disagrees with the frame by however much the two
// spellings drifted apart.
//
// The owner splits its own title the way the task-scoped renderer needs it,
// which is the only place the shared presentation still mentions a task: an
// owner that is not one names itself entirely in the prefix.
func (m *CommentsModel) panelTitle() string {
	title := m.owner.Title()
	return chrome.RenderTaskScopedFullscreenTitleWithPulse(
		title.Prefix,
		title.Task,
		"",
		m.CurrentWorkspacePulse(),
		m.Width(),
		m.AccentColor()) + "\n\n"
}

// expandedReaderHeader names the comment the reader is open on. It sits between
// the panel title and the body, and it never scrolls: the row number and the
// timestamp are what say which comment this is, and a reader who has paged into
// the middle of a long one needs that more than the reader who just opened it.
func (m *CommentsModel) expandedReaderHeader() string {
	entry := m.entries[m.Cursor]
	var b strings.Builder
	b.WriteString(theme.FieldLabelFocusedStyle.Render(fmt.Sprintf("Comment [%d]", m.Cursor)))
	b.WriteString("\n")
	b.WriteString(theme.MutedStyle.Render(chrome.FormatTimestampBracket(entry.CreatedAt)))
	b.WriteString("\n\n")
	return b.String()
}

// expandedReader is the whole geometry of the expanded comment: the header above
// the body, the body's rendered rows, and how many of those
// rows the terminal has room for.
//
// It is one value because the render and the scrolling keys both read it. A
// window a key moves and a window a frame draws have to be the same window, and
// the only way to guarantee that without a shared measurement is to take the
// same one twice and hope.
type expandedReader struct {
	header  string
	lines   []string
	visible int
	// counted reports whether a row was reserved for the position line, which
	// happens exactly when the body does not fit and the reader therefore has to
	// be told where in it they are.
	counted bool
}

// maxOffset is the last row the window may start at, so the final screenful is
// full rather than short and a body that fits has exactly one legal offset.
func (r expandedReader) maxOffset() int {
	return max(len(r.lines)-r.visible, 0)
}

func (r expandedReader) clamp(offset int) int {
	return min(max(offset, 0), r.maxOffset())
}

// expandedReaderVisibleRows splits the rows the shell will actually draw between
// the comment and the position line under it.
//
// A body that fits spends nothing on a counter, because there is no position to
// report. A body that does not gives up one row for it, since a reader who
// cannot see the end of a comment has to be told there is one. The floor is one
// content row: a terminal that short still shows the reader something they can
// scroll rather than a counter over nothing.
func expandedReaderVisibleRows(total, available int) (int, bool) {
	if total <= 0 || available <= 0 {
		return 0, false
	}
	if total <= available {
		return total, false
	}
	if available == 1 {
		return 1, false
	}
	return available - 1, true
}

// expandedReaderWindow measures the reader against the panel's own height, which
// is the height the shell will crop it to. Everything above the body is measured
// first because none of it is negotiable — the title says which entity the panel
// is for and the header says which comment — so what is left is the body's whole
// budget. The keys that move the body and close it are the shell footer's, and
// FrameHeight has already paid for the rows they take.
func (m *CommentsModel) expandedReaderWindow() expandedReader {
	reader := expandedReader{header: m.expandedReaderHeader()}
	reader.lines = strings.Split(textfit.RenderMarkdown(m.entries[m.Cursor].Text, max(m.Width()-4, 1)), "\n")
	available := taskToolAvailableRows(m.panelTitle()+reader.header, m.FrameHeight())
	reader.visible, reader.counted = expandedReaderVisibleRows(len(reader.lines), available)
	return reader
}

// expandedReaderPosition is the line that admits the body is longer than the
// window. It leads with the rows on screen and the rows there are, because that
// is the part a truncated line must not lose, and closes with the shortest
// spelling of the keys that move it — the footer names all of them, but a reader
// looking at a body that stops mid-sentence is looking here.
func expandedReaderPosition(offset, visible, total int) string {
	return fmt.Sprintf("lines %d-%d of %d · j/k scroll", offset+1, offset+visible, total)
}

// renderExpandedView draws the window of the comment the reader is on, never
// more of it than the shell will show. The body is complete and the offset is
// where the reader left it; nothing here writes that offset, so a redraw, a
// reload, and a resize all leave the window where the last key press put it.
func (m *CommentsModel) renderExpandedView(b *strings.Builder) tea.View {
	reader := m.expandedReaderWindow()
	b.WriteString(reader.header)

	offset := reader.clamp(m.expandedOffset)
	for _, line := range reader.lines[offset : offset+reader.visible] {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if reader.counted {
		position := expandedReaderPosition(offset, reader.visible, len(reader.lines))
		b.WriteString(theme.MutedStyle.Render(textfit.TruncateIntentional(position, m.proseWidth(), "…")))
		b.WriteString("\n")
	}
	return tea.NewView(b.String())
}

// proseWidth is the width every sentence this panel writes is laid out at, and it
// is the panel's whole width because that is what the title and the footer above
// and below it already use. The narrower excerpt width is a margin for quoted
// content, not for the panel's own prose: spending it on a sentence would break a
// line the terminal had room for.
func (m *CommentsModel) proseWidth() int {
	return max(m.Width(), 1)
}

// wrappedBlock lays one paragraph out at the panel's content width and styles the
// whole of it in one call. Every sentence the panel writes goes through it,
// because a sentence long enough to be worth saying is long enough to overflow a
// narrow terminal, and an overflowing row is one the shell crops rather than
// wraps. The style is applied to the block rather than to each row, so a style
// carrying a margin spends it once — the way it did when the sentence was one
// line — instead of once per wrapped row.
func wrappedBlock(text string, width int, render func(...string) string) string {
	return render(strings.Join(textfit.WrapText(text, width), "\n"))
}

func (m *CommentsModel) renderCommentsSummary(b *strings.Builder) {
	b.WriteString(theme.MutedStyle.Render(util.Count(len(m.entries), "comment")))
	b.WriteString("\n\n")
}

// renderMessages draws the refusal the panel is standing on. gap is the blank
// row that closes it, which the composer's drop ladder spends at its last rung;
// every other caller keeps it, since a block with nothing under it is only
// readable where the next block is prose that answers it.
func (m *CommentsModel) renderMessages(b *strings.Builder, gap bool) {
	if m.Err == nil {
		return
	}
	b.WriteString(wrappedBlock(m.Err.Error(), m.proseWidth(), theme.ValidationErrorStyle.Render))
	b.WriteString("\n")
	if gap {
		b.WriteString("\n")
	}
}

func (m *CommentsModel) renderCommentsList(b *strings.Builder, availableRows int) {
	if len(m.entries) == 0 {
		b.WriteString(theme.MutedStyle.Render("No comments yet. Press a to add one."))
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
			b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("  … and %d more (use j/k to scroll)", remaining)))
			b.WriteString("\n")
		}
	}

}

// renderStatusMessage prints the panel's sticky status line, unless that line is
// a read-only refusal whose state has since ended.
//
// Sticky is right for everything else the panel says: an exported draft's path
// has to stay on screen precisely because the user needs it after the draft is
// gone, and a refusal to save has to outlast the press that earned it. A refusal
// about what the box in front of you is is the one kind that cannot, because the
// box in front of you changes. Discarding a kept draft, committing it, or a save
// answering all leave the panel in a state the sentence is simply wrong about —
// browsing a list with no composer at all, or typing into a new and perfectly
// editable one — and none of them writes a status of its own to displace it. So
// the line is measured here rather than cleared at each of those exits: the
// record says whether the line is still the refusal, the panel says whether the
// refusal is still true, and a line that is neither is not drawn.
func (m *CommentsModel) renderStatusMessage(b *strings.Builder) {
	if m.StatusMessage == "" {
		return
	}
	if m.readOnlyRefusalOnScreen() && !m.readOnlyRefusalStillHolds() {
		return
	}
	// Wrapped rather than written flat, because the one status a user has to be
	// able to read character for character is an exported draft's path and a
	// generated path is longer than a narrow terminal. A path the shell cropped is
	// a path that opens nothing.
	b.WriteString(wrappedBlock(m.StatusMessage+m.statusEditorGuidance(), m.proseWidth(), theme.MutedStyle.Render))
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
// Three things narrow it, and all three are about not speaking out of turn. The
// status generation says this line is still the refusal rather than something
// written after it, which is what leaves an exported draft's path — the one status
// a user has to be able to read character for character — exactly as it was
// written. The refusal must be the preview one, because the other read-only state
// is a submission in flight and the keys this sentence names are keys it bounces.
// And the composer must still be previewing a kept draft, because a sentence about
// what to do with that draft means nothing once it has been committed or
// discarded.
func (m *CommentsModel) statusEditorGuidance() string {
	if !m.readOnlyRefusal.preview || !m.readOnlyRefusalOnScreen() || !m.readOnlyRefusalStillHolds() {
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
// how many remain, so a long comment costs the list a fixed three rows. A
// comment of exactly three lines spends that third row on its own last line
// rather than on a notice about it, so the notice always counts two or more.
func renderCommentPreview(entry *strings.Builder, text string, maxLen int) {
	renderedLines := strings.Split(textfit.RenderMarkdown(text, maxLen), "\n")
	for j, line := range renderedLines {
		if j > 1 && len(renderedLines) > 3 {
			fmt.Fprintf(entry, "      … (%d more lines, press Enter to expand)\n", len(renderedLines)-2)
			break
		}

		entry.WriteString("      ")
		entry.WriteString(textfit.TruncateIntentional(line, maxLen, ""))
		entry.WriteString("\n")
	}
}
