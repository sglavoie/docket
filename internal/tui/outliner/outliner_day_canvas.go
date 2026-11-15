package outliner

import (
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/query"
)

// The day canvas is what the Outliner draws in room its rows leave empty: in
// Today mode, the work finished today and the week ahead under the list; in All,
// a single progress reading; in an
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
	return m.PrimaryMode == chrome.ModeToday && m.scopeCanvasEligible()
}

// Both canvas presentations read the whole scope, so narrowing hides them.
func (m *OutlinerModel) scopeCanvasEligible() bool {
	return m.Project == nil &&
		!m.HasSearchProjection() && !m.ShowsRankedList() && !m.HasActiveLens() &&
		!m.Focus.Active() && !m.Focus.Zoom.Active() &&
		!query.HasActiveCommonFilters(&m.Filter.CommonFilterState)
}

// allProgressLines quotes the day canvas without lending it any list rows.
// The separator and reading are one block, including during transient paint.
func (m *OutlinerModel) allProgressLines(width, budget int) []string {
	if budget < 2 || !m.HasContentRows() || !m.scopeCanvasEligible() {
		return nil
	}
	if line := m.computeDayCanvas(time.Now()).ProgressLine(width); line != "" {
		return []string{"", line}
	}
	return nil
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
	selected := m.weekBrowseSelected()
	var out []string
	// While the week browse is open the strip is what the reader is working, so
	// the finished list yields its rows to the selected day's.
	if selected < 0 {
		if progress := m.todayProgressLine(len(canvas.Done), width); progress != "" {
			out = append(out, "", progress)
		}
		if done := canvas.DoneBlock(width, budget-len(out)-1); len(done) > 0 {
			out = append(out, "")
			out = append(out, done...)
		}
	}
	if ahead := canvas.AheadBlockSelecting(width, selected); len(ahead) > 0 && len(out)+1+len(ahead) <= budget {
		out = append(out, "")
		out = append(out, ahead...)
		out = append(out, canvas.DayItemsBlock(selected, width, budget-len(out))...)
	}
	if selected < 0 {
		if pick := canvas.PickUpBlock(width, budget-len(out)-1); len(pick) > 0 {
			out = append(out, "")
			out = append(out, pick...)
		}
	}
	return out
}

// writeDayCanvas draws Today's canvas, All's progress, or Agenda's left-out
// line in the window rows the list left empty.
// Today's blocks are chosen against the settled window — the rows alone — so a
// ghost or a closing gap borrowing a row for a few frames trims the canvas's
// tail rather than choosing different blocks and flickering. The rows it drew
// are recorded so the dock's padding counts them. All admits its separator and
// progress together against the remaining room after transient paint.
func (m *OutlinerModel) writeDayCanvas(b *strings.Builder, viewport outlinerViewport) {
	m.LastViewport.canvasRows = 0
	drawn := m.LastViewport.drawnRows()
	settled := drawn - m.LastViewport.gapRows - boolRows(m.LastViewport.ghostActive)
	lines := m.todayCanvasLines(viewport.rowWidth, viewport.VisibleRows-settled)
	if m.PrimaryMode == chrome.ModeAll {
		lines = m.allProgressLines(viewport.rowWidth, viewport.VisibleRows-drawn)
	} else if lines == nil {
		lines = m.agendaLeftOutLines(viewport.rowWidth, viewport.VisibleRows-settled)
	}
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
