package outliner

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
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
	if !m.ShowsRankedList() || !m.Search.withheld.any() {
		return ""
	}
	var parts []string
	if n := m.Search.withheld.completed; n > 0 {
		parts = append(parts, fmt.Sprintf("%s (add ?completed)", util.Count(n, "finished task")))
	}
	if n := m.Search.withheld.deferred; n > 0 {
		parts = append(parts, fmt.Sprintf("%s (add ?deferred)", util.Count(n, "deferred task")))
	}
	return strings.Join(parts, " and ")
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
			return fmt.Sprintf("No open tasks match search %q, but %s %s. Press %s.", m.SearchQuery(), hint, verb, m.searchExitClause())
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
		return "No tasks match this lens. Press Esc to leave the lens."
	case outlinerEmptyFiltered:
		return listsurface.EmptyStateFiltered
	case outlinerEmptyHidden:
		return OutlinerEmptyHiddenMessage
	case outlinerEmptyFocus:
		return "No tasks are visible in this Focus. Press Esc to leave Focus."
	case OutlinerEmptySection:
		return "No tasks are left in this section. Press Esc to unzoom."
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
		message = "Nothing planned for today."
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
