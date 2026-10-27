package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// pointerProbeView is a presentation that answers the pointer and records what
// it was handed, so a test can see what the seam delivered and when it did not.
type pointerProbeView struct {
	events []route.PointerEvent
}

func (v *pointerProbeView) Init() tea.Cmd                       { return nil }
func (v *pointerProbeView) Update(tea.Msg) (tea.Model, tea.Cmd) { return v, nil }
func (v *pointerProbeView) View() tea.View                      { return tea.NewView("probe") }
func (v *pointerProbeView) SetSize(int, int)                    {}
func (v *pointerProbeView) HandlePointer(event route.PointerEvent) tea.Cmd {
	v.events = append(v.events, event)
	return nil
}

func pointerDriver(t *testing.T) *UITestDriver {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	return driver
}

func mountPointerProbe(driver *UITestDriver) *pointerProbeView {
	probe := &pointerProbeView{}
	driver.App().showView(presentation.ViewLastMessage, probe)
	return probe
}

func TestViewDeclaresCellMotionUnlessTheMouseIsOff(t *testing.T) {
	driver := pointerDriver(t)
	if got := driver.App().View().MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("a default session declares mouse mode %v, want cell motion", got)
	}
	WithDisabledMouse()(driver.App())
	if got := driver.App().View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("--no-mouse still declares mouse mode %v", got)
	}
}

func TestMouseToggleWritesMachineStateAndTheSessionValue(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)
	if driver.App().domain.runtime.Machine().MouseDisabled || !outlinerView.MouseEnabled() {
		t.Fatal("the fixture launched with the mouse already off")
	}
	if err := driver.App().coreToggleMouse(); err != nil {
		t.Fatalf("coreToggleMouse: %v", err)
	}
	if !driver.App().domain.runtime.Machine().MouseDisabled {
		t.Fatal("turning the mouse off did not reach machine state")
	}
	if outlinerView.MouseEnabled() || driver.App().View().MouseMode != tea.MouseModeNone {
		t.Fatal("the session still reports the mouse after the toggle")
	}
	if err := driver.App().coreToggleMouse(); err != nil {
		t.Fatalf("coreToggleMouse back: %v", err)
	}
	if driver.App().domain.runtime.Machine().MouseDisabled || !outlinerView.MouseEnabled() {
		t.Fatal("turning the mouse back on did not reach machine state")
	}
}

func TestMachineMouseSettingIsReadAtLaunch(t *testing.T) {
	driver := pointerDriver(t)
	if err := driver.App().coreToggleMouse(); err != nil {
		t.Fatalf("coreToggleMouse: %v", err)
	}
	if err := driver.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !driver.App().mouseDisabled() {
		t.Fatal("a relaunch forgot the machine-local mouse setting")
	}
}

func TestClickAndWheelRetireTheInteractionButMotionAndReleaseDoNot(t *testing.T) {
	m := &AppModel{}
	for _, msg := range []tea.Msg{tea.MouseClickMsg{}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}} {
		if !m.interactionIsRetiredBy(msg) || !isInteractionInput(msg) {
			t.Errorf("%T is the user's input and must count as one", msg)
		}
	}
	for _, msg := range []tea.Msg{tea.MouseMotionMsg{}, tea.MouseReleaseMsg{}} {
		if m.interactionIsRetiredBy(msg) || isInteractionInput(msg) {
			t.Errorf("%T counts for nothing and must not retire the interaction", msg)
		}
	}
}

func TestDoubleClickIsTheSecondPressOnOneCellInsideTheInterval(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	m := &AppModel{}
	m.pointer.clock = func() time.Time { return now }
	click := func(x, y int, button tea.MouseButton) int {
		event, ok := m.pointerEventFromMsg(tea.MouseClickMsg{X: x, Y: y, Button: button})
		if !ok {
			t.Fatalf("a %v click was dropped", button)
		}
		return event.Clicks
	}

	if got := click(3, 4, tea.MouseLeft); got != 1 {
		t.Fatalf("first click counted %d", got)
	}
	now = now.Add(pointerDoubleClickInterval)
	if got := click(3, 4, tea.MouseLeft); got != 2 {
		t.Fatalf("a second press at the interval's edge counted %d, want a double-click", got)
	}
	if got := click(3, 4, tea.MouseLeft); got != 1 {
		t.Fatalf("a third press counted %d; a double-click spends the memory", got)
	}
	if got := click(4, 4, tea.MouseLeft); got != 1 {
		t.Fatalf("a press on another cell counted %d", got)
	}
	if got := click(4, 4, tea.MouseRight); got != 1 {
		t.Fatalf("a press with another button counted %d", got)
	}
	now = now.Add(pointerDoubleClickInterval + time.Millisecond)
	if got := click(4, 4, tea.MouseRight); got != 1 {
		t.Fatalf("a press after the interval counted %d", got)
	}
	click(5, 5, tea.MouseLeft)
	if _, ok := m.pointerEventFromMsg(tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown}); !ok {
		t.Fatal("a vertical wheel step was dropped")
	}
	if got := click(5, 5, tea.MouseLeft); got != 1 {
		t.Fatalf("a wheel step between two presses still made a double-click (%d)", got)
	}
}

func TestPointerSeamDropsWhatCountsForNothing(t *testing.T) {
	m := &AppModel{}
	for _, msg := range []tea.Msg{
		tea.MouseMotionMsg{X: 1, Y: 1},
		tea.MouseReleaseMsg{X: 1, Y: 1, Button: tea.MouseLeft},
		tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelLeft},
		tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseBackward},
	} {
		if _, ok := m.pointerEventFromMsg(msg); ok {
			t.Errorf("%#v became a pointer event", msg)
		}
	}
	event, ok := m.pointerEventFromMsg(tea.MouseWheelMsg{X: 1, Y: 2, Button: tea.MouseWheelUp})
	if !ok || event.WheelDelta != -1 || event.Clicks != 0 || !event.IsWheel() {
		t.Fatalf("a wheel step up folded to %+v", event)
	}
}

func TestPointerReachesAnAnsweringPresentation(t *testing.T) {
	driver := pointerDriver(t)
	probe := mountPointerProbe(driver)
	driver.dispatch(tea.MouseClickMsg{X: 7, Y: 2, Button: tea.MouseRight, Mod: tea.ModShift})
	driver.dispatch(tea.MouseReleaseMsg{X: 7, Y: 2, Button: tea.MouseRight})
	driver.dispatch(tea.MouseMotionMsg{X: 8, Y: 2})
	driver.dispatch(tea.MouseWheelMsg{X: 7, Y: 2, Button: tea.MouseWheelDown})
	want := []route.PointerEvent{
		{X: 7, Y: 2, Button: route.PointerButtonRight, Clicks: 1},
		{X: 7, Y: 2, WheelDelta: 1},
	}
	if len(probe.events) != len(want) {
		t.Fatalf("the presentation was handed %+v, want %+v", probe.events, want)
	}
	for i := range want {
		if probe.events[i] != want[i] {
			t.Errorf("event %d = %+v, want %+v", i, probe.events[i], want[i])
		}
	}
}

func TestPointerIsSwallowedWhenTheMouseIsOff(t *testing.T) {
	driver := pointerDriver(t)
	probe := mountPointerProbe(driver)
	WithDisabledMouse()(driver.App())
	driver.dispatch(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	if len(probe.events) != 0 {
		t.Fatalf("a session with the mouse off handed on %+v", probe.events)
	}
}

func TestClickAbandonsAPendingChordSilently(t *testing.T) {
	driver := pointerDriver(t)
	if err := driver.SendKey("w"); err != nil {
		t.Fatal(err)
	}
	driver.drain()
	if len(driver.App().operations.Chord) == 0 {
		t.Fatal("w did not leave a pending chord")
	}
	before := driver.App().feedback.message
	driver.dispatch(tea.MouseClickMsg{X: 1, Y: 5, Button: tea.MouseLeft})
	driver.drain()
	if len(driver.App().operations.Chord) != 0 {
		t.Fatal("a click left the chord pending")
	}
	if got := driver.App().feedback.message; got != before {
		t.Fatalf("abandoning the chord by click spoke: %q", got)
	}
}

func TestClickDropsTheLaunchFocusIntent(t *testing.T) {
	driver := pointerDriver(t)
	driver.App().lifecycle.launchFocus = true
	driver.dispatch(tea.MouseClickMsg{X: 1, Y: 5, Button: tea.MouseLeft})
	if driver.App().lifecycle.launchFocusArmed() {
		t.Fatal("a click left the launch focus intent armed")
	}
}

func TestPointerNeverFallsThroughAShellModal(t *testing.T) {
	driver := pointerDriver(t)
	probe := mountPointerProbe(driver)
	driver.App().checkInSetup = &checkInSetupPass{}
	driver.dispatch(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	driver.dispatch(tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelUp})
	if len(probe.events) != 0 {
		t.Fatalf("a mounted modal let %+v reach the presentation beneath it", probe.events)
	}
}
