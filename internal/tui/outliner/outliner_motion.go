package outliner

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/motion"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The departure ghost is the one frame of motion a committed departure earns:
// the row the keypress was aimed at stays on screen at the position it
// vacated for a few phases, then the rows under it close up. What it paints
// says how the task left. A completion lands its check on the row as it stood,
// then strikes and fades it, so finishing reads as a check being made. A task
// that went somewhere — another plan, status, or project — is never struck:
// it names where it went, `→ Next` or `→ Website`, then fades, so a move reads
// differently from a finish without a glance at the footer. Only a departure
// with no destination to name — a cancel, a delete — keeps the bare strike. It is render-only — never a row of m.Rows, never a cursor
// position, never a window input — so every row invariant, every index, and
// ComputeOutlinerViewport are untouched; the render records the screen slot it
// borrowed in LastViewport so the two readers of screen rows, CursorScreenRow
// and the pointer's hit map, step over it.
//
// It is motion, so it exists only where the shell armed motion: an interactive
// launch. The harness and every test leave it off, which is what keeps a
// scenario's observations independent of a timer they never wait for.
var (
	departureGhostSchedule  = []time.Duration{160 * time.Millisecond, 160 * time.Millisecond}
	completionGhostSchedule = []time.Duration{120 * time.Millisecond, 150 * time.Millisecond, 150 * time.Millisecond}
	// A destination is read, so its first beat holds longer than a strike.
	relocationGhostSchedule = []time.Duration{280 * time.Millisecond, 160 * time.Millisecond}
)

// The looks a ghost paints. A completion runs ghostChecked, ghostStruck,
// ghostFaded; a relocation ghostLabeled then ghostLabelFaded; any other
// departure ghostStruck then ghostFaded.
const (
	ghostChecked = iota
	ghostStruck
	ghostFaded
	ghostLabeled
	ghostLabelFaded
)

// departureGhost is the struck row a departure leaves behind for a moment.
type departureGhost struct {
	// line is the row's plain text as the keypress saw it, and screenOffset
	// the list line it was drawn on, counted from the window's first line.
	line         string
	screenOffset int
	// done reports that the task departed by completing, so its state glyph
	// is repainted as the check it became, and icon is that check.
	done bool
	icon string
	// destination names where a task that did not finish went — a plan, a
	// status, or a project — and destinationStyle inks it. It is empty for a
	// completion and for a departure with nowhere to name.
	destination      string
	destinationStyle lipgloss.Style
	phase            int
	seq              int
}

// relocated reports a ghost that names its destination instead of striking.
func (g *departureGhost) relocated() bool { return !g.done && g.destination != "" }

// schedule is the delay each phase holds before the next.
func (g *departureGhost) schedule() []time.Duration {
	switch {
	case g.done:
		return completionGhostSchedule
	case g.relocated():
		return relocationGhostSchedule
	}
	return departureGhostSchedule
}

// look is what the current phase paints.
func (g *departureGhost) look() int {
	switch {
	case g.done:
		return g.phase
	case g.relocated():
		return g.phase + ghostLabeled
	}
	return g.phase + ghostStruck
}

// DepartureGhostTickMsg advances the ghost Seq names; a stale one is inert.
type DepartureGhostTickMsg struct{ Seq int }

// SetMotion arms or disarms the session's motion. Disarming drops a live
// ghost, the arrival, and every motion run at once.
func (m *OutlinerModel) SetMotion(enabled bool) {
	m.motion = enabled
	m.anim.SetEmit(emitMotionTick)
	m.anim.SetArmed(enabled)
	if !enabled {
		m.ghost = nil
		m.arrival = nil
	}
}

func departureGhostTick(delay time.Duration, seq int) tea.Cmd {
	return status.TickCmd(delay, func(time.Time) tea.Msg {
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
	schedule := m.ghost.schedule()
	if m.ghost.phase >= len(schedule) {
		m.ghost = nil
		return nil
	}
	return departureGhostTick(schedule[m.ghost.phase], msg.Seq)
}

// DropMotion retires every animation still settling on this view, and every
// intent still waiting to become one; the shell calls it on each keystroke,
// click, and wheel step, ahead of the operation the input reaches, so that
// operation's own intent is the only one standing when the update settles.
func (m *OutlinerModel) DropMotion() {
	if m != nil {
		m.dropDepartureGhost()
		m.anim.Drop()
		m.foldBefore = nil
	}
}

// collapseGapLines is the blank rows a live collapse holds open under each
// row, by index. A ghost, which borrows a slot of its own, keeps it still.
func (m *OutlinerModel) collapseGapLines() map[int]int {
	if m.ghost != nil {
		return nil
	}
	gaps := m.anim.CollapseGaps(len(m.Rows))
	if len(gaps) == 0 {
		return nil
	}
	out := make(map[int]int, len(gaps))
	for _, gap := range gaps {
		out[gap.After] += gap.Rows
	}
	return out
}

// dockRollLines blanks the tail of an opening dock's document for the phase
// it is in: the divider and the rows a third, then two thirds, of the way
// down are drawn, and every line keeps its place, so the dock is the height
// it reserved on every frame.
func (m *OutlinerModel) dockRollLines(lines []string, width int) []string {
	phase, ok := m.anim.DockPhase()
	if !ok || len(lines) <= 1 {
		return lines
	}
	body := len(lines) - 1
	shown := motion.DockRows(body, phase)
	if shown >= body {
		return lines
	}
	out := append([]string(nil), lines[:1+shown]...)
	blank := strings.Repeat(" ", max(width, 0))
	for range body - shown {
		out = append(out, blank)
	}
	return out
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
// It carries motion's own tick so the shell can route it without importing
// motion; emitMotionTick, the fire hook SetMotion hands the runs, is its one
// emitter.
type MotionTickMsg struct{ Tick motion.TickMsg }

func emitMotionTick(tick motion.TickMsg) tea.Msg { return MotionTickMsg{Tick: tick} }

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
// opens can be told from the rows that were already there, and the rows it
// closes from the rows that stay.
func (m *OutlinerModel) NoteFoldReveal() {
	if m == nil || !m.anim.Armed() {
		return
	}
	before := make(map[string]struct{}, len(m.Rows))
	order := make([]string, 0, len(m.Rows))
	for _, row := range m.Rows {
		key := m.motionRowKey(row)
		before[key] = struct{}{}
		order = append(order, key)
	}
	m.anim.NoteReveal(before)
	m.foldBefore = order
}

// foldCollapseGaps names the blocks a fold closed: each maximal run of rows
// drawn before the fold and gone after it, under the surviving row it hung
// from. A block above the cursor is not animated, since closing it moves the
// cursor's own screen row, and CursorScreenRow reads the settled window; a
// block is capped at the window it could have filled.
func (m *OutlinerModel) foldCollapseGaps(before []string) []motion.Gap {
	after := make(map[string]int, len(m.Rows))
	for index, row := range m.Rows {
		after[m.motionRowKey(row)] = index
	}
	limit := max(m.LastViewport.VisibleRows, 0)
	var gaps []motion.Gap
	anchor, run := -1, 0
	flush := func() {
		if run > 0 && anchor >= 0 && anchor >= m.Cursor {
			gaps = append(gaps, motion.Gap{After: anchor, Rows: min(run, limit)})
		}
		run = 0
	}
	for _, key := range before {
		index, kept := after[key]
		if !kept {
			run++
			continue
		}
		flush()
		anchor = index
	}
	flush()
	return gaps
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
	if before := m.foldBefore; before != nil {
		m.foldBefore = nil
		if gaps := m.foldCollapseGaps(before); len(gaps) > 0 && m.ghost == nil {
			cmds = append(cmds, m.anim.StartCollapse(gaps))
		}
	}
	// The dock rolls out when it opens, never when it is already standing: the
	// cursor moving between rows under an open dock swaps the document in place.
	dockOpen := m.PeekDockVisible() && !m.PeekFullscreen
	if dockOpen && !m.dockSeen {
		cmds = append(cmds, m.anim.StartDock())
	}
	m.dockSeen = dockOpen
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

// landingPaint is the trail a landed task wears. On the cursor's band the mark
// lights in the primary accent, over the flash's dimmer band for the first
// phase and the ordinary band for the second. It is a variant of the one band,
// never a second background, so background precedence is unchanged; a flash or
// a resting cursor already says something else and keeps its paint.
//
// A task that landed off the cursor — a capture, which keeps the user's place —
// wears the mark alone, in the accent and then dimmed, and no band: the row says
// where the new work went without claiming the keyboard is there.
func (m *OutlinerModel) landingPaint(row OutlinerRow, paint cursorPaint) cursorPaint {
	if row.Task == nil {
		return paint
	}
	task, phase, ok := m.anim.TrailTask()
	if !ok || task != row.Task.ID {
		return paint
	}
	if !paint.marked {
		paint.landing = theme.ColorPrimary
		if phase > 0 {
			paint.landing = theme.ColorDimmed
		}
		return paint
	}
	if paint.resting || paint.flash {
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
		return m.anim.Advance(msg.Tick), true
	}
	return nil, false
}

// LightLanding lights the trail on the task a creation just minted, wherever
// the rebuilt projection draws it: the same accent a move leaves where it
// lands, so a capture is seen arriving rather than only counted in a status.
// The cursor is not moved; a task the view does not draw lights nothing, and a
// session without motion starts nothing.
func (m *OutlinerModel) LightLanding(taskID string) tea.Cmd {
	if m == nil || !m.anim.Armed() || taskID == "" {
		return nil
	}
	return m.anim.StartTrail(taskID, "")
}

// renderLandedCheck paints a completed row's first beat: what leads the check
// stays muted, the check is bold in the success ink, and the title keeps the
// ordinary text color, unstruck, so the check is the one thing that changed.
func renderLandedCheck(text, icon string) string {
	before, after, found := "", text, false
	if icon != "" {
		before, after, found = strings.Cut(text, icon)
	}
	check := lipgloss.NewStyle().Foreground(theme.ColorSuccess).Bold(true)
	if !found {
		return check.Render(text)
	}
	return theme.MutedStyle.Render(before) + check.Render(icon) + after
}

// appendGapLines adds the blank rows a closing gap still holds under a row.
func appendGapLines(lines []string, rows, width int) []string {
	for range rows {
		lines = append(lines, strings.Repeat(" ", max(width, 0)))
	}
	return lines
}

// fitCollapseGap bounds the list block a closing gap lengthened. The gap pushes
// the rows under it down by the rows it still holds: into the padding under a
// short projection, and otherwise the window's tail slides out under the dock
// rather than past it, so the block stays the height the window paid for.
// gapRows tells the dock's padding what the gap already spent.
func (m *OutlinerModel) fitCollapseGap(lines []string, settled, visible int, gapping bool) []string {
	m.LastViewport.gapRows = 0
	if !gapping || len(lines) <= settled {
		return lines
	}
	lines = lines[:min(len(lines), max(settled, visible))]
	m.LastViewport.gapRows = len(lines) - settled
	return lines
}
