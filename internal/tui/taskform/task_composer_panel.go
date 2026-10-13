package taskform

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	// composerPanelFixedRows is the labeled divider and the route summary: the
	// two rows the panel draws at every size, whatever the draft says.
	composerPanelFixedRows = 2
	// composerPanelMinRows is those two rows plus the one input row a capture
	// cannot be without — the whole of a panel nobody has typed into yet.
	composerPanelMinRows = composerPanelFixedRows + 1
	// composerPanelMaxRows caps the panel on a tall terminal: capture is a
	// bounded act, and the rows past this one are worth more to the outline
	// behind it.
	composerPanelMaxRows = 14
	// composerHostMinRows is what the panel leaves the presentation underneath
	// even in the smallest frame the app will render at all.
	composerHostMinRows = 4
)

// composerPanelShareRows is the panel's whole share of a content area of the
// given height: what its content asks for, plus the rows the shell's footer
// spends beyond the two every view is sized against, held between the one input
// row a capture always has and the ceiling the frame leaves the host.
//
// The excess rides the share rather than being subtracted from it afterwards,
// because the panel renders contentRows either way and a footer that grew a row
// must not be paid for out of the line being typed.
func composerPanelShareRows(height, contentRows, excessRows int) int {
	if height <= composerPanelMinRows+composerHostMinRows {
		return max(height-composerHostMinRows, 1)
	}
	ceiling := min(composerPanelMaxRows, height-composerHostMinRows)
	floor := min(composerPanelMinRows+excessRows, ceiling)
	return min(max(contentRows+excessRows, floor), ceiling)
}

// HostedPanelLayout answers route.HostedPanelView. Compact capture is a panel
// over the presentation that raised it; the expanded form owns the content area
// outright and says so with the zero layout.
//
// The share is measured from the panel's own content, so a blank capture leaves
// the host all but three rows and a draft carrying lines, a refusal, or a
// preview borrows more as it earns them. Those rows are a loan: the host is
// told the new budget through route.HostedBudgetView and a list surface answers
// by keeping the window it was reading, so growth costs the outline its rows
// and never its place.
//
// Expanding the syntax guide hides the host for those frames rather than
// shrinking it further: reference text is read instead of the outline, not
// beside it. The guide moves no term of the measurement either way, so the host
// keeps the budget it already had and closing the guide costs it no scroll
// window.
func (m *TaskComposerModel) HostedPanelLayout(width, height int) chrome.HostedPanelLayout {
	if m.Mode == ComposerExpanded || m.matchingTaskPreview != nil {
		return chrome.HostedPanelLayout{}
	}
	if width <= 0 {
		width = m.width
	}
	share := composerPanelShareRows(height, m.composerPanelContentRows(max(width, 1)), m.shellFrame.ExcessRows())
	return chrome.HostedPanelLayout{
		HostRows:  max(height-share, 1),
		HostDrawn: !m.HelpExpanded,
	}
}

// composerPanelContentRows is what the panel would draw if it were given every
// row it asked for: its two fixed rows, the draft as it wraps in the input, the
// refusals, and the one pane the current interaction is about.
//
// Every term reads state a keystroke already settled — the wrapped value, the
// line errors, the suggestion list, the stored preview — and none of them reads
// the panel's height, which is what keeps the measurement out of the geometry it
// decides.
func (m *TaskComposerModel) composerPanelContentRows(width int) int {
	rows := composerPanelFixedRows + m.composerInputContentRows()
	rows += panelBlockRows(m.composerPanelErrorBlock(width))
	rows += m.composerPanelPaneContentRows()
	return rows
}

// composerInputContentRows is how many visual rows the draft occupies in the
// input, counting the soft wrapping the textarea does at its own width.
// textarea.LineCount is the logical count alone, so a long pasted line would
// otherwise measure as one row and the panel would grow by nothing.
//
// An estimate is honest here: the fallback for measuring one row short is the
// textarea scrolling inside the height it was given, which keeps the cursor
// visible and loses no input.
func (m *TaskComposerModel) composerInputContentRows() int {
	wrapWidth := m.Input.Width()
	rows := 0
	for _, line := range strings.Split(m.Input.Value(), "\n") {
		rows += max(len(textfit.WrapText(line, wrapWidth)), 1)
	}
	return max(rows, 1)
}

// composerPanelPaneContentRows is what the open pane asks for. Autocomplete
// wants one row per suggestion, since RenderDropdownWithLimit spends rows on
// overflow hints only once the list no longer fits; a multiline draft adds
// one summary row to the current line's preview.
//
// The syntax guide is not among them, in either direction: it is read over the
// host rather than beside it, so the share stays whatever the pane it replaced
// had asked for and the guide spends the whole area HostDrawn false hands it.
// Measuring it as nothing would shrink the share while it was open, which is the
// scroll window session 6 gave the host back.
func (m *TaskComposerModel) composerPanelPaneContentRows() int {
	switch {
	case m.IsAutocompleteActive():
		return len(m.AC.Items)
	default:
		rows := 0
		if !m.previewRepeatsRefusal() {
			rows = m.PreviewBlockRows()
		}
		if m.nonblankLineCount() > 1 {
			rows++
		}
		if m.batchSpansProjects() {
			rows++
		}
		if m.batchPlanSummaryFits() {
			rows++
		}
		return rows
	}
}

// previewRepeatsRefusal reports whether the live preview would only restate a
// refusal the panel already leads with. After a refused Enter the same parse
// error is both a line error and the preview's error, and drawing it twice
// spends a row to say nothing new.
func (m *TaskComposerModel) previewRepeatsRefusal() bool {
	if m.PreviewErr == "" {
		return false
	}
	for _, lineErr := range m.Errors {
		if lineErr.Message == m.PreviewErr {
			return true
		}
	}
	return false
}

// composerInputRows splits the panel's body between the draft and what the
// panel has to say about it. The first input row is never spent on anything
// else, since a capture with no line to type on is not a capture; the rows past
// it yield to a refusal, because a validation message names work the typist has
// to do and the line it refuses is still scrollable inside the input.
func (m *TaskComposerModel) composerInputRows(panelRows, want, errorRows int) int {
	body := max(panelRows-composerPanelFixedRows, 1)
	previewRows := 0
	if m.nonblankLineCount() > 1 && !m.IsAutocompleteActive() {
		// Keep the count and at least one preview row alongside a long batch.
		previewRows = min(m.composerPanelPaneContentRows(), 2)
	}
	return max(min(want, body-errorRows-previewRows), 1)
}

// syncInputHeight sizes the input to what the current geometry leaves it. The
// render does this too, from the height it was handed; doing it here as well is
// what keeps the height the textarea is scrolling and the height the panel draws
// one answer at the seams that resize the panel without drawing it.
func (m *TaskComposerModel) syncInputHeight() {
	width := max(m.width, 1)
	rows := m.composerInputRows(
		m.compactPanelRows(),
		m.composerInputContentRows(),
		panelBlockRows(m.composerPanelErrorBlock(width)),
	)
	m.setInputHeight(rows)
}

// setInputHeight resizes the input and re-seats its scroll. The textarea only
// ever scrolls to keep the cursor visible, so an offset taken while it was short
// survives it growing: a pasted batch lands with the cursor on its last line, the
// panel then grows to fit every line, and the input would still start at the
// last one with blank rows beneath it. When the offset leaves rows unused below
// the draft, the cursor is walked back from the top, which scrolls exactly as far
// as keeping it visible needs.
func (m *TaskComposerModel) setInputHeight(rows int) {
	m.Input.SetHeight(rows)
	offset := m.Input.ScrollYOffset()
	if offset == 0 || offset+m.Input.Height() <= m.composerInputContentRows() {
		return
	}
	row, col := m.Input.Line(), m.Input.Column()
	m.Input.MoveToBegin()
	for guard := m.composerInputContentRows(); m.Input.Line() < row && guard > 0; guard-- {
		m.Input.CursorDown()
	}
	m.Input.SetCursorColumn(col)
}

// compactPanelRows is the height this panel renders to: the share the layout
// left it, less whatever the shell's footer spends beyond the two rows every
// view is sized against. A footer carrying feedback beside its controls, or
// controls wrapped at minimum width, is taller than that, and the panel is what
// pays — the host below it keeps the rows the layout promised it, so opening
// capture and then saving costs the outline no scroll window.
//
// The excess is taken from the shell's own measurement rather than re-derived
// from this view's guidance, which is the only way the panel and the footer can
// be reserving and drawing the same rows.
func (m *TaskComposerModel) compactPanelRows() int {
	rows := m.HostedPanelLayout(m.width, m.height).PanelRows(m.height)
	return max(rows-m.shellFrame.ExcessRows(), 1)
}

// SetShellFrameMetrics answers route.ShellFrameAware. It is not a resize: the
// panel is recomputed from the frame on every render, so the budget only has to
// be current, and routing it through SetSize would reset the input and the
// preview every time a message arrived.
func (m *TaskComposerModel) SetShellFrameMetrics(metrics chrome.ShellFrameMetrics) {
	m.shellFrame = metrics
	if m.matchingTaskPreview != nil {
		m.resizeMatchingTaskPreview()
	}
}

// renderCompactPanel draws the bounded capture panel: what the line will do,
// the line itself, and the one thing the panel has room to say about it.
//
// The input is sized here, from the rows this frame was handed, so the panel
// spends exactly the height its own content asked for. That is a write during
// render for the same reason the syntax viewport's is: the height is the frame's
// answer, and routing it through SetSize would reset the draft and the preview
// every time a message arrived.
func (m *TaskComposerModel) renderCompactPanel(rows int) string {
	width := max(m.width, 1)
	errors := m.composerPanelErrorBlock(width)
	m.setInputHeight(m.composerInputRows(rows, m.composerInputContentRows(), panelBlockRows(errors)))

	var b strings.Builder
	b.WriteString(chrome.RenderLabeledDivider(m.Preset.label(), width))
	b.WriteString("\n")
	b.WriteString(m.renderComposerRouteRow(width))
	b.WriteString("\n")
	b.WriteString(m.Input.View())

	if tail := m.renderComposerPanelTail(rows-lipgloss.Height(b.String()), errors); tail != "" {
		b.WriteString("\n")
		b.WriteString(tail)
	}
	return chrome.FitBlockHeight(strings.TrimRight(b.String(), "\n"), rows)
}

// renderComposerRouteRow states the effective route on one row. Every axis is
// named, because a project without a landing place and a landing place without a
// project each describe a different destination, and because the plan and the
// status a capture lands in are two answers one sigil gives at once: `$today`
// plans for Today and lands in Backlog, so a row saying only `$today` left the
// second half for the user to know rather than to read.
//
// The plan half appears only when there is a membership to name. The project
// spends whatever the named axes leave, since it is the one value that can be
// arbitrarily long and the one a truncation still leaves recognizable.
func (m *TaskComposerModel) renderComposerRouteRow(width int) string {
	header := m.effectiveRouteHeader()
	project, plan, status := header.strings()
	// FieldLabelStyle rather than SubtitleStyle: the latter carries a bottom
	// margin, which on a one-row summary breaks the row in two. Each value wears
	// the color of the authority that owns it — the project's slot, the plan's and
	// the status's section badge — and Mixed or Unavailable reads muted.
	const separator = " · "
	route := theme.FieldLabelStyle.Render(chrome.StatusFieldLabel+": ") + header.statusStyle().Render(status)
	if plan != "" {
		route = theme.FieldLabelStyle.Render(chrome.PlanFieldLabel+": ") + header.planStyle().Render(plan) + separator + route
	}
	projectLabel := theme.FieldLabelStyle.Render("Project: ")
	budget := width - lipgloss.Width(projectLabel) - lipgloss.Width(route) - lipgloss.Width(separator)
	projectName := header.projectStyle().Render(textfit.TruncateToVisualWidth(project, max(budget, 1)))
	row := projectLabel + projectName + separator + route
	return textfit.TruncateToVisualWidth(row, width)
}

// renderComposerPanelTail fills what the panel has left after its chrome.
// Refusals lead, because they are the rows a bounded panel must never drop;
// below them the panel shows the one pane the current interaction is about. The
// discard question is not among them: it is the shell's own confirm dialog,
// drawn over this panel, and a second copy underneath would cost the preview a
// row to repeat what the overlay already asks.
func (m *TaskComposerModel) renderComposerPanelTail(rows int, errors string) string {
	if rows <= 0 {
		return ""
	}

	block := errors
	remaining := rows - panelBlockRows(block)
	if remaining > 0 {
		block = joinPanelBlocks(block, m.renderComposerPanelPane(remaining))
	}
	return chrome.FitBlockHeight(block, rows)
}

// composerPanelErrorBlock renders the refusals the panel must never drop. It is
// one definition because the panel both measures it — the rows a validation
// message needs are rows the panel grows by — and draws it, and a second copy
// of the wrapping would let those two disagree.
func (m *TaskComposerModel) composerPanelErrorBlock(width int) string {
	var block string
	if m.pasteRefusal != "" {
		block = theme.ValidationErrorStyle.Width(width).Render(m.pasteRefusal)
	}
	errors := m.Errors
	// Put the caret's problem first so a bounded panel always shows its reason.
	if len(errors) > 1 {
		for index, problem := range errors {
			if problem.Line == m.Input.Line()+1 {
				block = joinPanelBlocks(block, theme.SubtitleStyle.Render(fmt.Sprintf("Problem %d of %d · Line %d", index+1, len(errors), problem.Line)))
				errors = append(append([]grammar.LineError{problem}, errors[:index]...), errors[index+1:]...)
				break
			}
		}
	}
	for _, lineErr := range errors {
		block = joinPanelBlocks(block, theme.ValidationErrorStyle.Width(width).
			Render(fmt.Sprintf("Line %d: %s", lineErr.Line, lineErr.Message)))
	}
	if m.pasteRefusal == "" && m.Input.CharLimit > 0 {
		remaining := max(m.Input.CharLimit-m.Input.Length(), 0)
		if remaining <= m.Input.CharLimit/10 {
			unit := "characters"
			if remaining == 1 {
				unit = "character"
			}
			block = joinPanelBlocks(block, theme.MutedStyle.Render(fmt.Sprintf("%d %s remaining", remaining, unit)))
		}
	}
	return block
}

// panelBlockRows is the height of a rendered block, counting an absent one as
// no rows rather than the one lipgloss.Height reports for an empty string.
func panelBlockRows(block string) int {
	if block == "" {
		return 0
	}
	return lipgloss.Height(block)
}

// renderComposerPanelPane picks the one pane the panel's remaining rows go to.
// Autocomplete and the syntax guide are live interactions the keyboard is
// already inside, so each owns the space while it is open; otherwise the rows
// belong to the preview of what Enter would create.
func (m *TaskComposerModel) renderComposerPanelPane(rows int) string {
	switch {
	case m.IsAutocompleteActive():
		return strings.TrimRight(m.AC.RenderDropdownWithLimit("", rows), "\n")
	case m.HelpExpanded:
		m.SetSyntaxViewportHeight(rows)
		return m.SyntaxVP.View()
	default:
		return m.renderComposerPanelPreview(rows)
	}
}

func (m *TaskComposerModel) renderComposerPanelPreview(rows int) string {
	if count := m.nonblankLineCount(); count > 1 {
		summaryText := fmt.Sprintf("%d tasks · Line %d", count, m.Input.Line()+1)
		if m.batchInvalidLines == 1 {
			summaryText += " · 1 line needs attention"
		} else if m.batchInvalidLines > 1 {
			summaryText += fmt.Sprintf(" · %d lines need attention", m.batchInvalidLines)
		}
		summary := theme.MutedStyle.Render(textfit.TruncateToVisualWidth(summaryText, m.Width))
		if rows > 1 && m.batchSpansProjects() {
			summary = joinPanelBlocks(summary, theme.MutedStyle.Render(m.batchDestinationSummary()))
			rows--
		}
		if rows > 2 && m.batchPlanSummaryFits() {
			summary = joinPanelBlocks(summary, theme.MutedStyle.Render(m.batchPlanSummary))
			rows--
		}
		if rows <= 1 || m.previewRepeatsRefusal() {
			return summary
		}
		return joinPanelBlocks(summary, m.ComposeBoundedPreviewBlock(rows-1))
	}
	if (m.PreviewErr == "" && m.Preview == nil) || m.previewRepeatsRefusal() {
		return ""
	}
	return m.ComposeBoundedPreviewBlock(rows)
}

func (m *TaskComposerModel) batchPlanSummaryFits() bool {
	return m.batchPlanSummary != "" && lipgloss.Width(m.batchPlanSummary) <= m.Width
}

func joinPanelBlocks(above, below string) string {
	if above == "" {
		return below
	}
	if below == "" {
		return above
	}
	return above + "\n" + below
}

// Pack whole destinations, reserving space for an explicit overflow count.
// batchSpansProjects is the one test for the destination counts, read by the
// measurement and the render alike. A batch landing in a single project is
// already named whole by the route row above, so the counts earn their row only
// when there is more than one project to tell apart.
func (m *TaskComposerModel) batchSpansProjects() bool {
	return len(m.batchDestinations) > 1
}

func (m *TaskComposerModel) batchDestinationSummary() string {
	for shown := len(m.batchDestinations); shown >= 0; shown-- {
		parts := append([]string(nil), m.batchDestinations[:shown]...)
		if remaining := len(m.batchDestinations) - shown; remaining > 0 {
			parts = append(parts, fmt.Sprintf("+%d projects", remaining))
		}
		text := "To: " + strings.Join(parts, " · ")
		if lipgloss.Width(text) <= m.Width {
			return text
		}
	}
	return ""
}
