package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The pointer half of the harness: `click` and `wheel`. Each names a screen
// cell, by coordinates or by a literal the rendered screen draws, and sends the
// mouse messages a terminal would. Detection of a double-click is the app's own,
// on the driver's virtual clock, so `count: 2` and two clicks with no `wait`
// between them are the same press pair.

// validatePointerAction checks one click or wheel action whole.
func (a UITestAction) validatePointerAction() error {
	if err := a.validatePointerTarget(); err != nil {
		return err
	}
	if a.Type == UITestActionWheel {
		if a.Delta == 0 {
			return errors.New("wheel action needs a nonzero delta")
		}
		if a.Button != "" {
			return errors.New("wheel action takes no button")
		}
		return nil
	}
	if _, err := a.pointerButton(); err != nil {
		return err
	}
	if a.Count < 0 || a.Count > 3 {
		return fmt.Errorf("click count must be 1 to 3, got %d", a.Count)
	}
	return nil
}

// validatePointerTarget checks that a click or a wheel step names exactly one
// cell: both coordinates, or a literal.
func (a UITestAction) validatePointerTarget() error {
	byCoordinates := a.X != nil || a.Y != nil
	switch {
	case byCoordinates && a.On != "":
		return errors.New("pointer action names both x/y and on")
	case byCoordinates:
		if a.X == nil || a.Y == nil {
			return errors.New("pointer action needs both x and y")
		}
		if *a.X < 0 || *a.Y < 0 {
			return fmt.Errorf("pointer action needs a non-negative cell, got %d,%d", *a.X, *a.Y)
		}
		if a.Occurrence != 0 {
			return errors.New("occurrence only qualifies on")
		}
	case a.On == "":
		return errors.New("pointer action needs x/y or on")
	case a.Occurrence < 0:
		return fmt.Errorf("occurrence must be positive, got %d", a.Occurrence)
	}
	return nil
}

// pointerButton maps the action's button name onto the tea button.
func (a UITestAction) pointerButton() (tea.MouseButton, error) {
	switch a.Button {
	case "", "left":
		return tea.MouseLeft, nil
	case "right":
		return tea.MouseRight, nil
	default:
		return tea.MouseNone, fmt.Errorf("unknown click button %q (want left or right)", a.Button)
	}
}

// pointerCell resolves the action's cell against the screen as it is drawn now.
func (d *UITestDriver) pointerCell(a UITestAction) (int, int, error) {
	if a.On == "" {
		return *a.X, *a.Y, nil
	}
	occurrence := max(a.Occurrence, 1)
	seen := 0
	for y, line := range strings.Split(d.Render(), "\n") {
		offset := 0
		for {
			index := strings.Index(line[offset:], a.On)
			if index < 0 {
				break
			}
			seen++
			if seen == occurrence {
				return ansi.StringWidth(line[:offset+index]), y, nil
			}
			offset += index + len(a.On)
		}
	}
	if seen == 0 {
		return 0, 0, fmt.Errorf("screen does not contain %q to point at", a.On)
	}
	return 0, 0, fmt.Errorf("screen contains %q %d time(s), occurrence %d asked", a.On, seen, occurrence)
}

// runPointer sends the mouse messages one click or one wheel action stands for.
// A click is a press and its release, as a terminal reports it; the release is
// sent so the seam's drop of it is exercised rather than assumed.
func (d *UITestDriver) runPointer(a UITestAction) error {
	x, y, err := d.pointerCell(a)
	if err != nil {
		return err
	}
	if a.Type == UITestActionWheel {
		button, steps := tea.MouseWheelDown, a.Delta
		if steps < 0 {
			button, steps = tea.MouseWheelUp, -steps
		}
		for range steps {
			// A step, like a press, lands on the frame drawn after the last one.
			_ = d.Render()
			d.dispatch(tea.MouseWheelMsg{X: x, Y: y, Button: button})
			d.drain()
		}
		return nil
	}
	button, err := a.pointerButton()
	if err != nil {
		return err
	}
	for range max(a.Count, 1) {
		// A terminal user can only click what was drawn, and a surface answers
		// from the geometry its last render recorded, so every press lands on a
		// fresh frame the way Bubble Tea's render-after-update would leave one.
		_ = d.Render()
		d.dispatch(tea.MouseClickMsg{X: x, Y: y, Button: button})
		d.drain()
		d.dispatch(tea.MouseReleaseMsg{X: x, Y: y, Button: button})
		d.drain()
	}
	return nil
}

// pointerClock is the clock the app's double-click test reads under the
// harness: the driver's own, so only a `wait` separates two clicks.
func (d *UITestDriver) pointerClock() time.Time { return d.now }
