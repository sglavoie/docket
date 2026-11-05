package outliner

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/motion"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The departure ghost is the one frame of motion a committed departure earns:
// the row the keypress was aimed at stays on screen, struck through, at the
// position it vacated, for departureGhostPhaseDuration per phase, then the rows
// under it close up. It is render-only — never a row of m.Rows, never a cursor
// position, never a window input — so every row invariant, every index, and
// ComputeOutlinerViewport are untouched; the render records the screen slot it
// borrowed in LastViewport so the two readers of screen rows, CursorScreenRow
// and the pointer's hit map, step over it.
//
// It is motion, so it exists only where the shell armed motion: an interactive
// launch. The harness and every test leave it off, which is what keeps a
// scenario's observations independent of a timer they never wait for.
const (
	departureGhostPhaseDuration = 160 * time.Millisecond
	departureGhostPhases        = 2
)

// departureGhost is the struck row a departure leaves behind for a moment.
type departureGhost struct {
	// line is the row's plain text as the keypress saw it, and screenOffset
	// the list line it was drawn on, counted from the window's first line.
	line         string
	screenOffset int
	// done reports that the task departed by completing, so its state glyph
	// is repainted as the check it became.
	done  bool
	phase int
	seq   int
}

// DepartureGhostTickMsg advances the ghost Seq names; a stale one is inert.
type DepartureGhostTickMsg struct{ Seq int }

// SetMotion arms or disarms the session's motion. Disarming drops a live
// ghost, the arrival, and every motion run at once.
func (m *OutlinerModel) SetMotion(enabled bool) {
	m.motion = enabled
	m.anim.SetArmed(enabled)
	if !enabled {
		m.ghost = nil
		m.arrival = nil
	}
}

func departureGhostTick(seq int) tea.Cmd {
	return status.TickCmd(departureGhostPhaseDuration, func(time.Time) tea.Msg {
		return DepartureGhostTickMsg{Seq: seq}
	})
}

// advanceDepartureGhost moves the ghost to its next phase, or retires it after
// the last one.
func (m *OutlinerModel) advanceDepartureGhost(msg DepartureGhostTickMsg) tea.Cmd {
	if m.ghost == nil || m.ghost.seq != msg.Seq {
		return nil
	}
	m.ghost.phase++
	if m.ghost.phase >= departureGhostPhases {
		m.ghost = nil
		return nil
	}
	return departureGhostTick(msg.Seq)
}

// DropMotion retires every animation still settling on this view, and every
// intent still waiting to become one; the shell calls it on each keystroke,
// click, and wheel step, ahead of the operation the input reaches, so that
// operation's own intent is the only one standing when the update settles.
func (m *OutlinerModel) DropMotion() {
	if m != nil {
		m.dropDepartureGhost()
		m.anim.Drop()
	}
}

// dropDepartureGhost retires a live ghost at once: a keypress means the user
// has moved on, and the rows close up under their next action.
func (m *OutlinerModel) dropDepartureGhost() {
	m.ghost = nil
}

// The three runs below are motion.Runs': a scroll between windows, a fold
// rolling out, and the trail a moved task leaves where it landed. Each starts as
// an intent the shell notes before the operation runs, and becomes a run in
// SettleMotion, once the update that carried the operation has rebuilt its rows,
// so a run is measured against the state it will end on. None writes a row, the
// cursor, ViewScrollOffset, or a width: the render asks motion.Runs for this
// frame's paint, and LastViewport stays the window ComputeOutlinerViewport
// decided, which is what CursorScreenRow and the pointer read.

// MotionTickMsg advances one of the three motion runs; a stale one is inert.
type MotionTickMsg = motion.TickMsg

// NoteScrollJump records the window a page key is about to leave: Ctrl+D,
// Ctrl+U, PgDn, PgUp, gg/Home, G/End, and the z chord. The Peek dock's own
// scrolling never reaches here, so it never animates.
func (m *OutlinerModel) NoteScrollJump() {
	if m == nil || m.PeekFullscreen || m.LastViewport.VisibleRows <= 0 {
		return
	}
	m.anim.NoteScroll(m.LastViewport.startIdx)
}

// NoteFoldReveal records the rows drawn before a fold operation, so the rows it
// opens can be told from the rows that were already there.
func (m *OutlinerModel) NoteFoldReveal() {
	if m == nil || !m.anim.Armed() {
		return
	}
	before := make(map[string]struct{}, len(m.Rows))
	for _, row := range m.Rows {
		before[m.motionRowKey(row)] = struct{}{}
	}
	m.anim.NoteReveal(before)
}

// NoteMoveTrail records the cursor's task and the place it holds before a move
// that may land it somewhere else.
func (m *OutlinerModel) NoteMoveTrail() {
	if m == nil || !m.anim.Armed() {
		return
	}
	if task, place, ok := m.cursorTaskPlace(); ok {
		m.anim.NoteTrail(task, place)
	}
}

// SettleMotion turns the intents this update left into runs. The shell calls
// it once per update, after every handler has run.
//
// A scroll starts when the window the cursor now keeps begins more than one row
// from the window the key left; it asks the window authority's pure form, never
// the authority itself. A reveal starts when the rebuilt rows hold any row the
// fold did not draw before. A trail starts each time the cursor's row names the
// noted task at a place other than the one last seen for it — a commit lands
// after its key, so the intent stands until input and re-arms once per landing.
func (m *OutlinerModel) SettleMotion() tea.Cmd {
	if m == nil || !m.anim.Armed() {
		return nil
	}
	var cmds []tea.Cmd
	if from, ok := m.anim.TakeScroll(); ok && m.LastViewport.VisibleRows > 0 && len(m.Rows) > 0 {
		window, _ := m.pinnedContextWindowAt(m.ViewScrollOffset, m.LastViewport.VisibleRows)
		if scrollJumps(from, window.startIdx) {
			cmds = append(cmds, m.anim.StartScroll(from))
		}
	}
	if before, ok := m.anim.TakeReveal(); ok {
		revealed, any := make([]bool, len(m.Rows)), false
		for index, row := range m.Rows {
			if _, drawn := before[m.motionRowKey(row)]; !drawn {
				revealed[index], any = true, true
			}
		}
		if any {
			cmds = append(cmds, m.anim.StartReveal(revealed))
		}
	}
	if task, place, ok := m.anim.TrailIntent(); ok {
		if current, now, onTask := m.cursorTaskPlace(); onTask && current == task && now != place {
			cmds = append(cmds, m.anim.StartTrail(task, now))
		}
	}
	return tea.Batch(cmds...)
}

func scrollJumps(from, to int) bool {
	return to-from > 1 || from-to > 1
}

// motionRowKey names a row across a rebuild: its kind, its task, and the folds
// that hold it. It is what a reveal compares, and nothing else reads it.
func (m *OutlinerModel) motionRowKey(row OutlinerRow) string {
	identity := m.rowIdentity(row)
	return fmt.Sprintf("%t|%t|%t|%t|%t|%s|%s|%s|%s|%s|%d|%v",
		row.IsSectionHeader, row.IsProjectHeader, row.IsSeparator, row.IsChecklistItem, row.IsNoteRow,
		identity.ProjectID, identity.TaskID, row.CollapseKey, row.Group.SectionKey, row.Group.ProjectKey,
		row.ChecklistIndex, row.NoteRow)
}

// cursorTaskPlace names the cursor's task and where it stands: its row, its
// project, its parent, its tree, and its group. A reorder moves the row, an
// indent the tree, a reparent the parent, and a project move the project.
func (m *OutlinerModel) cursorTaskPlace() (task, place string, ok bool) {
	if m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return "", "", false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return "", "", false
	}
	identity := m.rowIdentity(row)
	parent := ""
	if row.Task.ParentID != nil {
		parent = *row.Task.ParentID
	}
	return row.Task.ID, fmt.Sprintf("%d|%s|%s|%s|%s|%s", m.Cursor, identity.ProjectID, parent,
		row.Prefix, row.Group.SectionKey, row.Group.ProjectKey), true
}

// motionRowRange is the rows this frame paints. At rest it is the window
// ComputeOutlinerViewport decided. During a scroll it is the same number of
// screen rows starting at the interpolated offset, the pinned context row
// spent on one more row since the lane it would name is the settled window's;
// a ghost, which borrows a slot of the settled window, keeps the window still.
func (m *OutlinerModel) motionRowRange(viewport outlinerViewport, endIdx int) (first, last int, pinned bool) {
	first, last, pinned = viewport.startIdx, endIdx, viewport.pinned
	from, phase, ok := m.anim.ScrollFrom()
	if !ok || m.ghost != nil || !scrollJumps(from, viewport.startIdx) {
		return first, last, pinned
	}
	count := last - first
	if pinned {
		count++
	}
	start := motion.ScrollStart(from, viewport.startIdx, phase)
	start = min(max(start, 0), max(len(m.Rows)-count, 0))
	return start, min(start+count, len(m.Rows)), false
}

// revealedBlank is the line a reveal draws in place of a row it still
// withholds, or false where the row is drawn. The cursor's row is always drawn.
func (m *OutlinerModel) revealedBlank(withheld []bool, index, rowWidth int) (string, bool) {
	if index >= len(withheld) || !withheld[index] || index == m.Cursor {
		return "", false
	}
	return strings.Repeat(" ", max(rowWidth, 0)), true
}

// landingPaint is the trail on the cursor's band: the task the move landed
// lights the mark in the primary accent, over the flash's dimmer band for the
// first phase and the ordinary band for the second. It is a variant of the one
// band, never a second background, so background precedence is unchanged; a
// flash or a resting cursor already says something else and keeps its paint.
func (m *OutlinerModel) landingPaint(row OutlinerRow, paint cursorPaint) cursorPaint {
	if !paint.marked || paint.resting || paint.flash || row.Task == nil {
		return paint
	}
	task, phase, ok := m.anim.TrailTask()
	if !ok || task != row.Task.ID {
		return paint
	}
	paint.landing = theme.ColorPrimary
	if phase == 0 {
		paint.rowBg = theme.ColorFlashDimBg
	}
	return paint
}

// updatePaintTick answers the timers that change paint alone: the cursor
// flash, the departure ghost, the mode arrival, and the motion runs. It reports
// whether msg was one of them, so Update returns its command unchanged.
func (m *OutlinerModel) updatePaintTick(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case status.CursorFlashClearMsg:
		m.CursorFlash = false
		return nil, true
	case DepartureGhostTickMsg:
		return m.advanceDepartureGhost(msg), true
	case ModeArrivalTickMsg:
		m.endModeArrival(msg)
		return nil, true
	case MotionTickMsg:
		return m.anim.Advance(msg), true
	}
	return nil, false
}
