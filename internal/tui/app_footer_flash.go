package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

const (
	footerDividerIdleGlyph  = '─'
	footerDividerFlashGlyph = '━'
)

// renderFooterDivider turns a status change into three visible phases: a brief
// full-width impact, an eased severity sweep, and a shrinking dissolve. The
// glyph motion remains visible when the explicit --no-color flag is active.
func (m *AppModel) renderFooterDivider(width int) string {
	if width <= 0 {
		width = 80
	}
	base := theme.ColorMuted
	if !m.feedback.flash.active() {
		if cells := m.undoCountdownCells(width); cells > 0 {
			return renderUndoCountdownDivider(width, cells)
		}
		return chrome.RenderHorizontalDivider(width)
	}

	highlight := m.feedback.flash.severity.color()
	frame := min(max(m.feedback.flash.frame, 0), footerFlashFrames-1)
	if frame < footerFlashImpactFrames {
		return chrome.RenderHorizontalDivider(
			width,
			chrome.WithDividerColor(highlight),
			chrome.WithDividerGlyph(footerDividerFlashGlyph),
		)
	}

	segmentWidth := footerFlashSegmentWidth(width)
	if frame < footerFlashImpactFrames+footerFlashSweepFrames {
		sweepFrame := frame - footerFlashImpactFrames
		progress := float64(sweepFrame) / float64(footerFlashSweepFrames-1)
		start := int(math.Round(easeInOutCubic(progress) * float64(width-segmentWidth)))
		return renderFooterSweep(width, start, segmentWidth, base, highlight)
	}

	settleFrame := frame - footerFlashImpactFrames - footerFlashSweepFrames
	progress := float64(settleFrame+1) / float64(footerFlashSettleFrames+1)
	eased := easeInOutCubic(progress)
	segmentWidth = max(int(math.Round(float64(segmentWidth)*(1-eased))), 1)
	start := width - segmentWidth
	highlight = blendColor(highlight, base, eased)
	return renderFooterSweep(width, start, segmentWidth, base, highlight)
}

func renderFooterSweep(width, start, segmentWidth int, accent, highlight color.Color) string {
	start = min(max(start, 0), width)
	segmentWidth = min(max(segmentWidth, 0), width-start)

	var divider strings.Builder
	divider.Grow(width * 4)
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, start, accent)
	for index := range segmentWidth {
		headProgress := 1.0
		if segmentWidth > 1 {
			headProgress = float64(index) / float64(segmentWidth-1)
		}
		trailFade := footerFlashTrailFade * (1 - easeInOutCubic(headProgress))
		cellColor := blendColor(highlight, accent, trailFade)
		writeFooterDividerRun(&divider, footerDividerFlashGlyph, 1, cellColor)
	}
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, width-start-segmentWidth, accent)
	return divider.String()
}

func writeFooterDividerRun(builder *strings.Builder, glyph rune, count int, foreground color.Color) {
	if count <= 0 {
		return
	}
	style := lipgloss.NewStyle().Foreground(foreground)
	builder.WriteString(style.Render(strings.Repeat(string(glyph), count)))
}

// renderFooterContent fills each footer content row with the active flash wash.
// Child foreground and badge styles remain intact; after a child style resets,
// the wash is restored so unstyled text and trailing cells do not develop holes.
func (m *AppModel) renderFooterContent(content string, width int) string {
	if !m.feedback.flash.active() || content == "" {
		return content
	}
	if width <= 0 {
		width = 80
	}

	background := m.footerFlashBackgroundColor()
	prefix, suffix := footerBackgroundStyleParts(background)
	if prefix == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	for index, line := range lines {
		line = textfit.TruncateToVisualWidth(line, width)
		if gap := width - lipgloss.Width(line); gap > 0 {
			line += strings.Repeat(" ", gap)
		}
		line = strings.ReplaceAll(line, ansi.ResetStyle, ansi.ResetStyle+prefix)
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+prefix)
		lines[index] = prefix + line + suffix
	}
	return strings.Join(lines, "\n")
}

func footerBackgroundStyleParts(background color.Color) (prefix, suffix string) {
	const marker = "x"
	styled := lipgloss.NewStyle().Background(background).Render(marker)
	prefix, suffix, ok := strings.Cut(styled, marker)
	if !ok || prefix == "" {
		return "", ""
	}
	return prefix, suffix
}

// The footer divider takes a severity color the moment warning or error text
// arrives, lands as a full-width impact, then sweeps and dissolves back into the
// view accent so a message that needs attention at the bottom of a busy screen
// is not missed. Routine outcomes change the text and nothing else.
// Named flash, not pulse: workspacePulse is the title row's counts chip.
const (
	footerFlashFrames        = 20
	footerFlashFrameInterval = 25 * time.Millisecond
	footerFlashImpactFrames  = 4
	footerFlashSweepFrames   = 13
	footerFlashSettleFrames  = footerFlashFrames - footerFlashImpactFrames - footerFlashSweepFrames
	footerFlashTrailFade     = 0.8
	footerFlashBackgroundMix = 0.14
)

type footerFlashSeverity int

const (
	flashNone footerFlashSeverity = iota
	flashInfo
	flashSuccess
	flashWarning
	flashError
	// flashCelebrate is the one success that moves: the commit that empties
	// Today. It is never recorded by an ordinary setter, only by
	// celebrateClearedToday, and only in a session with motion.
	flashCelebrate
)

// color resolves against the live palette rather than the palette in effect
// when the flash was armed, so a theme toggle mid-flash cannot strand a frame
// on the previous theme's color.
func (s footerFlashSeverity) color() color.Color {
	switch s {
	case flashSuccess, flashCelebrate:
		return theme.ColorSuccess
	case flashWarning:
		return theme.ColorWarning
	case flashError:
		return theme.ColorError
	default:
		return theme.ColorPrimary
	}
}

// animates reports whether this severity earns the divider animation and the
// footer wash beneath it. Only a warning and an error do.
//
// A success or an informational line is the expected answer to the key that was
// just pressed: the text says what happened and the reader already knew to look
// for it, so motion at the bottom of the screen buys nothing and spends the one
// thing the warning needs — that the same motion is unusual. Severity is still
// recorded for every status, because that record is exactly what tells the two
// apart here. The one exception is flashCelebrate, the commit that empties
// Today: it happens at most once a day, so its motion stays unusual too.
func (s footerFlashSeverity) animates() bool {
	return s == flashWarning || s == flashError || s == flashCelebrate
}

type footerFlashState struct {
	severity footerFlashSeverity
	frame    int
	seq      uint64
}

func (s footerFlashState) active() bool { return s.severity != flashNone }

// advanceFooterFlash steps one frame and reports the command that schedules the
// next. A stale tick from a superseded flash is rejected by sequence.
func (m *AppModel) advanceFooterFlash(seq uint64) tea.Cmd {
	if !m.feedback.advanceFlash(seq) {
		return nil
	}
	return footerFlashTick(seq)
}

// blendColor mixes from toward to. Palette entries are opaque, so a per-channel
// lerp over 8-bit components is exact; a translucent input cannot be lerped
// meaningfully through alpha-premultiplied values, so it steps instead.
func blendColor(from, to color.Color, t float64) color.Color {
	if from == nil {
		return to
	}
	if to == nil {
		return from
	}
	t = min(max(t, 0), 1)

	fromR, fromG, fromB, fromA := rgba8(from)
	toR, toG, toB, toA := rgba8(to)
	if fromA != 0xff || toA != 0xff {
		if t < 0.5 {
			return from
		}
		return to
	}

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x",
		lerp8(fromR, toR, t),
		lerp8(fromG, toG, t),
		lerp8(fromB, toB, t),
	))
}

func rgba8(c color.Color) (r, g, b, a uint8) {
	red, green, blue, alpha := c.RGBA()
	return uint8(red >> 8), uint8(green >> 8), uint8(blue >> 8), uint8(alpha >> 8)
}

func lerp8(from, to uint8, t float64) uint8 {
	return uint8(math.Round(float64(from) + (float64(to)-float64(from))*t))
}

// easeInOutCubic gives the sweep a deliberate start and soft landing instead
// of moving a fixed number of cells on every frame.
func easeInOutCubic(t float64) float64 {
	t = min(max(t, 0), 1)
	if t < 0.5 {
		return 4 * t * t * t
	}
	remaining := -2*t + 2
	return 1 - remaining*remaining*remaining/2
}

func footerFlashSegmentWidth(width int) int {
	return min(max(width/6, 12), min(20, width))
}

// footerFlashBackgroundColor adds a restrained severity wash beneath the
// footer content. It arrives with the divider impact and eases all the way back
// to the theme background, leaving the moving divider as the dominant cue.
func (m *AppModel) footerFlashBackgroundColor() color.Color {
	background := theme.ColorBackground
	if !m.feedback.flash.active() {
		return background
	}

	frame := min(max(m.feedback.flash.frame, 0), footerFlashFrames-1)
	progress := float64(frame) / float64(footerFlashFrames-1)
	mix := footerFlashBackgroundMix * (1 - easeInOutCubic(progress))
	return blendColor(background, m.feedback.flash.severity.color(), mix)
}

// The undo countdown draws the inline undo's window on the footer divider
// above it: a run of accented rule from the left edge that drains as the
// window closes, so how long `u undo` still answers is visible rather than
// guessed. It changes no text and no width, so nothing in the footer sheds or
// moves for it, and it yields to the flash, which owns that divider whenever
// it runs. It is motion, so only a session with motion draws it.
//
// It is frame-counted rather than read off the clock, as the flash is: the
// render stays pure, and a superseded window's ticks are voided by sequence.
const (
	undoCountdownSteps = 24
	undoCountdownMix   = 0.55
)

type undoCountdown struct {
	seq   uint64
	frame int
	live  bool
}

// UndoCountdownTickMsg drains the countdown Seq names by one step.
type UndoCountdownTickMsg struct{ Seq uint64 }

func undoCountdownTick(seq uint64) tea.Cmd {
	return status.TickCmd(UndoToastTimeout/undoCountdownSteps, func(time.Time) tea.Msg {
		return UndoCountdownTickMsg{Seq: seq}
	})
}

// startUndoCountdown begins draining for the undo window just armed.
func (m *AppModel) startUndoCountdown() tea.Cmd {
	if !m.lifecycle.motion {
		return nil
	}
	c := &m.feedback.countdown
	c.seq++
	c.frame = 0
	c.live = true
	return undoCountdownTick(c.seq)
}

// advanceUndoCountdown drains one step, stopping when the window has closed
// or the toast it measured was replaced.
func (m *AppModel) advanceUndoCountdown(seq uint64) tea.Cmd {
	c := &m.feedback.countdown
	if !c.live || c.seq != seq {
		return nil
	}
	if m.feedback.undoCb == nil {
		c.live = false
		return nil
	}
	c.frame++
	if c.frame >= undoCountdownSteps {
		c.live = false
		return nil
	}
	return undoCountdownTick(seq)
}

// undoCountdownCells is how much of a width-wide divider the countdown still
// holds, or zero when none is drawn.
func (m *AppModel) undoCountdownCells(width int) int {
	c := m.feedback.countdown
	if !c.live || m.feedback.undoCb == nil || width <= 0 {
		return 0
	}
	remaining := 1 - float64(c.frame)/float64(undoCountdownSteps)
	return min(max(int(math.Round(remaining*float64(width))), 0), width)
}

// renderUndoCountdownDivider is the idle divider with its draining run.
func renderUndoCountdownDivider(width, cells int) string {
	var divider strings.Builder
	accent := blendColor(theme.ColorMuted, theme.ColorPrimary, undoCountdownMix)
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, cells, accent)
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, width-cells, theme.ColorMuted)
	return divider.String()
}

// outlinerModeForMotion is the mode the Update's motion compares across one
// Update, or the zero mode when there is no Outliner.
func (m *AppModel) outlinerModeForMotion() chrome.OutlinerModeID {
	if m.crossOutliner == nil {
		return ""
	}
	return m.crossOutliner.ActiveOutlinerMode()
}

// The two phases a changed pulse count is drawn in: lit in its accent, then
// bold in its own color, then plain. Long enough for an eye on the row to
// catch which number moved, short enough that nothing waits on it.
const (
	pulseMotionFreshDuration    = 260 * time.Millisecond
	pulseMotionSettlingDuration = 420 * time.Millisecond
	// pulseMotionTickerFrame is each in-between value a count that jumped by
	// several draws ahead of the fresh phase: a few frames, under a fifth of a
	// second however far it moved.
	pulseMotionTickerFrame = 40 * time.Millisecond
)

// pulseMotion is the workspace pulse's count change being drawn, and the
// generation that voids a superseded change's ticks.
type pulseMotion struct {
	change pulse.PulseChange
	seq    int
	// from and to are the pulses a ticking change counts between, frame the
	// in-between value it draws, and frames how many it draws in all.
	from, to      pulse.WorkspacePulse
	frame, frames int
	// away is the pulse as it stood when the terminal lost focus, held while it
	// stays unfocused: a count that moves meanwhile — a sync, a reload, the
	// periodic tick — is lit when the user comes back to see it, measured from
	// here, rather than settling unseen in a background window. returning is
	// the focus report that spends it.
	away      *pulse.WorkspacePulse
	returning bool
}

// notePulseFocus records the terminal's focus reports for the pulse: a blur
// holds the counts as they stand, and the focus after it marks the update that
// lights whatever moved since.
func (m *AppModel) notePulseFocus(focused bool) {
	if !focused {
		if m.pulseMotion.away == nil {
			away := m.workspacePulse
			m.pulseMotion.away = &away
		}
		m.pulseMotion.returning = false
		return
	}
	m.pulseMotion.returning = m.pulseMotion.away != nil
}

// pulseMotionTickMsg advances the change Seq names one phase.
type pulseMotionTickMsg struct{ Seq int }

// markPulseChange draws the counts that moved across one Update. It runs from
// the deferred closure in AppModel.Update, so every seam that recomputes the
// pulse — a commit, an undo, a sync, the periodic tick — is covered without
// one of them asking. A session without motion draws nothing, and neither does
// the launch, whose first pulse is a reading rather than a change: launchArmed
// is whether the launch focus was still owed when the Update began.
func (m *AppModel) markPulseChange(before pulse.WorkspacePulse, launchArmed bool) tea.Cmd {
	if away := m.pulseMotion.away; away != nil {
		if !m.pulseMotion.returning {
			return nil
		}
		before = *away
		m.pulseMotion.away, m.pulseMotion.returning = nil, false
	}
	if !m.lifecycle.motion || launchArmed || m.lifecycle.launchFocusArmed() {
		return nil
	}
	change := pulse.PulseChangeBetween(before, m.workspacePulse)
	if !change.Any() {
		return nil
	}
	m.pulseMotion.seq++
	m.pulseMotion.from, m.pulseMotion.to = before, m.workspacePulse
	m.pulseMotion.frame, m.pulseMotion.frames = 0, pulse.TickerFrames(before, m.workspacePulse)
	if m.pulseMotion.frames > 0 {
		m.pulseMotion.change = change.TickerAt(before, m.workspacePulse, 0, m.pulseMotion.frames)
		return pulseMotionTick(pulseMotionTickerFrame, m.pulseMotion.seq)
	}
	m.pulseMotion.change = change
	return pulseMotionTick(pulseMotionFreshDuration, m.pulseMotion.seq)
}

func (m *AppModel) advancePulseMotion(msg pulseMotionTickMsg) tea.Cmd {
	if msg.Seq != m.pulseMotion.seq || !m.pulseMotion.change.Any() {
		return nil
	}
	if m.pulseMotion.change.Ticking != 0 {
		m.pulseMotion.frame++
		m.pulseMotion.change = m.pulseMotion.change.TickerAt(m.pulseMotion.from, m.pulseMotion.to, m.pulseMotion.frame, m.pulseMotion.frames)
		if m.pulseMotion.change.Ticking != 0 {
			return pulseMotionTick(pulseMotionTickerFrame, msg.Seq)
		}
		return pulseMotionTick(pulseMotionFreshDuration, msg.Seq)
	}
	if !m.pulseMotion.change.Settling {
		m.pulseMotion.change.Settling = true
		return pulseMotionTick(pulseMotionSettlingDuration, msg.Seq)
	}
	m.pulseMotion.change = pulse.PulseChange{}
	return nil
}

func pulseMotionTick(delay time.Duration, seq int) tea.Cmd {
	return status.TickCmd(delay, func(time.Time) tea.Msg { return pulseMotionTickMsg{Seq: seq} })
}

// shownWorkspacePulse is the pulse a view is handed to draw: the measured
// counts, annotated with the change still settling.
func (m *AppModel) shownWorkspacePulse() pulse.WorkspacePulse {
	return m.workspacePulse.Lit(m.pulseMotion.change)
}
