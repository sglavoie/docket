package outliner

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

func (m *OutlinerModel) renderOutlinerEmptyState(width int) string {
	var b strings.Builder
	// The empty state carries the bar for the same reason it carries the
	// suggestion pane: a view with no rows is exactly where a user reaches for the
	// map of what they can do next. It is a host like any other: a menu opened
	// over it composites above this block rather than replacing it.
	if m.MenuBarVisible() {
		b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	}
	m.writeOutlinerHeader(&b, width)
	// The strip and the suggestion pane are charged here exactly as the populated
	// path charges them, because they are what the list block is measured after:
	// a reservation that ignored them would dock Peek at a different screen row
	// the moment a status line or a suggestion arrived.
	extraFooter := m.StatusStripHeight(width)
	// A query that matches nothing yet is exactly when suggestions are worth the
	// most, so the empty state carries the pane too.
	if dropdown := m.renderSearchAutocompleteDropdown(width); dropdown != "" {
		b.WriteString(dropdown + "\n")
		extraFooter += lipgloss.Height(dropdown)
	}

	// The header and the explanation are this path's list block: they stand where
	// the rows stand, so they are written into their own builder and measured the
	// way the rows are measured.
	//
	// A focused projection keeps its project header when it holds no content, in
	// the same place and drawn by the same renderer as when it does: the header is
	// context for the projection, and the explanation below belongs under it. It
	// takes the full width — the scrollbar column the row path subtracts is never
	// reserved here — and it is one line, because the detail an empty project is
	// exactly the place to open is in the dock below rather than hanging off the
	// row.
	var body strings.Builder
	if header, ok := m.focusedProjectHeaderRow(); ok {
		m.pointerFrame = outlinerPointerFrame{emptyHeader: true, emptyHeaderRow: strings.Count(b.String(), "\n")}
		body.WriteString(m.RenderOutlinerProjectHeaderRow(header, m.Cursor == 0, width, 0))
		body.WriteString("\n")
	}
	// The explanation is wrapped here rather than by the terminal, so the block
	// measured below is the block drawn: a sentence the terminal wrapped would
	// count as one row and push the dock down by the rows it spilled onto.
	explanation := textfit.WrapText(m.OutlinerEmptyMessage(), max(width, 1))
	for i, line := range explanation {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(theme.MutedStyle.Render(line))
	}
	// A card earned by this empty view — the welcome, or Today's resting card
	// — is centered in the rows the explanation left, drawn whole or not at
	// all, so it never pushes the dock or the footer.
	if card := m.emptyStateCard(width); len(card) > 0 && fitCardLines(card, width) {
		room := m.outlinerListRows(extraFooter) - lipgloss.Height(body.String())
		if len(card)+1 <= room {
			// A third of the way down rather than the middle, so the card reads
			// as following the sentence above it rather than floating free.
			top := max((room-len(card))/3, 1)
			body.WriteString(strings.Repeat("\n", top))
			body.WriteString(strings.Join(card, "\n"))
		}
	}
	b.WriteString(body.String())

	// The dock sits at the screen row the reservation paid for, on this path for
	// the same reason as on the populated one: the block above it is padded out to
	// the whole list window, so an empty projection docks Peek where a full one
	// docks it instead of hanging it off the explanation. It is drawn whole — a
	// dock this path shortened would be a different height and a different
	// position per projection, which is the moving target docking retired. This
	// path has no rows to window, so it reads the one reservation without
	// computing a window against it; the bound is taken once rather than per
	// padded row, since measuring it now measures the status strip too.
	if dock := m.PeekDockLines(); len(dock) > 0 {
		listRows := m.outlinerListRows(extraFooter)
		for drawn := lipgloss.Height(body.String()); drawn < listRows; drawn++ {
			b.WriteString("\n")
		}
		m.pointerFrame.recordDock(strings.Count(b.String(), "\n")+1, len(dock))
		for _, line := range dock {
			b.WriteString("\n")
			b.WriteString(line)
		}
	}
	return b.String()
}

// focusedProjectHeaderRow names the focused-project header the current
// projection leads with, if it has one. It reads the rows rather than resolving
// the owner again so that the empty state cannot draw a header the row pipeline
// declined to build.
func (m *OutlinerModel) focusedProjectHeaderRow() (OutlinerRow, bool) {
	if len(m.Rows) > 0 && m.Rows[0].IsFocusedProjectHeader {
		return m.Rows[0], true
	}
	return OutlinerRow{}, false
}

// hasContentRows reports whether the projection holds anything the user could
// act on beyond the focused-project header. It is what separates "this project
// is empty" from "there are rows": the header is always present in a focused
// projection, so a bare length test would claim content that is not there and
// suppress the explanation the empty state owes the user.
func (m *OutlinerModel) HasContentRows() bool {
	for _, row := range m.Rows {
		if !row.IsFocusedProjectHeader {
			return true
		}
	}
	return false
}

// withheldSearchHint names the finished and deferred tasks a ranked list
// kept out, with the sigil that brings each population in. The ranked list
// ignores the mode, so adding that sigil is exactly what shows them; a narrowed
// mode would still hide them for its own reasons, so it is never promised there.
func (m *OutlinerModel) withheldSearchHint() string {
	ctx := m.SearchRecoveryContext()
	if !ctx.Ranked || ctx.Completed+ctx.Deferred == 0 {
		return ""
	}
	var parts []string
	if n := ctx.Completed; n > 0 {
		parts = append(parts, fmt.Sprintf("%s (add ?completed)", util.Count(n, "finished task")))
	}
	if n := ctx.Deferred; n > 0 {
		parts = append(parts, fmt.Sprintf("%s (add ?deferred)", util.Count(n, "deferred task")))
	}
	return strings.Join(parts, " and ")
}

func (m *OutlinerModel) searchRecoveryHint() string {
	key := m.searchRecoveryKey()
	if key == "" {
		return ""
	}
	return key + " search recovery"
}

func (m *OutlinerModel) searchRecoveryKey() string {
	if m.OperationRegistry == nil || !m.ShowsRankedList() || strings.TrimSpace(m.SearchQuery()) == "" {
		return ""
	}
	return m.OperationRegistry.InputShortcutForOperation(operation.InputStrategySimplified, operation.OperationCoreSearchRecovery)
}

// OutlinerEmptyHiddenMessage is what fH answers when it finds nothing: the view
// holds no hidden task, and wh is how one gets there.
const OutlinerEmptyHiddenMessage = "Nothing hidden here. Press wh on a task to hide it."

func (m *OutlinerModel) OutlinerEmptyMessage() string {
	if m.Search.flight != nil && m.Search.flight.result == nil {
		return "Searching… Keep typing to refine; Esc returns to your view."
	}
	switch m.ClassifyEmptyCause() {
	case outlinerEmptyNoProjects:
		return "No projects are available. Press P to open Projects."
	case outlinerEmptyProjectFiltered:
		return "No projects match project filters. Press Fx to clear project filters."
	case outlinerEmptySearch:
		if hint := m.withheldSearchHint(); hint != "" {
			verb := "does"
			if m.Search.withheld.completed+m.Search.withheld.deferred > 1 {
				verb = "do"
			}
			recovery := ""
			if key := m.searchRecoveryKey(); key != "" {
				recovery = key + " for search recovery, or "
			}
			return fmt.Sprintf("No open tasks match search %q, but %s %s. Press %s%s.", m.SearchQuery(), hint, verb, recovery, m.searchExitClause())
		}
		if hint := m.Search.eliminationHint; hint != "" {
			return fmt.Sprintf("No tasks match search %q. %s Press %s.", m.SearchQuery(), hint, m.searchExitClause())
		}
		return fmt.Sprintf("No tasks match search %q. Press %s.", m.SearchQuery(), m.searchExitClause())
	case OutlinerEmptySearchFiltered:
		// The second exit is spelled out here rather than taken from the shared
		// clause: beside "clear filters" a bare "clear" would read as clearing them.
		exit := "Esc to clear the search"
		if m.HasSuspendedLens() {
			exit = "Esc to restore the lens"
		}
		if m.ShowsRankedList() && m.Search.hiddenFilterHint != "" {
			return fmt.Sprintf("%s Press Q to inspect individual filters, fx to clear filters or %s.", m.Search.hiddenFilterHint, exit)
		}
		return fmt.Sprintf(
			"No tasks match search %q under the active filters. Press fx to clear filters or %s.",
			m.SearchQuery(), exit)
	case outlinerEmptySearchInvalid:
		return fmt.Sprintf("%s. Press %s.", m.Search.Gate.GuidanceText(), m.searchExitClause())
	case outlinerEmptyLens:
		// The three sentences below name the act the cascade's top step performs,
		// in the verb the footer entry beside them uses, so the prose and the hint
		// are one answer read twice.
		return "No tasks match this lens. " + m.emptyEscSentence(escActionLeaveLens)
	case outlinerEmptyFiltered:
		return listsurface.EmptyStateFiltered
	case outlinerEmptyHidden:
		return OutlinerEmptyHiddenMessage
	case outlinerEmptyFocus:
		return "No tasks are visible in this Focus. " + m.emptyEscSentence(escActionLeaveFocus)
	case OutlinerEmptySection:
		return "No tasks are left in this section. " + m.emptyEscSentence(escActionUnzoom)
	case outlinerEmptyMode:
		mode := m.ActiveOutlinerModeEntry()
		if message, hint := m.emptyModeNextStep(mode.ID); hint != "" {
			return message + " Press " + hint + "."
		}
		return fmt.Sprintf("%s is empty. Press Tab or Shift+Tab to change modes.", mode.Label)
	default:
		return listsurface.EmptyStateNoTasks
	}
}

// emptyModeNextStep offers navigation that works without a selected task. Both
// the explanation and footer read the registry's key, so neither advertises a
// task mutation on an empty list.
func (m *OutlinerModel) emptyModeNextStep(mode chrome.OutlinerModeID) (message, hint string) {
	var (
		operationID operation.OperationID
		act         string
	)
	switch mode {
	case chrome.ModeInbox:
		message = "Inbox is clear."
		operationID, act = operation.OperationCoreGotoToday, "to open Today"
	case chrome.ModeToday:
		message = "Nothing planned or due by today."
		operationID, act = operation.OperationCoreGotoNext, "to choose from Next"
	case chrome.ModeNext:
		message = "Nothing planned for Next."
		operationID, act = operation.OperationCoreGotoBacklog, "to choose from Backlog"
	case chrome.ModeDue:
		message = "No upcoming or overdue deadlines."
		operationID, act = operation.OperationCoreGotoToday, "to open Today"
	default:
		return "", ""
	}
	if m == nil || m.OperationRegistry == nil {
		return "", ""
	}
	key := m.OperationRegistry.InputShortcutForOperation(operation.InputStrategySimplified, operationID)
	if key == "" {
		return "", ""
	}
	key, _, _ = strings.Cut(key, "/")
	return message, key + " " + act
}

// emptyEscSentence is the Esc clause of the lens, Focus, and section sentences.
// It reads the cascade rather than the cause, because a step standing above the
// cause — an open Peek, a selection, a Focus taken inside a zoom — is what the
// next press actually unwinds, and the footer beside the sentence already says
// so through EscHint. expected is the step the cause itself owns.
func (m *OutlinerModel) emptyEscSentence(expected string) string {
	action := m.EscAction()
	if action == "" {
		action = expected
	}
	return "Press Esc to " + escActionClause(action) + "."
}

// escActionClause spells a cascade act inside a sentence. The footer's terse
// forms read as imperatives except two: the lens takes its article, and the
// scope step names a place rather than an act.
func escActionClause(action string) string {
	switch action {
	case escActionLeaveLens:
		return "leave the lens"
	case escActionLeaveScope:
		return "show all projects"
	default:
		return action
	}
}

// The day canvas is what the Outliner draws in room its rows leave empty: in
// Today mode, the work finished today and the week ahead under the list; in an
// empty Today, a resting card that says the day is clear; and in a workspace
// with no task at all, a welcome that teaches capture by example.
//
// It is paint in the padding, never a row: it is not in m.Rows, takes no
// cursor, answers no click, and moves no window, so ComputeOutlinerViewport,
// CursorScreenRow, and the pointer's hit map are untouched. Each block is drawn
// whole or not at all, from what the window left after the rows, the ghost,
// and a closing gap took their share; a canvas that does not fit is simply not
// drawn, so the list never yields a row to it.

// dayCanvasEligible reports whether the canvas may speak for the view: the
// plain cross-project Today, with nothing narrowing it, since the canvas reads
// the whole scope and a narrowed list beside it would disagree with it.
func (m *OutlinerModel) dayCanvasEligible() bool {
	return m.PrimaryMode == chrome.ModeToday && m.Project == nil &&
		!m.HasSearchProjection() && !m.ShowsRankedList() && !m.HasActiveLens() &&
		!m.Focus.Active() && !m.Focus.Zoom.Active() &&
		!query.HasActiveCommonFilters(&m.Filter.CommonFilterState)
}

func (m *OutlinerModel) computeDayCanvas(now time.Time) pulse.DayCanvas {
	return pulse.ComputeDayCanvas(m.ScopeProjects(), m.CompletionActivity, now)
}

// todayCanvasLines is the canvas under a populated Today: as many whole blocks
// as budget rows hold, each after one blank row, so it never touches the list.
func (m *OutlinerModel) todayCanvasLines(width, budget int) []string {
	if budget < 3 || !m.dayCanvasEligible() {
		return nil
	}
	canvas := m.computeDayCanvas(time.Now())
	var out []string
	if done := canvas.DoneBlock(width, budget-1); len(done) > 0 {
		out = append(out, "")
		out = append(out, done...)
	}
	if ahead := canvas.AheadBlock(width); len(ahead) > 0 && len(out)+1+len(ahead) <= budget {
		out = append(out, "")
		out = append(out, ahead...)
	}
	return out
}

// workspaceHoldsNoTask reports whether no project in scope holds a task of
// any state: the one moment the welcome is for. A workspace whose work is all
// finished, hidden, or deferred is not new, and keeps the plain sentence.
func (m *OutlinerModel) workspaceHoldsNoTask() bool {
	if m.Project != nil || m.Scope.scoped() || !m.Population.SnapshotReady {
		return false
	}
	for _, project := range m.Population.Projects {
		if project != nil && len(project.Tasks) > 0 {
			return false
		}
	}
	return true
}

// emptyStateCard is the card the empty state centers under its explanation,
// if this empty view earns one.
func (m *OutlinerModel) emptyStateCard(width int) []string {
	switch m.ClassifyEmptyCause() {
	case outlinerEmptyNoTasks:
		if m.workspaceHoldsNoTask() {
			return grammar.WelcomeCard(width)
		}
	case outlinerEmptyMode:
		if m.dayCanvasEligible() {
			return m.computeDayCanvas(time.Now()).RestingCard(width)
		}
	default:
		// Every other empty view explains itself in its sentence alone.
	}
	return nil
}

// fitCardLines reports whether every card line fits width unclipped: a card is
// drawn whole or not at all, never cut.
func fitCardLines(lines []string, width int) bool {
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			return false
		}
	}
	return true
}

// writeDayCanvas draws Today's canvas in the window rows the list left empty.
// Its blocks are chosen against the settled window — the rows alone — so a
// ghost or a closing gap borrowing a row for a few frames trims the canvas's
// tail rather than choosing different blocks and flickering. The rows it drew
// are recorded so the dock's padding counts them.
func (m *OutlinerModel) writeDayCanvas(b *strings.Builder, viewport outlinerViewport) {
	m.LastViewport.canvasRows = 0
	drawn := m.LastViewport.drawnRows()
	settled := drawn - m.LastViewport.gapRows - boolRows(m.LastViewport.ghostActive)
	lines := m.todayCanvasLines(viewport.rowWidth, viewport.VisibleRows-settled)
	lines = lines[:min(len(lines), max(viewport.VisibleRows-drawn, 0))]
	for _, line := range lines {
		b.WriteString("\n")
		b.WriteString(line)
	}
	m.LastViewport.canvasRows = len(lines)
}

func boolRows(b bool) int {
	if b {
		return 1
	}
	return 0
}
