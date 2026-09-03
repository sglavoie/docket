package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/status"
)

// FooterFlashTickMsg advances the footer divider flash one frame. Seq
// identifies the flash generation so a tick from a superseded flash is ignored.
type FooterFlashTickMsg struct{ Seq uint64 }

// footerFlashTick schedules the next frame of the footer divider flash.
func footerFlashTick(seq uint64) tea.Cmd {
	return status.TickCmd(footerFlashFrameInterval, func(time.Time) tea.Msg {
		return FooterFlashTickMsg{Seq: seq}
	})
}
