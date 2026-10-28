package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// These tests hold a pointer event's command instead of draining it, let other
// input land first, and only then deliver what the command produced. Bubble Tea
// runs commands concurrently, so that order is one a real terminal can reach;
// the harness's per-event drain never does.

const (
	pointerHubFirstRow  = 2 // Rich Project
	pointerHubSecondRow = 3 // Unfiled
)

func pointerHubDriver(t *testing.T) *UITestDriver {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	pointerSendKey(t, driver, "P")
	if screen := driver.Render(); !strings.Contains(screen, "› Rich Project") || !strings.Contains(screen, "Unfiled") {
		t.Fatalf("Select Project did not open on Rich Project:\n%s", screen)
	}
	return driver
}

func pointerSendKey(t *testing.T, driver *UITestDriver, key string) {
	t.Helper()
	if err := driver.SendKey(key); err != nil {
		t.Fatal(err)
	}
	driver.drain()
}

// retainPointer dispatches one mouse message on a fresh frame and resolves the
// command it returned into the messages it would deliver, without delivering
// any of them.
func retainPointer(t *testing.T, driver *UITestDriver, msg tea.Msg) []tea.Msg {
	t.Helper()
	_ = driver.Render()
	driver.dispatch(msg)
	cmd := driver.pendingCmd
	driver.pendingCmd = nil
	messages := resolvePointerCmd(cmd)
	for _, message := range messages {
		if _, ok := message.(pointerKeyMsg); ok {
			return messages
		}
	}
	t.Fatalf("the pointer event produced no key to retain: %#v", messages)
	return nil
}

func resolvePointerCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil || isAnimationCmd(cmd) {
		return nil
	}
	switch msg := cmd().(type) {
	case nil:
		return nil
	case originTaggedCommandMsg:
		return resolvePointerCmd(runOriginTaggedCommand(msg))
	case tea.BatchMsg:
		var out []tea.Msg
		for _, child := range msg {
			out = append(out, resolvePointerCmd(child)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

// deliverRetained hands the retained messages to the app now, draining whatever
// they produce.
func deliverRetained(driver *UITestDriver, messages []tea.Msg) {
	for _, message := range messages {
		driver.processCmds(driver.handleMsg(message), driver.drainOpts())
	}
}

func pointerClick(driver *UITestDriver, x, y int, button tea.MouseButton) {
	_ = driver.Render()
	driver.dispatch(tea.MouseClickMsg{X: x, Y: y, Button: button})
	driver.drain()
	driver.dispatch(tea.MouseReleaseMsg{X: x, Y: y, Button: button})
	driver.drain()
}

func openRichProjectSheet(t *testing.T, driver *UITestDriver) {
	t.Helper()
	pointerClick(driver, 4, pointerHubFirstRow, tea.MouseRight)
	if screen := driver.Render(); !strings.Contains(screen, "Actions: Rich Project") {
		t.Fatalf("a right-click on Rich Project did not open its action sheet:\n%s", screen)
	}
}

func TestPointerActionKeepsClickedTarget(t *testing.T) {
	// Control: delivered with nothing in between, the right-click is Space on
	// the row it named.
	control := pointerHubDriver(t)
	deliverRetained(control, retainPointer(t, control, tea.MouseClickMsg{X: 4, Y: pointerHubFirstRow, Button: tea.MouseRight}))
	if screen := control.Render(); !strings.Contains(screen, "Actions: Rich Project") {
		t.Fatalf("an uninterrupted right-click did not open the clicked project's sheet:\n%s", screen)
	}

	// Right-click Unfiled, then seat Rich Project before that Space arrives.
	driver := pointerHubDriver(t)
	retained := retainPointer(t, driver, tea.MouseClickMsg{X: 4, Y: pointerHubSecondRow, Button: tea.MouseRight})
	pointerClick(driver, 4, pointerHubFirstRow, tea.MouseLeft)
	if screen := driver.Render(); !strings.Contains(screen, "› Rich Project") {
		t.Fatalf("the left-click did not seat Rich Project:\n%s", screen)
	}
	deliverRetained(driver, retained)
	screen := driver.Render()
	if strings.Contains(screen, "Actions:") {
		t.Fatalf("a stale right-click opened a menu for the row seated after it:\n%s", screen)
	}
	if !strings.Contains(screen, "› Rich Project") {
		t.Fatalf("the stale right-click moved the cursor:\n%s", screen)
	}
}

func TestPointerActionCannotReachReplacementOwner(t *testing.T) {
	driver := pointerHubDriver(t)
	openRichProjectSheet(t, driver)
	x, y, err := driver.pointerCell(UITestAction{On: "Archive Project"})
	if err != nil {
		t.Fatal(err)
	}
	// The first half of the double-click seats the action and runs nothing; the
	// second half is Enter, held back while Esc puts the sheet away.
	pointerClick(driver, x, y, tea.MouseLeft)
	retained := retainPointer(t, driver, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	pointerSendKey(t, driver, "esc")
	screen := driver.Render()
	if strings.Contains(screen, "Actions:") || !strings.Contains(screen, "› Rich Project") {
		t.Fatalf("Esc did not return to the Hub list on Rich Project:\n%s", screen)
	}

	deliverRetained(driver, retained)
	screen = driver.Render()
	if strings.Contains(screen, "y confirm") {
		t.Fatalf("a stale double-click raised a confirmation in the replacement owner:\n%s", screen)
	}
	if !strings.Contains(screen, "Select Project") || !strings.Contains(screen, "› Rich Project") {
		t.Fatalf("a stale double-click activated the Hub list beneath the closed sheet:\n%s", screen)
	}
	if got := len(driver.App().domain.runtime.AllProjects()); got == 0 {
		t.Fatal("the workspace lost its projects")
	}
	for _, project := range driver.App().domain.runtime.AllProjects() {
		if project.Archived {
			t.Fatalf("a stale double-click archived %q", project.Name)
		}
	}
}

func TestPointerWheelCannotReachReplacementOwner(t *testing.T) {
	driver := pointerHubDriver(t)
	openRichProjectSheet(t, driver)
	x, y, err := driver.pointerCell(UITestAction{On: "Archive Project"})
	if err != nil {
		t.Fatal(err)
	}
	retained := retainPointer(t, driver, tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	pointerSendKey(t, driver, "esc")
	if screen := driver.Render(); strings.Contains(screen, "Actions:") || !strings.Contains(screen, "› Rich Project") {
		t.Fatalf("Esc did not return to the Hub list on Rich Project:\n%s", screen)
	}

	deliverRetained(driver, retained)
	screen := driver.Render()
	if !strings.Contains(screen, "› Rich Project") || strings.Contains(screen, "› Unfiled") {
		t.Fatalf("a stale wheel step navigated the Hub list that replaced the sheet:\n%s", screen)
	}
}
