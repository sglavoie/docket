package taskdetail

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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
// the body, the footer under it, the body's rendered rows, and how many of those
// rows the terminal has room for.
//
// It is one value because the render and the scrolling keys both read it. A
// window a key moves and a window a frame draws have to be the same window, and
// the only way to guarantee that without a shared measurement is to take the
// same one twice and hope.
type expandedReader struct {
	header  string
	footer  string
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
// is the height the shell will crop it to. Everything above and below the body is
// measured first because none of it is negotiable — the title says which entity
// the panel is for, the header says which comment, and the footer names the keys
// that move the body and close it — so what is left is the body's whole budget.
func (m *CommentsModel) expandedReaderWindow() expandedReader {
	reader := expandedReader{header: m.expandedReaderHeader()}
	spec, ok := m.guidanceSpec()
	reader.footer = renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor())
	reader.lines = strings.Split(textfit.RenderMarkdown(m.entries[m.Cursor].Text, max(m.Width()-4, 1)), "\n")
	available := taskToolAvailableRows(m.panelTitle()+reader.header, reader.footer, m.Height())
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
	b.WriteString(reader.footer)
	return tea.NewView(b.String())
}

// orphanedEditDraftNotice heads a composer whose comment left the list. It says
// both things a reader needs at once — the comment is gone, and the text below is
// still theirs — and it says them without repeating whichever refusal renderMessages
// is about to print, so a save, an editor launch, and a bare reload each read
// honestly under the same heading.
const orphanedEditDraftNotice = "This comment is no longer in the list. Your unsaved text is kept below; Esc discards it."

// recoveredDraftNoticeLead heads the block that shows a draft the composer above
// it cannot hold in full. It says the two things a reader needs before they decide
// what to do: the box is not the whole of their text, and the whole of it is
// still there and is what a save will write.
//
// It is punctuated by whichever of the two constants below the panel has room
// for, because the sentence introduces the excerpt under it and there is not
// always room for one. A colon promising rows that were budgeted away would read
// as a rendering fault at exactly the width where the user is least able to
// afford doubting the panel.
const (
	recoveredDraftNoticeLead        = "The box above cannot show all of this draft. The complete text is kept and is what a save writes"
	recoveredDraftNotice            = recoveredDraftNoticeLead + ":"
	recoveredDraftNoticeUnexcerpted = recoveredDraftNoticeLead + "."
)

// recoveredDraftExcerptRows bounds that block from above, because a recovered
// draft is unbounded by construction and the panel below it still has to be
// readable. The panel height bounds it from below: the excerpt is the one part of
// a composer that proves something the user could confirm another way, so it is
// the first thing that yields when the rows run out.
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

// composedRows counts the terminal rows a rendered fragment occupies. It differs
// from lipgloss.Height on exactly the fragments this panel is assembled from: a
// fragment ending in a newline has terminated its last row rather than started a
// new one, and counting the phantom row would make every part of a composer
// reserve one row more than it draws.
func composedRows(fragment string) int {
	if fragment == "" {
		return 0
	}
	rows := strings.Count(fragment, "\n")
	if !strings.HasSuffix(fragment, "\n") {
		rows++
	}
	return rows
}

// recoveredDraftPlan is one rung of the recovery block's drop ladder: how much of
// the block the panel draws when it cannot draw all of it.
type recoveredDraftPlan struct {
	excerptRows int
	notice      bool
	editorRoute bool
	// trailing is the blank row that separates the block from the status line
	// under it. It is a rung of its own because it is the last thing the block
	// can give up while still saying everything it has to say.
	trailing bool
}

// recoveredDraftLadder is that ladder, richest first, and it is a list rather
// than an arithmetic rule for the same reason indicatorDropLadder is: what goes
// first is a judgement about what a reader can afford to lose, and a new rung
// gets a position in it rather than an append.
//
// The excerpt goes first and goes gradually, because the retained bytes are
// intact whether or not any of them are on screen and the user can confirm them
// another way. The editor sentence goes next: Ctrl+E is a second route to a draft
// Ctrl+X already recovers unconditionally, and the footer still names it. The
// notice goes after that, because the hint under it makes the same two claims in
// fewer words — the box is a preview, and one key writes the whole draft out. The
// hint itself never goes: a block that stopped naming the recovery key would
// leave the user with a truncated box and no way to know it was not their text.
func recoveredDraftLadder() []recoveredDraftPlan {
	plans := make([]recoveredDraftPlan, 0, recoveredDraftExcerptRows+4)
	for rows := recoveredDraftExcerptRows; rows > 0; rows-- {
		plans = append(plans, recoveredDraftPlan{excerptRows: rows, notice: true, editorRoute: true, trailing: true})
	}
	return append(plans,
		recoveredDraftPlan{notice: true, editorRoute: true, trailing: true},
		recoveredDraftPlan{notice: true, trailing: true},
		recoveredDraftPlan{trailing: true},
		recoveredDraftPlan{},
	)
}

// recoveredDraftRows is the height of one rung. It measures a real render rather
// than counting wrapped lines, so the margins and blank rows the block is
// actually drawn with cannot drift away from the number reserved for it, and it
// is zero for a composer holding no lossy draft — which is what keeps an ordinary
// composer laid out exactly as it was before any of this existed.
func (m *CommentsModel) recoveredDraftRows(plan recoveredDraftPlan) int {
	var probe strings.Builder
	m.renderRecoveredDraft(&probe, plan)
	return composedRows(probe.String())
}

// recoveredDraftHint is the closing line of the block, assembled where its two
// halves are: the unconditional export sentence and the editor sentence the
// workspace decides — and that the ladder drops before it drops anything the
// export depends on.
func (m *CommentsModel) recoveredDraftHint(editorRoute bool) string {
	if !editorRoute {
		return recoveredDraftReadOnlyHint
	}
	return recoveredDraftReadOnlyHint + m.draftEditorGuidance(m.openComposerKind())
}

// renderRecoveredDraft draws the part of a retained draft the composer is not
// showing, at the rung the panel could afford. It is the only place the text
// beyond the textarea's own limits reaches the screen, so a reader can confirm
// their work survived rather than take the panel's word for it — and where there
// is no room to show any of it, the block still says the box is a preview and
// still names the key that writes the draft out, because those are the two
// claims the block exists to make.
func (m *CommentsModel) renderRecoveredDraft(b *strings.Builder, plan recoveredDraftPlan) {
	text, ok := m.lossyRetainedDraft()
	if !ok {
		return
	}
	excerptWidth := max(m.Width()-4, 1)
	excerpt := recoveredDraftExcerpt(text, excerptWidth, plan.excerptRows)
	if plan.notice {
		notice := recoveredDraftNotice
		if len(excerpt) == 0 {
			notice = recoveredDraftNoticeUnexcerpted
		}
		b.WriteString(wrappedBlock(notice, m.proseWidth(), theme.ValidationErrorStyle.Render))
		b.WriteString("\n")
	}
	for _, row := range excerpt {
		b.WriteString(theme.MutedStyle.Render(textfit.TruncateIntentional(row, excerptWidth, "…")))
		b.WriteString("\n")
	}
	b.WriteString(wrappedBlock(m.recoveredDraftHint(plan.editorRoute), m.proseWidth(), theme.MutedStyle.Render))
	b.WriteString("\n")
	if plan.trailing {
		b.WriteString("\n")
	}
}

// recoveredDraftExcerpt bounds the retained text to the rows it was given while
// keeping its end on screen. What the composer dropped is the tail, so an
// excerpt that elided the tail would prove nothing to the reader it exists for;
// the beginning is kept too, wherever three rows or more are available, so the
// excerpt is recognizable as their own text.
func recoveredDraftExcerpt(text string, width, maxRows int) []string {
	if maxRows <= 0 {
		return nil
	}
	rows := textfit.WrapText(text, width)
	if len(rows) <= maxRows {
		return rows
	}
	if maxRows < 3 {
		return rows[len(rows)-maxRows:]
	}
	headRows := 2
	if maxRows < 4 {
		headRows = 1
	}
	tail := rows[len(rows)-(maxRows-headRows-1):]
	excerpt := append([]string{}, rows[:headRows]...)
	excerpt = append(excerpt, fmt.Sprintf("... %d more line(s) ...", len(rows)-headRows-len(tail)))
	return append(excerpt, tail...)
}

// renderComposerBody closes every open composer, and is the one place the panel
// decides how many rows each part of one gets.
//
// The parts are not equal. The refusal on screen, the keys in the footer, and
// the path a Ctrl+X just named are the whole of what a user recovering text
// after a loss can act on, and none of them is negotiable, so all three are
// measured before anything else is decided. What is left is spent on the box
// first, down to its floor, because a composer whose bottom is off screen is
// still a composer; only when the box would go below that floor does the
// recovery block start down its own ladder. Nothing here touches the retained
// draft, so a short terminal costs rows and never text.
func (m *CommentsModel) renderComposerBody(b *strings.Builder, composer *textarea.Model, naturalRows int) tea.View {
	var refusal strings.Builder
	m.renderMessages(&refusal)

	var tail strings.Builder
	m.renderStatusMessage(&tail)
	spec, ok := m.guidanceSpec()
	tail.WriteString(renderTaskToolGuidance(spec, ok, m.Width(), m.AccentColor()))

	boxRows, plan := m.planComposerRows(
		m.composerBodyRows(b.String()+refusal.String()+tail.String()),
		naturalRows)

	composer.SetHeight(boxRows)
	theme.ApplyThemedTextarea(composer)
	b.WriteString(composer.View())
	b.WriteString("\n")
	b.WriteString(refusal.String())
	m.renderRecoveredDraft(b, plan)
	b.WriteString(tail.String())
	return tea.NewView(b.String())
}

// composerBodyRows is what is left for the box and the recovery block once the
// parts that are not negotiable have been measured. A panel with no height yet is
// unbounded rather than empty, because a measurement taken before the first
// WindowSizeMsg is not a small terminal.
func (m *CommentsModel) composerBodyRows(fixed string) int {
	if m.Height() <= 0 {
		return 1 << 20
	}
	return max(m.Height()-composedRows(fixed), 0)
}

// planComposerRows spends those rows. It walks the drop ladder from the richest
// rung down and takes the first one that still leaves the composer its floor,
// because the box is the one part of an open composer that cannot yield entirely:
// a box with no rows is indistinguishable from a lost draft. Where no rung does,
// the poorest is drawn anyway and the box takes what is left, which is the only
// case in which this panel knowingly asks for a row it may not get.
func (m *CommentsModel) planComposerRows(available, naturalRows int) (int, recoveredDraftPlan) {
	ladder := recoveredDraftLadder()
	for _, plan := range ladder {
		if boxRows := min(naturalRows, available-m.recoveredDraftRows(plan)); boxRows >= composerMinRows {
			return boxRows, plan
		}
	}
	last := ladder[len(ladder)-1]
	return max(available-m.recoveredDraftRows(last), 1), last
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
	return m.renderComposerBody(b, &m.EditTextInput, editComposerRows)
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
	b.WriteString(wrappedBlock(orphanedEditDraftNotice, m.proseWidth(), theme.ValidationErrorStyle.Render))
	b.WriteString("\n\n")
	return m.renderComposerBody(b, &m.EditTextInput, editComposerRows)
}

func (m *CommentsModel) renderAddMode(b *strings.Builder) tea.View {
	b.WriteString(theme.FieldLabelFocusedStyle.Render("Add Comment:"))
	b.WriteString("\n")
	return m.renderComposerBody(b, &m.TextInput, addComposerRows)
}

func (m *CommentsModel) renderCommentsSummary(b *strings.Builder) {
	b.WriteString(theme.MutedStyle.Render(fmt.Sprintf("%d comment(s)", len(m.entries))))
	b.WriteString("\n\n")
}

func (m *CommentsModel) renderMessages(b *strings.Builder) {
	if m.Err == nil {
		return
	}
	b.WriteString(wrappedBlock(m.Err.Error(), m.proseWidth(), theme.ValidationErrorStyle.Render))
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
