package tui

import (
	"fmt"
	"image/color"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"

	"charm.land/lipgloss/v2"
)

// The footer divider takes a severity color the moment status text changes,
// lands as a full-width impact, then sweeps and dissolves back into the view
// accent so a message replaced at the bottom of a busy screen is not missed.
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
)

// color resolves against the live palette rather than the palette in effect
// when the flash was armed, so a theme toggle mid-flash cannot strand a frame
// on the previous theme's color.
func (s footerFlashSeverity) color() color.Color {
	switch s {
	case flashSuccess:
		return theme.ColorSuccess
	case flashWarning:
		return theme.ColorWarning
	case flashError:
		return theme.ColorError
	default:
		return theme.ColorPrimary
	}
}

type footerFlashState struct {
	severity footerFlashSeverity
	frame    int
	seq      uint64
}

func (s footerFlashState) active() bool { return s.severity != flashNone }

// statusStamper is the second status surface: views embedding
// InteractiveViewState own a status line the shell does not write.
type statusStamper interface {
	StatusStamp() uint64
}

// viewStatusOwner is that same status line seen by the expiry timer: it stamps
// generations, declares whether the current message is transient, and clears
// itself when its tick comes back.
//
// Every method here must be exported. An unexported method name is qualified by
// its declaring package in a Go method set, so once InteractiveViewState moved
// down to internal/tui/listsurface, a lowercase clearStatus() in this literal
// became unsatisfiable by any view — a silent ok == false that no build, vet, or
// lint pass reports. The assertion below is what makes that failure loud.
type viewStatusOwner interface {
	statusStamper
	StatusIsTransient() bool
	ClearStatus()
}

// The Outliner is the view this interface exists for, so its conformance is
// asserted at compile time rather than discovered as a dead type switch.
var _ viewStatusOwner = (*outliner.OutlinerModel)(nil)

// footerFlashProbe is the status identity compared across one Update. The
// shell's own message carries a sequence, but the error beside it has ~87
// assignment sites and no clock, so it is compared by text. The view stamp is
// only meaningful alongside the view it came from, hence the presentation ID.
type footerFlashProbe struct {
	statusSeq     uint64
	errText       string
	view          presentation.PresentationID
	viewStatusSeq uint64
}

func (m *AppModel) captureFooterFlashProbe() footerFlashProbe {
	probe := footerFlashProbe{
		statusSeq: m.feedback.seq,
		view:      m.currentView(),
	}
	if m.feedback.err != nil {
		probe.errText = m.feedback.err.Error()
	}
	if stamper, ok := m.activeView().(statusStamper); ok {
		probe.viewStatusSeq = stamper.StatusStamp()
	}
	return probe
}

// armFooterFlashSince starts a flash when this Update changed status text.
// Cleared text arms nothing: neither clearStatus nor the auto-clear timers
// advance a sequence, and a vanished error fails the non-empty test.
func (m *AppModel) armFooterFlashSince(before footerFlashProbe) tea.Cmd {
	after := m.captureFooterFlashProbe()

	severity := flashNone
	switch {
	case after.errText != "" && after.errText != before.errText:
		severity = flashError
	case after.statusSeq != before.statusSeq:
		severity = m.feedback.severity
	case after.view == before.view && after.viewStatusSeq != before.viewStatusSeq:
		severity = flashInfo
	}
	if severity == flashNone {
		return nil
	}

	return footerFlashTick(m.feedback.armFlash(severity))
}

// scheduleViewStatusExpiry arms the auto-clear for a view-owned status message
// this Update introduced. It reads the same before/after comparison as the
// flash, since a new message is exactly what both react to; a sticky message
// (the multi-select summary) schedules nothing and stays until its view clears it.
func (m *AppModel) scheduleViewStatusExpiry(before footerFlashProbe) tea.Cmd {
	after := m.captureFooterFlashProbe()
	if after.view != before.view || after.viewStatusSeq == before.viewStatusSeq {
		return nil
	}
	owner, ok := m.activeView().(viewStatusOwner)
	if !ok || !owner.StatusIsTransient() {
		return nil
	}
	return status.ViewStatusTimeout(after.view, after.viewStatusSeq)
}

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
