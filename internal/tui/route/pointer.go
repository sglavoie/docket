package route

import tea "charm.land/bubbletea/v2"

// The pointer is a second way to reach acts the keyboard already has. Bubble Tea
// delivers four mouse messages; the shell keeps clicks and vertical wheel steps,
// drops motion and release where they arrive, and folds what it keeps into the
// one PointerEvent below before routing it in key-ownership order. A surface
// therefore never sees a raw tea mouse message, and never has to decide whether
// a press was the second half of a double-click: the shell already did. The
// event carries no modifiers: Shift-drag belongs to the terminal's own
// selection, and no act reads a modified click.

// PointerButton is the button a click names. A wheel step carries none.
type PointerButton uint8

const (
	// PointerButtonNone is a wheel step, which names no button.
	PointerButtonNone PointerButton = iota
	// PointerButtonLeft is the primary click: the cursor, or with Clicks == 2,
	// Enter on that row.
	PointerButtonLeft
	// PointerButtonRight is the click followed by Space.
	PointerButtonRight
	// PointerButtonMiddle is reported for completeness; nothing answers it.
	PointerButtonMiddle
)

// PointerEvent is one click or one wheel step, in the receiving surface's own
// cell coordinates: the shell translates the screen cell before handing it on.
// Exactly one of Clicks and WheelDelta is nonzero.
type PointerEvent struct {
	X, Y   int
	Button PointerButton
	// Clicks is 1 for a click and 2 for the second press of a double-click on
	// the same cell inside the double-click interval. A double-click is always
	// preceded by its own single click, so a surface answering both acts twice,
	// in order: the cursor lands, then the row opens.
	Clicks int
	// WheelDelta is -1 for one step up and +1 for one step down. The shell never
	// coalesces steps, so a fast flick is several events.
	WheelDelta int
}

// IsWheel reports whether the event is a wheel step rather than a click.
func (e PointerEvent) IsWheel() bool { return e.WheelDelta != 0 }

// IsDoubleClick reports whether the event is the second press of a double-click.
func (e PointerEvent) IsDoubleClick() bool { return e.Clicks >= 2 }

// Offset returns the event moved into a surface drawn at (left, top) on the
// screen, which is the whole of the shell's coordinate translation.
func (e PointerEvent) Offset(left, top int) PointerEvent {
	e.X -= left
	e.Y -= top
	return e
}

// PointerView is implemented by a surface that answers the pointer. The shell
// calls it only when that surface is the layer a key press would reach, so an
// implementation never asks whether a modal stands over it. It answers from the
// geometry its last render recorded, never by rendering again or recomputing a
// window, and anything that acts goes through the OperationExecutor under the
// operation ID the equivalent key reaches.
//
// A surface that does not implement it swallows every pointer event, which is
// what text inputs and the task form are meant to do.
type PointerView interface {
	HandlePointer(event PointerEvent) tea.Cmd
}

// PointerTargetView is implemented by a surface whose pressed keys act on what
// its cursor names. PointerTarget is that entity's identity, empty when the
// cursor names nothing. The shell reads it when it stamps the keys a pointer
// answer produced and again when one is delivered: a refresh that removed or
// replaced the clicked entity, and seated the cursor elsewhere, changes it, and
// the key is dropped rather than pressed on whatever the cursor holds now.
type PointerTargetView interface {
	PointerTarget() string
}

// PressKey is the act a pointer event stands for once its surface has done the
// part no key can: seating the cursor on the row the click named. The key is
// delivered as the message a keyboard sends, so it takes the shell's ordinary
// key path to whichever layer owns keys, and every operation it reaches, every
// refusal, and every undo entry is that key's own. An empty key presses nothing.
//
// The command runs concurrently, so the shell stamps the key it yields with the
// click's interaction, the layer that answered it, and that layer's
// PointerTarget, and delivers it only if none has changed: a key a stale click
// stood for reaches no one.
//
// Only the keys a pointer re-expresses are spelled: Enter, Space, the two
// arrows the wheel steps by, and the two preview paging keys.
func PressKey(key string) tea.Cmd {
	var press tea.KeyPressMsg
	switch key {
	case "enter":
		press = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "space":
		press = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "up":
		press = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		press = tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+f":
		press = tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}
	case "ctrl+b":
		press = tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl}
	default:
		return nil
	}
	return func() tea.Msg { return press }
}
