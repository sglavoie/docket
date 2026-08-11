package tui

import (
	"fmt"
	"image/color"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"
)

// The footer divider takes a severity color the moment status text changes and
// blends back to the view accent, so a message replaced at the bottom of a busy
// screen is not missed. Named flash, not pulse: workspacePulse is the title
// row's Inbox/Today/Overdue chip.
const (
	footerFlashFrames        = 5
	footerFlashFrameInterval = 60 * time.Millisecond
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
		return ColorSuccess
	case flashWarning:
		return ColorWarning
	case flashError:
		return ColorError
	default:
		return ColorPrimary
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

// footerFlashProbe is the status identity compared across one Update. The
// shell's own message carries a sequence, but m.err has ~87 assignment sites
// and no clock, so it is compared by text. The view stamp is only meaningful
// alongside the view it came from, hence the presentation ID.
type footerFlashProbe struct {
	statusSeq     uint64
	errText       string
	view          PresentationID
	viewStatusSeq uint64
}

func (m *AppModel) captureFooterFlashProbe() footerFlashProbe {
	probe := footerFlashProbe{
		statusSeq: m.statusSeq,
		view:      m.currentView(),
	}
	if m.err != nil {
		probe.errText = m.err.Error()
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
		severity = m.statusSeverity
	case after.view == before.view && after.viewStatusSeq != before.viewStatusSeq:
		severity = flashInfo
	}
	if severity == flashNone {
		return nil
	}

	m.footerFlashSeq++
	m.footerFlash = footerFlashState{severity: severity, seq: m.footerFlashSeq}
	return footerFlashTick(m.footerFlashSeq)
}

// advanceFooterFlash steps one frame and reports the command that schedules the
// next. A stale tick from a superseded flash is rejected by sequence.
func (m *AppModel) advanceFooterFlash(seq uint64) tea.Cmd {
	if !m.footerFlash.active() || m.footerFlash.seq != seq {
		return nil
	}
	m.footerFlash.frame++
	if m.footerFlash.frame >= footerFlashFrames {
		m.footerFlash = footerFlashState{}
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
