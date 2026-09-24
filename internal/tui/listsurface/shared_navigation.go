package listsurface

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
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

// HandleListNavigation handles standard direct list navigation keys.
func HandleListNavigation(key string, cursor *int, total, height int, flash *bool) (bool, tea.Cmd) {
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
		return true, FlashIfJumped(old, *cursor, flash)
	case "G", "end":
		if total > 0 {
			*cursor = total - 1
		}
		return true, FlashIfJumped(old, *cursor, flash)
	case "ctrl+u", "pgup":
		jumpCursor(cursor, total, -chrome.HalfPage(height))
		return true, FlashIfJumped(old, *cursor, flash)
	case "ctrl+d", "pgdown":
		jumpCursor(cursor, total, chrome.HalfPage(height))
		return true, FlashIfJumped(old, *cursor, flash)
	}
	return false, nil
}

func FlashIfJumped(oldCursor, newCursor int, flash *bool) tea.Cmd {
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

// EscStep represents one layer in an Esc-key dismissal cascade. Action names
// what clearing this layer does, in the imperative a footer spells after the
// key. Dispatch and guidance read the same slice, so a hint promising an act
// the cascade will not perform is not expressible.
type EscStep struct {
	Active func() bool
	Action string
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

// NextEscAction names the act the next Esc press performs, or "" when no layer
// is standing. It walks the cascade in the order RunEscCascade walks it, so the
// answer is the step that press would actually run rather than the widest one
// available.
func NextEscAction(steps []EscStep) string {
	for _, step := range steps {
		if step.Active() {
			return step.Action
		}
	}
	return ""
}

func AnyEscStepActive(steps []EscStep) bool {
	for _, step := range steps {
		if step.Active() {
			return true
		}
	}
	return false
}
