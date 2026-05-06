package tui

import (
	tea "charm.land/bubbletea/v2"
)

// handleGotoTopPrefix handles the common "gg" → jump-to-top key sequence.
// Call this when pendingPrefix == "g". It resets the cursor to 0 and invokes
// any extra callbacks (e.g. resetting a scroll offset). Returns true so
// callers can short-circuit. For views with custom g-prefix keys (e.g. gX),
// check those first and fall through to this for the default "gg" behavior.
func handleGotoTopPrefix(key string, cursor *int, flash *bool, extras ...func()) (bool, tea.Cmd) {
	if key == "g" {
		old := *cursor
		*cursor = 0
		for _, fn := range extras {
			fn()
		}
		return true, flashIfJumped(old, *cursor, flash)
	}
	return true, nil
}

// jumpCursor moves cursor by delta within [0, total-1].
func jumpCursor(cursor *int, total, delta int) {
	*cursor += delta
	if *cursor >= total {
		*cursor = total - 1
	}
	if *cursor < 0 {
		*cursor = 0
	}
}

// handleListNavigation handles standard list navigation keys (up/k, down/j,
// G, ctrl+u/pgup, ctrl+d/pgdown). Returns true if the key was handled.
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

	case "G":
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

// flashIfJumped sets *flash to true and returns cursorFlashTimeout() if the
// cursor moved by more than 1 position. Otherwise returns nil.
func flashIfJumped(oldCursor, newCursor int, flash *bool) tea.Cmd {
	delta := newCursor - oldCursor
	if delta < 0 {
		delta = -delta
	}
	if delta > 1 {
		*flash = true
		return cursorFlashTimeout()
	}
	return nil
}

// getCurrentSectionIdx returns the index of the section (or group) whose start
// position is ≤ cursor. sectionStarts must be sorted ascending. Returns 0 if
// cursor is before all sections.
func getCurrentSectionIdx(cursor int, sectionStarts []int) int {
	for i := len(sectionStarts) - 1; i >= 0; i-- {
		if cursor >= sectionStarts[i] {
			return i
		}
	}
	return 0
}

// navigateToNextSection returns the cursor position of the next section header
// with wrapping. Returns -1 if there is fewer than 2 sections.
func navigateToNextSection(cursor int, sectionStarts []int) int {
	if len(sectionStarts) <= 1 {
		return -1
	}
	idx := getCurrentSectionIdx(cursor, sectionStarts)
	next := (idx + 1) % len(sectionStarts)
	return sectionStarts[next]
}

// navigateToPrevSection returns the cursor position of the previous section
// header with wrapping. Returns -1 if there is fewer than 2 sections.
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

// handleTabCycleKey handles the standard tab cycling keys (shift+left and
// shift+right). It calls Cycle on the TabState and then invokes afterCycle.
// Returns true if the key was consumed.
func handleTabCycleKey(key string, tabs *TabState, afterCycle func()) bool {
	delta := 0
	switch key {
	case "shift+left":
		delta = -1
	case "shift+right":
		delta = 1
	}
	if delta == 0 {
		return false
	}
	tabs.Cycle(delta)
	if afterCycle != nil {
		afterCycle()
	}
	return true
}

// EscStep represents one layer in an Esc-key dismissal cascade.
// Active returns true when this layer is engaged (e.g. filters are active).
// Clear performs the dismissal action (e.g. clear filters).
type EscStep struct {
	Active func() bool
	Clear  func()
}

// RunEscCascade walks the steps in order, dismissing the first active layer.
// Returns true if a step was dismissed, false if no step was active.
func RunEscCascade(steps []EscStep) bool {
	for _, step := range steps {
		if step.Active() {
			step.Clear()
			return true
		}
	}
	return false
}

// AnyEscStepActive returns true if any step in the cascade is currently active.
// Use this to implement ShouldCaptureEsc() alongside DefaultShouldCaptureEsc().
func AnyEscStepActive(steps []EscStep) bool {
	for _, step := range steps {
		if step.Active() {
			return true
		}
	}
	return false
}

// FilterClearEscStep builds an EscStep that clears filters and refreshes.
func FilterClearEscStep(hasFilters func() bool, clearAndRefresh func()) EscStep {
	return EscStep{Active: hasFilters, Clear: clearAndRefresh}
}

// MultiSelectEscStep builds an EscStep for exiting multi-select mode.
func MultiSelectEscStep(isActive func() bool, toggle func()) EscStep {
	return EscStep{Active: isActive, Clear: toggle}
}

// SecondaryEscCapture returns the ShouldCaptureEsc value for secondary views
// that follow the DefaultShouldCaptureEsc || AnyEscStepActive pattern.
func SecondaryEscCapture(ivs *InteractiveViewState, steps []EscStep) bool {
	return ivs.DefaultShouldCaptureEsc() || AnyEscStepActive(steps)
}

// helpCmd returns a tea.Cmd that emits a ShowHelpMsg.
func helpCmd(source ViewType, name string, items []HelpItem) tea.Cmd {
	return func() tea.Msg {
		return ShowHelpMsg{SourceView: source, ViewName: name, Items: items}
	}
}
