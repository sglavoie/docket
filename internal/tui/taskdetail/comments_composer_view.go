package taskdetail

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

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
	// refusalGap is the blank row under the refusal above the block. It is not
	// part of the block at all, which is exactly why it goes last: every rung
	// above it spends something the block was drawing, and this one spends the
	// only row left that carries no words.
	refusalGap bool
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
//
// The blank row under the refusal is the one rung below all of that, because it
// is the only row on the panel that says nothing: at the bottom of the ladder
// the refusal and the hint stand against each other rather than one of them
// leaving the screen.
func recoveredDraftLadder() []recoveredDraftPlan {
	plans := make([]recoveredDraftPlan, 0, recoveredDraftExcerptRows+5)
	for rows := recoveredDraftExcerptRows; rows > 0; rows-- {
		plans = append(plans, recoveredDraftPlan{excerptRows: rows, notice: true, editorRoute: true, trailing: true, refusalGap: true})
	}
	return append(plans,
		recoveredDraftPlan{notice: true, editorRoute: true, trailing: true, refusalGap: true},
		recoveredDraftPlan{notice: true, trailing: true, refusalGap: true},
		recoveredDraftPlan{trailing: true, refusalGap: true},
		recoveredDraftPlan{refusalGap: true},
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
	excerpt = append(excerpt, fmt.Sprintf("... %s more ...", util.Count(len(rows)-headRows-len(tail), "line")))
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
	var tail strings.Builder
	m.renderStatusMessage(&tail)

	// The refusal is measured by the ladder rather than counted as fixed, because
	// its last rung is the blank row underneath it. Its words are not negotiable
	// and no rung drops them.
	boxRows, plan := m.planComposerRows(
		m.composerBodyRows(b.String()+tail.String()),
		naturalRows)

	composer.SetHeight(boxRows)
	theme.ApplyThemedTextarea(composer)
	b.WriteString(composer.View())
	b.WriteString("\n")
	m.renderMessages(b, plan.refusalGap)
	m.renderRecoveredDraft(b, plan)
	b.WriteString(tail.String())
	return tea.NewView(b.String())
}

// composerBodyRows is what is left for the box and the recovery block once the
// parts that are not negotiable have been measured. A panel with no height yet is
// unbounded rather than empty, because a measurement taken before the first
// WindowSizeMsg is not a small terminal. The budget is FrameHeight rather than
// Height because a refused save is exactly when the footer grows a row to carry
// the refusal, and planning against the taller frame is what would crop the
// composer's own `Ctrl+S save · Esc discard` off the bottom of it.
func (m *CommentsModel) composerBodyRows(fixed string) int {
	if m.Height() <= 0 {
		return 1 << 20
	}
	return max(m.FrameHeight()-composedRows(fixed), 0)
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
		if boxRows := min(naturalRows, available-m.rungRows(plan)); boxRows >= composerMinRows {
			return boxRows, plan
		}
	}
	last := ladder[len(ladder)-1]
	return max(available-m.rungRows(last), 1), last
}

// rungRows is what one rung costs above the box: the recovery block, plus the
// refusal whose closing blank row is the rung's own last concession.
func (m *CommentsModel) rungRows(plan recoveredDraftPlan) int {
	var probe strings.Builder
	m.renderMessages(&probe, plan.refusalGap)
	return composedRows(probe.String()) + m.recoveredDraftRows(plan)
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
