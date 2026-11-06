package outliner

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

// statusStripBody builds the outstanding-conflict indicator, the focus notice,
// the status message, and the expanded query readout the shell pins above the
// footer. It is pure: the height reservation and the strip itself must measure
// the same string.
//
// The indicator is an ambient advisory, so it is gated by advisoryLine: hiding
// it drops the row it reserved too, because the reservation measures this same
// body.
//
// The indicator leads because it is the one durable line here — it stands until
// the CLI resolves the last bundle, while the lines below it come and go — so a
// transient line arriving never shifts it to another row. The readout sits last
// for the same reason from the other end: it is the tallest thing the strip can
// hold, and growing it must not push the three one-line signals around.
func (m *OutlinerModel) StatusStripBody(width int) string {
	lines := m.statusStripSignals(width)
	// The expanded stack changes this return value rather than painting outside
	// it, so statusStripHeight reserves the rows it needs from the list.
	if readout := m.QueryReadoutBody(width); readout != "" {
		lines = append(lines, readout)
	}
	return strings.Join(lines, "\n")
}

// statusStripSignals is the strip without the readout: the three one-line
// signals that stand above it. They are separated out because the readout has
// to budget its own viewport against them, and it cannot ask statusStripHeight
// for that — the height it would answer with is the one the readout is still
// deciding.
func (m *OutlinerModel) statusStripSignals(width int) []string {
	width = max(width, 1)
	lines := make([]string, 0, 4)
	if indicator := m.AdvisoryLine(m.conflictIndicator()); indicator != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(indicator))
	}
	if indicator := m.AdvisoryLine(m.syncIndicator()); indicator != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(indicator))
	}
	if m.Focus.Notice != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(m.Focus.Notice))
	}
	if m.StatusMessage != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(m.StatusMessage))
	}
	if m.HasContentRows() {
		if hint := m.withheldSearchHint(); hint != "" {
			if recovery := m.searchRecoveryHint(); recovery != "" {
				hint += ". " + recovery
			}
			lines = append(lines, theme.MutedStyle.Width(width).Render("Also matches: "+hint+"."))
		}
	}
	return lines
}

// statusStripSignalRows is what those signals cost once wrapped, which is not
// their count: each is rendered at the strip's width and a long one takes more
// than a row.
func (m *OutlinerModel) statusStripSignalRows(width int) int {
	signals := m.statusStripSignals(width)
	if len(signals) == 0 {
		return 0
	}
	return lipgloss.Height(strings.Join(signals, "\n"))
}

// statusStripHeight is the strip's divider plus its wrapped lines. The Outliner
// reserves it from its own row budget, so the strip takes rows from the list
// rather than pushing the footer off screen.
func (m *OutlinerModel) StatusStripHeight(width int) int {
	body := m.StatusStripBody(width)
	if body == "" {
		return 0
	}
	return 1 + lipgloss.Height(body)
}

// ViewStatusStrip hands the strip to the shell and consumes the one-shot focus
// notice. AppModel.View renders the view before the status bar, so the
// reservation above and this call see the same inputs in the same frame.
func (m *OutlinerModel) ViewStatusStrip(width int) string {
	body := m.StatusStripBody(width)
	m.Focus.clearNotice()
	return body
}

func (m *OutlinerModel) outlinerFocusContext() string {
	if m.Project == nil {
		return m.CrossProjectFocusContext()
	}
	return m.ProjectFocusContext()
}

// buildOutlinerSearchIndicator renders the search chrome, appending a badge only
// when the title row can hold it: at narrow widths the query itself is the more
// useful half of the segment, so the match count sheds before it does. A refused
// sigil wins over the count and the case badge, because a query that cannot run
// explains an empty list better than how it was matched.
func (m *OutlinerModel) buildOutlinerSearchIndicator(width int) string {
	return m.searchIndicator(width, true)
}

// searchIndicator is that chrome with or without the match count. The context
// row draws the counted form in full and the uncounted one compact, so a row
// too narrow for every entry gives up the count before it drops the query.
func (m *OutlinerModel) searchIndicator(width int, counted bool) string {
	if m.IsSearchMode() && m.Search.historyPicker != nil {
		picker := m.Search.historyPicker
		picker.input.SetWidth(max(width-10, 1))
		theme.RefreshThemedTextInput(&picker.input)
		return theme.MutedStyle.Render("History: ") + picker.input.View()
	}
	var indicator string
	switch {
	case m.HasActiveLens() && m.Lens.Query != "":
		return m.appendLensQueryBadge(theme.MutedStyle.Render(fmt.Sprintf("Lens query: %q", m.Lens.Query)), width)
	case m.IsSearchMode():
		indicator = theme.MutedStyle.Render("Search: ") + m.SearchInputView()
	case m.SearchQuery() != "":
		indicator = theme.MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	default:
		return ""
	}
	if guidance := m.Search.Gate.GuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if count := m.searchMatchCountLabel(); counted && count != "" {
		indicator = appendSearchBadge(indicator, " · "+count, width)
	}
	if m.Search.caseRelaxed {
		indicator = appendSearchBadge(indicator, " (case ignored)", width)
	}
	if m.IsSearchMode() && !m.Search.Autocomplete.Active() && m.Search.history.notice != "" {
		indicator = appendSearchBadge(indicator, " · "+m.Search.history.notice, width)
	}
	return indicator
}

// searchMatchCountLabel spells how many task and note rows the projection
// draws for the held query, the same in the typing and the applied form. It is
// empty with no projection standing, since a query installed without one has
// drawn nothing of its own to count, and over an empty query, which matched
// nothing and narrowed nothing.
func (m *OutlinerModel) searchMatchCountLabel() string {
	if m.Search.flight != nil && m.Search.flight.result == nil {
		return "Searching…"
	}
	if !m.HasSearchProjection() || strings.TrimSpace(m.SearchQuery()) == "" {
		return ""
	}
	return util.CountPlural(m.Search.drawn, "match", "matches")
}

// appendLensQueryBadge gives the lens query the badges ordinary search already
// uses, read off the lens compilation: a refused sigil wins over the case badge
// there for the same reason it does in search.
func (m *OutlinerModel) appendLensQueryBadge(indicator string, width int) string {
	if guidance := m.LensGuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if !m.Lens.NarrowRelaxed {
		return indicator
	}
	return appendSearchBadge(indicator, " (case ignored)", width)
}

func appendSearchBadge(indicator, text string, width int) string {
	badge := theme.MutedStyle.Render(text)
	if lipgloss.Width(indicator)+lipgloss.Width(badge) > width {
		return indicator
	}
	return indicator + badge
}

// An advisory is ambient workspace state the user did not ask for: a line that
// appears because of what the workspace is, not because of a key that was just
// pressed. The outstanding-conflict count is the archetype — it stands until the
// CLI resolves the last bundle — and the launch-time "N projects failed to load"
// notice is the other one. Feedback from the user's own operations is not an
// advisory, and neither is the query readout, which only exists because `Q` was
// pressed.
//
// Advisories are opt-out, and the opt-out has one rule: every ambient line
// routes through advisoryLine (a strip row) or advisoryStatus (a status
// message). A later ambient row inherits `--no-advisories` and the View menu's
// toggle by using those two helpers, without anyone editing this file.
//
// The session value those helpers read lives on the shell; see
// advisories_shell.go for where the machine-local setting and the
// `--no-advisories` flag feed it.

// setAdvisoriesHidden injects the shell's setting, mirroring
// setOutstandingConflicts: a nil accessor is inert and reads as showing, which
// is what a struct-literal test model and the Hub's preview painter get.
func (m *OutlinerModel) SetAdvisoriesHidden(accessor func() bool) {
	m.advisoriesHiddenAccessor = accessor
}

// advisoriesSuppressed answers the two helpers below.
func (m *OutlinerModel) AdvisoriesSuppressed() bool {
	if m == nil || m.advisoriesHiddenAccessor == nil {
		return false
	}
	return m.advisoriesHiddenAccessor()
}

// advisoryLine gates one status-strip row. Returning the empty string is what
// keeps statusStripHeight and the strip itself measuring the same body, so a
// suppressed advisory costs no reserved row either.
func (m *OutlinerModel) AdvisoryLine(text string) string {
	if m.AdvisoriesSuppressed() {
		return ""
	}
	return text
}

// advisoryStatus gates one ambient status message. A nil Cmd is the whole
// suppression: the message is never sent, so it neither reaches the footer nor
// supersedes the line already there. An advisory that is shown is informational
// and therefore quiet — it says what the workspace holds and moves nothing.
func (m *OutlinerModel) AdvisoryStatus(text string) tea.Cmd {
	if text == "" || m.AdvisoriesSuppressed() {
		return nil
	}
	return func() tea.Msg { return status.InfoStatusMsg(text) }
}

// loadFailureAdvisory is the notice a partly-loaded population reports: the
// projects that did not read are workspace state, not an answer to a keystroke,
// so both the cross-project load and the lens load route their wording through
// this one helper and inherit the gate with it.
func (m *OutlinerModel) LoadFailureAdvisory(failures []ProjectLoadFailure) tea.Cmd {
	if len(failures) == 0 {
		return nil
	}
	noun := "project"
	if len(failures) > 1 {
		noun = "projects"
	}
	return m.AdvisoryStatus(fmt.Sprintf("%d %s failed to load: %s", len(failures), noun, failures[0].Detail))
}

// The outstanding-conflict indicator is workspace state, not an event: it is a
// line the status strip carries for as long as `.docket/conflicts/` holds a bundle,
// and it disappears once the CLI resolves the last one. The count itself is
// cached on the shell and injected as an accessor; see conflict_indicator_shell.go
// for where it is read.

// conflictIndicatorText is the strip's line for a given count. Zero renders
// nothing at all, so the strip keeps its height only while there is state to
// report.
func conflictIndicatorText(count int) string {
	switch {
	case count <= 0:
		return ""
	case count == 1:
		return "1 unresolved merge conflict — gc to review"
	default:
		return fmt.Sprintf("%d unresolved merge conflicts — gc to review", count)
	}
}

// setOutstandingConflicts injects the shell's conflict accessor. Like the note
// index it is a setter rather than a constructor parameter, because the Hub's
// preview painter builds an Outliner that must keep the nil accessor.
func (m *OutlinerModel) SetOutstandingConflicts(accessor func() int) {
	m.outstandingConflicts = accessor
}

// conflictIndicator is the strip's line for the injected count.
func (m *OutlinerModel) conflictIndicator() string {
	if m == nil || m.outstandingConflicts == nil {
		return ""
	}
	return conflictIndicatorText(m.outstandingConflicts())
}

// The sync line is the strip's account of an enrolled workspace's exchange:
// work committed here and not yet published, work found and not yet applied,
// a transaction held until some device updates Docket, and an exchange that last
// failed. It is ambient state like the conflict count, so it is an advisory,
// and the shell writes its text; open sync conflicts are counted in the
// conflict line instead. A workspace that is not enrolled, or one with
// nothing waiting, draws no line.

// SetSyncAdvisory injects the shell's sync line accessor. A nil accessor is
// inert, as for the conflict count.
func (m *OutlinerModel) SetSyncAdvisory(accessor func() string) {
	m.syncAdvisory = accessor
}

// syncIndicator is the strip's sync line, or empty.
func (m *OutlinerModel) syncIndicator() string {
	if m == nil || m.syncAdvisory == nil {
		return ""
	}
	return m.syncAdvisory()
}

// SetMouseDisabled injects the shell's mouse setting, mirroring
// SetAdvisoriesHidden: a nil accessor is inert and reads as on, which is what a
// struct-literal test model and the Hub's preview painter get. The Outliner only
// reads it to tick the View menu's Mouse row; the shell owns reporting itself.
func (m *OutlinerModel) SetMouseDisabled(accessor func() bool) {
	m.mouseDisabledAccessor = accessor
}

// MouseEnabled reports whether the session has mouse reporting on.
func (m *OutlinerModel) MouseEnabled() bool {
	if m == nil || m.mouseDisabledAccessor == nil {
		return true
	}
	return !m.mouseDisabledAccessor()
}

// SetPresentationPreferences injects the shell's Motion preference and glyph
// set, mirroring SetMouseDisabled. The Outliner only reads them to tick the
// View menu's rows; a nil accessor reads as motion on and the Unicode set.
func (m *OutlinerModel) SetPresentationPreferences(motionPreferred, nerdGlyphs func() bool) {
	m.motionPreferredAccessor = motionPreferred
	m.nerdGlyphsAccessor = nerdGlyphs
}

// MotionPreferred reports whether the session's Motion preference is on.
func (m *OutlinerModel) MotionPreferred() bool {
	if m == nil || m.motionPreferredAccessor == nil {
		return true
	}
	return m.motionPreferredAccessor()
}

// NerdGlyphs reports whether the session draws the Nerd Font glyph set.
func (m *OutlinerModel) NerdGlyphs() bool {
	return m != nil && m.nerdGlyphsAccessor != nil && m.nerdGlyphsAccessor()
}
