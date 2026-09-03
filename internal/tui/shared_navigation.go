package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/status"
)

// jumpCursor moves cursor by delta within [0, total-1].
func jumpCursor(cursor *int, total, delta int) {
	if total <= 0 {
		*cursor = 0
		return
	}
	*cursor += delta
	if *cursor >= total {
		*cursor = total - 1
	}
	if *cursor < 0 {
		*cursor = 0
	}
}

// handleListNavigation handles standard direct list navigation keys.
func handleListNavigation(key string, cursor *int, total, height int, flash *bool) (bool, tea.Cmd) {
	old := *cursor
	switch key {
	case "up", "k":
		if *cursor > 0 {
			*cursor--
		}
		return true, nil
	case "down", "j":
		if *cursor < total-1 {
			*cursor++
		}
		return true, nil
	case "home":
		*cursor = 0
		return true, flashIfJumped(old, *cursor, flash)
	case "G", "end":
		if total > 0 {
			*cursor = total - 1
		}
		return true, flashIfJumped(old, *cursor, flash)
	case "ctrl+u", "pgup":
		jumpCursor(cursor, total, -HalfPage(height))
		return true, flashIfJumped(old, *cursor, flash)
	case "ctrl+d", "pgdown":
		jumpCursor(cursor, total, HalfPage(height))
		return true, flashIfJumped(old, *cursor, flash)
	}
	return false, nil
}

func flashIfJumped(oldCursor, newCursor int, flash *bool) tea.Cmd {
	delta := newCursor - oldCursor
	if delta < 0 {
		delta = -delta
	}
	if delta > 1 {
		*flash = true
		return status.CursorFlashTimeout()
	}
	return nil
}

func getCurrentSectionIdx(cursor int, sectionStarts []int) int {
	for i := len(sectionStarts) - 1; i >= 0; i-- {
		if cursor >= sectionStarts[i] {
			return i
		}
	}
	return 0
}

func navigateToNextSection(cursor int, sectionStarts []int) int {
	if len(sectionStarts) <= 1 {
		return -1
	}
	idx := getCurrentSectionIdx(cursor, sectionStarts)
	return sectionStarts[(idx+1)%len(sectionStarts)]
}

func navigateToPrevSection(cursor int, sectionStarts []int) int {
	if len(sectionStarts) <= 1 {
		return -1
	}
	idx := getCurrentSectionIdx(cursor, sectionStarts)
	prev := idx - 1
	if prev < 0 {
		prev = len(sectionStarts) - 1
	}
	return sectionStarts[prev]
}

// EscStep represents one layer in an Esc-key dismissal cascade.
type EscStep struct {
	Active func() bool
	Clear  func()
}

func RunEscCascade(steps []EscStep) bool {
	for _, step := range steps {
		if step.Active() {
			step.Clear()
			return true
		}
	}
	return false
}

func AnyEscStepActive(steps []EscStep) bool {
	for _, step := range steps {
		if step.Active() {
			return true
		}
	}
	return false
}

func helpCmd(name string, items []HelpItem) tea.Cmd {
	return func() tea.Msg {
		return ShowHelpMsg{ViewName: name, Items: items}
	}
}
